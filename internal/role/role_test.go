package role

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFrontmatterStaff(t *testing.T) {
	fm, err := ParseFrontmatter("../../roles/staff/ROLE.md")
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
	fm, err := ParseFrontmatter("../../roles/sr-dev-be/ROLE.md")
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
delegates_to:
  - foo
  - bar
---
# Rest of file`
	fm, err := ParseFrontmatterString(content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if fm.Role != "test" {
		t.Errorf("role: expected test, got %s", fm.Role)
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
