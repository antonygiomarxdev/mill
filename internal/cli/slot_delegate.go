package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/antonygiomarxdev/mill/internal/config"
	"github.com/antonygiomarxdev/mill/internal/slots"
)

// ErrSlotsExhausted is returned by AcquireSlot when no slot becomes available
// within the acquisition timeout (all slots occupied) or when the slot manager
// is shut down while the caller is queued. Callers map it to
// ENVIRONMENT_FAILURE and abort instead of blocking indefinitely.
var ErrSlotsExhausted = errors.New("slots agotados")

// slotAcquireTimeout bounds how long AcquireSlot waits for a free slot before
// declaring exhaustion. It is a var (not a const) so tests can shrink it.
var slotAcquireTimeout = 30 * time.Second

// MaxSlotsFromConfig extracts the maximum concurrent slots from config.
// Defaults to 4 when Concurrency.MaxSlots is zero or negative.
func MaxSlotsFromConfig(cfg config.Config) int {
	if cfg.Concurrency.MaxSlots <= 0 {
		return 4
	}
	return cfg.Concurrency.MaxSlots
}

// EnsureSlotManager returns an initialized slot manager.
// If existing is non-nil, it is returned as-is.
// Otherwise a new Manager is created with maxSlots from cfg.Concurrency.MaxSlots
// (defaulting to 4 when zero or negative).
func EnsureSlotManager(existing *slots.Manager, cfg config.Config) *slots.Manager {
	if existing != nil {
		return existing
	}
	return slots.NewManager(MaxSlotsFromConfig(cfg))
}

// ValidatePriority returns an error if priority is true and activeRole is not
// "staff". Only the staff role may use the --priority flag to preempt the queue.
func ValidatePriority(priority bool, activeRole string) error {
	if !priority {
		return nil
	}
	if activeRole != "staff" {
		return fmt.Errorf("--priority is restricted to staff role")
	}
	return nil
}

// AcquireSlot acquires a slot from the manager for the given issue and role.
// If mgr is nil, returns 0, nil immediately (no-op).
// On successful acquisition where the caller was queued (position > 0),
// formats a notification to errOut: "Delegation queued — <occ>/<max> slots occupied, position <pos>".
// Returns the queue position at enqueue time (0 = immediate) and any error.
// On error, the caller MUST NOT proceed with dispatch.
func AcquireSlot(ctx context.Context, mgr *slots.Manager, errOut io.Writer, issue int, role string, priority bool, maxSlots int) (int, error) {
	if mgr == nil {
		return 0, nil
	}

	// Bound the wait so slot exhaustion cannot block the dispatcher
	// indefinitely. A caller-supplied deadline is honored as-is; otherwise a
	// default timeout is applied.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, slotAcquireTimeout)
		defer cancel()
	}

	position, err := mgr.Acquire(ctx, issue, role, priority)
	if err != nil {
		if errors.Is(err, slots.ErrShutdown) || errors.Is(err, context.DeadlineExceeded) {
			fmt.Fprintln(errOut, "slots agotados")
			return position, ErrSlotsExhausted
		}
		return position, fmt.Errorf("slot acquisition failed: %w", err)
	}
	if position > 0 {
		status := mgr.Status()
		occupied := len(status.Occupied)
		fmt.Fprintf(errOut, "Delegation queued — %d/%d slots occupied, position %d\n", occupied, maxSlots, position)
	}
	return position, nil
}

// ReleaseSlot releases the calling goroutine's slot in the manager.
// No-op when mgr is nil.
func ReleaseSlot(mgr *slots.Manager) {
	if mgr == nil {
		return
	}
	mgr.Release()
}
