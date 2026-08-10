// Package slots provides a slot-based concurrency manager for agent dispatch.
// A Manager gates concurrent work behind a fixed-size pool of slots.
// When all slots are occupied, callers wait in a FIFO queue.
// Priority requests jump to the front of the queue without evicting running tasks.
package slots

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Defaults for slot ownership and warning thresholds.
const (
	DefaultHardLimit      = 5 * time.Minute
	DefaultWarningTimeout = 120 * time.Second
	DefaultMaxSlots       = 4
)
// ErrShutdown is returned by Acquire when the slot manager has been shut down.
var ErrShutdown = errors.New("slot manager shut down")

// Manager controls concurrent slot acquisition for agent dispatch.
//
// A nil *Manager is safe for all methods: Acquire returns immediately,
// Release and Shutdown are no-ops, and Status returns zero values.
type Manager struct {
	queue chan slotRequest // buffered channel carrying incoming requests
	stop  chan struct{}    // closed by Shutdown to stop the dispatch loop

	Warn           io.Writer    // warning output; defaults to os.Stderr
	Err            io.Writer    // error output; defaults to os.Stderr
	HardLimit      time.Duration // max slot ownership before forced reclaim
	WarningTimeout time.Duration // queue wait threshold for warning emission

	mu             sync.Mutex
	active         map[int]*activeSlot  // slotID → slot
	goroutineSlots map[uint64]int       // goroutine ID → slotID
	waiters        []*waiter            // ordered queue of pending requests
	nextSlotID     int
	maxSlots       int

	shutdownOnce sync.Once
}

// slotRequest carries an Acquire call through the dispatch loop.
type slotRequest struct {
	issue    int
	role     string
	result   chan slotResult // buffer 1; written exactly once by the dispatch loop
	priority bool
	ctx      context.Context
	gid      uint64 // goroutine ID of the caller (captured in Acquire)
}

// slotResult is the outcome written back to the caller's result channel.
type slotResult struct {
	err        error
	position   int       // queue position at enqueue time (0 = immediate)
	acquiredAt time.Time
}

// activeSlot represents a currently occupied slot.
type activeSlot struct {
	id         int
	issue      int
	role       string
	acquiredAt time.Time
}

// waiter is a pending request waiting for a slot to free.
type waiter struct {
	request    slotRequest
	enqueuedAt time.Time
	position   int // original queue position at enqueue time (immutable)
}

// SlotStatus is a snapshot of the manager's current state.
type SlotStatus struct {
	Occupied       []SlotInfo    `json:"occupied"`
	Queue          []QueueInfo   `json:"queue"`
	MaxSlots       int           `json:"max_slots"`
	HardLimit      time.Duration `json:"hard_limit"`
	WarningTimeout time.Duration `json:"warning_timeout"`
}

// SlotInfo describes an occupied slot.
type SlotInfo struct {
	SlotID     int           `json:"slot_id"`
	Issue      int           `json:"issue"`
	Role       string        `json:"role"`
	AcquiredAt time.Time     `json:"acquired_at"`
	Running    time.Duration `json:"running"`
}

// QueueInfo describes a waiting request.
type QueueInfo struct {
	Position   int           `json:"position"`
	Issue      int           `json:"issue"`
	Role       string        `json:"role"`
	Priority   bool          `json:"priority"`
	EnqueuedAt time.Time     `json:"enqueued_at"`
	Waiting    time.Duration `json:"waiting"`
}

// NewManager creates a slot manager with maxSlots concurrent slots and starts
// the background dispatch loop.
func NewManager(maxSlots int) *Manager {
	if maxSlots <= 0 {
		maxSlots = DefaultMaxSlots
	}

	m := &Manager{
		maxSlots:        maxSlots,
		queue:           make(chan slotRequest, 256),
		stop:            make(chan struct{}),
		active:          make(map[int]*activeSlot),
		goroutineSlots:  make(map[uint64]int),
		Warn:            os.Stderr,
		Err:             os.Stderr,
		HardLimit:       DefaultHardLimit,
		WarningTimeout:  DefaultWarningTimeout,
	}

	go m.dispatchLoop()

	return m
}

