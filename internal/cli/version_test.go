package cli

import (
	"bytes"
	"errors"
	"os"
	"testing"
)

// errGitNotFound is returned by the stubbed gitDescribe in tests to
// simulate git not being available.
var errGitNotFound = errors.New("git not found")

// chdirTemp changes to a temp directory and restores the original
// working directory on test completion.
func chdirTemp(t *testing.T) string {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		os.Chdir(orig)
	})
	return dir
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"with v prefix", "v0.1.0", "v0.1.0"},
		{"without v prefix", "0.1.0", "v0.1.0"},
		{"longer version", "1.2.3", "v1.2.3"},
		{"empty string", "", "dev"},
		{"whitespace only", "   ", "dev"},
		{"whitespace around version", "  v1.0.0  ", "v1.0.0"},
		{"whitespace around vless", "  1.0.0  ", "v1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeVersion(tt.input)
			if got != tt.want {
				t.Errorf("normalizeVersion(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveVersionFromVersionFile(t *testing.T) {
	chdirTemp(t)
	if err := os.WriteFile("VERSION", []byte("v0.5.0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got := resolveVersion()
	if got != "v0.5.0" {
		t.Errorf("resolveVersion() = %q, want %q", got, "v0.5.0")
	}
}

func TestResolveVersionFromVersionFileWithoutV(t *testing.T) {
	chdirTemp(t)
	if err := os.WriteFile("VERSION", []byte("0.5.0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// resolveVersion returns raw; normalization happens in runVersion
	got := resolveVersion()
	if got != "0.5.0" {
		t.Errorf("resolveVersion() = %q, want %q", got, "0.5.0")
	}
}

func TestResolveVersionFromGitDescribe(t *testing.T) {
	chdirTemp(t)
	orig := gitDescribe
	t.Cleanup(func() { gitDescribe = orig })
	gitDescribe = func() (string, error) {
		return "v1.2.3", nil
	}
	got := resolveVersion()
	if got != "v1.2.3" {
		t.Errorf("resolveVersion() = %q, want %q", got, "v1.2.3")
	}
}

func TestResolveVersionDevFallback(t *testing.T) {
	chdirTemp(t)
	orig := gitDescribe
	t.Cleanup(func() { gitDescribe = orig })
	gitDescribe = func() (string, error) {
		return "", errGitNotFound
	}
	got := resolveVersion()
	if got != "dev" {
		t.Errorf("resolveVersion() = %q, want %q", got, "dev")
	}
}

func TestResolveVersionEmptyGitDescribe(t *testing.T) {
	chdirTemp(t)
	orig := gitDescribe
	t.Cleanup(func() { gitDescribe = orig })
	gitDescribe = func() (string, error) {
		return "", nil
	}
	got := resolveVersion()
	if got != "dev" {
		t.Errorf("resolveVersion() = %q, want %q", got, "dev")
	}
}

func TestRunVersionLdflags(t *testing.T) {
	origVersion := Version
	t.Cleanup(func() { Version = origVersion })
	Version = "0.3.0"

	app := &App{Out: &bytes.Buffer{}}
	if err := app.runVersion(nil); err != nil {
		t.Fatalf("runVersion: %v", err)
	}
	got := app.Out.(*bytes.Buffer).String()
	want := "v0.3.0\n"
	if got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestRunVersionResolve(t *testing.T) {
	origVersion := Version
	t.Cleanup(func() { Version = origVersion })
	Version = ""

	chdirTemp(t)
	if err := os.WriteFile("VERSION", []byte("v2.0.0\n"), 0644); err != nil {
		t.Fatal(err)
	}

	app := &App{Out: &bytes.Buffer{}}
	if err := app.runVersion(nil); err != nil {
		t.Fatalf("runVersion: %v", err)
	}
	got := app.Out.(*bytes.Buffer).String()
	want := "v2.0.0\n"
	if got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
