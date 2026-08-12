package recursion

import (
	"fmt"
	"strings"

	"github.com/antonygiomarxdev/mill/internal/domain"
)

// View controls how a DelegationTree is rendered.
type View string

const (
	// ViewFinal renders only the final (deepest) leaf result.
	ViewFinal View = "final"
	// ViewTree renders the full delegation tree, one line per node.
	ViewTree View = "tree"
)

// ViewRenderer formats a DelegationTree as final-result-only or full-tree.
type ViewRenderer struct {
	View View
}

// Render renders the tree according to the configured View. The tree view
// includes, per node: role, artifact path, model tier, verdict, depth, and
// duration. The final view renders only the deepest leaf.
func (r ViewRenderer) Render(t *DelegationTree) string {
	if t == nil || t.Root == nil {
		return ""
	}
	switch r.View {
	case ViewTree:
		var b strings.Builder
		r.renderTree(&b, t.Root, 0)
		return strings.TrimRight(b.String(), "\n")
	default:
		return r.renderFinal(t.Root)
	}
}

func (r ViewRenderer) renderTree(b *strings.Builder, n *TreeNode, depth int) {
	if n == nil {
		return
	}
	indent := strings.Repeat("  ", depth)
	fmt.Fprintf(b, "%s● role: %s | artifact: %s | model_tier: %s | verdict: %s | depth: %d | duration: %s\n",
		indent, n.Role, artifactOrDash(n.ArtifactPath), n.ModelTier,
		verdictOrDash(n.Verdict), n.Depth, n.Duration)
	for _, c := range n.Children {
		r.renderTree(b, c, depth+1)
	}
}

// renderFinal renders the verdict of the deepest leaf node.
func (r ViewRenderer) renderFinal(root *TreeNode) string {
	leaf := deepestLeaf(root)
	if leaf == nil {
		return ""
	}
	return fmt.Sprintf("verdict: %s | role: %s | artifact: %s | duration: %s",
		verdictOrDash(leaf.Verdict), leaf.Role,
		artifactOrDash(leaf.ArtifactPath), leaf.Duration)
}

// deepestLeaf returns the leaf (node with no children) of greatest depth.
func deepestLeaf(n *TreeNode) *TreeNode {
	if n == nil {
		return nil
	}
	if len(n.Children) == 0 {
		return n
	}
	deepest, maxDepth := (*TreeNode)(nil), -1
	for _, c := range n.Children {
		dl := deepestLeaf(c)
		if dl != nil && dl.Depth > maxDepth {
			maxDepth = dl.Depth
			deepest = dl
		}
	}
	if deepest != nil {
		return deepest
	}
	return n
}

func artifactOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func verdictOrDash(v domain.Verdict) string {
	if v == "" {
		return "-"
	}
	return string(v)
}
