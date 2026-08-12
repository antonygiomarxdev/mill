package recursion

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/antonygiomarxdev/mill/internal/domain"
)

type testRoleDef struct {
	model     string
	delegates []string
}

// writeRole writes a ROLE.md frontmatter file for a role under rolesRoot.
func writeRole(t *testing.T, rolesRoot, name string, r testRoleDef) {
	t.Helper()
	dir := filepath.Join(rolesRoot, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("role: " + name + "\n")
	if r.model != "" {
		b.WriteString("model: " + r.model + "\n")
	}
	if len(r.delegates) == 0 {
		b.WriteString("delegates_to:\n")
	} else {
		b.WriteString("delegates_to:\n")
		for _, d := range r.delegates {
			b.WriteString("  - " + d + "\n")
		}
	}
	b.WriteString("---\n")
	if err := os.WriteFile(filepath.Join(dir, "ROLE.md"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newTestRolesRoot(t *testing.T, roles map[string]testRoleDef) string {
	root := filepath.Join(t.TempDir(), ".mill", "roles")
	for name, r := range roles {
		writeRole(t, root, name, r)
	}
	return root
}

// newEngine builds a Delegator with injected fakes capturing side effects.
type engineRecorder struct {
	handoffCalls    []handoffCall
	parentState     []parentStateCall
	createWorktrees []worktreeCall
	binaryCopies    []string
}

type handoffCall struct {
	worktree string
	phase    string
}
type parentStateCall struct {
	parentWT, childRole, childWT string
}
type worktreeCall struct {
	parentWT, childRole, model string
}

func (rec *engineRecorder) newDelegator(rolesRoot string, maxDepth int) *Delegator {
	models := map[string]string{
		"pro":   "deepseek-v4-pro",
		"paid":  "deepseek-v4-pro",
		"cheap": "laguna-s-2.1-free",
		"free":  "laguna-s-2.1-free",
	}
	return &Delegator{
		RolesRoot: rolesRoot,
		Cost:      &CostResolver{Models: models},
		MaxDepth:  maxDepth,
		CreateWorktree: func(parentWT, childRole, model string) (string, error) {
			wt := filepath.Join(parentWT, ".mill", "child", childRole)
			rec.createWorktrees = append(rec.createWorktrees, worktreeCall{parentWT, childRole, model})
			return wt, nil
		},
		CopyBinary: func(worktree string) domain.FailureClass {
			rec.binaryCopies = append(rec.binaryCopies, worktree)
			return domain.CLASS_OK
		},
		Handoff: func(worktree, phase string) error {
			rec.handoffCalls = append(rec.handoffCalls, handoffCall{worktree, phase})
			return nil
		},
		WriteParentState: func(parentWT, childRole, childWT string) error {
			rec.parentState = append(rec.parentState, parentStateCall{parentWT, childRole, childWT})
			return nil
		},
	}
}

func TestDelegateLeafTerminates(t *testing.T) {
	root := newTestRolesRoot(t, map[string]testRoleDef{
		"qa": {model: "free→paid"}, // leaf: no delegates
	})
	rec := &engineRecorder{}
	d := rec.newDelegator(root, 4)

	res, err := d.Delegate("qa", "parent-wt", "artifacts/qa.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Failure != domain.CLASS_OK {
		t.Fatalf("expected CLASS_OK, got %s", res.Failure)
	}
	if len(d.Tree.Root.Children) != 0 {
		t.Errorf("leaf should produce no children, got %d", len(d.Tree.Root.Children))
	}
	if d.Tree.Root.ModelTier != "free→paid" {
		t.Errorf("expected model tier free→paid, got %q", d.Tree.Root.ModelTier)
	}
	// Handoff still runs for the leaf node.
	if len(rec.handoffCalls) != 4 {
		t.Errorf("expected 4 handoff calls, got %d", len(rec.handoffCalls))
	}
	// No children created.
	if len(rec.createWorktrees) != 0 {
		t.Errorf("expected no worktree creations, got %d", len(rec.createWorktrees))
	}
}

func TestDelegateChainBuildsTree(t *testing.T) {
	root := newTestRolesRoot(t, map[string]testRoleDef{
		"pm": {model: "pro", delegates: []string{"ux"}},
		"ux": {model: "free→paid", delegates: []string{"ui"}},
		"ui": {model: "pro", delegates: []string{"qa"}},
		"qa": {model: "free→paid"}, // leaf
	})
	rec := &engineRecorder{}
	d := rec.newDelegator(root, 4)

	res, err := d.Delegate("pm", "parent-wt", "artifacts/pm.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Failure != domain.CLASS_OK {
		t.Fatalf("expected CLASS_OK, got %s", res.Failure)
	}

	// Tree: pm → ux → ui → qa (depth 3 for qa).
	pm := d.Tree.Find("pm")
	if pm == nil || pm.Depth != 0 {
		t.Fatalf("unexpected pm node: %+v", pm)
	}
	ux := d.Tree.Find("ux")
	if ux == nil || ux.Depth != 1 || ux.ModelTier != "free→paid" {
		t.Fatalf("unexpected ux node: %+v", ux)
	}
	ui := d.Tree.Find("ui")
	if ui == nil || ui.Depth != 2 || ui.ModelTier != "pro" {
		t.Fatalf("unexpected ui node: %+v", ui)
	}
	qa := d.Tree.Find("qa")
	if qa == nil || qa.Depth != 3 || len(qa.Children) != 0 {
		t.Fatalf("unexpected qa node: %+v", qa)
	}

	// Each non-leaf delegates to its single subordinate: 3 worktrees (ux, ui, qa).
	if len(rec.createWorktrees) != 3 {
		t.Errorf("expected 3 worktree creations, got %d", len(rec.createWorktrees))
	}
	if len(rec.parentState) != 3 {
		t.Errorf("expected 3 parent-state writes, got %d", len(rec.parentState))
	}
	// Binary copied once per created child worktree.
	if len(rec.binaryCopies) != 3 {
		t.Errorf("expected 3 binary copies, got %d", len(rec.binaryCopies))
	}
	// Handoff runs for each of the 4 nodes (4 phases each = 16).
	if len(rec.handoffCalls) != 16 {
		t.Errorf("expected 16 handoff calls, got %d", len(rec.handoffCalls))
	}
	// Child model resolution: ux (free→paid) → cheap; ui (pro) → pro.
	if rec.createWorktrees[0].model != "laguna-s-2.1-free" {
		t.Errorf("ux model = %q, want laguna-s-2.1-free", rec.createWorktrees[0].model)
	}
	if rec.createWorktrees[1].model != "deepseek-v4-pro" {
		t.Errorf("ui model = %q, want deepseek-v4-pro", rec.createWorktrees[1].model)
	}
}

func TestDelegateParentStateRecordsChildPath(t *testing.T) {
	root := newTestRolesRoot(t, map[string]testRoleDef{
		"staff": {model: "pro", delegates: []string{"pm"}},
		"pm":    {model: "pro"},
	})
	rec := &engineRecorder{}
	d := rec.newDelegator(root, 4)

	_, err := d.Delegate("staff", "parent-wt", "artifacts/frd.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.parentState) != 1 {
		t.Fatalf("expected 1 parent-state write, got %d", len(rec.parentState))
	}
	ps := rec.parentState[0]
	if ps.parentWT != "parent-wt" || ps.childRole != "pm" {
		t.Errorf("parentState=%+v, want parent-wt/pm", ps)
	}
	wantChild := filepath.Join("parent-wt", ".mill", "child", "pm")
	if ps.childWT != wantChild {
		t.Errorf("childWT=%q, want %q", ps.childWT, wantChild)
	}
}

func TestDelegateCycleIsFatal(t *testing.T) {
	root := newTestRolesRoot(t, map[string]testRoleDef{
		"a": {model: "pro", delegates: []string{"b"}},
		"b": {model: "pro", delegates: []string{"a"}},
	})
	rec := &engineRecorder{}
	d := rec.newDelegator(root, 4)

	res, err := d.Delegate("a", "parent-wt", "artifacts/a.md")
	if err == nil {
		t.Fatal("expected cycle error")
	}
	var ce *CycleError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *CycleError, got %T: %v", err, err)
	}
	if ce.Role != "a" {
		t.Errorf("cycle role = %q, want %q", ce.Role, "a")
	}
	if res.Failure != domain.FATAL {
		t.Errorf("expected FATAL failure, got %s", res.Failure)
	}
	// Tree records up to the point of the cycle: a → b.
	b := d.Tree.Find("b")
	if b == nil {
		t.Fatal("expected b node to exist before cycle aborted")
	}
}

func TestDelegateSelfCycleIsFatal(t *testing.T) {
	root := newTestRolesRoot(t, map[string]testRoleDef{
		"loop": {model: "pro", delegates: []string{"loop"}},
	})
	rec := &engineRecorder{}
	d := rec.newDelegator(root, 4)

	res, err := d.Delegate("loop", "parent-wt", "artifacts/loop.md")
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if res.Failure != domain.FATAL {
		t.Errorf("expected FATAL, got %s", res.Failure)
	}
}

func TestDelegateDepthGuardStops(t *testing.T) {
	root := newTestRolesRoot(t, map[string]testRoleDef{
		"root": {model: "pro", delegates: []string{"a"}},
		"a":    {model: "pro", delegates: []string{"b"}},
		"b":    {model: "pro", delegates: []string{"c"}},
		"c":    {model: "pro", delegates: []string{"d"}},
		"d":    {model: "pro"}, // would extend chain but depth=2 stops at b
	})
	rec := &engineRecorder{}
	d := rec.newDelegator(root, 2)

	res, err := d.Delegate("root", "parent-wt", "artifacts/root.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Failure != domain.CLASS_OK {
		t.Errorf("expected CLASS_OK at depth limit, got %s", res.Failure)
	}
	// depth 2: root(0) → a(1) → b(2); b stops (depth>=max), so c/d never created.
	if d.Tree.Height() != 2 {
		t.Errorf("expected tree height 2, got %d", d.Tree.Height())
	}
	b := d.Tree.Find("b")
	if b == nil {
		t.Fatal("expected b node")
	}
	if len(b.Children) != 0 {
		t.Errorf("b should be depth-stopped with no children, got %d", len(b.Children))
	}
	if d.Tree.Find("c") != nil || d.Tree.Find("d") != nil {
		t.Error("expected c and d to not be delegated (depth guard)")
	}
}

func TestDelegateDefaultMaxDepthWhenZero(t *testing.T) {
	root := newTestRolesRoot(t, map[string]testRoleDef{
		"root": {model: "pro", delegates: []string{"leaf"}},
		"leaf": {model: "free→paid"},
	})
	rec := &engineRecorder{}
	d := rec.newDelegator(root, 0) // 0 → defaultMaxDepth (4)

	_, err := d.Delegate("root", "parent-wt", "artifacts/root.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Tree.MaxDepth != 4 {
		t.Errorf("expected normalized maxDepth 4, got %d", d.Tree.MaxDepth)
	}
}

func TestDelegateBranchAbortsOnMissingRole(t *testing.T) {
	root := newTestRolesRoot(t, map[string]testRoleDef{
		"root": {model: "pro", delegates: []string{"ghost"}}, // ghost ROLE.md missing
	})
	rec := &engineRecorder{}
	d := rec.newDelegator(root, 4)

	res, err := d.Delegate("root", "parent-wt", "artifacts/root.md")
	if err == nil {
		t.Fatal("expected error for missing child role")
	}
	if res.Failure != domain.EXECUTION_FAILURE {
		t.Errorf("expected EXECUTION_FAILURE, got %s", res.Failure)
	}
}

func TestDelegateBinaryFailurePropagates(t *testing.T) {
	root := newTestRolesRoot(t, map[string]testRoleDef{
		"root":  {model: "pro", delegates: []string{"child"}},
		"child": {model: "pro"},
	})
	rec := &engineRecorder{}
	d := rec.newDelegator(root, 4)
	d.CopyBinary = func(worktree string) domain.FailureClass {
		return domain.ENVIRONMENT_FAILURE
	}

	res, err := d.Delegate("root", "parent-wt", "artifacts/root.md")
	if err == nil {
		t.Fatal("expected error on binary failure")
	}
	if res.Failure != domain.ENVIRONMENT_FAILURE {
		t.Errorf("expected ENVIRONMENT_FAILURE, got %s", res.Failure)
	}
}

func TestDelegateHandoffOrder(t *testing.T) {
	root := newTestRolesRoot(t, map[string]testRoleDef{
		"leaf": {model: "pro"},
	})
	rec := &engineRecorder{}
	d := rec.newDelegator(root, 4)

	if _, err := d.Delegate("leaf", "wt", "a.md"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.handoffCalls) != 4 {
		t.Fatalf("expected 4 phases, got %d", len(rec.handoffCalls))
	}
	want := []string{"frd", "spec", "tasks", "implementation"}
	for i, c := range rec.handoffCalls {
		if c.phase != want[i] {
			t.Errorf("handoff[%d].phase=%q, want %q", i, c.phase, want[i])
		}
		if c.worktree != "wt" {
			t.Errorf("handoff[%d].worktree=%q, want wt", i, c.worktree)
		}
	}
}

func TestDelegateHandoffFailureAborts(t *testing.T) {
	root := newTestRolesRoot(t, map[string]testRoleDef{
		"leaf": {model: "pro"},
	})
	rec := &engineRecorder{}
	d := rec.newDelegator(root, 4)
	d.Handoff = func(worktree, phase string) error {
		if phase == "spec" {
			return errors.New("spec gate failed")
		}
		return nil
	}

	res, err := d.Delegate("leaf", "wt", "a.md")
	if err == nil {
		t.Fatal("expected handoff error")
	}
	if res.Failure != domain.CONTRACT_FAILURE {
		t.Errorf("expected CONTRACT_FAILURE, got %s", res.Failure)
	}
}

func TestDelegatePersistsTreeToStatePath(t *testing.T) {
	root := newTestRolesRoot(t, map[string]testRoleDef{
		"root": {model: "pro", delegates: []string{"leaf"}},
		"leaf": {model: "free→paid"},
	})
	rec := &engineRecorder{}
	d := rec.newDelegator(root, 4)
	statePath := filepath.Join(t.TempDir(), ".mill", "state", "recursion.json")
	d.StatePath = statePath

	if _, err := d.Delegate("root", "parent-wt", "artifacts/root.md"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := Load(statePath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Root.Role != "root" {
		t.Errorf("loaded root=%q, want root", loaded.Root.Role)
	}
	if loaded.Find("leaf") == nil {
		t.Error("expected leaf node persisted")
	}
}

func TestDelegateBranchingVisitsEachChild(t *testing.T) {
	root := newTestRolesRoot(t, map[string]testRoleDef{
		"staff": {model: "pro", delegates: []string{"pm", "arch"}},
		"pm":    {model: "pro"},
		"arch":  {model: "pro"},
	})
	rec := &engineRecorder{}
	d := rec.newDelegator(root, 4)

	res, err := d.Delegate("staff", "parent-wt", "artifacts/frd.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Failure != domain.CLASS_OK {
		t.Errorf("expected CLASS_OK, got %s", res.Failure)
	}
	if len(rec.createWorktrees) != 2 {
		t.Errorf("expected 2 worktree creations (pm + arch), got %d", len(rec.createWorktrees))
	}
	roles := map[string]bool{}
	for _, c := range rec.createWorktrees {
		roles[c.childRole] = true
	}
	if !roles["pm"] || !roles["arch"] {
		t.Errorf("expected pm and arch, got %v", roles)
	}
}
