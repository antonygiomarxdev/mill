package cli

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/antonygiomarxdev/mill/internal/config"
)

func TestInitCreatesMillYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "-yes", "-target", dir)
	if err != nil {
		t.Fatalf("init returned error: %v", err)
	}

	millYML := filepath.Join(dir, "mill.yml")
	content, err := os.ReadFile(millYML)
	if err != nil {
		t.Fatalf("expected mill.yml to be created: %v", err)
	}

	if !strings.Contains(string(content), "project:") {
		t.Error("expected mill.yml to contain 'project:'")
	}
	if !strings.Contains(string(content), "provider:") {
		t.Error("expected mill.yml to contain 'provider:'")
	}
	if !strings.Contains(string(content), "commandcode") {
		t.Error("expected mill.yml to contain default provider 'commandcode'")
	}
	if !strings.Contains(string(content), "laguna-free") {
		t.Error("expected mill.yml to contain default model 'laguna-free'")
	}
	if !strings.Contains(string(content), "max-rounds:") {
		t.Error("expected mill.yml to contain 'max-rounds:'")
	}
}

func TestInitCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "-yes", "-target", dir)
	if err != nil {
		t.Fatalf("init returned error: %v", err)
	}

	for _, dirName := range []string{".mill/roles", ".mill/checks", ".mill/skills", ".mill/docs"} {
		info, err := os.Stat(filepath.Join(dir, dirName))
		if err != nil {
			t.Errorf("expected %s directory to be created: %v", dirName, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", dirName)
		}
	}
}

func TestInitCreatesRecursionDirsAndLessons(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "-yes", "-target", dir)
	if err != nil {
		t.Fatalf("init returned error: %v", err)
	}

	for _, sub := range []string{"logs", "state"} {
		info, err := os.Stat(filepath.Join(dir, ".mill", sub))
		if err != nil {
			t.Errorf("expected .mill/%s directory to be created: %v", sub, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected .mill/%s to be a directory", sub)
		}
	}

	// Every scaffolded role gets a lessons.md file.
	for _, role := range []string{"staff", "pm", "architect", "tech-lead", "sr-dev-be", "ux-designer", "ui-designer", "qa-docs", "reviewer"} {
		p := filepath.Join(dir, ".mill", "roles", role, "lessons.md")
		content, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("expected lessons.md for role %s: %v", role, err)
			continue
		}
		if !strings.Contains(string(content), "# Lessons for "+role) {
			t.Errorf("expected lessons.md for %s to contain title, got: %q", role, string(content))
		}
	}
}

func TestInitCopiesRoleFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "-yes", "-target", dir)
	if err != nil {
		t.Fatalf("init returned error: %v", err)
	}

	roleFile := filepath.Join(dir, ".mill", "roles", "sr-dev-be", "ROLE.md")
	if _, err := os.Stat(roleFile); os.IsNotExist(err) {
		t.Error("expected .mill/roles/sr-dev-be/ROLE.md to be created")
	}

	commonFile := filepath.Join(dir, ".mill", "roles", "COMMON.md")
	if _, err := os.Stat(commonFile); os.IsNotExist(err) {
		t.Error("expected .mill/roles/COMMON.md to be created")
	}
}

func TestInitCopiesCheckFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "-yes", "-target", dir)
	if err != nil {
		t.Fatalf("init returned error: %v", err)
	}

	for _, file := range []string{".mill/checks/pre-commit", ".mill/checks/pre-push", ".mill/checks/common.sh"} {
		if _, err := os.Stat(filepath.Join(dir, file)); os.IsNotExist(err) {
			t.Errorf("expected %s to be created", file)
		}
	}
}

