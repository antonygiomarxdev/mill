package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/antonygiomarxdev/mill/internal/adapter"
	"github.com/antonygiomarxdev/mill/internal/config"
	"github.com/antonygiomarxdev/mill/internal/domain"
	"github.com/antonygiomarxdev/mill/internal/slots"
	"github.com/antonygiomarxdev/mill/internal/state"
)

func TestDelegateAcquiresSlot(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)
	fa := &fakeAdapter{result: adapter.SessionResult{ExitCode: 0}}
	buf := new(bytes.Buffer)
	origFn := modelAvailableFn
	modelAvailableFn = func(string) bool { return true }
	defer func() { modelAvailableFn = origFn }()

	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader}
	app.slots = slots.NewManager(2)

	err := app.Run("delegate", "--wait", "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	status := app.slots.Status()
	if len(status.Occupied) != 0 {
		t.Fatalf("expected 0 occupied slots after --wait completion, got %d", len(status.Occupied))
	}
}

func TestDelegateReleasesSlotOnCompletion(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)
	fa := &fakeAdapter{result: adapter.SessionResult{ExitCode: 0}}
	buf := new(bytes.Buffer)

	origFn := modelAvailableFn
	modelAvailableFn = func(string) bool { return true }
	defer func() { modelAvailableFn = origFn }()

	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader}
	app.slots = slots.NewManager(2)

	err := app.Run("delegate", "--wait", "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	status := app.slots.Status()
	if len(status.Occupied) != 0 {
		t.Fatalf("expected 0 occupied slots after completion, got %d", len(status.Occupied))
	}
}

func TestDelegateReleasesSlotOnError(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)
	fa := &fakeAdapter{result: adapter.SessionResult{ExitCode: 1}}
	buf := new(bytes.Buffer)

	origFn := modelAvailableFn
	modelAvailableFn = func(string) bool { return true }
	defer func() { modelAvailableFn = origFn }()

	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader}
	app.slots = slots.NewManager(2)

	err := app.Run("delegate", "--wait", "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	status := app.slots.Status()
	if len(status.Occupied) != 0 {
		t.Fatalf("expected 0 occupied slots after error, got %d", len(status.Occupied))
	}
}

func TestDelegateBlocksUntilSlotFree(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)
	fa := &fakeAdapter{result: adapter.SessionResult{ExitCode: 0}}
	var errBuf bytes.Buffer

	origFn := modelAvailableFn
	modelAvailableFn = func(string) bool { return true }
	defer func() { modelAvailableFn = origFn }()

	app := &App{Adapter: fa, MillDir: dir, Out: &errBuf, Err: &errBuf, IssueReader: defaultIssueReader}
	app.slots = slots.NewManager(1)

	_, err := app.slots.Acquire(context.Background(), 99, "staff", false)
	if err != nil {
		t.Fatalf("failed to acquire external slot: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- app.Run("delegate", "--wait", "42")
	}()

	select {
	case <-done:
		t.Fatal("delegate returned immediately despite all slots being full")
	case <-time.After(300 * time.Millisecond):
	}

	app.slots.Release()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		output := errBuf.String()
		if !strings.Contains(output, "Delegation queued") {
			t.Fatalf("expected 'Delegation queued' in output, got: %s", output)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("delegate blocked indefinitely after slot release")
	}
}

func TestDelegatePriorityStaff(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)
	fa := &fakeAdapter{result: adapter.SessionResult{ExitCode: 0}}
	buf := new(bytes.Buffer)

	os.WriteFile(filepath.Join(dir, "role"), []byte("staff"), 0o644)

	origFn := modelAvailableFn
	modelAvailableFn = func(string) bool { return true }
	defer func() { modelAvailableFn = origFn }()

	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader}
	app.slots = slots.NewManager(2)

	err := app.Run("delegate", "--priority", "--wait", "42")
	if err != nil {
		t.Fatalf("staff should be able to use --priority: %v", err)
	}
}

func TestDelegatePriorityNonStaffRejected(t *testing.T) {
	dir := t.TempDir()
	fa := &fakeAdapter{result: adapter.SessionResult{ExitCode: 0}}
	buf := new(bytes.Buffer)

	os.WriteFile(filepath.Join(dir, "role"), []byte("sr-dev-be"), 0o644)

	origFn := modelAvailableFn
	modelAvailableFn = func(string) bool { return true }
	defer func() { modelAvailableFn = origFn }()

	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader}
	app.slots = slots.NewManager(2)

	err := app.Run("delegate", "--priority", "42")
	if err == nil {
		t.Fatal("expected error for non-staff --priority")
	}
	if !strings.Contains(err.Error(), "restricted to staff role") {
		t.Fatalf("expected 'restricted to staff role' in error, got: %v", err)
	}
}

