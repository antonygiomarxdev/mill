package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/antonygiomarxdev/mill/internal/slots"
)

func TestSlotsUsage(t *testing.T) {
	usage := SlotsUsage()
	if !strings.Contains(usage, "slots") {
		t.Errorf("expected 'slots' in usage, got: %s", usage)
	}
}

func TestPrintSlotsUsage(t *testing.T) {
	buf := new(bytes.Buffer)
	PrintSlotsUsage(buf)
	if buf.Len() == 0 {
		t.Error("expected output from PrintSlotsUsage")
	}
	if !strings.Contains(buf.String(), "slots") {
		t.Errorf("expected 'slots' in output, got: %s", buf.String())
	}
}

func TestRunSlotsCommand(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{Out: buf, Err: buf}
	app.slots = slots.NewManager(4)

	err := RunSlotsCommand(app, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSlotsCommandNilManager(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{Out: buf, Err: buf}

	err := RunSlotsCommand(app, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "No active slot manager") {
		t.Errorf("expected 'No active slot manager', got: %s", output)
	}
}
