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

func TestDetectRoleProduct(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"feature request"},
		{"user story"},
		{"design review"},
		{"spec update"},
		{"priority change"},
		{"product roadmap"},
		{"ui improvement"},
		{"ux feedback"},
	}
	for _, tt := range tests {
		if got := detectRole(tt.input); got != "pm" {
			t.Errorf("detectRole(%q) = %q, want pm", tt.input, got)
		}
	}
}

func TestDetectRoleTechnical(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"code review"},
		{"bug report"},
		{"architecture decision"},
		{"deploy pipeline"},
		{"test coverage"},
		{"build failure"},
		{"refactor module"},
		{"impl details"},
		{"fix the bug"},
	}
	for _, tt := range tests {
		if got := detectRole(tt.input); got != "staff" {
			t.Errorf("detectRole(%q) = %q, want staff", tt.input, got)
		}
	}
}

func TestDetectRoleUnknownDefaultsToStaff(t *testing.T) {
	tests := []string{
		"random text",
		"hello world",
		"something else entirely",
	}
	for _, input := range tests {
		if got := detectRole(input); got != "staff" {
			t.Errorf("detectRole(%q) = %q, want staff", input, got)
		}
	}
}

func TestDetectRoleEmptyInput(t *testing.T) {
	if got := detectRole(""); got != "staff" {
		t.Errorf("detectRole(\"\") = %q, want staff", got)
	}
}

func TestRoleGetEmptyFile(t *testing.T) {
	dir := t.TempDir()
	roleFile := filepath.Join(dir, "role")
	if err := os.WriteFile(roleFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}
	if err := app.Run("role", "get"); err != nil {
		t.Fatalf("role get: %v", err)
	}

	if got := buf.String(); got != "none\n" {
		t.Errorf("expected %q, got %q", "none\n", got)
	}
}

func TestRoleSetWriteError(t *testing.T) {
	dir := t.TempDir()
	// Make the MillDir read-only so os.WriteFile fails.
	if err := os.Chmod(dir, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}

	if err := app.roleSet("staff"); err == nil {
		t.Error("expected write error, got nil")
	}
}

func TestRoleSetAlreadySet(t *testing.T) {
	dir := t.TempDir()
	roleFile := filepath.Join(dir, "role")
	if err := os.WriteFile(roleFile, []byte("staff"), 0o644); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}

	if err := app.roleSet("staff"); err != nil {
		t.Fatalf("roleSet staff: %v", err)
	}

	data, err := os.ReadFile(roleFile)
	if err != nil {
		t.Fatalf("read role file: %v", err)
	}
	if got := string(data); got != "staff" {
		t.Errorf("file changed: expected %q, got %q", "staff", got)
	}
}
