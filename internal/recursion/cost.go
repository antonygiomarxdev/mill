package recursion

import "fmt"

// DefaultModels maps model-tier keys (as found in role frontmatter) to
// concrete provider model names. Used when config supplies no models map.
var DefaultModels = map[string]string{
	"pro":   "deepseek-v4-pro",
	"paid":  "deepseek-v4-pro",
	"cheap": "laguna-s-2.1-free",
	"free":  "laguna-s-2.1-free",
}

// CostResolver maps a role frontmatter model tier (e.g. "pro", "free→paid")
// to concrete provider model names, applying fallback chains where defined.
type CostResolver struct {
	// Models maps tier keys to model names (recursion.models in config).
	Models map[string]string
}

// Resolve maps a frontmatter model tier to a primary model name and an
// ordered fallback chain.
//
// Tier handling:
//   - "free→paid": primary = the cheap/free model, fallback chain = [paid].
//     On rate-limit the dispatcher escalates from cheap to paid.
//   - "pro"/"paid"/"cheap"/"free": resolves directly to a single model.
//   - "": resolves to the cheapest-available tier (cheap/free then paid).
//   - an unknown tier escalates cheap/free → paid → pro; errors if unresolved.
//
// When Models is nil, or a tier is absent from it, DefaultModels is used.
func (r CostResolver) Resolve(tier string) (primary string, chain []string, err error) {
	models := r.Models
	if models == nil {
		models = DefaultModels
	}

	// free→paid: cheap model primary with paid fallback on rate-limit.
	if isFreeToPaid(tier) {
		primary = pick(models, "free", DefaultModels["free"])
		chain = []string{pick(models, "paid", DefaultModels["paid"])}
		return primary, chain, nil
	}

	// Direct tier lookup.
	if m := models[tier]; m != "" {
		return m, nil, nil
	}

	// Escalate cheap/free → paid → pro for unknown/blank tiers.
	for _, t := range []string{"cheap", "free", "paid", "pro"} {
		if mm := models[t]; mm != "" {
			return mm, nil, nil
		}
	}

	return "", nil, fmt.Errorf("recursion: no model configured for tier %q", tier)
}

// isFreeToPaid reports whether tier is the "free→paid" (cheap with pro
// fallback) marker. Accepts both the unicode (→) and ASCII (->) spellings.
func isFreeToPaid(tier string) bool {
	return tier == "free→paid" || tier == "free->paid"
}

// pick returns models[tier], or fallback when the key is absent or empty.
func pick(models map[string]string, tier, fallback string) string {
	if v := models[tier]; v != "" {
		return v
	}
	return fallback
}
