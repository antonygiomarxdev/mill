package slots

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)


// safeBuffer is a goroutine-safe bytes.Buffer.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}
func TestAcquireReleaseBasic(t *testing.T) {
	m := NewManager(4)
	defer m.Shutdown()

	pos, err := m.Acquire(context.Background(), 1, "sr-dev-be", false)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if pos != 0 {
		t.Errorf("expected position 0 (immediate), got %d", pos)
	}

	status := m.Status()
	if len(status.Occupied) != 1 {
		t.Fatalf("expected 1 occupied, got %d", len(status.Occupied))
	}
	if status.Occupied[0].Issue != 1 {
		t.Errorf("expected issue 1, got %d", status.Occupied[0].Issue)
	}
	if status.Occupied[0].Role != "sr-dev-be" {
		t.Errorf("expected role sr-dev-be, got %s", status.Occupied[0].Role)
	}

	m.Release()

	status = m.Status()
	if len(status.Occupied) != 0 {
		t.Errorf("expected 0 occupied after release, got %d", len(status.Occupied))
	}
}

func TestAcquireBlocksWhenFull(t *testing.T) {
	m := NewManager(2)
	defer m.Shutdown()

	// Acquire both slots (in test goroutine).
	for i := 0; i < 2; i++ {
		_, err := m.Acquire(context.Background(), i+1, "sr-dev-be", false)
		if err != nil {
			t.Fatalf("Acquire %d failed: %v", i, err)
		}
	}

	// Third Acquire in background goroutine — should block.
	acquired := make(chan error, 1)
	go func() {
		_, err := m.Acquire(context.Background(), 3, "sr-dev-be", false)
		acquired <- err
	}()

	// Verify it blocks (doesn't return immediately).
	select {
	case <-acquired:
		t.Fatal("third Acquire should have blocked, but returned immediately")
	case <-time.After(100 * time.Millisecond):
	}

	// Release one slot (held by test goroutine) — third should acquire.
	m.Release()

	select {
	case err := <-acquired:
		if err != nil {
			t.Fatalf("third Acquire returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("third Acquire did not receive slot after release")
	}

	status := m.Status()
	if len(status.Occupied) != 2 {
		t.Errorf("expected 2 occupied after reassignment, got %d", len(status.Occupied))
	}
}

func TestFIFOOrdering(t *testing.T) {
	m := NewManager(1)
	defer m.Shutdown()

	// Acquire the only slot.
	_, err := m.Acquire(context.Background(), 1, "slot-1", false)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// Track which issue was acquired first.
	var acquiredFirst int
	var mu sync.Mutex
	firstDone := make(chan struct{})
	secondDone := make(chan struct{})

	// Enqueue two waiters.
	go func() {
		_, err := m.Acquire(context.Background(), 2, "slot-2", false)
		if err != nil {
			t.Errorf("slot-2 error: %v", err)
		}
		mu.Lock()
		if acquiredFirst == 0 {
			acquiredFirst = 2
		}
		mu.Unlock()
		m.Release()
		close(firstDone)
	}()

	time.Sleep(50 * time.Millisecond)

	go func() {
		_, err := m.Acquire(context.Background(), 3, "slot-3", false)
		if err != nil {
			t.Errorf("slot-3 error: %v", err)
		}
		mu.Lock()
		if acquiredFirst == 0 {
			acquiredFirst = 3
		}
		mu.Unlock()
		m.Release()
		close(secondDone)
	}()

	// Verify both are queued.
	time.Sleep(100 * time.Millisecond)
	status := m.Status()
	if len(status.Queue) < 2 {
		t.Fatalf("expected at least 2 queued, got %d", len(status.Queue))
	}

	// Release — slot-2 should acquire first (FIFO).
	m.Release()

	// Wait for slot-2 to acquire and release, then slot-3 to acquire.
	<-firstDone

	if acquiredFirst != 2 {
		t.Errorf("FIFO violation: expected issue 2 to acquire first, got %d", acquiredFirst)
	}

	<-secondDone
}

func TestPriorityPreemption(t *testing.T) {
	m := NewManager(1)
	defer m.Shutdown()

	// Acquire the only slot.
	_, err := m.Acquire(context.Background(), 1, "normal-1", false)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	normalAcquired := make(chan struct{})
	priorityAcquired := make(chan struct{})
	releaseNormal := make(chan struct{})
	releasePriority := make(chan struct{})
	normalDone := make(chan struct{})
	priorityDone := make(chan struct{})

	// Enqueue normal waiter first, then priority.
	go func() {
		defer close(normalDone)
		_, err := m.Acquire(context.Background(), 2, "normal-2", false)
		if err != nil {
			t.Errorf("normal-2 error: %v", err)
		}
		close(normalAcquired)
		<-releaseNormal
		m.Release()
	}()

	time.Sleep(50 * time.Millisecond)

	go func() {
		defer close(priorityDone)
		_, err := m.Acquire(context.Background(), 3, "priority-3", true)
		if err != nil {
			t.Errorf("priority-3 error: %v", err)
		}
		close(priorityAcquired)
		<-releasePriority
		m.Release()
	}()

	// Let them both enqueue.
	time.Sleep(100 * time.Millisecond)

	// Release the holder — priority should get the next slot.
	m.Release()

	// Wait for priority to acquire.
	select {
	case <-priorityAcquired:
	case <-time.After(time.Second):
		t.Fatal("priority did not acquire slot")
	}

	// Verify the occupied slot is the priority one.
	status := m.Status()
	if len(status.Occupied) != 1 {
		t.Fatalf("expected 1 occupied, got %d", len(status.Occupied))
	}
	if status.Occupied[0].Issue != 3 {
		t.Errorf("priority preemption failed: expected issue 3, got issue %d",
			status.Occupied[0].Issue)
	}

	// Normal should NOT have acquired yet.
	select {
	case <-normalAcquired:
		t.Fatal("normal waiter acquired before priority released")
	default:
	}

	// Clean up.
	close(releasePriority)
	close(releaseNormal)
	<-priorityDone
	<-normalDone
}

func TestPriorityDoesNotEvictRunning(t *testing.T) {
	m := NewManager(1)
	defer m.Shutdown()

	// Acquire the only slot.
	_, err := m.Acquire(context.Background(), 1, "normal-1", false)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// Enqueue a priority waiter.
	priorityDone := make(chan struct{})
	go func() {
		_, err := m.Acquire(context.Background(), 2, "priority-2", true)
		if err != nil {
			t.Errorf("priority error: %v", err)
		}
		m.Release()
		close(priorityDone)
	}()

	// Verify the running slot is NOT evicted.
	time.Sleep(200 * time.Millisecond)

	status := m.Status()
	if len(status.Occupied) != 1 {
		t.Fatalf("expected 1 occupied (not evicted), got %d", len(status.Occupied))
	}
	if status.Occupied[0].Issue != 1 {
		t.Errorf("expected issue 1 still running, got %d", status.Occupied[0].Issue)
	}

	// Release — priority should now get the slot.
	m.Release()
	<-priorityDone
}

func TestContextCancellation(t *testing.T) {
	m := NewManager(1)
	defer m.Shutdown()

	// Fill the slot.
	_, err := m.Acquire(context.Background(), 1, "holder", false)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// Try Acquire with an already-cancelled context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = m.Acquire(ctx, 2, "cancelled", false)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("expected 'cancelled' in error, got: %v", err)
	}
}

func TestContextDeadline(t *testing.T) {
	m := NewManager(1)
	defer m.Shutdown()

	// Fill the slot.
	_, err := m.Acquire(context.Background(), 1, "holder", false)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// Try Acquire with a short deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = m.Acquire(ctx, 2, "timeout", false)
	if err == nil {
		t.Fatal("expected deadline exceeded error, got nil")
	}
	if !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("expected deadline/cancelled error, got: %v", err)
	}
}