// Acquire requests a slot. It blocks until a slot is available or ctx is
// cancelled. Returns the queue position at enqueue time (0 = immediate
// acquisition) and any error.
//
// Priority requests jump to the front of the queue ahead of non-priority
// waiters. Priority never evicts a running task.
func (m *Manager) Acquire(ctx context.Context, issue int, role string, priority bool) (int, error) {
	if m == nil {
		return 0, nil
	}

	req := slotRequest{
		issue:    issue,
		role:     role,
		result:   make(chan slotResult, 1),
		priority: priority,
		ctx:      ctx,
		gid:      goroutineID(),
	}

	select {
	case m.queue <- req:
	case <-ctx.Done():
		return 0, fmt.Errorf("slot acquisition cancelled: %w", ctx.Err())
	}

	select {
	case res := <-req.result:
		return res.position, res.err
	case <-ctx.Done():
		return 0, fmt.Errorf("slot acquisition cancelled: %w", ctx.Err())
	}
}

// Release frees the calling goroutine's slot and signals the dispatch loop
// to assign the next waiter. It is a no-op if the caller holds no slot.
func (m *Manager) Release() {
	if m == nil {
		return
	}

	gid := goroutineID()

	m.mu.Lock()
	slotID, ok := m.goroutineSlots[gid]
	if !ok {
		m.mu.Unlock()
		return
	}
	delete(m.goroutineSlots, gid)
	delete(m.active, slotID)

	m.assignNextLocked()
	m.mu.Unlock()
}

// Status returns a snapshot of occupied slots and queued waiters.
func (m *Manager) Status() SlotStatus {
	if m == nil {
		return SlotStatus{}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return m.statusLocked()
}

// Shutdown stops the dispatch loop goroutine. After Shutdown returns, the
// manager is no longer usable.
func (m *Manager) Shutdown() {
	if m == nil {
		return
	}
	m.shutdownOnce.Do(func() {
		close(m.stop)
	})
}

// ---------------------------------------------------------------------------
// dispatch loop
// ---------------------------------------------------------------------------

func (m *Manager) dispatchLoop() {
	warnTicker := time.NewTicker(100 * time.Millisecond)
	defer warnTicker.Stop()

	hardLimitTicker := time.NewTicker(100 * time.Millisecond)
	defer hardLimitTicker.Stop()

	for {
		select {
		case req := <-m.queue:
			m.handleRequest(req)
		case <-warnTicker.C:
			m.checkWarnings()
		case <-hardLimitTicker.C:
			m.checkHardLimits()
		case <-m.stop:
			m.drainWaiters()
			return
		}
	}
}

// handleRequest either assigns a free slot immediately or enqueues the
// request into the waiters slice.
func (m *Manager) handleRequest(req slotRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.active) < m.maxSlots {
		m.assignSlotLocked(req, 0)
		return
	}

	// All slots occupied — enqueue.
	position := len(m.waiters) + 1

	w := &waiter{
		request:    req,
		enqueuedAt: time.Now(),
		position:   position,
	}

	if req.priority {
		m.insertPriorityLocked(w)
	} else {
		m.waiters = append(m.waiters, w)
	}

	// Queue depth safety valve.
	if len(m.waiters) > 50 {
		m.emitErr("Slot queue exceeded 50 items — possible deadlock\n")
	}
}

// insertPriorityLocked inserts a priority waiter at the front of the
// non-priority section of the queue.
func (m *Manager) insertPriorityLocked(w *waiter) {
	idx := 0
	for i, existing := range m.waiters {
		if !existing.request.priority {
			idx = i
			break
		}
		idx = i + 1
	}
	m.waiters = append(m.waiters[:idx], append([]*waiter{w}, m.waiters[idx:]...)...)

}

// assignSlotLocked creates a new active slot for req and sends the result.
// position is the queue position at enqueue time (0 = immediate).
func (m *Manager) assignSlotLocked(req slotRequest, position int) {
	m.nextSlotID++
	slot := &activeSlot{
		id:         m.nextSlotID,
		issue:      req.issue,
		role:       req.role,
		acquiredAt: time.Now(),
	}
	m.active[slot.id] = slot
	m.goroutineSlots[req.gid] = slot.id

	req.result <- slotResult{
		acquiredAt: slot.acquiredAt,
		position:   position,
	}
}