func TestInitWithCustomFlags(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "-yes", "-target", dir, "-name", "myproject", "-provider", "claude", "-model", "claude-sonnet-5")
	if err != nil {
		t.Fatalf("init returned error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "mill.yml"))
	if err != nil {
		t.Fatalf("expected mill.yml to be created: %v", err)
	}

	s := string(content)
	if !strings.Contains(s, "myproject") {
		t.Error("expected mill.yml to contain custom project name 'myproject'")
	}
	if !strings.Contains(s, "claude") {
		t.Error("expected mill.yml to contain custom provider 'claude'")
	}
	if !strings.Contains(s, "claude-sonnet-5") {
		t.Error("expected mill.yml to contain custom model 'claude-sonnet-5'")
	}
}

func TestInitInteractiveUsesDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	// Simulate pressing Enter for all prompts (use defaults)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("\n\n\n\n")}

	err := app.Run("init", "-target", dir)
	if err != nil {
		t.Fatalf("init returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "mill.yml")); os.IsNotExist(err) {
		t.Error("expected mill.yml to be created")
	}
	if _, err := os.Stat(filepath.Join(dir, ".mill", "roles")); os.IsNotExist(err) {
		t.Error("expected roles/ directory to be created")
	}
}

func TestInitPrintOutput(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "-yes", "-target", dir)
	if err != nil {
		t.Fatalf("init returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Created") {
		t.Error("expected output to contain 'Created'")
	}
	if !strings.Contains(output, "initialized") {
		t.Error("expected output to contain 'initialized'")
	}
}

func TestGenerateMillYAMLWriteError(t *testing.T) {
	dir := t.TempDir()
	// Pre-create mill.yml as a directory so os.WriteFile fails.
	millPath := filepath.Join(dir, "mill.yml")
	if err := os.Mkdir(millPath, 0o755); err != nil {
		t.Fatalf("failed to create mill.yml as directory: %v", err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.generateMillYAML(dir, initConfig{
		Name:      "testproj",
		Provider:  "commandcode",
		Model:     "laguna-free",
		MaxRounds: 4,
	})
	if err == nil {
		t.Error("expected error when writing to mill.yml that is a directory")
	}
}

func TestPromptDefaultPath(t *testing.T) {
	r := strings.NewReader("\n")
	var buf bytes.Buffer
	got := prompt(bufio.NewReader(r), &buf, "Name", "default")
	if got != "default" {
		t.Errorf("expected 'default', got %q", got)
	}
}

func TestPromptEOF(t *testing.T) {
	r := strings.NewReader("")
	var buf bytes.Buffer
	got := prompt(bufio.NewReader(r), &buf, "Name", "default")
	if got != "default" {
		t.Errorf("expected 'default' on EOF, got %q", got)
	}
}

func TestCopyScaffoldWriteError(t *testing.T) {
	dir := t.TempDir()
	// Create a regular file at .omp — copyScaffold tries MkdirAll on .omp/ which
	// fails because a non-directory exists there.
	if err := os.WriteFile(filepath.Join(dir, ".omp"), []byte("block"), 0o644); err != nil {
		t.Fatalf("failed to create blocker file: %v", err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.copyScaffold(dir, initConfig{}, false)
	if err == nil {
		t.Error("expected error when .omp is a file instead of a directory")
	}
}

func TestProjectRootGoModPresent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	got, err := projectRoot()
	if err != nil {
		t.Fatalf("projectRoot returned error: %v", err)
	}
	if got != dir {
		t.Errorf("expected %q, got %q", dir, got)
	}
}

func TestProjectRootNoMarker(t *testing.T) {
	dir := t.TempDir()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	_, err = projectRoot()
	if err == nil {
		t.Error("expected error when no go.mod or mill.yml found")
	}
}

func TestRunInitParseError(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "--nonexistent")
	if err == nil {
		t.Error("expected error for unknown flag --nonexistent")
	}
}

func TestRunInitMissingTargetDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "sub", "child")

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "-yes", "-target", target)
	if err != nil {
		t.Fatalf("init returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(target, "mill.yml")); err != nil {
		t.Errorf("expected mill.yml in target dir: %v", err)
	}
}

func TestRunInitCustomNameFlag(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "-yes", "-name", "testproj", "-target", dir)
	if err != nil {
		t.Fatalf("init returned error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "mill.yml"))
	if err != nil {
		t.Fatalf("expected mill.yml to be created: %v", err)
	}
	if !strings.Contains(string(content), "project: testproj") {
		t.Error("expected mill.yml to contain 'project: testproj'")
	}
}

// --- Overwrite interaction tests ---

// TestInitOverwriteMillYMLReject: create mill.yml (not a preserved dir) before init,
// reject the overwrite prompt.
func TestInitOverwriteMillYMLReject(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create mill.yml to trigger a collision that is NOT preserved.
	if err := os.WriteFile(filepath.Join(dir, "mill.yml"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: errBuf, In: strings.NewReader("n\n")}

	err := app.Run("init", "-yes", "-target", dir)
	if err == nil {
		t.Error("expected error when rejecting overwrite")
	}
	if !strings.Contains(err.Error(), "aborted") {
		t.Errorf("expected error to contain 'aborted', got: %v", err)
	}

	// Verify mill.yml was NOT modified.
	content, err := os.ReadFile(filepath.Join(dir, "mill.yml"))
	if err != nil {
		t.Fatalf("expected mill.yml to still exist: %v", err)
	}
	if string(content) != "old" {
		t.Errorf("expected mill.yml to be unchanged, got: %q", string(content))
	}
}

// TestInitOverwriteMillYMLAccept: create mill.yml, accept overwrite.
func TestInitOverwriteMillYMLAccept(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mill.yml"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("y\n\n\n\n\n")}

	err := app.Run("init", "-target", dir)
	if err != nil {
		t.Fatalf("expected init to succeed after accepting overwrite, got: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "mill.yml")); os.IsNotExist(err) {
		t.Error("expected mill.yml to be created after accepting overwrite")
	}
}

func TestInitOverwriteMillYMLCaseInsensitive(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"uppercase Y", "Y\n\n\n\n\n"},
		{"uppercase YES", "YES\n\n\n\n\n"},
		{"mixed Yes", "Yes\n\n\n\n\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "mill.yml"), []byte("old"), 0o644); err != nil {
				t.Fatal(err)
			}

			buf := new(bytes.Buffer)
			app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader(tt.input)}

			err := app.Run("init", "-target", dir)
			if err != nil {
				t.Fatalf("expected init to succeed with %q response, got: %v", tt.input, err)
			}

			if _, err := os.Stat(filepath.Join(dir, "mill.yml")); os.IsNotExist(err) {
				t.Error("expected mill.yml to be created")
			}
		})
	}
}