func TestHardLimitReclaim(t *testing.T) {
	var errBuf safeBuffer

	m := NewManager(1)
	m.HardLimit = 100 * time.Millisecond
	m.Err = &errBuf
	defer m.Shutdown()

	// Acquire the slot.
	_, err := m.Acquire(context.Background(), 1, "long-runner", false)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// Wait for hard limit to trigger.
	time.Sleep(300 * time.Millisecond)

	status := m.Status()
	if len(status.Occupied) != 0 {
		t.Errorf("expected 0 occupied after hard limit, got %d", len(status.Occupied))
	}

	if !strings.Contains(errBuf.String(), "forced release") {
		t.Errorf("expected 'forced release' in error output, got: %s", errBuf.String())
	}
}

func TestWarningTimeout(t *testing.T) {
	var warnBuf safeBuffer

	m := NewManager(1)
	m.WarningTimeout = 50 * time.Millisecond
	m.Warn = &warnBuf
	defer m.Shutdown()

	// Fill the slot.
	_, err := m.Acquire(context.Background(), 1, "holder", false)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// Enqueue a waiter.
	waiterDone := make(chan struct{})
	go func() {
		_, err := m.Acquire(context.Background(), 2, "sr-dev-be", false)
		if err != nil {
			t.Errorf("waiter error: %v", err)
		}
		m.Release()
		close(waiterDone)
	}()

	// Wait for warning to fire.
	time.Sleep(200 * time.Millisecond)

	output := warnBuf.String()
	if len(output) == 0 {
		t.Fatal("expected warning output, got none")
	}
	if !strings.Contains(output, "Delegation to sr-dev-be waiting") {
		t.Errorf("expected warning about sr-dev-be, got: %s", output)
	}
	if !strings.Contains(output, "position 1") {
		t.Errorf("expected 'position 1' in warning, got: %s", output)
	}

	// Release — waiter gets slot, releases, and we drain it.
	m.Release()
	<-waiterDone
}

