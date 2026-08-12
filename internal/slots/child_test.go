package slots

import (
	"context"
	"testing"

	"github.com/antonygiomarxdev/mill/internal/config"
)

// TestChildSlotsIndependentOfParent verifies that acquiring slots in a child
// pool does not consume the parent's slots, and that the child has its own
// capacity.
func TestChildSlotsIndependentOfParent(t *testing.T) {
	parent := NewManager(1)
	defer parent.Shutdown()

	child := NewChildSlotManager(parent, config.Config{Concurrency: config.Concurrency{MaxSlots: 2}})
	defer child.Shutdown()

	// Acquire both child slots. The parent has capacity 1, so if the child
	// shared the parent's pool this would block.
	for i := 0; i < 2; i++ {
		pos, err := child.Acquire(context.Background(), i+1, "sr-dev-be", false)
		if err != nil {
			t.Fatalf("child Acquire %d failed: %v", i, err)
		}
		if pos != 0 {
			t.Errorf("child Acquire %d: expected position 0 (immediate), got %d", i, pos)
		}
	}

	// Child pool: 2 occupied. Parent pool: untouched.
	if got := len(child.Status().Occupied); got != 2 {
		t.Errorf("expected 2 occupied child slots, got %d", got)
	}
	if got := len(parent.Status().Occupied); got != 0 {
		t.Errorf("expected 0 occupied parent slots, got %d", got)
	}

	// Parent can still acquire its own slot.
	if _, err := parent.Acquire(context.Background(), 99, "staff", false); err != nil {
		t.Fatalf("parent Acquire failed: %v", err)
	}
	if got := len(parent.Status().Occupied); got != 1 {
		t.Errorf("expected 1 occupied parent slot, got %d", got)
	}
	parent.Release()

	// Releasing a child slot frees the child pool, not the parent's.
	child.Release()
	if got := len(child.Status().Occupied); got != 1 {
		t.Errorf("expected 1 occupied child slot after release, got %d", got)
	}
	if got := len(parent.Status().Occupied); got != 0 {
		t.Errorf("expected 0 occupied parent slots after child release, got %d", got)
	}
}

// TestChildSlotMaxFromConfig verifies the child pool capacity comes from
// config, and falls back to DefaultMaxSlots when config is zero or negative.
func TestChildSlotMaxFromConfig(t *testing.T) {
	parent := NewManager(1)
	defer parent.Shutdown()

	cases := []struct {
		name string
		max  int
		want int
	}{
		{name: "explicit", max: 3, want: 3},
		{name: "zero defaults to DefaultMaxSlots", max: 0, want: DefaultMaxSlots},
		{name: "negative defaults to DefaultMaxSlots", max: -1, want: DefaultMaxSlots},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			child := NewChildSlotManager(parent, config.Config{Concurrency: config.Concurrency{MaxSlots: tc.max}})
			defer child.Shutdown()

			if got := child.MaxSlots(); got != tc.want {
				t.Errorf("MaxSlots() = %d, want %d", got, tc.want)
			}
			if got := child.Status().MaxSlots; got != tc.want {
				t.Errorf("Status().MaxSlots = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestChildNeverConsumesParentSlots verifies that even when the child pool is
// saturated and a parent slot is free, the child blocks instead of borrowing
// from the parent.
func TestChildNeverConsumesParentSlots(t *testing.T) {
	parent := NewManager(1)
	defer parent.Shutdown()

	child := NewChildSlotManager(parent, config.Config{Concurrency: config.Concurrency{MaxSlots: 1}})
	defer child.Shutdown()

	// Saturate the child pool.
	if _, err := child.Acquire(context.Background(), 1, "sr-dev-be", false); err != nil {
		t.Fatalf("child Acquire failed: %v", err)
	}

	// A second child Acquire must block even though the parent slot is free.
	acquired := make(chan error, 1)
	go func() {
		_, err := child.Acquire(context.Background(), 2, "sr-dev-be", false)
		acquired <- err
	}()

	select {
	case err := <-acquired:
		t.Fatalf("child Acquire returned while pool saturated (err=%v); parent must not be consumed", err)
	default:
	}

	// Parent remains untouched.
	if got := len(parent.Status().Occupied); got != 0 {
		t.Errorf("expected 0 occupied parent slots, got %d", got)
	}

	// Freeing the child slot lets the blocked acquire proceed.
	child.Release()
	if err := <-acquired; err != nil {
		t.Fatalf("child Acquire after release failed: %v", err)
	}
}
