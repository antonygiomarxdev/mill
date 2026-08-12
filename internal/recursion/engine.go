package recursion

import (
	"fmt"
	"path/filepath"

	"github.com/antonygiomarxdev/mill/internal/domain"
	"github.com/antonygiomarxdev/mill/internal/role"
)

// DelegationResult captures the aggregate outcome of recursively delegating
// from a start role.
type DelegationResult struct {
	Role     string              `json:"role"`
	Depth    int                 `json:"depth"`
	Worktree string              `json:"worktree,omitempty"`
	Failure  domain.FailureClass `json:"failure_class"`
}

// CycleError is returned when the delegation chain revisits a role already on
// the current ancestor path, indicating a cyclic role graph. The chain aborts
// with domain.FATAL.
type CycleError struct {
	Role string
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("recursion: cycle detected at role %q", e.Role)
}

// Delegator orchestrates the recursive delegation chain. It reads each role's
// delegates_to frontmatter (via role.ParseFrontmatter), enforces leaf
// termination, a max-depth guard, and cycle detection, triggers per-node
// artifact handoff, and propagates child worktree paths to parent state.
type Delegator struct {
	// RolesRoot is the .mill/roles directory holding <role>/ROLE.md files.
	RolesRoot string
	// Cost resolves a frontmatter model tier to concrete model names. If nil,
	// CostResolver (DefaultModels) is used.
	Cost *CostResolver
	// MaxDepth bounds the delegation chain depth (0 → defaultMaxDepth, 4).
	MaxDepth int
	// Tree is the in-memory delegation tree, built during Delegate.
	Tree *DelegationTree
	// StatePath is where the tree is persisted (.mill/state/recursion.json).
	// When set, the tree is saved at the end of Delegate.
	StatePath string

	// CreateWorktree creates a child worktree for childRole (under the given
	// model) and returns its path. Injected so tests stay filesystem-free.
	CreateWorktree func(parentWorktree, childRole, model string) (string, error)
	// CopyBinary copies the mill binary into a worktree before spawning it.
	CopyBinary func(worktree string) domain.FailureClass
	// Handoff advances the worktree through each phase in PhaseOrder.
	Handoff func(worktree, phase string) error
	// WriteParentState records the child worktree path in the parent's state.
	WriteParentState func(parentWorktree, childRole, childWorktree string) error
}

// Delegate recursively delegates from startRole, building the delegation
// tree rooted at parentWorktree. It returns the aggregate result; on a cycle
// the error is a *CycleError and result.Failure is domain.FATAL.
func (d *Delegator) Delegate(startRole, parentWorktree, artifactPath string) (*DelegationResult, error) {
	maxDepth := d.MaxDepth
	if maxDepth <= 0 {
		maxDepth = defaultMaxDepth
	}
	d.Tree = &DelegationTree{MaxDepth: maxDepth}

	node, res, err := d.buildNode(startRole, parentWorktree, artifactPath, 0, maxDepth, map[string]bool{})
	d.Tree.Root = node
	if d.StatePath != "" && d.Tree.Root != nil {
		_ = d.Tree.Save(d.StatePath)
	}
	if res == nil {
		res = &DelegationResult{Role: startRole, Failure: domain.CLASS_OK}
	}
	return res, err
}

