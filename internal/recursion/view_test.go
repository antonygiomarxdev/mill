package recursion

import (
	"strings"
	"testing"
	"time"

	"github.com/antonygiomarxdev/mill/internal/domain"
)

func TestRenderNilAndEmpty(t *testing.T) {
	r := ViewRenderer{View: ViewTree}
	if got := r.Render(nil); got != "" {
		t.Errorf("nil tree: expected empty, got %q", got)
	}
	if got := r.Render(&DelegationTree{}); got != "" {
		t.Errorf("empty tree: expected empty, got %q", got)
	}
}

func TestRenderFinalDefaultsToFinalView(t *testing.T) {
	r := ViewRenderer{} // zero View → default branch (final)
	tr := &DelegationTree{Root: &TreeNode{
		Role:     "qa-docs",
		Depth:    3,
		Verdict:  domain.VerdictApproved,
		Duration: 2 * time.Second,
	}}
	got := r.Render(tr)
	if !strings.Contains(got, "verdict: approved") {
		t.Errorf("final render: missing verdict, got %q", got)
	}
	if !strings.Contains(got, "role: qa-docs") {
		t.Errorf("final render: missing role, got %q", got)
	}
}

func TestRenderFinalDeepestLeaf(t *testing.T) {
	leaf := &TreeNode{Role: "qa", Depth: 4, Verdict: domain.VerdictChanges, ArtifactPath: "a/qa.md", Duration: 3 * time.Second}
	root := &TreeNode{Role: "staff", Depth: 0, Children: []*TreeNode{
		{Role: "pm", Depth: 1, Children: []*TreeNode{{Role: "arch", Depth: 2, Children: []*TreeNode{leaf}}}},
		// competing deeper-ish branch that is shallower
		{Role: "rev", Depth: 1, Children: []*TreeNode{{Role: "x", Depth: 2}}},
	}}
	tr := &DelegationTree{Root: root}
	got := (ViewRenderer{View: ViewFinal}).Render(tr)
	if !strings.Contains(got, "role: qa") {
		t.Errorf("expected deepest leaf qa in final, got %q", got)
	}
	if strings.Contains(got, "role: staff") || strings.Contains(got, "role: pm") {
		t.Errorf("final should not include non-leaf roles, got %q", got)
	}
}

func TestRenderTreeIncludesAllFields(t *testing.T) {
	leaf := &TreeNode{
		Role:         "qa-docs",
		Depth:        4,
		ArtifactPath: "artifacts/qa.md",
		ModelTier:    "free→paid",
		Verdict:      domain.VerdictApproved,
		Duration:     5 * time.Second,
	}
	root := &TreeNode{
		Role:      "staff",
		Depth:     0,
		ModelTier: "pro",
		Children:  []*TreeNode{{Role: "pm", Depth: 1, Children: []*TreeNode{leaf}}},
	}
	tr := &DelegationTree{Root: root}
	out := (ViewRenderer{View: ViewTree}).Render(tr)

	for _, want := range []string{"role: staff", "role: pm", "role: qa-docs"} {
		if !strings.Contains(out, want) {
			t.Errorf("tree render missing %q\n%s", want, out)
		}
	}
	// per-node fields present on the leaf line
	leafLine := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "role: qa-docs") {
			leafLine = line
		}
	}
	if leafLine == "" {
		t.Fatal("leaf line not found")
	}
	for _, want := range []string{"artifact: artifacts/qa.md", "model_tier: free→paid",
		"verdict: approved", "duration: 5s"} {
		if !strings.Contains(leafLine, want) {
			t.Errorf("leaf line missing %q: %q", want, leafLine)
		}
	}
	// indentation reflects depth
	staffIdx := strings.Index(out, "● role: staff")
	qaidx := strings.Index(out, "● role: qa-docs")
	if staffIdx >= 0 && qaidx >= 0 && qaidx <= staffIdx {
		t.Errorf("qa-docs should be indented deeper than staff")
	}
}

func TestRenderTreeDepthLines(t *testing.T) {
	leaf := &TreeNode{Role: "leaf", Depth: 3}
	root := &TreeNode{Role: "root", Depth: 0, Children: []*TreeNode{
		{Role: "mid", Depth: 1, Children: []*TreeNode{{Role: "node", Depth: 2, Children: []*TreeNode{leaf}}}},
	}}
	out := (ViewRenderer{View: ViewTree}).Render(&DelegationTree{Root: root})
	lines := strings.Split(out, "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %q", len(lines), out)
	}
}