func TestInitOverwriteForceBypass(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	millDir := filepath.Join(dir, ".mill")
	if err := os.MkdirAll(millDir, 0o755); err != nil {
		t.Fatalf("failed to create .mill/: %v", err)
	}
	// Create a known file inside .mill/ to verify overwrite happens.
	markerFile := filepath.Join(millDir, "marker.txt")
	if err := os.WriteFile(markerFile, []byte("old"), 0o644); err != nil {
		t.Fatalf("failed to create marker.txt: %v", err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "--force", "--yes", "-target", dir)
	if err != nil {
		t.Fatalf("expected init --force --yes to succeed, got: %v", err)
	}

	// Verify no overwrite prompt was shown.
	output := buf.String()
	if strings.Contains(output, "Overwrite?") {
		t.Error("expected no 'Overwrite?' prompt with --force flag")
	}

	// Verify init completed.
	if _, err := os.Stat(filepath.Join(dir, "mill.yml")); os.IsNotExist(err) {
		t.Error("expected mill.yml to be created")
	}
}

func TestInitOverwriteForceWithoutYes(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	millDir := filepath.Join(dir, ".mill")
	if err := os.MkdirAll(millDir, 0o755); err != nil {
		t.Fatalf("failed to create .mill/: %v", err)
	}

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: errBuf, In: strings.NewReader("")}

	err := app.Run("init", "--force", "-target", dir)
	if err != nil {
		t.Fatalf("expected init --force to succeed (prompts use defaults on EOF), got: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "Overwrite?") {
		t.Error("expected no 'Overwrite?' prompt with --force flag")
	}

	if _, err := os.Stat(filepath.Join(dir, "mill.yml")); os.IsNotExist(err) {
		t.Error("expected mill.yml to be created")
	}
}

func TestInitOverwriteNoMillDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "-yes", "-target", dir)
	if err != nil {
		t.Fatalf("expected init to succeed in clean dir, got: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "Overwrite?") {
		t.Error("expected no 'Overwrite?' prompt when .mill/ does not exist")
	}

	millInfo, err := os.Stat(filepath.Join(dir, ".mill"))
	if err != nil {
		t.Fatalf("expected .mill/ to exist after init: %v", err)
	}
	if !millInfo.IsDir() {
		t.Error("expected .mill/ to be a directory")
	}
}

func TestInitOverwriteMillDirIsFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create .mill as a regular file, not a directory.
	millPath := filepath.Join(dir, ".mill")
	if err := os.WriteFile(millPath, []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("failed to create .mill as file: %v", err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "-yes", "-target", dir)
	if err == nil {
		t.Error("expected error when .mill is a file, not a directory")
	}
	if !strings.Contains(err.Error(), "file, not a directory") {
		t.Errorf("expected error to mention 'file, not a directory', got: %v", err)
	}
}

