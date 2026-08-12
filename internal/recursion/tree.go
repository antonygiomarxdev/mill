// Package recursion implements the recursive delegation engine: a delegator
// walks the role org-chart (read from .mill/roles/<role>/ROLE.md frontmatter),
// handing a child worktree to each subordinate, advancing artifacts through the
// frd → spec → tasks → implementation pipeline, and aborting on cycles or depth
// overrun.
package recursion

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/antonygiomarxdev/mill/internal/domain"
)

// ArtifactPhase is a phase in the artifact handoff pipeline. The engine
// advances each child worktree through these phases in order:
// frd → spec → tasks → implementation.
type ArtifactPhase string

const (
	// PhaseFRD is the Front Requirements Document phase (owned by PM).
	PhaseFRD ArtifactPhase = "frd"
	// PhaseSpec is the architecture/design spec phase (owned by Architect).
	PhaseSpec ArtifactPhase = "spec"
	// PhaseTasks is the task decomposition phase (owned by Tech Lead).
	PhaseTasks ArtifactPhase = "tasks"
	// PhaseImplementation is the implementation phase (owned by Sr Dev).
	PhaseImplementation ArtifactPhase = "implementation"
)

// PhaseOrder is the canonical handoff sequence between child phases.
var PhaseOrder = []ArtifactPhase{PhaseFRD, PhaseSpec, PhaseTasks, PhaseImplementation}

// defaultMaxDepth is the deepest org-chart delegation chain:
// staff → pm → ux-designer → ui-designer → qa-docs (4 hops).
const defaultMaxDepth = 4

// TreeNode represents a single role node in the delegation tree.
type TreeNode struct {
	Role         string         `json:"role"`
	Depth        int            `json:"depth"`
	ArtifactPath string         `json:"artifact_path,omitempty"`
	ModelTier    string         `json:"model_tier,omitempty"`
	Verdict      domain.Verdict `json:"verdict,omitempty"`
	Duration     time.Duration  `json:"duration"`
	Children     []*TreeNode    `json:"children,omitempty"`
}

// DelegationTree is the in-memory, persisted tree of child worktrees. It
// tracks depth, per-node role, artifact path, model tier, verdict, and the
// child worktree paths (the node's Children). It is persisted to
// .mill/state/recursion.json.
type DelegationTree struct {
	Root     *TreeNode `json:"root"`
	MaxDepth int       `json:"max_depth"`
}

// NewTree constructs a fresh DelegationTree rooted at role with the given
// maxDepth (0 normalizes to defaultMaxDepth).
func NewTree(role string, maxDepth int) *DelegationTree {
	if maxDepth <= 0 {
		maxDepth = defaultMaxDepth
	}
	return &DelegationTree{
		Root:     &TreeNode{Role: role, Depth: 0},
		MaxDepth: maxDepth,
	}
}

// Parse deserializes a DelegationTree from JSON bytes.
func Parse(data []byte) (*DelegationTree, error) {
	var t DelegationTree
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	if t.Root == nil {
		return nil, fmt.Errorf("recursion: tree has no root node")
	}
	if t.MaxDepth <= 0 {
		t.MaxDepth = defaultMaxDepth
	}
	return &t, nil
}

// Load reads and reconstructs a DelegationTree from the JSON file at path.
func Load(path string) (*DelegationTree, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Save persists the tree to path as indented JSON, creating parent dirs.
func (t *DelegationTree) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// AddChild appends child beneath the first node whose role matches parentRole.
// If no such node exists, the child is appended beneath the root.
func (t *DelegationTree) AddChild(parentRole string, child *TreeNode) *TreeNode {
	p := t.find(parentRole)
	if p == nil {
		p = t.Root
	}
	p.Children = append(p.Children, child)
	return child
}

// Find returns the first node whose role matches, or nil.
func (t *DelegationTree) Find(role string) *TreeNode {
	return findNode(t.Root, role)
}

func (t *DelegationTree) find(role string) *TreeNode {
	return findNode(t.Root, role)
}

func findNode(n *TreeNode, role string) *TreeNode {
	if n == nil {
		return nil
	}
	if n.Role == role {
		return n
	}
	for _, c := range n.Children {
		if f := findNode(c, role); f != nil {
			return f
		}
	}
	return nil
}

// Height returns the greatest depth of any node in the tree.
func (t *DelegationTree) Height() int {
	if t.Root == nil {
		return 0
	}
	return height(t.Root)
}

func height(n *TreeNode) int {
	if n == nil {
		return 0
	}
	max := n.Depth
	for _, c := range n.Children {
		if h := height(c); h > max {
			max = h
		}
	}
	return max
}
