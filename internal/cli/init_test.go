package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesMillYAML(t *testing.T) {
	dir := t.TempDir()
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
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "-yes", "-target", dir)
	if err != nil {
		t.Fatalf("init returned error: %v", err)
	}

	for _, dirName := range []string{"roles", "checks", "skills", "docs"} {
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

func TestInitCopiesRoleFiles(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "-yes", "-target", dir)
	if err != nil {
		t.Fatalf("init returned error: %v", err)
	}

	roleFile := filepath.Join(dir, "roles", "sr-dev-be", "ROLE.md")
	if _, err := os.Stat(roleFile); os.IsNotExist(err) {
		t.Error("expected roles/sr-dev-be/ROLE.md to be created")
	}

	commonFile := filepath.Join(dir, "roles", "COMMON.md")
	if _, err := os.Stat(commonFile); os.IsNotExist(err) {
		t.Error("expected roles/COMMON.md to be created")
	}
}

func TestInitCopiesCheckFiles(t *testing.T) {
	dir := t.TempDir()
	buf := new(bytes.Buffer)
	app := &App{MillDir: dir, Out: buf, Err: buf, In: strings.NewReader("")}

	err := app.Run("init", "-yes", "-target", dir)
	if err != nil {
		t.Fatalf("init returned error: %v", err)
	}

	for _, file := range []string{"checks/pre-commit", "checks/pre-push", "checks/common.sh"} {
		if _, err := os.Stat(filepath.Join(dir, file)); os.IsNotExist(err) {
			t.Errorf("expected %s to be created", file)
		}
	}
}

func TestInitWithCustomFlags(t *testing.T) {
	dir := t.TempDir()
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
	if _, err := os.Stat(filepath.Join(dir, "roles")); os.IsNotExist(err) {
		t.Error("expected roles/ directory to be created")
	}
}

func TestInitPrintOutput(t *testing.T) {
	dir := t.TempDir()
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