// buildNode recurses one level: parses frontmatter, runs handoff, and either
// terminates (leaf / depth guard) or delegates to each subordinate.
func (d *Delegator) buildNode(roleName, worktree, artifactPath string, depth, maxDepth int, path map[string]bool) (*TreeNode, *DelegationResult, error) {
	// Cycle detection: role is already on the current ancestor path.
	if path[roleName] {
		return nil, &DelegationResult{Role: roleName, Depth: depth, Failure: domain.FATAL}, &CycleError{Role: roleName}
	}
	path[roleName] = true
	defer delete(path, roleName)

	node := &TreeNode{
		Role:         roleName,
		Depth:        depth,
		ArtifactPath: artifactPath,
	}

	// Read delegates_to + model tier from ROLE.md frontmatter.
	fm, err := role.ParseFrontmatter(d.rolePath(roleName))
	if err != nil {
		return node, &DelegationResult{Role: roleName, Depth: depth, Worktree: worktree, Failure: domain.EXECUTION_FAILURE}, err
	}
	node.ModelTier = fm.Model

	// Trigger artifact handoff across phases in this node's worktree.
	if err := d.runHandoff(worktree); err != nil {
		return node, &DelegationResult{Role: roleName, Depth: depth, Worktree: worktree, Failure: domain.CONTRACT_FAILURE}, err
	}

	// Leaf termination: a role with no delegates_to stops the chain.
	if len(fm.DelegatesTo) == 0 {
		return node, &DelegationResult{Role: roleName, Depth: depth, Worktree: worktree, Failure: domain.CLASS_OK}, nil
	}

	// Max-depth guard: stop recursing beyond the configured depth.
	if depth >= maxDepth {
		return node, &DelegationResult{Role: roleName, Depth: depth, Worktree: worktree, Failure: domain.CLASS_OK}, nil
	}

	// Delegate to each subordinate (branching). A cycle or fatal failure in
	// any branch aborts the whole delegation.
	for _, child := range fm.DelegatesTo {
		// The child runs at its own model tier, resolved via CostResolver.
		model := d.childModel(child)
		childWT, cerr := d.createWorktree(worktree, child, model)
		if cerr != nil {
			return node, &DelegationResult{Role: roleName, Depth: depth, Worktree: worktree, Failure: domain.EXECUTION_FAILURE}, cerr
		}
		if fc := d.copyBinary(childWT); fc != domain.CLASS_OK {
			return node, &DelegationResult{Role: roleName, Depth: depth, Worktree: worktree, Failure: fc},
				fmt.Errorf("recursion: binary copy failed for %s: %v", child, fc)
		}
		if werr := d.writeParentState(worktree, child, childWT); werr != nil {
			return node, &DelegationResult{Role: roleName, Depth: depth, Worktree: worktree, Failure: domain.EXECUTION_FAILURE}, werr
		}
		childArtifact := filepath.Join(childWT, "artifact")
		childNode, childRes, cerr := d.buildNode(child, childWT, childArtifact, depth+1, maxDepth, path)
		if childNode != nil {
			node.Children = append(node.Children, childNode)
		}
		if cerr != nil {
			return node, childRes, cerr
		}
	}

	return node, &DelegationResult{Role: roleName, Depth: depth, Worktree: worktree, Failure: domain.CLASS_OK}, nil
}

// rolePath resolves a role's ROLE.md within RolesRoot.
func (d *Delegator) rolePath(roleName string) string {
	return filepath.Join(d.RolesRoot, roleName, "ROLE.md")
}

// childModel resolves the model tier of the child role (read from its
// frontmatter) to a concrete model name via CostResolver. A parse failure
// falls back to a passthrough of the (empty) tier so delegation can proceed
// onto the next guard.
func (d *Delegator) childModel(childRole string) string {
	fm, err := role.ParseFrontmatter(d.rolePath(childRole))
	if err != nil {
		return ""
	}
	return d.resolveModel(fm.Model)
}

// resolveModel maps a frontmatter model tier to a concrete model name.
func (d *Delegator) resolveModel(tier string) string {
	cost := d.Cost
	if cost == nil {
		cost = &CostResolver{}
	}
	m, _, err := cost.Resolve(tier)
	if err != nil || m == "" {
		return tier
	}
	return m
}

// createWorktree creates a child worktree for childRole (or a default path).
func (d *Delegator) createWorktree(parentWalk, childRole, model string) (string, error) {
	if d.CreateWorktree != nil {
		return d.CreateWorktree(parentWalk, childRole, model)
	}
	return filepath.Join(parentWalk, ".mill", "child", childRole), nil
}

// copyBinary copies the mill binary into a worktree (no-op if unset).
func (d *Delegator) copyBinary(worktree string) domain.FailureClass {
	if d.CopyBinary != nil {
		return d.CopyBinary(worktree)
	}
	return domain.CLASS_OK
}

// runHandoff advances the worktree through every phase in PhaseOrder.
func (d *Delegator) runHandoff(worktree string) error {
	if d.Handoff == nil {
		return nil
	}
	for _, p := range PhaseOrder {
		if err := d.Handoff(worktree, string(p)); err != nil {
			return err
		}
	}
	return nil
}

// writeParentState records the child worktree path in parent state (no-op if unset).
func (d *Delegator) writeParentState(parentWalk, childRole, childWT string) error {
	if d.WriteParentState != nil {
		return d.WriteParentState(parentWalk, childRole, childWT)
	}
	return nil
}