func TestQueueDepthWarning(t *testing.T) {
	var errBuf safeBuffer

	m := NewManager(1)
	m.Err = &errBuf

	// Fill the slot.
	_, err := m.Acquire(context.Background(), 1, "holder", false)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// Enqueue 51 waiters. Each uses its own context with a timeout,
	// and each goroutine releases its slot before returning.
	var wg sync.WaitGroup
	wg.Add(51)
	for i := 0; i < 51; i++ {
		go func(id int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			// This will block until the holder releases or ctx times out.
			_, err := m.Acquire(ctx, 100+id, "waiter", false)
			if err == nil {
				// Got a slot — release it. (We acquire and release in the
				// same goroutine for correctness.)
				m.Release()
			}
		}(i)
	}

	// Give the dispatch loop time to process all enqueues.
	time.Sleep(200 * time.Millisecond)

	output := errBuf.String()
	if !strings.Contains(output, "Slot queue exceeded 50 items") {
		t.Errorf("expected queue depth warning, got: %s", output)
	}

	// Release the holder so waiters can drain.
	m.Release()

	// Shut down to unblock any remaining waiters.
	m.Shutdown()
	wg.Wait()
}

func TestStatusSnapshot(t *testing.T) {
	m := NewManager(2)
	defer m.Shutdown()

	// Acquire 2 slots (all capacity).
	for i := 0; i < 2; i++ {
		_, err := m.Acquire(context.Background(), i+1, "worker", false)
		if err != nil {
			t.Fatalf("Acquire %d failed: %v", i, err)
		}
	}

	// Enqueue 2 waiters.
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(issue int) {
			defer wg.Done()
			_, err := m.Acquire(context.Background(), issue, "waiter", false)
			if err != nil {
				t.Errorf("waiter %d: %v", issue, err)
				return
			}
			m.Release()
		}(10 + i)
	}

	time.Sleep(100 * time.Millisecond)

	status := m.Status()

	if len(status.Occupied) != 2 {
		t.Errorf("expected 2 occupied, got %d", len(status.Occupied))
	}
	if len(status.Queue) != 2 {
		t.Errorf("expected 2 queued, got %d", len(status.Queue))
	}
	if status.MaxSlots != 2 {
		t.Errorf("expected MaxSlots 2, got %d", status.MaxSlots)
	}

	for _, info := range status.Occupied {
		if info.Running == 0 {
			t.Errorf("expected non-zero Running for occupied slot %d", info.SlotID)
		}
	}
	for _, info := range status.Queue {
		if info.Waiting == 0 {
			t.Errorf("expected non-zero Waiting for queued item at position %d", info.Position)
		}
	}

	// Release the two occupied slots so waiters can drain.
	m.Release()
	m.Release()
	wg.Wait()
}

func TestReleaseUnknownSlot(t *testing.T) {
	m := NewManager(4)
	defer m.Shutdown()

	// Release when no slot is held — should not panic.
	m.Release()

	// Acquire and release normally to sanity-check.
	_, err := m.Acquire(context.Background(), 1, "worker", false)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	m.Release()

	// Double release — no panic.
	m.Release()
}

