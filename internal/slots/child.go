package slots

import (
	"context"

	"github.com/antonygiomarxdev/mill/internal/config"
)

// ChildSlotManager wraps a parent *Manager with its own independent slot pool.
//
// Each child worktree gets a separate Manager created from the same config, so
// slots acquired through the child never consume the parent's pool. The
// parent Manager is only used to read the configured concurrency limit.
type ChildSlotManager struct {
	parent *Manager
	child  *Manager
}

// NewChildSlotManager creates a child slot pool whose capacity comes from
// cfg.Concurrency.MaxSlots (defaulting to DefaultMaxSlots when zero or
// negative). The child pool is fully independent of the parent manager.
func NewChildSlotManager(parent *Manager, cfg config.Config) *ChildSlotManager {
	maxSlots := cfg.Concurrency.MaxSlots
	if maxSlots <= 0 {
		maxSlots = DefaultMaxSlots
	}
	return &ChildSlotManager{
		parent: parent,
		child:  NewManager(maxSlots),
	}
}

// Acquire requests a slot from the child's own pool. It blocks until a slot
// is available or ctx is cancelled, mirroring *Manager.Acquire.
func (c *ChildSlotManager) Acquire(ctx context.Context, issue int, role string, priority bool) (int, error) {
	return c.child.Acquire(ctx, issue, role, priority)
}

// Release frees the calling goroutine's slot in the child pool. It is a
// no-op if the caller holds no slot.
func (c *ChildSlotManager) Release() {
	c.child.Release()
}

// Status returns a snapshot of the child pool's occupied slots and waiters.
func (c *ChildSlotManager) Status() SlotStatus {
	return c.child.Status()
}

// Shutdown stops the child pool's dispatch loop. The parent manager is left
// untouched so it keeps serving other worktrees.
func (c *ChildSlotManager) Shutdown() {
	c.child.Shutdown()
}

// MaxSlots returns the child pool's configured capacity.
func (c *ChildSlotManager) MaxSlots() int {
	return c.child.Status().MaxSlots
}

// ParentMaxSlots returns the parent pool's configured capacity.
func (c *ChildSlotManager) ParentMaxSlots() int {
	if c.parent == nil {
		return 0
	}
	return c.parent.Status().MaxSlots
}
