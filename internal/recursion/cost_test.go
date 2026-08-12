package recursion

import "testing"

func TestResolveDefaultModels(t *testing.T) {
	r := CostResolver{} // nil Models → defaults

	tests := []struct {
		name   string
		tier   string
		expect string
	}{
		{"pro", "pro", "deepseek-v4-pro"},
		{"paid", "paid", "deepseek-v4-pro"},
		{"cheap", "cheap", "laguna-s-2.1-free"},
		{"free", "free", "laguna-s-2.1-free"},
		{"free→paid", "free→paid", "laguna-s-2.1-free"},
		{"free->paid ascii", "free->paid", "laguna-s-2.1-free"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			primary, chain, err := r.Resolve(tc.tier)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if primary != tc.expect {
				t.Errorf("primary=%q, want %q", primary, tc.expect)
			}
			if tc.tier != "free→paid" && tc.tier != "free->paid" && len(chain) != 0 {
				t.Errorf("expected no chain for %q, got %v", tc.tier, chain)
			}
		})
	}
}

func TestResolveFreeToPaidFallbackChain(t *testing.T) {
	r := CostResolver{}
	primary, chain, err := r.Resolve("free→paid")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if primary != "laguna-s-2.1-free" {
		t.Errorf("primary=%q, want cheap/free model", primary)
	}
	if len(chain) != 1 || chain[0] != "deepseek-v4-pro" {
		t.Errorf("chain=%v, want [deepseek-v4-pro]", chain)
	}
}

func TestResolveCustomModelsOverride(t *testing.T) {
	r := CostResolver{Models: map[string]string{
		"pro":   "claude-opus-5",
		"paid":  "claude-sonnet-5",
		"free":  "deepseek-v4-flash",
		"cheap": "deepseek-v4-flash",
	}}
	primary, chain, err := r.Resolve("pro")
	if err != nil || primary != "claude-opus-5" {
		t.Fatalf("pro: primary=%q err=%v", primary, err)
	}
	primary, chain, err = r.Resolve("free→paid")
	if err != nil || primary != "deepseek-v4-flash" {
		t.Fatalf("free→paid primary=%q err=%v, want deepseek-v4-flash", primary, err)
	}
	if len(chain) != 1 || chain[0] != "claude-sonnet-5" {
		t.Errorf("free→paid chain=%v, want [claude-sonnet-5]", chain)
	}
}

func TestResolveEscalatesUnknownTier(t *testing.T) {
	r := CostResolver{Models: map[string]string{
		"paid": "claude-sonnet-5",
		"pro":  "claude-opus-5",
	}}
	// unknown tier with cheap/free absent escalates to paid/pro
	primary, _, err := r.Resolve("weird-tier")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if primary != "claude-sonnet-5" {
		t.Errorf("expected escalation to paid model, got %q", primary)
	}
}

func TestResolveEmptyTierEscalates(t *testing.T) {
	r := CostResolver{Models: map[string]string{"pro": "claude-opus-5"}}
	primary, _, err := r.Resolve("")
	if err != nil || primary != "claude-opus-5" {
		t.Fatalf("empty tier: primary=%q err=%v", primary, err)
	}
}

func TestResolveUnknownTierNoFallbackErrors(t *testing.T) {
	// No cheap/free/paid/pro configured and tier absent → no resolution.
	r := CostResolver{Models: map[string]string{}}
	_, _, err := r.Resolve("pro")
	if err == nil {
		t.Fatal("expected error when tier and escalations unresolved")
	}
}

func TestResolveDefaultsWhenModelsPartial(t *testing.T) {
	// Only "free" configured; "paid" absent → fallback to DefaultModels["paid"].
	r := CostResolver{Models: map[string]string{"free": "custom-free"}}
	primary, chain, err := r.Resolve("free→paid")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if primary != "custom-free" {
		t.Errorf("primary=%q, want custom-free", primary)
	}
	if len(chain) != 1 || chain[0] != "deepseek-v4-pro" {
		t.Errorf("chain=%v, want default paid [deepseek-v4-pro]", chain)
	}
}
