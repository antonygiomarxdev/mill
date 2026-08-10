package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRoleGetPrintsCurrentRole(t *testing.T) {
	dir := t.TempDir()
	roleFile := filepath.Join(dir, "role")
	if err := os.WriteFile(roleFile, []byte("staff"), 0644); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}
	if err := app.Run("role", "get"); err != nil {
		t.Fatalf("role get: %v", err)
	}

	if got := buf.String(); got != "staff\n" {
		t.Errorf("expected %q, got %q", "staff\n", got)
	}
}

func TestRoleGetNoFileShowsNone(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}
	if err := app.Run("role", "get"); err != nil {
		t.Fatalf("role get: %v", err)
	}

	if got := buf.String(); got != "none\n" {
		t.Errorf("expected %q, got %q", "none\n", got)
	}
}

func TestRoleSetValidRole(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}

	if err := app.Run("role", "set", "staff"); err != nil {
		t.Fatalf("role set staff: %v", err)
	}

	// Verify file was written
	data, err := os.ReadFile(filepath.Join(dir, "role"))
	if err != nil {
		t.Fatalf("read role file: %v", err)
	}
	if got := string(data); got != "staff" {
		t.Errorf("expected %q, got %q", "staff", got)
	}

	// Verify output notification
	if got := buf.String(); got != "mill: switched to staff\n" {
		t.Errorf("expected %q, got %q", "mill: switched to staff\n", got)
	}
}

func TestRoleSetPM(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}

	if err := app.Run("role", "set", "pm"); err != nil {
		t.Fatalf("role set pm: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "role"))
	if err != nil {
		t.Fatalf("read role file: %v", err)
	}
	if got := string(data); got != "pm" {
		t.Errorf("expected %q, got %q", "pm", got)
	}
}

func TestRoleSetDelegationOnlyRoleRejected(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}

	err := app.Run("role", "set", "sr-dev")
	if err == nil {
		t.Fatal("expected error for delegation-only role")
	}

	if got := err.Error(); got != "sr-dev is delegation-only, not an active role. Valid: staff, pm" {
		t.Errorf("expected delegation-only error, got: %v", got)
	}

	_ = buf
}

func TestRoleSetInvalidRoleRejected(t *testing.T) {
	dir := t.TempDir()
	app := &App{MillDir: dir, Out: new(bytes.Buffer), Err: new(bytes.Buffer)}

	err := app.Run("role", "set", "invalid")
	if err == nil {
		t.Fatal("expected error for invalid role")
	}

	if got := err.Error(); got != "unknown role: invalid. Valid: staff, pm" {
		t.Errorf("expected unknown role error, got: %v", got)
	}
}

func TestRoleSetNoArgsShowsUsage(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}

	err := app.Run("role", "set")
	if err == nil {
		t.Fatal("expected error for missing role")
	}
}

func TestRoleUnknownSubcommand(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}

	err := app.Run("role", "delete")
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}
