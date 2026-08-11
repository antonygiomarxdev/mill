package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/antonygiomarxdev/mill/internal/slots"
)

func TestSlotsIdle(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{Out: buf, Err: buf}
	app.slots = slots.NewManager(4)

	err := app.Run("slots")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "0/4") {
		t.Fatalf("expected '0/4' in output, got: %s", output)
	}
	if !strings.Contains(output, "idle") {
		t.Fatalf("expected 'idle' in output, got: %s", output)
	}
}

func TestSlotsWithOccupied(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{Out: buf, Err: buf}
	app.slots = slots.NewManager(4)

	// Acquire 2 slots.
	app.slots.Acquire(context.Background(), 55, "Sr.Dev FE", false)
	app.slots.Acquire(context.Background(), 60, "Tech Lead", false)

	err := app.Run("slots")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "2/4 occupied") {
		t.Fatalf("expected '2/4 occupied' in output, got: %s", output)
	}
	if !strings.Contains(output, "Sr.Dev FE") {
		t.Fatalf("expected 'Sr.Dev FE' in output, got: %s", output)
	}
	if !strings.Contains(output, "issue #55") {
		t.Fatalf("expected 'issue #55' in output, got: %s", output)
	}
	if !strings.Contains(output, "Tech Lead") {
		t.Fatalf("expected 'Tech Lead' in output, got: %s", output)
	}
	if !strings.Contains(output, "issue #60") {
		t.Fatalf("expected 'issue #60' in output, got: %s", output)
	}

	// Release to clean up.
	app.slots.Release()
	app.slots.Release()
}

func TestSlotsWithQueue(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{Out: buf, Err: buf}
	app.slots = slots.NewManager(1)

	// Fill the slot.
	app.slots.Acquire(context.Background(), 55, "Sr.Dev FE", false)

	// Enqueue 2 waiters with short-lived contexts (will be cancelled).
	ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel1()
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	go func() { app.slots.Acquire(ctx1, 61, "Sr.Dev BE", false) }()
	go func() { app.slots.Acquire(ctx2, 62, "QA/Docs", false) }()

	// Wait for queue to populate.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status := app.slots.Status()
		if len(status.Queue) >= 2 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	status := app.slots.Status()
	if len(status.Queue) < 2 {
		t.Fatalf("expected at least 2 waiting, got %d", len(status.Queue))
	}

	err := app.Run("slots")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "QUEUE:") {
		t.Fatalf("expected QUEUE section in output, got: %s", output)
	}
	if !strings.Contains(output, "waiting") {
		t.Fatalf("expected 'waiting' in output, got: %s", output)
	}

	// Release to clean up.
	app.slots.Release()
	// Cancel waiters.
	cancel1()
	cancel2()
	time.Sleep(100 * time.Millisecond)
}

func TestSlotsNilManager(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{Out: buf, Err: buf}
	// slots is nil by default.

	err := app.Run("slots")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No active slot manager") {
		t.Fatalf("expected 'No active slot manager' in output, got: %s", output)
	}
}

func TestSlotsLimit(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{Out: buf, Err: buf}
	app.slots = slots.NewManager(8)

	err := app.runSlots([]string{"limit"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "8") {
		t.Errorf("expected '8' in output, got: %s", output)
	}
}

func TestSlotsUnknownSubcommand(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{Out: buf, Err: buf}
	app.slots = slots.NewManager(4)

	err := app.runSlots([]string{"unknown"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
	if !strings.Contains(err.Error(), "unknown slots subcommand") {
		t.Errorf("expected 'unknown slots subcommand', got: %v", err)
	}
}
