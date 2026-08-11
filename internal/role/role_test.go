package role

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFrontmatterStaff(t *testing.T) {
	fm, err := ParseFrontmatter("../../.mill/roles/staff/ROLE.md")
	if err != nil {
		t.Fatalf("parse staff: %v", err)
	}

	if fm.Role != "staff" {
		t.Errorf("role: expected staff, got %s", fm.Role)
	}
	if fm.Model != "pro" {
		t.Errorf("model: expected pro, got %s", fm.Model)
	}
	if fm.ReviewedBy != "cto" {
		t.Errorf("reviewed_by: expected cto, got %s", fm.ReviewedBy)
	}

	expectedDelegates := []string{"pm", "architect", "reviewer", "qa-docs"}
	if len(fm.DelegatesTo) != len(expectedDelegates) {
		t.Fatalf("delegates_to: expected %d items, got %d: %v", len(expectedDelegates), len(fm.DelegatesTo), fm.DelegatesTo)
	}
	for i, v := range expectedDelegates {
		if fm.DelegatesTo[i] != v {
			t.Errorf("delegates_to[%d]: expected %s, got %s", i, v, fm.DelegatesTo[i])
		}
	}

	if len(fm.Skills) == 0 {
		t.Error("skills: expected non-empty")
	}
}

func TestParseFrontmatterSrDev(t *testing.T) {
	fm, err := ParseFrontmatter("../../.mill/roles/sr-dev-be/ROLE.md")
	if err != nil {
		t.Fatalf("parse sr-dev-be: %v", err)
	}

	if fm.Role != "sr-dev-be" {
		t.Errorf("role: expected sr-dev-be, got %s", fm.Role)
	}
	if fm.ReviewedBy != "tech-lead" {
		t.Errorf("reviewed_by: expected tech-lead, got %s", fm.ReviewedBy)
	}

	expectedDelegates := []string{"qa-docs"}
	if len(fm.DelegatesTo) != len(expectedDelegates) {
		t.Fatalf("delegates_to: expected %d items, got %d", len(expectedDelegates), len(fm.DelegatesTo))
	}
	if fm.DelegatesTo[0] != "qa-docs" {
		t.Errorf("delegates_to[0]: expected qa-docs, got %s", fm.DelegatesTo[0])
	}
}

func TestParseFrontmatterString(t *testing.T) {
	content := `---
role: test
model: free
agent: cavecrew-builder
delegates_to:
  - foo
  - bar
---`
	fm, err := ParseFrontmatterString(content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if fm.Role != "test" {
		t.Errorf("role: expected test, got %s", fm.Role)
	}
	if fm.Agent != "cavecrew-builder" {
		t.Errorf("agent: expected cavecrew-builder, got %s", fm.Agent)
	}
	if len(fm.DelegatesTo) != 2 {
		t.Fatalf("delegates_to: expected 2, got %d", len(fm.DelegatesTo))
	}
	if fm.DelegatesTo[0] != "foo" || fm.DelegatesTo[1] != "bar" {
		t.Errorf("delegates_to: expected [foo, bar], got %v", fm.DelegatesTo)
	}
}

func TestParseFrontmatterNoFrontmatter(t *testing.T) {
	fm, err := ParseFrontmatterString("# Just a markdown file\n\nNo frontmatter here.")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if fm.Role != "" {
		t.Errorf("expected empty role, got %s", fm.Role)
	}
}

func TestParseFrontmatterEmptyDelegates(t *testing.T) {
	content := `---
role: lone
model: pro
delegates_to:
---
# No delegates`
	fm, err := ParseFrontmatterString(content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if fm.Role != "lone" {
		t.Errorf("role: expected lone, got %s", fm.Role)
	}
	if len(fm.DelegatesTo) != 0 {
		t.Errorf("delegates_to: expected empty, got %v", fm.DelegatesTo)
	}
}

func TestLoadRole(t *testing.T) {
	// Save cwd and chdir to repo root so role paths resolve
	orig, _ := os.Getwd()
	repoRoot := findRepoRoot(t)
	os.Chdir(repoRoot)
	defer os.Chdir(orig)

	content, err := Load("staff")
	if err != nil {
		t.Fatalf("load staff: %v", err)
	}
	if content == "" {
		t.Fatal("expected non-empty content")
	}
	// Should contain COMMON.md content
	if len(content) < 100 {
		t.Errorf("content too short: %d bytes", len(content))
	}
}

func TestLoadRoleNotFound(t *testing.T) {
	orig, _ := os.Getwd()
	repoRoot := findRepoRoot(t)
	os.Chdir(repoRoot)
	defer os.Chdir(orig)

	_, err := Load("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent role")
	}
}

func TestParseAllRoleFiles(t *testing.T) {
	roleDirs := []string{
		"architect", "pm", "qa-docs", "reviewer",
		"sr-dev-be", "sr-dev-data", "sr-dev-fe",
		"staff", "tech-lead", "ui-designer", "ux-designer",
	}

	if len(roleDirs) != 11 {
		t.Fatalf("expected 11 roles, got %d", len(roleDirs))
	}

	for _, name := range roleDirs {
		path := filepath.Join("../../.mill/roles", name, "ROLE.md")
		fm, err := ParseFrontmatter(path)
		if err != nil {
			t.Errorf("parse %s: %v", name, err)
			continue
		}

		if fm.Role == "" {
			t.Errorf("%s: role is empty", name)
		}
		if fm.Role != name {
			t.Errorf("%s: role mismatch: expected %s, got %s", name, name, fm.Role)
		}

		// Verify allowed_files parsed correctly for roles that have entries
		switch name {
		case "staff":
			if len(fm.AllowedFiles) != 1 {
				t.Errorf("%s: expected 1 allowed_file, got %d", name, len(fm.AllowedFiles))
			}
		case "sr-dev-be", "sr-dev-data", "sr-dev-fe":
			if len(fm.AllowedFiles) != 5 {
				t.Errorf("%s: expected 5 allowed_files, got %d: %v", name, len(fm.AllowedFiles), fm.AllowedFiles)
			}
			if len(fm.ForbiddenPatterns) != 1 || fm.ForbiddenPatterns[0] != "ROLE.md" {
				t.Errorf("%s: expected forbidden_patterns [ROLE.md], got %v", name, fm.ForbiddenPatterns)
			}
		case "architect":
			if len(fm.AllowedFiles) != 3 {
				t.Errorf("%s: expected 3 allowed_files, got %d", name, len(fm.AllowedFiles))
			}
		case "pm", "reviewer":
			if len(fm.AllowedFiles) != 1 {
				t.Errorf("%s: expected 1 allowed_file, got %d", name, len(fm.AllowedFiles))
			}
		case "qa-docs", "tech-lead", "ui-designer", "ux-designer":
			if len(fm.AllowedFiles) != 2 {
				t.Errorf("%s: expected 2 allowed_files, got %d", name, len(fm.AllowedFiles))
			}
		}
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}
		dir = parent
	}
}
