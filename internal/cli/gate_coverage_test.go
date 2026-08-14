package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGateCoverageReportsProjectTotal is a regression test for the coverage
// gate sampling a single per-package "coverage:" line instead of the project
// total. `go test` reports packages in scheduling-dependent order, so
// `grep | head -1` (the previous .mill/checks/gate-coverage implementation)
// picked a different package on different runs — a coin-flip verdict.
//
// The fixed gate computes the project-wide statement coverage via
// `go test -coverprofile` + `go tool cover -func` (the `total:` line), which is
// an order-independent aggregate. This test asserts:
//   - the same tree yields identical output across two runs (determinism), and
//   - the reported value equals the true aggregate total (not one package's).
//
// The temp module below has two packages of different coverage (pkga=0%,
// pkgb=100%) so their aggregate total (50.0%) can never equal either sample.
// With the default 90% threshold the gate must block — and an old `head -1`
// gate could false-pass on the 100% package.
func TestGateCoverageReportsProjectTotal(t *testing.T) {
	for _, tool := range []string{"go"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available", tool)
		}
	}
	// The gate compares with bc; skip if absent rather than fail obscurely.
	if _, err := exec.LookPath("bc"); err != nil {
		t.Skip("bc not available")
	}

	content, err := staticFS.ReadFile("static/scaffold/.mill/checks/gate-coverage")
	if err != nil {
		t.Fatalf("read embedded gate-coverage: %v", err)
	}

	dir := t.TempDir()
	writeTempModule(t, dir)

	script := filepath.Join(dir, "gate-coverage")
	if err := os.WriteFile(script, content, 0o755); err != nil {
		t.Fatalf("write gate-coverage: %v", err)
	}

	run := func(t *testing.T) (int, string, string) {
		t.Helper()
		cmd := exec.Command("bash", script)
		cmd.Dir = dir // pkg defaults to ./...
		var out, errOut bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errOut
		var code int
		if e := cmd.Run(); e != nil {
			if ee, ok := e.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else {
				t.Fatalf("run gate: %v", e)
			}
		}
		return code, out.String(), errOut.String()
	}

	code1, out1, err1 := run(t)
	code2, out2, err2 := run(t)

	// Acceptance: identical output on repeated runs of an unchanged tree.
	if out1 != out2 || err1 != err2 {
		t.Errorf("gate output not deterministic across runs:\nrun1: %q stderr=%q\nrun2: %q stderr=%q",
			out1, err1, out2, err2)
	}
	if code1 != code2 {
		t.Errorf("exit code not deterministic: run1=%d run2=%d", code1, code2)
	}

	// The reported figure must be the aggregate total, not a sampled package.
	// pkga (0%) cannot false-pass; pkgb (100%) cannot be mistaken for the total.
	want := aggregateTotal(t, dir)
	if !strings.Contains(out1, want) {
		t.Errorf("gate reported %q, want output to contain the project total %q", out1, want)
	}
	if code1 != 1 {
		t.Errorf("want gate to block (total 50.0%% < 90%% threshold), got exit %d", code1)
	}
}

// writeTempModule creates a Go module with two packages of different coverage so
// that no single per-package "coverage:" line equals the aggregate total.
func writeTempModule(t *testing.T, dir string) {
	t.Helper()
	files := map[string]string{
		"go.mod":                           "module gatecoveragetest\n\ngo 1.23\n\n",
		filepath.Join("pkga", "a.go"):      "package pkga\n\nfunc A() int { return 1 }\n",
		filepath.Join("pkga", "a_test.go"): "package pkga\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n",
		filepath.Join("pkgb", "b.go"):      "package pkgb\n\nfunc B() int { return 1 }\n",
		filepath.Join("pkgb", "b_test.go"): "package pkgb\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) { _ = B() }\n",
	}
	for rel, body := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
}

// aggregateTotal independently computes the project-wide statement coverage of
// the temp module via `go test -coverprofile` + `go tool cover -func`.
func aggregateTotal(t *testing.T, dir string) string {
	t.Helper()
	profile := filepath.Join(t.TempDir(), "cover.out")
	cmd := exec.Command("go", "test", "-count=1", "-coverprofile", profile, "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go test: %v\n%s", err, out)
	}
	// go tool cover resolves the profile's relative source paths against its
	// working directory, so run it from the module root.
	coverCmd := exec.Command("go", "tool", "cover", "-func="+profile)
	coverCmd.Dir = dir
	out, err := coverCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go tool cover: %v\noutput:\n%s\nprofile=%s", err, out, profile)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "total:") {
			continue
		}
		fields := strings.Fields(line)
		if pct := strings.TrimSuffix(fields[len(fields)-1], "%"); pct != "" {
			return pct
		}
	}
	t.Fatalf("no total line in go tool cover -func output:\n%s", out)
	return ""
}
