package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runGateHook writes the embedded gate script to a temp dir, optionally seeds a
// phase fixture, runs it via bash with issue 999, and returns exit code, stdout,
// and stderr. It is a shell-level test: it exercises the real scaffold scripts.
func runGateHook(t *testing.T, gate, fixtureName, fixtureContent string) (int, string, string) {
	t.Helper()

	content, err := staticFS.ReadFile("static/scaffold/.mill/checks/gate-" + gate)
	if err != nil {
		t.Fatalf("read embedded gate-%s: %v", gate, err)
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "gate-"+gate)
	if err := os.WriteFile(script, content, 0o755); err != nil {
		t.Fatalf("write gate-%s: %v", gate, err)
	}

	if fixtureName != "" {
		fixture := filepath.Join(dir, ".mill", "phases", "999", fixtureName)
		if err := os.MkdirAll(filepath.Dir(fixture), 0o755); err != nil {
			t.Fatalf("mkdir fixture dir: %v", err)
		}
		if err := os.WriteFile(fixture, []byte(fixtureContent), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}

	cmd := exec.Command("bash", script, "999")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode(), stdout.String(), stderr.String()
		}
		t.Fatalf("run gate-%s: %v", gate, err)
	}
	return 0, stdout.String(), stderr.String()
}

// TestGateHooksEmitStderrNamingGate asserts every rejection path of the three
// phase gates exits 1 and names the gate (gate-frd:/gate-spec:/gate-tasks:) on
// stderr — the GATE_FAILURE signal surface. The gate name must NOT appear on
// stdout, proving the fix moved the message to stderr.
func TestGateHooksEmitStderrNamingGate(t *testing.T) {
	cases := []struct {
		name           string
		gate           string
		fixtureName    string
		fixtureContent string
	}{
		{name: "frd missing file", gate: "frd"},
		{name: "frd missing section", gate: "frd", fixtureName: "frd.md", fixtureContent: "# FRD\n"},
		{name: "spec missing file", gate: "spec"},
		{name: "spec missing section", gate: "spec", fixtureName: "spec.md", fixtureContent: "# SPEC\n"},
		{name: "tasks missing file", gate: "tasks"},
		{name: "tasks no role assignments", gate: "tasks", fixtureName: "tasks.md", fixtureContent: "# Tasks\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := runGateHook(t, tc.gate, tc.fixtureName, tc.fixtureContent)

			wantPrefix := "gate-" + tc.gate + ":"
			if code != 1 {
				t.Errorf("gate-%s exit code = %d, want 1", tc.gate, code)
			}
			if !strings.Contains(stderr, wantPrefix) {
				t.Errorf("gate-%s stderr = %q, want it to contain %q", tc.gate, stderr, wantPrefix)
			}
			if strings.Contains(stdout, wantPrefix) {
				t.Errorf("gate-%s wrote %q to stdout; gate name must go to stderr", tc.gate, wantPrefix)
			}
		})
	}
}
