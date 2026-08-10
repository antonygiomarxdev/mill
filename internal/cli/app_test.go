package cli

import (
	"bytes"
	"testing"
)

func TestRunUnknownCommandReturnsError(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{MillDir: t.TempDir(), Out: buf, Err: buf}
	err := app.Run("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

func TestRunNoArgsShowsUsage(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{MillDir: t.TempDir(), Out: buf, Err: buf}
	err := app.Run()
	if err != nil {
		t.Fatalf("expected no error for help, got: %v", err)
	}
}

func TestRunHelpShowsUsage(t *testing.T) {
	buf := new(bytes.Buffer)
	app := &App{MillDir: t.TempDir(), Out: buf, Err: buf}
	err := app.Run("--help")
	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	if !bytes.Contains(buf.Bytes(), []byte("mill")) {
		t.Error("expected usage output to contain 'mill'")
	}
	if !bytes.Contains(buf.Bytes(), []byte("delegate")) {
		t.Error("expected usage output to contain 'delegate' command")
	}
	if !bytes.Contains(buf.Bytes(), []byte("status")) {
		t.Error("expected usage output to contain 'status' command")
	}
}

func TestAppPathMethods(t *testing.T) {
	app := &App{MillDir: "/tmp/milltest"}

	if app.statePath() != "/tmp/milltest/state.json" {
		t.Errorf("statePath = %q, want %q", app.statePath(), "/tmp/milltest/state.json")
	}
	if app.configPath() != "/tmp/milltest/config.json" {
		t.Errorf("configPath = %q, want %q", app.configPath(), "/tmp/milltest/config.json")
	}
	if app.ledgerPath(390) != "/tmp/milltest/ledger/390.jsonl" {
		t.Errorf("ledgerPath(390) = %q, want %q", app.ledgerPath(390), "/tmp/milltest/ledger/390.jsonl")
	}
	if app.worktreePath(390) != "/tmp/milltest/worktrees/issue-390" {
		t.Errorf("worktreePath(390) = %q, want %q", app.worktreePath(390), "/tmp/milltest/worktrees/issue-390")
	}
}