func TestConcurrentAcquireRelease(t *testing.T) {
	m := NewManager(3)
	defer m.Shutdown()

	const numGoroutines = 20
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	ctx := context.Background()

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			_, err := m.Acquire(ctx, id, "worker", false)
			if err != nil {
				t.Errorf("goroutine %d: Acquire failed: %v", id, err)
				return
			}
			// Simulate work.
			time.Sleep(20 * time.Millisecond)
			m.Release()
		}(i)
	}

	wg.Wait()

	// After all goroutines complete, no slots should be held and no waiters.
	status := m.Status()
	if len(status.Occupied) != 0 {
		t.Errorf("expected 0 occupied, got %d", len(status.Occupied))
	}
	if len(status.Queue) != 0 {
		t.Errorf("expected 0 queued, got %d", len(status.Queue))
	}
}

func TestNilManager(t *testing.T) {
	var m *Manager

	// All methods are safe on nil receiver.
	pos, err := m.Acquire(context.Background(), 1, "worker", false)
	if err != nil {
		t.Errorf("nil Acquire should succeed, got: %v", err)
	}
	if pos != 0 {
		t.Errorf("nil Acquire position should be 0, got %d", pos)
	}

	m.Release()  // no-op, no panic
	m.Shutdown() // no-op, no panic

	status := m.Status()
	if len(status.Occupied) != 0 {
		t.Errorf("nil Status should have 0 occupied")
	}
}

func TestShutdown(t *testing.T) {
	m := NewManager(4)

	// Acquire a slot.
	_, err := m.Acquire(context.Background(), 1, "worker", false)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	m.Shutdown()

	// Release after shutdown should work (it just modifies maps).
	m.Release()
}

func TestAcquirePosition(t *testing.T) {
	m := NewManager(1)
	defer m.Shutdown()

	// Fill the slot.
	_, err := m.Acquire(context.Background(), 1, "holder", false)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// Enqueue first waiter and wait until it's in the queue.
	firstInQueue := make(chan struct{})
	go func() {
		for {
			status := m.Status()
			if len(status.Queue) > 0 {
				close(firstInQueue)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	var pos1, pos2 int
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		p, err := m.Acquire(context.Background(), 2, "first-waiter", false)
		if err != nil {
			t.Errorf("first waiter: %v", err)
			return
		}
		pos1 = p
		m.Release()
	}()

	// Wait for first waiter to be enqueued.
	select {
	case <-firstInQueue:
	case <-time.After(time.Second):
		t.Fatal("first waiter never enqueued")
	}

	// Verify first waiter is in queue at position 1.
	status := m.Status()
	if len(status.Queue) != 1 || status.Queue[0].Position != 1 {
		t.Fatalf("expected 1st waiter at position 1, got queue len=%d pos=%d",
			len(status.Queue), status.Queue[0].Position)
	}

	go func() {
		defer wg.Done()
		p, err := m.Acquire(context.Background(), 3, "second-waiter", false)
		if err != nil {
			t.Errorf("second waiter: %v", err)
			return
		}
		pos2 = p
		m.Release()
	}()

	time.Sleep(100 * time.Millisecond)

	// Release the holder — first waiter gets slot, releases, second gets slot.
	m.Release()

	wg.Wait()

	if pos1 != 1 {
		t.Errorf("first waiter position: expected 1, got %d", pos1)
	}
	if pos2 != 2 {
		t.Errorf("second waiter position: expected 2, got %d", pos2)
	}
}

func TestShutdownReleasesWaiters(t *testing.T) {
	m := NewManager(1)
	defer m.Shutdown()

	// Fill the slot.
	_, err := m.Acquire(context.Background(), 1, "holder", false)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// Enqueue a waiter that should be released on shutdown.
	waiterErr := make(chan error, 1)
	go func() {
		_, err := m.Acquire(context.Background(), 2, "waiter", false)
		waiterErr <- err
	}()

	// Wait for waiter to enqueue.
	time.Sleep(100 * time.Millisecond)
	status := m.Status()
	if len(status.Queue) != 1 {
		t.Fatalf("expected 1 queued, got %d", len(status.Queue))
	}

	// Shutdown should unblock the waiter with ErrShutdown.
	m.Shutdown()

	select {
	case err := <-waiterErr:
		if err == nil {
			t.Fatal("expected error on shutdown, got nil")
		}
		if !strings.Contains(err.Error(), "shut down") {
			t.Errorf("expected 'shut down' in error, got: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("waiter did not unblock on shutdown")
	}
}
