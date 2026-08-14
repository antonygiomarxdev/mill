package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

			want := "role '" + role + "' is delegation-only. Use mill delegate to dispatch work to this role."
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

	if got := err.Error(); got != "role 'invalid' is delegation-only. Use mill delegate to dispatch work to this role." {
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

func TestRoleGetInvalidFileContent(t *testing.T) {
	dir := t.TempDir()
	roleFile := filepath.Join(dir, "role")
	if err := os.WriteFile(roleFile, []byte("architect"), 0o644); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}
	err := app.Run("role", "get")
	if err == nil {
		t.Fatal("expected error for invalid role in .mill/role")
	}

	want := `invalid role "architect" in .mill/role: only staff and pm are valid active roles; correct the file or run 'mill role set staff'`
	if got := err.Error(); got != want {
		t.Errorf("error mismatch\n  got:  %v\n  want: %v", got, want)
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
		{"pm", "foo.go", 1}, // AC 6
		{"pm", "foo.md", 0}, // AC 7
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
		// Issue #147: cover every one of the 11 defined roles so a new role
		// in .mill/roles/ can never regress into the unknown-role branch.
		{"staff", "notes.md", 0},
		{"staff", "internal.go", 1},
		{"reviewer", "notes.md", 0},
		{"reviewer", "notes.go", 1},
		{"sr-dev-fe", "a.go", 0},
		{"sr-dev-fe", "a.pen", 1},
		{"sr-dev-data", "a.go", 0},
		{"sr-dev-data", "a.pen", 1},
		{"ui-designer", "w.pen", 0},
		{"ui-designer", "w.go", 1},
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

// runRoleEnforceTest runs `bash <hook> --test <role> <file>` in dir and
// returns the combined output and exit error. A nil error means the role was
// allowed for that file; a non-nil error means the commit would be blocked
// (or, for an unrecognised role, refused as a usage error).
func runRoleEnforceTest(t *testing.T, hook, dir, role, file string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", hook, "--test", role, file)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// TestRoleEnforceLiveMatchesCanonical guards against the exact drift that
// caused issue #147: the live gauntlet runs .mill/checks/role-enforce, so it
// must always match the canonical checks/role-enforce. Hand-written per-role
// branches in either file would silently diverge from .mill/roles/*/ROLE.md.
func TestRoleEnforceLiveMatchesCanonical(t *testing.T) {
	root, err := projectRoot()
	if err != nil {
		t.Skipf("cannot find project root: %v", err)
	}
	canonical, err := os.ReadFile(filepath.Join(root, "checks", "role-enforce"))
	if err != nil {
		t.Fatalf("read canonical checks/role-enforce: %v", err)
	}
	live, err := os.ReadFile(filepath.Join(root, ".mill", "checks", "role-enforce"))
	if err != nil {
		t.Fatalf("read live .mill/checks/role-enforce: %v", err)
	}
	if !bytes.Equal(canonical, live) {
		t.Errorf(".mill/checks/role-enforce has drifted from checks/role-enforce; see issue #147")
	}
}

// TestLiveRoleEnforceFixesIssue147 verifies the gauntlet entry point
// (.mill/checks/role-enforce) enforces role capability from ROLE.md
// frontmatter instead of the stale hand-written case dispatch that raised
// "Unknown role: architect" for eight of the eleven roles.
func TestLiveRoleEnforceFixesIssue147(t *testing.T) {
	root, err := projectRoot()
	if err != nil {
		t.Skipf("cannot find project root: %v", err)
	}
	hook := filepath.Join(root, ".mill", "checks", "role-enforce")
	if _, err := os.Stat(hook); err != nil {
		t.Skipf("live role-enforce not present: %v", err)
	}

	// Structural: the stale hand-written case dispatch and its "Unknown role"
	// fallback must be gone (issue #147's root cause).
	data, err := os.ReadFile(hook)
	if err != nil {
		t.Fatalf("read live role-enforce: %v", err)
	}
	if bytes.Contains(data, []byte("case \"$ROLE\"")) {
		t.Errorf("live role-enforce still uses the stale hand-written case dispatch")
	}
	if bytes.Contains(data, []byte("Unknown role: $ROLE")) {
		t.Errorf("live role-enforce still contains the stale unknown-role branch")
	}

	// Behavioral: architect committing a .go file is refused by an allowed_files
	// rule (allowed_files: .md .yml .yaml), NOT by the unknown-role branch.
	out, err := runRoleEnforceTest(t, hook, root, "architect", "main.go")
	if err == nil {
		t.Fatal("expected architect committing .go to be blocked, got exit 0")
	}
	if strings.Contains(out, "Unknown role") {
		t.Errorf("architect hit the unknown-role branch (the #147 symptom); output:\n%s", out)
	}
	if !strings.Contains(out, "BLOCKED") {
		t.Errorf("expected architect blocked by an allowed_files rule, got:\n%s", out)
	}

	// Acceptance: sr-dev-be CAN commit .go; pm CANNOT.
	if _, err := runRoleEnforceTest(t, hook, root, "sr-dev-be", "internal/foo.go"); err != nil {
		t.Errorf("expected sr-dev-be to commit internal/foo.go, got error: %v", err)
	}
	if out, err := runRoleEnforceTest(t, hook, root, "pm", "foo.go"); err == nil || !strings.Contains(out, "BLOCKED") {
		t.Errorf("expected pm committing .go to be BLOCKED, got: %v\n%s", err, out)
	}

	// An unrecognised role is still refused (the `*` equivalent must stay).
	out, err = runRoleEnforceTest(t, hook, root, "ninja", "foo.go")
	if err == nil {
		t.Fatal("expected unknown role 'ninja' to be refused, got exit 0")
	}
	if !strings.Contains(out, "unknown role") {
		t.Errorf("expected refusal for unknown role, got:\n%s", out)
	}
}

func TestRoleEnforceHookStaffBlockedOnGo(t *testing.T) {
	root, err := projectRoot()
	if err != nil {
		t.Skipf("skipping: cannot find project root: %v", err)
	}
	hook := filepath.Join(root, "checks", "role-enforce")

	// Staff is BLOCKED from writing .go files (role contract enforcement #25)
	cmd := exec.Command("bash", hook, "--test", "staff", "anything.go")
	cmd.Dir = root
	if err := cmd.Run(); err == nil {
		t.Error("staff should be blocked from committing .go files")
	}

	// Staff is ALLOWED to write .md files
	cmd = exec.Command("bash", hook, "--test", "staff", "notes.md")
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		t.Errorf("staff should be allowed for .md files, got error: %v", err)
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

func TestRoleNoArgs(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf}

	err := app.runRole(nil)
	if err == nil {
		t.Fatal("expected error for no args")
	}
	if !strings.Contains(err.Error(), "usage: mill role") {
		t.Errorf("expected usage error, got: %v", err)
	}
}