func TestInitOverwriteForceYesSkipsAllPrompts(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	millDir := filepath.Join(dir, ".mill")
	if err := os.MkdirAll(millDir, 0o755); err != nil {
		t.Fatalf("failed to create .mill/: %v", err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "--force", "--yes", "-name", "testproj", "-target", dir)
	if err != nil {
		t.Fatalf("expected init --force --yes -name to succeed, got: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "mill.yml"))
	if err != nil {
		t.Fatalf("expected mill.yml to be created: %v", err)
	}
	if !strings.Contains(string(content), "project: testproj") {
		t.Error("expected mill.yml to contain 'project: testproj'")
	}

	output := buf.String()
	if strings.Contains(output, "Overwrite?") {
		t.Error("expected no 'Overwrite?' prompt with --force flag")
	}
}

// --- Pre-init validation tests (#69) ---

func TestValidatePreInitSuccess(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validatePreInit(dir, ProjectGoModule); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidatePreInitNoGitDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := validatePreInit(dir, ProjectGoModule)
	if err == nil {
		t.Error("expected error for missing .git directory")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("expected 'not a git repository' message, got: %v", err)
	}
}

func TestValidatePreInitNoGoModStillSucceeds(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// For generic projects, missing go.mod is fine — just a warning.
	err := validatePreInit(dir, ProjectGeneric)
	if err != nil {
		t.Errorf("expected no error for generic project without go.mod, got: %v", err)
	}
}

func TestValidateBinariesMissingGit(t *testing.T) {
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)
	os.Setenv("PATH", t.TempDir())

	err := validateBinaries(false)
	if err == nil {
		t.Error("expected error for missing git binary")
	}
	if !strings.Contains(err.Error(), "git not found") {
		t.Errorf("expected 'git not found' message, got: %v", err)
	}
}

func TestValidateBinariesGoRequiredForGoModule(t *testing.T) {
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	fakeBin := t.TempDir()
	// Create a fake git script so only go is missing.
	gitScript := filepath.Join(fakeBin, "git")
	if err := os.WriteFile(gitScript, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.Setenv("PATH", fakeBin)

	err := validateBinaries(true)
	if err == nil {
		t.Error("expected error for missing go binary when required")
	}
	if !strings.Contains(err.Error(), "go not found") {
		t.Errorf("expected 'go not found' message, got: %v", err)
	}
}

func TestValidateBinariesGoOptionalForGeneric(t *testing.T) {
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	fakeBin := t.TempDir()
	// Create a fake git script so only go is missing.
	gitScript := filepath.Join(fakeBin, "git")
	if err := os.WriteFile(gitScript, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.Setenv("PATH", fakeBin)

	// requireGo=false: go binary not needed for generic projects.
	err := validateBinaries(false)
	if err != nil {
		t.Errorf("expected no error when go is not required, got: %v", err)
	}
}

func TestValidatePreInitEmptyGoMod(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Empty go.mod is valid.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validatePreInit(dir, ProjectGoModule); err != nil {
		t.Errorf("expected no error for empty go.mod, got: %v", err)
	}
}

func TestValidatePreInitSubdirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(dir, "sub", "child")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	// validatePreInit walks up from subdir to find root with go.mod + .git.
	if err := validatePreInit(subdir, ProjectGoModule); err != nil {
		t.Errorf("expected no error from subdirectory, got: %v", err)
	}
}

func TestRunInitValidationGate(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	// Run init in a directory with no .git — must fail
	// before any scaffold files are written.
	err := app.Run("init", "-yes", "-target", dir)
	if err == nil {
		t.Error("expected error from validation gate (no .git)")
	}
	// No scaffold files should have been created.
	if _, err := os.Stat(filepath.Join(dir, "mill.yml")); err == nil {
		t.Error("expected no mill.yml to be created when validation fails")
	}
}

// --- collisionReport tests ---

func TestCollisionReportEmptyDir(t *testing.T) {
	dir := t.TempDir()
	conflicts, err := collisionReport(dir, initConfig{}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conflicts != nil {
		t.Errorf("expected nil conflicts for empty dir, got: %v", conflicts)
	}
}

func TestCollisionReportGitOnly(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	conflicts, err := collisionReport(dir, initConfig{}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conflicts != nil {
		t.Errorf("expected nil conflicts for .git-only dir, got: %v", conflicts)
	}
}

func TestCollisionReportNonexistentTarget(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent")
	conflicts, err := collisionReport(dir, initConfig{}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conflicts != nil {
		t.Errorf("expected nil conflicts for nonexistent target, got: %v", conflicts)
	}
}

func TestCollisionReportMillDirPreserved(t *testing.T) {
	dir := t.TempDir()
	// Write a file so the dir is not "empty"
	if err := os.WriteFile(filepath.Join(dir, "somefile"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".mill"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Add a file inside .mill/ so it's non-empty (will be preserved).
	if err := os.WriteFile(filepath.Join(dir, ".mill", "existing"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	conflicts, err := collisionReport(dir, initConfig{}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// .mill/ is non-empty and a harness directory — it should be preserved, NOT in conflicts.
	for _, c := range conflicts {
		if c == ".mill/" {
			t.Errorf("expected '.mill/' to be preserved (not in conflicts), got: %v", conflicts)
		}
	}
}

func TestCollisionReportMillIsFile(t *testing.T) {
	dir := t.TempDir()
	// Write a file so the dir is not "empty"
	if err := os.WriteFile(filepath.Join(dir, "somefile"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".mill"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := collisionReport(dir, initConfig{}, false)
	if err == nil {
		t.Error("expected error when .mill is a file")
	}
	if !strings.Contains(err.Error(), "file, not a directory") {
		t.Errorf("expected 'file, not a directory' in error, got: %v", err)
	}
}

func TestCollisionReportMillYML(t *testing.T) {
	dir := t.TempDir()
	// Write a file so the dir is not "empty"
	if err := os.WriteFile(filepath.Join(dir, "somefile"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mill.yml"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	conflicts, err := collisionReport(dir, initConfig{}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, c := range conflicts {
		if c == "mill.yml" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'mill.yml' in collisions, got: %v", conflicts)
	}
}

// --- dry-run tests ---

func TestDryRunNoWrites(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "--dry-run", "-target", dir)
	if err != nil {
		t.Fatalf("dry-run returned error: %v", err)
	}

	// Verify no files were created.
	if _, err := os.Stat(filepath.Join(dir, "mill.yml")); err == nil {
		t.Error("dry-run should not create mill.yml")
	}
	if _, err := os.Stat(filepath.Join(dir, ".mill")); err == nil {
		t.Error("dry-run should not create .mill/")
	}
	if _, err := os.Stat(filepath.Join(dir, ".omp")); err == nil {
		t.Error("dry-run should not create .omp/")
	}

	output := buf.String()
	if !strings.Contains(output, "Would create") {
		t.Errorf("expected confirmation in output, got: %s", output)
	}
}

func TestDryRunReportsCollisions(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create mill.yml — not a preserved directory, so it shows as collision.
	if err := os.WriteFile(filepath.Join(dir, "mill.yml"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "--dry-run", "-target", dir)
	if err != nil {
		t.Fatalf("dry-run returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Would overwrite") {
		t.Errorf("expected 'Would overwrite' in dry-run output, got: %s", output)
	}
	if !strings.Contains(output, "mill.yml") {
		t.Errorf("expected mill.yml collision in dry-run output, got: %s", output)
	}
	if !strings.Contains(output, "Would create") {
		t.Errorf("expected confirmation in output, got: %s", output)
	}
}

func TestDryRunCleanDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "--dry-run", "-target", dir)
	if err != nil {
		t.Fatalf("dry-run returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No existing files would be overwritten") {
		t.Errorf("expected 'No existing files' in dry-run output, got: %s", output)
	}
}

// --- force tests ---

func TestForceAloneWarns(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".mill"), 0o755); err != nil {
		t.Fatal(err)
	}

	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: outBuf, Err: errBuf, In: strings.NewReader("\n\n\n\n")}

	err := app.Run("init", "--force", "-target", dir)
	if err != nil {
		t.Fatalf("init --force returned error: %v", err)
	}

	// Check that warning was logged to stderr.
	if !strings.Contains(errBuf.String(), "Warning") {
		t.Errorf("expected warning on stderr with --force alone, got: %s", errBuf.String())
	}
	// Check that no overwrite prompt appeared on stdout.
	if strings.Contains(outBuf.String(), "Overwrite?") {
		t.Error("expected no 'Overwrite?' prompt with --force")
	}
}

// --- promptOverwrite tests ---

func TestPromptOverwriteShowsFileList(t *testing.T) {
	dir := t.TempDir()
	// Make dir non-empty so collisionReport doesn't early-return.
	if err := os.WriteFile(filepath.Join(dir, "somefile"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create mill.yml to trigger a non-preserved collision.
	if err := os.WriteFile(filepath.Join(dir, "mill.yml"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	in := bufio.NewReader(strings.NewReader("y\n"))
	err := promptOverwrite(dir, initConfig{}, in, &buf, false)
	if err != nil {
		t.Fatalf("promptOverwrite returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "file(s) would be overwritten") {
		t.Errorf("expected 'file(s) would be overwritten' in output, got: %s", output)
	}
	if !strings.Contains(output, "mill.yml") {
		t.Errorf("expected 'mill.yml' in output, got: %s", output)
	}
	if !strings.Contains(output, "Overwrite?") {
		t.Error("expected 'Overwrite?' prompt")
	}
}

func TestPromptOverwriteTruncatesLongList(t *testing.T) {
	dir := t.TempDir()
	// Make dir non-empty so collisionReport doesn't early-return.
	if err := os.WriteFile(filepath.Join(dir, "somefile"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create many files in non-preserved locations to trigger truncation.
	// Create scaffold files that would collide.
	scaffoldFiles := []string{
		".omp/AGENTS.md", ".omp/RULES.md",
		".claude/CLAUDE.md",
		".github/copilot-instructions.md",
		".github/ISSUE_TEMPLATE/bug.md",
		".github/ISSUE_TEMPLATE/feature.md",
	}
	for _, f := range scaffoldFiles {
		fullPath := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Also add mill.yml for good measure.
	if err := os.WriteFile(filepath.Join(dir, "mill.yml"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Use --force so that preserved directories are NOT skipped.
	var buf bytes.Buffer
	in := bufio.NewReader(strings.NewReader("y\n"))
	err := promptOverwrite(dir, initConfig{}, in, &buf, true)
	if err != nil {
		t.Fatalf("promptOverwrite returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "... and") {
		t.Errorf("expected '... and N more' for truncated list, got: %s", output)
	}
}

func TestPromptOverwriteNoCollisions(t *testing.T) {
	dir := t.TempDir()
	// Empty dir -> no collisions -> promptOverwrite returns nil without writing output.
	in := bufio.NewReader(strings.NewReader(""))
	var buf bytes.Buffer
	err := promptOverwrite(dir, initConfig{}, in, &buf, false)
	if err != nil {
		t.Fatalf("promptOverwrite returned error: %v", err)
	}
	if buf.Len() > 0 {
		t.Errorf("expected no output when no collisions, got: %s", buf.String())
	}
}

func TestPromptOverwriteUserRejects(t *testing.T) {
	dir := t.TempDir()
	// Make dir non-empty so collisionReport doesn't early-return.
	if err := os.WriteFile(filepath.Join(dir, "somefile"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create mill.yml to trigger collision.
	if err := os.WriteFile(filepath.Join(dir, "mill.yml"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	in := bufio.NewReader(strings.NewReader("n\n"))
	err := promptOverwrite(dir, initConfig{}, in, &buf, false)
	if err == nil {
		t.Error("expected error when user rejects overwrite")
	}
	if !strings.Contains(err.Error(), "aborted") {
		t.Errorf("expected 'aborted' in error, got: %v", err)
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0B"},
		{512, "512B"},
		{1023, "1023B"},
		{1024, "1.0KB"},
		{1536, "1.5KB"},
		{1048576, "1.0MB"},
		{2097152, "2.0MB"},
	}
	for _, tt := range tests {
		got := formatSize(tt.n)
		if got != tt.want {
			t.Errorf("formatSize(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

// --- Project type detection tests (#17) ---

func TestDetectGoModule(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pt := detectProjectType(dir)
	if pt != ProjectGoModule {
		t.Errorf("expected ProjectGoModule, got %v", pt)
	}
}

func TestDetectGoModuleInParent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(dir, "src", "cmd")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	pt := detectProjectType(subdir)
	if pt != ProjectGoModule {
		t.Errorf("expected ProjectGoModule from parent walk-up, got %v", pt)
	}
}

func TestDetectJSProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	pt := detectProjectType(dir)
	if pt != ProjectJSProject {
		t.Errorf("expected ProjectJSProject, got %v", pt)
	}
}

func TestDetectMonoRepo(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	frontend := filepath.Join(dir, "frontend")
	if err := os.MkdirAll(frontend, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(frontend, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Target is the frontend subdir — walk up finds package.json first,
	// then go.mod higher up.
	pt := detectProjectType(frontend)
	if pt != ProjectMonoRepo {
		t.Errorf("expected ProjectMonoRepo, got %v", pt)
	}
}

func TestDetectGeneric(t *testing.T) {
	dir := t.TempDir()
	pt := detectProjectType(dir)
	if pt != ProjectGeneric {
		t.Errorf("expected ProjectGeneric for empty dir, got %v", pt)
	}
}

func TestDetectGoModWins(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Both at same level: go.mod is the stronger signal.
	pt := detectProjectType(dir)
	if pt != ProjectGoModule {
		t.Errorf("expected ProjectGoModule (go.mod wins), got %v", pt)
	}
}

func TestDetectGoWorkIgnored(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.work"), []byte("go 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// go.work is not used for detection, so we detect JSProject.
	pt := detectProjectType(dir)
	if pt != ProjectJSProject {
		t.Errorf("expected ProjectJSProject (go.work ignored), got %v", pt)
	}
}

// --- Directory preservation tests (#17) ---

func TestPreserveGithubDir(t *testing.T) {
	dir := t.TempDir()
	githubDir := filepath.Join(dir, ".github")
	if err := os.MkdirAll(filepath.Join(githubDir, "ISSUE_TEMPLATE"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(githubDir, "ISSUE_TEMPLATE", "bug.md"), []byte("custom"), 0o644); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: errBuf, In: strings.NewReader("")}

	// Run init with force=false — .github/ should be preserved.
	err := app.copyScaffold(dir, initConfig{}, false)
	if err != nil {
		t.Fatalf("copyScaffold returned error: %v", err)
	}

	// Verify the custom file is untouched.
	content, err := os.ReadFile(filepath.Join(githubDir, "ISSUE_TEMPLATE", "bug.md"))
	if err != nil {
		t.Fatalf("expected bug.md to still exist: %v", err)
	}
	if string(content) != "custom" {
		t.Errorf("expected custom bug.md to be preserved, got: %q", string(content))
	}
	// Verify preservation message was logged.
	if !strings.Contains(errBuf.String(), "Preserving") {
		t.Errorf("expected preservation message on stderr, got: %s", errBuf.String())
	}
}

func TestPreserveClaudeDir(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte("custom"), 0o644); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: errBuf, In: strings.NewReader("")}

	err := app.copyScaffold(dir, initConfig{}, false)
	if err != nil {
		t.Fatalf("copyScaffold returned error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(claudeDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("expected CLAUDE.md to still exist: %v", err)
	}
	if string(content) != "custom" {
		t.Errorf("expected custom CLAUDE.md to be preserved, got: %q", string(content))
	}
}

func TestPreserveChecksDir(t *testing.T) {
	dir := t.TempDir()
	checksDir := filepath.Join(dir, "checks")
	if err := os.MkdirAll(checksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(checksDir, "gate-review"), []byte("custom"), 0o755); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: errBuf, In: strings.NewReader("")}

	err := app.copyScaffold(dir, initConfig{}, false)
	if err != nil {
		t.Fatalf("copyScaffold returned error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(checksDir, "gate-review"))
	if err != nil {
		t.Fatalf("expected gate-review to still exist: %v", err)
	}
	if string(content) != "custom" {
		t.Errorf("expected custom gate-review to be preserved, got: %q", string(content))
	}
}

func TestOverwriteEmptyGithubDir(t *testing.T) {
	dir := t.TempDir()
	// Create empty .github/ — empty dirs are NOT preserved (no user investment).
	if err := os.MkdirAll(filepath.Join(dir, ".github", "ISSUE_TEMPLATE"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Remove the ISSUE_TEMPLATE dir so .github/ is empty.
	if err := os.RemoveAll(filepath.Join(dir, ".github", "ISSUE_TEMPLATE")); err != nil {
		t.Fatal(err)
	}
	// Verify .github/ is empty.
	entries, _ := os.ReadDir(filepath.Join(dir, ".github"))
	if len(entries) != 0 {
		t.Fatalf("expected empty .github/ for test setup")
	}

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: errBuf, In: strings.NewReader("")}

	err := app.copyScaffold(dir, initConfig{}, false)
	if err != nil {
		t.Fatalf("copyScaffold returned error: %v", err)
	}

	// Empty .github/ should be overwritten — so scaffold files should exist.
	if _, err := os.Stat(filepath.Join(dir, ".github", "ISSUE_TEMPLATE")); os.IsNotExist(err) {
		t.Error("expected .github/ISSUE_TEMPLATE/ to be created (empty dir overwrite)")
	}
}

func TestOverwriteWithForce(t *testing.T) {
	dir := t.TempDir()
	githubDir := filepath.Join(dir, ".github")
	if err := os.MkdirAll(filepath.Join(githubDir, "ISSUE_TEMPLATE"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(githubDir, "ISSUE_TEMPLATE", "bug.md"), []byte("custom"), 0o644); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: errBuf, In: strings.NewReader("")}

	// --force overrides preservation.
	err := app.copyScaffold(dir, initConfig{}, true)
	if err != nil {
		t.Fatalf("copyScaffold with force returned error: %v", err)
	}

	// Verify the custom file was overwritten.
	content, err := os.ReadFile(filepath.Join(githubDir, "ISSUE_TEMPLATE", "bug.md"))
	if err != nil {
		t.Fatalf("expected bug.md to exist after force: %v", err)
	}
	if string(content) == "custom" {
		t.Errorf("expected custom bug.md to be overwritten with --force, got: %q", string(content))
	}
}

func TestPreserveIsPerDirectory(t *testing.T) {
	dir := t.TempDir()
	// Create .github/ with content (will be preserved).
	githubDir := filepath.Join(dir, ".github")
	if err := os.MkdirAll(filepath.Join(githubDir, "ISSUE_TEMPLATE"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(githubDir, "ISSUE_TEMPLATE", "bug.md"), []byte("custom"), 0o644); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: errBuf, In: strings.NewReader("")}

	err := app.copyScaffold(dir, initConfig{}, false)
	if err != nil {
		t.Fatalf("copyScaffold returned error: %v", err)
	}

	// .github/ preserved.
	content, err := os.ReadFile(filepath.Join(githubDir, "ISSUE_TEMPLATE", "bug.md"))
	if err != nil {
		t.Fatalf("expected bug.md to still exist: %v", err)
	}
	if string(content) != "custom" {
		t.Errorf("expected custom bug.md to be preserved, got: %q", string(content))
	}
	// .claude/ should be created fresh (not preserved).
	if _, err := os.Stat(filepath.Join(dir, ".claude", "CLAUDE.md")); os.IsNotExist(err) {
		t.Error("expected .claude/CLAUDE.md to be created fresh")
	}
}

func TestPreserveUnknownDir(t *testing.T) {
	dir := t.TempDir()
	// Create a src/ directory — this is NOT a known harness directory.
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	// src/ is not a known harness dir, so it should NOT be preserved
	// (preserveDir returns false).
	d := filepath.Join(dir, "src")
	if preserveDir(dir, filepath.Join(d, "main.go")) {
		t.Error("src/ should NOT be preserved (not a harness directory)")
	}
}

// --- --minimal tests (#17) ---

func TestMinimalOnlyEssentialFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "--minimal", "--yes", "-target", dir)
	if err != nil {
		t.Fatalf("init --minimal returned error: %v", err)
	}

	// Must exist.
	for _, f := range []string{
		"mill.yml",
		".mill/COMMON.md",
		".mill/roles/staff/ROLE.md",
		".mill/roles/pm/ROLE.md",
	} {
		if _, err := os.Stat(filepath.Join(dir, f)); os.IsNotExist(err) {
			t.Errorf("expected %s to be created in minimal mode", f)
		}
	}
}

func TestMinimalNoGithubDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "--minimal", "--yes", "-target", dir)
	if err != nil {
		t.Fatalf("init --minimal returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".github")); err == nil {
		t.Error("expected .github/ to NOT exist in minimal mode")
	}
}

func TestMinimalNoChecks(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "--minimal", "--yes", "-target", dir)
	if err != nil {
		t.Fatalf("init --minimal returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "checks")); err == nil {
		t.Error("expected checks/ to NOT exist in minimal mode")
	}
}

func TestMinimalNoSkills(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "--minimal", "--yes", "-target", dir)
	if err != nil {
		t.Fatalf("init --minimal returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".mill", "skills")); err == nil {
		t.Error("expected .mill/skills/ to NOT exist in minimal mode")
	}
}

func TestMinimalRuntimeDirsExist(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "--minimal", "--yes", "-target", dir)
	if err != nil {
		t.Fatalf("init --minimal returned error: %v", err)
	}

	for _, sub := range []string{"ledger", "worktrees", "phases"} {
		p := filepath.Join(dir, ".mill", sub)
		if info, err := os.Stat(p); err != nil || !info.IsDir() {
			t.Errorf("expected .mill/%s/ directory to exist in minimal mode", sub)
		}
	}
}

func TestMinimalSkipsDetection(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Create go.mod — detection would find GoModule, but minimal should use generic.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: outBuf, Err: errBuf, In: strings.NewReader("")}

	err := app.Run("init", "--minimal", "--yes", "-target", dir)
	if err != nil {
		t.Fatalf("init --minimal returned error: %v", err)
	}

	// Verify mill.yml has type: generic (not go).
	content, err := os.ReadFile(filepath.Join(dir, "mill.yml"))
	if err != nil {
		t.Fatalf("expected mill.yml: %v", err)
	}
	if !strings.Contains(string(content), "type: generic") {
		t.Errorf("expected mill.yml type: generic in minimal mode, got: %s", string(content))
	}
}

func TestMinimalWithTypeGo(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: outBuf, Err: errBuf, In: strings.NewReader("")}

	err := app.Run("init", "--minimal", "--yes", "-type", "go", "-target", dir)
	if err != nil {
		t.Fatalf("init --minimal --type go returned error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "mill.yml"))
	if err != nil {
		t.Fatalf("expected mill.yml: %v", err)
	}
	if !strings.Contains(string(content), "type: go") {
		t.Errorf("expected mill.yml type: go, got: %s", string(content))
	}
}

// --- parseProjectType tests (#17) ---

func TestParseProjectTypeGo(t *testing.T) {
	pt, err := parseProjectType("go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pt != ProjectGoModule {
		t.Errorf("expected ProjectGoModule, got %v", pt)
	}
}

func TestParseProjectTypeJS(t *testing.T) {
	pt, err := parseProjectType("js")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pt != ProjectJSProject {
		t.Errorf("expected ProjectJSProject, got %v", pt)
	}
}

func TestParseProjectTypeMonoRepo(t *testing.T) {
	pt, err := parseProjectType("monorepo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pt != ProjectMonoRepo {
		t.Errorf("expected ProjectMonoRepo, got %v", pt)
	}
}

func TestParseProjectTypeGeneric(t *testing.T) {
	pt, err := parseProjectType("generic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pt != ProjectGeneric {
		t.Errorf("expected ProjectGeneric, got %v", pt)
	}
}

func TestParseProjectTypeEmpty(t *testing.T) {
	pt, err := parseProjectType("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pt != ProjectGeneric {
		t.Errorf("expected ProjectGeneric for empty string, got %v", pt)
	}
}

func TestParseProjectTypeCaseInsensitive(t *testing.T) {
	pt, err := parseProjectType("GO")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pt != ProjectGoModule {
		t.Errorf("expected ProjectGoModule for 'GO', got %v", pt)
	}
}

func TestParseProjectTypeUnknown(t *testing.T) {
	_, err := parseProjectType("foobar")
	if err == nil {
		t.Error("expected error for unknown project type")
	}
	if !strings.Contains(err.Error(), "unknown project type") {
		t.Errorf("expected 'unknown project type' in error, got: %v", err)
	}
}

// --- Integration tests: full runInit paths ---

func TestInitDetectsGoModule(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: outBuf, Err: errBuf, In: strings.NewReader("")}

	err := app.Run("init", "-yes", "-target", dir)
	if err != nil {
		t.Fatalf("init returned error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "mill.yml"))
	if err != nil {
		t.Fatalf("expected mill.yml: %v", err)
	}
	if !strings.Contains(string(content), "type: go") {
		t.Errorf("expected mill.yml type: go for GoModule, got: %s", string(content))
	}
}

func TestInitDetectsGeneric(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	errBuf := new(bytes.Buffer)
	outBuf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: outBuf, Err: errBuf, In: strings.NewReader("")}

	err := app.Run("init", "-yes", "-target", dir)
	if err != nil {
		t.Fatalf("init returned error: %v", err)
	}

	// Warning should be printed to stderr.
	if !strings.Contains(errBuf.String(), "Warning") {
		t.Errorf("expected warning for generic project, got stderr: %s", errBuf.String())
	}

	content, err := os.ReadFile(filepath.Join(dir, "mill.yml"))
	if err != nil {
		t.Fatalf("expected mill.yml: %v", err)
	}
	if !strings.Contains(string(content), "type: generic") {
		t.Errorf("expected mill.yml type: generic, got: %s", string(content))
	}
}

func TestInitPreservesExistingDirs(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Pre-create .github/ and .claude/ with files.
	githubDir := filepath.Join(dir, ".github")
	if err := os.MkdirAll(filepath.Join(githubDir, "ISSUE_TEMPLATE"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(githubDir, "ISSUE_TEMPLATE", "bug.md"), []byte("custom"), 0o644); err != nil {
		t.Fatal(err)
	}
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte("custom"), 0o644); err != nil {
		t.Fatal(err)
	}

	errBuf := new(bytes.Buffer)
	outBuf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: outBuf, Err: errBuf, In: strings.NewReader("")}

	err := app.Run("init", "-yes", "-target", dir)
	if err != nil {
		t.Fatalf("init returned error: %v", err)
	}

	// .github/ preserved.
	content, err := os.ReadFile(filepath.Join(githubDir, "ISSUE_TEMPLATE", "bug.md"))
	if err != nil {
		t.Fatalf("expected bug.md to still exist: %v", err)
	}
	if string(content) != "custom" {
		t.Errorf("expected custom bug.md to be preserved, got: %q", string(content))
	}

	// .claude/ preserved.
	content, err = os.ReadFile(filepath.Join(claudeDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("expected CLAUDE.md to still exist: %v", err)
	}
	if string(content) != "custom" {
		t.Errorf("expected custom CLAUDE.md to be preserved, got: %q", string(content))
	}

	// Other scaffold files created (e.g., .omp/).
	if _, err := os.Stat(filepath.Join(dir, ".omp", "AGENTS.md")); os.IsNotExist(err) {
		t.Error("expected .omp/AGENTS.md to be created")
	}

	// No collision prompt for preserved dirs.
	if strings.Contains(outBuf.String(), "Overwrite?") {
		t.Error("expected no 'Overwrite?' prompt when dirs are preserved")
	}
}

func TestInitForceOverridesPreservation(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Pre-create .github/ with a custom file.
	githubDir := filepath.Join(dir, ".github")
	if err := os.MkdirAll(filepath.Join(githubDir, "ISSUE_TEMPLATE"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(githubDir, "ISSUE_TEMPLATE", "bug.md"), []byte("custom"), 0o644); err != nil {
		t.Fatal(err)
	}

	errBuf := new(bytes.Buffer)
	outBuf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: outBuf, Err: errBuf, In: strings.NewReader("")}

	err := app.Run("init", "--force", "--yes", "-target", dir)
	if err != nil {
		t.Fatalf("init --force --yes returned error: %v", err)
	}

	// .github/ should have been overwritten.
	content, err := os.ReadFile(filepath.Join(githubDir, "ISSUE_TEMPLATE", "bug.md"))
	if err != nil {
		t.Fatalf("expected bug.md to exist after force: %v", err)
	}
	if string(content) == "custom" {
		t.Errorf("expected custom bug.md to be overwritten with --force")
	}
}

func TestInitMinimalFlag(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	errBuf := new(bytes.Buffer)
	outBuf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: outBuf, Err: errBuf, In: strings.NewReader("")}

	err := app.Run("init", "--minimal", "--yes", "-target", dir)
	if err != nil {
		t.Fatalf("init --minimal returned error: %v", err)
	}

	// No warning about missing go.mod in minimal mode.
	if strings.Contains(errBuf.String(), "no go.mod") {
		t.Error("expected no 'no go.mod' warning in minimal mode")
	}

	// Only essential files created.
	for _, f := range []string{"mill.yml", ".mill/COMMON.md", ".mill/roles/staff/ROLE.md", ".mill/roles/pm/ROLE.md"} {
		if _, err := os.Stat(filepath.Join(dir, f)); os.IsNotExist(err) {
			t.Errorf("expected %s in minimal mode", f)
		}
	}

	// Non-essential files NOT created.
	for _, f := range []string{".github", ".claude", ".omp", "checks", ".mill/skills"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			t.Errorf("expected %s to NOT exist in minimal mode", f)
		}
	}

	// Runtime dirs exist.
	for _, sub := range []string{"ledger", "worktrees", "phases", "artifacts", "memory"} {
		p := filepath.Join(dir, ".mill", sub)
		if info, err := os.Stat(p); err != nil || !info.IsDir() {
			t.Errorf("expected .mill/%s/ directory in minimal mode", sub)
		}
	}
}

func TestInitScaffoldsRecursionConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "-yes", "-target", dir)
	if err != nil {
		t.Fatalf("init returned error: %v", err)
	}

	// Load and validate the generated mill.yml using the real loader
	millYMLPath := filepath.Join(dir, "mill.yml")
	myml, err := config.LoadAndValidate(millYMLPath)
	if err != nil {
		t.Fatalf("LoadAndValidate returned error: %v", err)
	}

	// Assert recursion is fully configured (not zero)
	if myml.Recursion.IsZero() {
		t.Errorf("expected Recursion.IsZero() to be false, got true")
	}

	// Assert View is "result"
	if myml.Recursion.View != "result" {
		t.Errorf("expected Recursion.View == \"result\", got %q", myml.Recursion.View)
	}

	// Assert MaxDepth is 4
	if myml.Recursion.MaxDepth != 4 {
		t.Errorf("expected Recursion.MaxDepth == 4, got %d", myml.Recursion.MaxDepth)
	}

	// Assert Models["pro"] is set correctly
	if myml.Recursion.Models["pro"] != "deepseek/deepseek-v4-pro" {
		t.Errorf("expected Models[\"pro\"] == \"deepseek/deepseek-v4-pro\", got %q", myml.Recursion.Models["pro"])
	}

	// Assert Models["cheap"] is set correctly
	if myml.Recursion.Models["cheap"] != "deepseek/deepseek-v4-flash" {
		t.Errorf("expected Models[\"cheap\"] == \"deepseek/deepseek-v4-flash\", got %q", myml.Recursion.Models["cheap"])
	}
}
