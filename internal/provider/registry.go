package provider

// Registry holds the enabled providers and aggregates their candidates. It
// isolates per-provider failures and deduplicates candidates that resolve to
// the same session name.
type Registry struct {
	providers []Provider
	enabled   map[string]bool // type -> enabled; absent means enabled
	warn      func(string)
}

// NewRegistry returns an empty registry. warn receives non-fatal messages (a
// provider erroring, a tool missing); a nil warn discards them.
func NewRegistry(warn func(string)) *Registry {
	if warn == nil {
		warn = func(string) {}
	}
	return &Registry{enabled: map[string]bool{}, warn: warn}
}

// Register adds a provider. Registration order is the tie-breaker for dedup.
func (r *Registry) Register(p Provider) {
	r.providers = append(r.providers, p)
}

// SetEnabled records which provider types are enabled. A type absent from the
// map is treated as enabled, so callers only need to list disabled types.
func (r *Registry) SetEnabled(enabled map[string]bool) {
	r.enabled = map[string]bool{}
	for k, v := range enabled {
		r.enabled[k] = v
	}
}

func (r *Registry) isEnabled(typ string) bool {
	if v, ok := r.enabled[typ]; ok {
		return v
	}
	return true
}

// Candidates queries every enabled provider, skips disabled ones, isolates
// per-provider errors (skip + warn), and deduplicates by canonical session
// name with attach candidates taking precedence over create candidates.
func (r *Registry) Candidates() []Candidate {
	var out []Candidate
	index := map[string]int{} // name -> position in out

	for _, p := range r.providers {
		if !r.isEnabled(p.Type()) {
			continue
		}
		cands, err := p.Candidates()
		if err != nil {
			r.warn(p.Type() + " provider skipped: " + err.Error())
			continue
		}
		for _, c := range cands {
			if i, seen := index[c.Name]; seen {
				// Existing-session (attach) beats creator for the same name.
				if out[i].Kind == KindCreate && c.Kind == KindAttach {
					out[i] = c
				}
				continue
			}
			index[c.Name] = len(out)
			out = append(out, c)
		}
	}
	return out
}