// assignNextLocked pops the first non-cancelled waiter from the queue and
// assigns a slot. Must be called with m.mu held.
func (m *Manager) assignNextLocked() {
	if len(m.active) >= m.maxSlots || len(m.waiters) == 0 {
		return
	}

	// Remove cancelled waiters from the front and find first valid one.
	for len(m.waiters) > 0 {
		w := m.waiters[0]
		if w.request.ctx.Err() != nil {
			// Cancelled — notify the waiter goroutine and remove.
			w.request.result <- slotResult{err: w.request.ctx.Err()}
			m.waiters = m.waiters[1:]
			continue
		}
		// Valid waiter — assign the slot.
		savedPosition := w.position
		m.waiters = m.waiters[1:]
		m.assignSlotLocked(w.request, savedPosition)
		return
	}
}


// ---------------------------------------------------------------------------
// periodic checks
// ---------------------------------------------------------------------------

// checkWarnings emits a warning for waiters that have been queued longer
// than WarningTimeout.
func (m *Manager) checkWarnings() {
	if m.WarningTimeout <= 0 || m.Warn == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for i, w := range m.waiters {
		waiting := now.Sub(w.enqueuedAt)
		if waiting >= m.WarningTimeout {
			fmt.Fprintf(m.Warn, "Delegation to %s waiting %s (position %d)\n",
				w.request.role, waiting.Truncate(time.Second), i+1)
		}
	}
}

// checkHardLimits forcefully reclaims slots held beyond HardLimit.
func (m *Manager) checkHardLimits() {
	if m.HardLimit <= 0 {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var reclaimed []int

	for id, slot := range m.active {
		if now.Sub(slot.acquiredAt) >= m.HardLimit {
			reclaimed = append(reclaimed, id)
		}
	}

	for _, id := range reclaimed {
		slot := m.active[id]
		delete(m.active, id)

		// Remove goroutine mapping.
		for gid, sid := range m.goroutineSlots {
			if sid == id {
				delete(m.goroutineSlots, gid)
				break
			}
		}

		m.emitErr("Slot %d held by %s for %s — forced release\n",
			id, slot.role, now.Sub(slot.acquiredAt).Truncate(time.Second))
	}

	// Assign waiters for each reclaimed slot.
	for range reclaimed {
		m.assignNextLocked()
	}
}

// drainWaiters rejects all pending waiters with ErrShutdown.
func (m *Manager) drainWaiters() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, w := range m.waiters {
		w.request.result <- slotResult{err: ErrShutdown}
	}
	m.waiters = nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// statusLocked builds a SlotStatus snapshot. Must be called with m.mu held.
func (m *Manager) statusLocked() SlotStatus {
	now := time.Now()
	s := SlotStatus{
		MaxSlots:       m.maxSlots,
		HardLimit:      m.HardLimit,
		WarningTimeout: m.WarningTimeout,
	}

	for _, slot := range m.active {
		s.Occupied = append(s.Occupied, SlotInfo{
			SlotID:     slot.id,
			Issue:      slot.issue,
			Role:       slot.role,
			AcquiredAt: slot.acquiredAt,
			Running:    now.Sub(slot.acquiredAt),
		})
	}

	for i, w := range m.waiters {
		s.Queue = append(s.Queue, QueueInfo{
			Position:   i + 1,
			Issue:      w.request.issue,
			Role:       w.request.role,
			Priority:   w.request.priority,
			EnqueuedAt: w.enqueuedAt,
			Waiting:    now.Sub(w.enqueuedAt),
		})
	}

	return s
}

// emitErr writes to m.Err if configured.
func (m *Manager) emitErr(format string, args ...interface{}) {
	if m.Err != nil {
		fmt.Fprintf(m.Err, format, args...)
	}
}

// goroutineID returns the ID of the calling goroutine.
func goroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(string(buf[:n]))
	if len(idField) < 2 {
		return 0
	}
	id, err := strconv.ParseUint(idField[1], 10, 64)
	if err != nil {
		return 0
	}
	return id
}
