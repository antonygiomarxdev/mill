package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/antonygiomarxdev/mill/internal/adapter"
	"github.com/antonygiomarxdev/mill/internal/config"
	"github.com/antonygiomarxdev/mill/internal/slots"
)

func TestDelegateAcquiresSlot(t *testing.T) {
	dir := t.TempDir()
	fa := &fakeAdapter{result: adapter.SessionResult{ExitCode: 0}}
	buf := new(bytes.Buffer)
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
	fa := &fakeAdapter{result: adapter.SessionResult{ExitCode: 0}}
	buf := new(bytes.Buffer)
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
	fa := &fakeAdapter{result: adapter.SessionResult{ExitCode: 1}}
	buf := new(bytes.Buffer)
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
	fa := &fakeAdapter{result: adapter.SessionResult{ExitCode: 0}}
	var errBuf bytes.Buffer
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
	fa := &fakeAdapter{result: adapter.SessionResult{ExitCode: 0}}
	buf := new(bytes.Buffer)

	os.WriteFile(filepath.Join(dir, "role"), []byte("staff"), 0o644)

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
	fa := &fakeAdapter{result: adapter.SessionResult{ExitCode: 0}}
	buf := new(bytes.Buffer)

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
