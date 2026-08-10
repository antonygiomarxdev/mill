package cli

import (
	"bytes"
	"os"
	"os/exec"
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

func TestRoleGetNoFileDefaultsToStaff(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}
	if err := app.Run("role", "get"); err != nil {
		t.Fatalf("role get: %v", err)
	}

	if got := buf.String(); got != "staff\n" {
		t.Errorf("expected %q, got %q", "staff\n", got)
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
	roles := []string{
		"sr-dev", "sr-dev-be", "sr-dev-fe", "sr-dev-data",
		"tech-lead", "architect",
		"ux-designer", "ui-designer",
		"reviewer", "qa-docs",
	}

	for _, role := range roles {
		t.Run(role, func(t *testing.T) {
			dir := t.TempDir()
			buf := new(bytes.Buffer)
			app := &App{MillDir: dir, Out: buf, Err: buf}

			err := app.Run("role", "set", role)
			if err == nil {
				t.Fatalf("expected error for delegation-only role %q", role)
			}

			want := role + " is a delegation-only role, not an active role. Valid: staff, pm"
			if got := err.Error(); got != want {
				t.Errorf("error mismatch\n  got:  %v\n  want: %v", got, want)
			}
		})
	}
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

	if got := buf.String(); got != "staff\n" {
		t.Errorf("expected %q, got %q", "staff\n", got)
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

func TestRoleEnforceHookTestMode(t *testing.T) {
	// Find project root to locate checks/role-enforce
	root, err := projectRoot()
	if err != nil {
		t.Skipf("skipping: cannot find project root: %v", err)
	}
	hook := filepath.Join(root, "checks", "role-enforce")

	cases := []struct {
		role     string
		file     string
		wantExit int // 0 = allowed, 1 = blocked
	}{
		{"pm", "foo.go", 1},              // AC 6
		{"pm", "foo.md", 0},              // AC 7
		{"sr-dev-be", "main.go", 0},
		{"sr-dev-be", "layout.pen", 1},
		{"tech-lead", "main.go", 0},
		{"tech-lead", "config.yml", 1},
		{"qa-docs", "README.md", 0},
		{"qa-docs", "main.go", 1},
		{"ux-designer", "wireframe.pen", 0},
		{"ux-designer", "main.go", 1},
		{"architect", "adr.yml", 0},
		{"architect", "main.go", 1},
	}

	for _, tc := range cases {
		t.Run(tc.role+"/"+tc.file, func(t *testing.T) {
			cmd := exec.Command("bash", hook, "--test", tc.role, tc.file)
			cmd.Dir = root
			err := cmd.Run()
			if tc.wantExit == 0 {
				if err != nil {
					t.Errorf("expected exit 0 for %s committing %s, got error: %v", tc.role, tc.file, err)
				}
			} else {
				if err == nil {
					t.Errorf("expected non-zero exit for %s committing %s, got 0", tc.role, tc.file)
				}
			}
		})
	}
}

func TestRoleEnforceHookStaffBypass(t *testing.T) {
	root, err := projectRoot()
	if err != nil {
		t.Skipf("skipping: cannot find project root: %v", err)
	}
	hook := filepath.Join(root, "checks", "role-enforce")

	cmd := exec.Command("bash", hook, "--test", "staff", "anything.go")
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		t.Errorf("staff should bypass enforcement for .go files, got error: %v", err)
	}

	cmd = exec.Command("bash", hook, "--test", "staff", "notes.md")
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		t.Errorf("staff should bypass enforcement for .md files, got error: %v", err)
	}
}

func TestRoleEnforceHookMissingRole(t *testing.T) {
	root, err := projectRoot()
	if err != nil {
		t.Skipf("skipping: cannot find project root: %v", err)
	}
	hook := filepath.Join(root, "checks", "role-enforce")

	dir := t.TempDir()
	millDir := filepath.Join(dir, ".mill")
	if err := os.MkdirAll(millDir, 0755); err != nil {
		t.Fatal(err)
	}

	// No .mill/role file → pre-commit mode exits 0
	cmd := exec.Command("bash", hook)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("hook should exit 0 when .mill/role is missing, got error: %v\noutput: %s", err, string(out))
	}
}
