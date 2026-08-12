package recursion

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/antonygiomarxdev/mill/internal/domain"
)

func TestNewTreeDefaultsMaxDepth(t *testing.T) {
	tr := NewTree("staff", 0)
	if tr.MaxDepth != defaultMaxDepth {
		t.Fatalf("expected defaultMaxDepth %d, got %d", defaultMaxDepth, tr.MaxDepth)
	}
	if tr.Root == nil || tr.Root.Role != "staff" || tr.Root.Depth != 0 {
		t.Fatalf("unexpected root: %+v", tr.Root)
	}
}

func TestNewTreeKeepsExplicitMaxDepth(t *testing.T) {
	tr := NewTree("staff", 7)
	if tr.MaxDepth != 7 {
		t.Fatalf("expected 7, got %d", tr.MaxDepth)
	}
}

func TestParseRejectsMissingRoot(t *testing.T) {
	_, err := Parse([]byte(`{"max_depth":4}`))
	if err == nil {
		t.Fatal("expected error for missing root")
	}
}

func TestParseNormalizesMaxDepth(t *testing.T) {
	tr, err := Parse([]byte(`{"root":{"role":"a","depth":0},"max_depth":0}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if tr.MaxDepth != defaultMaxDepth {
		t.Fatalf("expected normalized maxDepth %d, got %d", defaultMaxDepth, tr.MaxDepth)
	}
}

func TestParseBadJSON(t *testing.T) {
	_, err := Parse([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".mill", "state", "recursion.json")

	leaf := &TreeNode{
		Role:         "qa-docs",
		Depth:        4,
		ArtifactPath: "artifacts/qa-docs.md",
		ModelTier:    "free→paid",
		Verdict:      domain.VerdictApproved,
		Duration:     5 * time.Second,
	}
	root := &TreeNode{
		Role:     "staff",
		Depth:    0,
		Children: []*TreeNode{{Role: "pm", Depth: 1, Children: []*TreeNode{leaf}}},
	}
	t1 := &DelegationTree{Root: root, MaxDepth: 4}
	if err := t1.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	t2, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if t2.MaxDepth != 4 {
		t.Errorf("expected maxDepth 4, got %d", t2.MaxDepth)
	}
	if t2.Root.Role != "staff" {
		t.Errorf("expected root staff, got %q", t2.Root.Role)
	}
	if got := t2.Find("qa-docs"); got == nil || got.ArtifactPath != "artifacts/qa-docs.md" {
		t.Errorf("expected qa-docs node with artifact, got %+v", got)
	}
	if got := t2.Find("pm"); got == nil || len(got.Children) != 1 {
		t.Errorf("expected pm with 1 child, got %+v", got)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestSaveCreatesParentDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "c", "recursion.json")
	if err := (&DelegationTree{Root: &TreeNode{Role: "root", Depth: 0}}).Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
}

func TestAddChildAndFind(t *testing.T) {
	tr := NewTree("staff", 4)
	child := &TreeNode{Role: "pm", Depth: 1}
	tr.AddChild("staff", child)
	if got := tr.Find("pm"); got != child {
		t.Fatalf("Find(pm) returned %p, want %p", got, child)
	}
	// Adding under a non-existent parent falls back to root.
	grand := &TreeNode{Role: "qa-docs", Depth: 2}
	tr.AddChild("nope", grand)
	if got := tr.Find("qa-docs"); got != grand {
		t.Fatalf("Find(qa-docs) = %p, want %p", got, grand)
	}
	if len(tr.Root.Children) != 2 {
		t.Fatalf("expected 2 root children, got %d", len(tr.Root.Children))
	}
}

func TestAddChildToGrandchildParent(t *testing.T) {
	tr := NewTree("staff", 4)
	tr.AddChild("staff", &TreeNode{Role: "pm", Depth: 1})
	tr.AddChild("pm", &TreeNode{Role: "ux", Depth: 2})
	if got := tr.Find("ux"); got == nil {
		t.Fatal("expected ux node under pm")
	}
	if len(tr.Root.Children[0].Children) != 1 {
		t.Fatalf("expected pm to have 1 child, got %d", len(tr.Root.Children[0].Children))
	}
}

func TestHeight(t *testing.T) {
	tr := NewTree("staff", 4)
	tr.AddChild("staff", &TreeNode{Role: "pm", Depth: 1, Children: []*TreeNode{
		{Role: "ux", Depth: 2, Children: []*TreeNode{{Role: "qa", Depth: 3}}},
	}})
	// shallow branch
	tr.AddChild("staff", &TreeNode{Role: "arch", Depth: 1})

	if got := tr.Height(); got != 3 {
		t.Fatalf("expected height 3, got %d", got)
	}
}

func TestHeightEmpty(t *testing.T) {
	if (&DelegationTree{}).Height() != 0 {
		t.Fatal("expected 0 height for empty tree")
	}
}

func TestPhaseOrder(t *testing.T) {
	want := []ArtifactPhase{PhaseFRD, PhaseSpec, PhaseTasks, PhaseImplementation}
	if len(PhaseOrder) != len(want) {
		t.Fatalf("len(PhaseOrder)=%d, want %d", len(PhaseOrder), len(want))
	}
	for i, p := range PhaseOrder {
		if p != want[i] {
			t.Errorf("PhaseOrder[%d]=%q, want %q", i, p, want[i])
		}
	}
}