func TestDelegatePriorityPreemptsQueue(t *testing.T) {
	dir := t.TempDir()
	setupTestGitRepo(t, dir)
	fa := &fakeAdapter{result: adapter.SessionResult{ExitCode: 0}}
	buf := new(bytes.Buffer)

	origFn := modelAvailableFn
	modelAvailableFn = func(string) bool { return true }
	defer func() { modelAvailableFn = origFn }()

	app := &App{Adapter: fa, MillDir: dir, Out: buf, Err: buf, IssueReader: defaultIssueReader}
	app.slots = slots.NewManager(1)

	_, err := app.slots.Acquire(context.Background(), 99, "staff", false)
	if err != nil {
		t.Fatalf("failed to acquire external slot: %v", err)
	}

	normalDone := make(chan error, 1)
	go func() {
		normalDone <- app.Run("delegate", "--wait", "10")
	}()

	time.Sleep(200 * time.Millisecond)

	priorityDone := make(chan error, 1)
	go func() {
		priorityDone <- app.Run("delegate", "--wait", "--priority", "20")
	}()

	time.Sleep(200 * time.Millisecond)

	status := app.slots.Status()
	if len(status.Queue) < 2 {
		t.Fatalf("expected at least 2 in queue, got %d", len(status.Queue))
	}
	if !status.Queue[0].Priority {
		t.Errorf("expected priority entry at queue position 1, got: %+v", status.Queue)
	}

	app.slots.Release()

	select {
	case err := <-priorityDone:
		if err != nil {
			t.Fatalf("priority delegate error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("priority delegate blocked indefinitely")
	}

	select {
	case err := <-normalDone:
		if err != nil {
			t.Fatalf("normal delegate error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("normal delegate blocked indefinitely")
	}
}

func TestSlotIntegrationConfigConcurrencyDefault(t *testing.T) {
	cfg := config.Default()
	if cfg.Concurrency.MaxSlots != 4 {
		t.Fatalf("expected MaxSlots=4, got %d", cfg.Concurrency.MaxSlots)
	}
}

func TestSlotIntegrationConfigConcurrencyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := config.Config{
		Provider:    "test-provider",
		Model:       "test-model",
		MaxRounds:   2,
		Concurrency: config.Concurrency{MaxSlots: 8},
	}

	if err := cfg.Save(path); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loaded.Concurrency.MaxSlots != 8 {
		t.Fatalf("expected MaxSlots=8 after round-trip, got %d", loaded.Concurrency.MaxSlots)
	}
}

func TestReleaseSlotNilManager(t *testing.T) {
	// Should not panic with nil manager.
	ReleaseSlot(nil)
}

func TestReleaseSlotNonNil(t *testing.T) {
	mgr := slots.NewManager(4)
	ctx := context.Background()
	_, err := mgr.Acquire(ctx, 1, "staff", false)
	if err != nil {
		t.Fatalf("failed to acquire slot: %v", err)
	}
	ReleaseSlot(mgr)
	// After release, slot should be free.
	status := mgr.Status()
	if len(status.Occupied) != 0 {
		t.Errorf("expected 0 occupied slots after release, got %d", len(status.Occupied))
	}
}

func TestEnsureSlotManagerExisting(t *testing.T) {
	mgr := slots.NewManager(8)
	result := EnsureSlotManager(mgr, config.Config{Concurrency: config.Concurrency{MaxSlots: 4}})
	if result != mgr {
		t.Error("EnsureSlotManager should return existing manager")
	}
}

func TestEnsureSlotManagerNew(t *testing.T) {
	result := EnsureSlotManager(nil, config.Config{Concurrency: config.Concurrency{MaxSlots: 2}})
	if result == nil {
		t.Fatal("EnsureSlotManager should create new manager")
	}
	status := result.Status()
	if status.MaxSlots != 2 {
		t.Errorf("expected MaxSlots=2, got %d", status.MaxSlots)
	}
}

func TestMaxSlotsFromConfigZero(t *testing.T) {
	if s := MaxSlotsFromConfig(config.Config{Concurrency: config.Concurrency{MaxSlots: 0}}); s != 4 {
		t.Errorf("MaxSlotsFromConfig(0) = %d, want 4", s)
	}
}

func TestMaxSlotsFromConfigNegative(t *testing.T) {
	if s := MaxSlotsFromConfig(config.Config{Concurrency: config.Concurrency{MaxSlots: -1}}); s != 4 {
		t.Errorf("MaxSlotsFromConfig(-1) = %d, want 4", s)
	}
}

func TestMaxSlotsFromConfigCustom(t *testing.T) {
	if s := MaxSlotsFromConfig(config.Config{Concurrency: config.Concurrency{MaxSlots: 10}}); s != 10 {
		t.Errorf("MaxSlotsFromConfig(10) = %d, want 10", s)
	}
}

func TestAcquireSlotWithCancelledContext(t *testing.T) {
	mgr := slots.NewManager(1)
	// Fill the only slot
	ctx := context.Background()
	_, err := mgr.Acquire(ctx, 1, "staff", false)
	if err != nil {
		t.Fatalf("failed to acquire slot: %v", err)
	}

	// Cancel the context for the next acquire
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	buf := new(bytes.Buffer)
	_, err = AcquireSlot(cancelCtx, mgr, buf, 2, "staff", false, 1)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestAcquireSlotExhaustionReturnsEnvFailure(t *testing.T) {
	origTimeout := slotAcquireTimeout
	slotAcquireTimeout = 50 * time.Millisecond
	defer func() { slotAcquireTimeout = origTimeout }()

	mgr := slots.NewManager(1)
	defer mgr.Shutdown()

	// Occupy the only slot.
	if _, err := mgr.Acquire(context.Background(), 1, "staff", false); err != nil {
		t.Fatalf("failed to acquire slot: %v", err)
	}

	buf := new(bytes.Buffer)
	start := time.Now()
	_, err := AcquireSlot(context.Background(), mgr, buf, 2, "staff", false, 1)
	elapsed := time.Since(start)

	if !errors.Is(err, ErrSlotsExhausted) {
		t.Fatalf("expected ErrSlotsExhausted, got %v", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("AcquireSlot blocked %s instead of timing out (deadlock)", elapsed)
	}
	if !strings.Contains(buf.String(), "slots agotados") {
		t.Fatalf("expected 'slots agotados' notification, got %q", buf.String())
	}
}

func TestAcquireSlotShutdownReturnsExhausted(t *testing.T) {
	mgr := slots.NewManager(1)
	defer mgr.Shutdown()

	// Occupy the only slot.
	if _, err := mgr.Acquire(context.Background(), 1, "staff", false); err != nil {
		t.Fatalf("failed to acquire slot: %v", err)
	}

	buf := new(bytes.Buffer)
	done := make(chan error, 1)
	go func() {
		_, err := AcquireSlot(context.Background(), mgr, buf, 2, "staff", false, 1)
		done <- err
	}()

	// Wait until the waiter is enqueued so Shutdown can drain it.
	deadline := time.Now().Add(2 * time.Second)
	for len(mgr.Status().Queue) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("waiter never enqueued")
		}
		time.Sleep(10 * time.Millisecond)
	}

	mgr.Shutdown()

	select {
	case err := <-done:
		if !errors.Is(err, ErrSlotsExhausted) {
			t.Fatalf("expected ErrSlotsExhausted, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("AcquireSlot blocked after shutdown (deadlock)")
	}
}

func TestDelegateSlotExhaustionAbortsWithEnvFailure(t *testing.T) {
	origTimeout := slotAcquireTimeout
	slotAcquireTimeout = 50 * time.Millisecond
	defer func() { slotAcquireTimeout = origTimeout }()

	dir := t.TempDir()
	setupTestGitRepo(t, dir)
	fa := &fakeAdapter{result: adapter.SessionResult{ExitCode: 0}}
	var errBuf bytes.Buffer

	origFn := modelAvailableFn
	modelAvailableFn = func(string) bool { return true }
	defer func() { modelAvailableFn = origFn }()

	app := &App{Adapter: fa, MillDir: dir, Out: &errBuf, Err: &errBuf, IssueReader: defaultIssueReader}
	app.slots = slots.NewManager(1)
	defer app.slots.Shutdown()

	// Occupy the only slot.
	if _, err := app.slots.Acquire(context.Background(), 99, "staff", false); err != nil {
		t.Fatalf("failed to acquire external slot: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- app.Run("delegate", "--wait", "42")
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("delegate returned error on slot exhaustion: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("delegate blocked indefinitely on slot exhaustion (deadlock)")
	}

	if !strings.Contains(errBuf.String(), "slots agotados") {
		t.Fatalf("expected 'slots agotados' notification, got: %s", errBuf.String())
	}

	s, _ := state.Load(app.statePath())
	task, ok := s.Task("task-42")
	if !ok {
		t.Fatal("expected task-42 to exist")
	}
	if task.Phase != domain.TaskPhaseAborted {
		t.Errorf("expected phase %q, got %q", domain.TaskPhaseAborted, task.Phase)
	}
	if task.FailureClass != domain.ENVIRONMENT_FAILURE {
		t.Errorf("expected failure class %q, got %q", domain.ENVIRONMENT_FAILURE, task.FailureClass)
	}
	if task.AbortReason != "slots agotados" {
		t.Errorf("expected abort reason %q, got %q", "slots agotados", task.AbortReason)
	}
}
