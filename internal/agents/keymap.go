package agents

import (
	"fmt"
	"unicode"
)

// Action is a navigation or command action the agents view can perform. Its
// string value is also the config key under [agents.keys].
type Action string

const (
	ActionUp          Action = "up"
	ActionDown        Action = "down"
	ActionTop         Action = "top"
	ActionBottom      Action = "bottom"
	ActionHalfUp      Action = "half_up"
	ActionHalfDown    Action = "half_down"
	ActionPrevSection Action = "prev_section"
	ActionNextSection Action = "next_section"
	ActionSelect      Action = "select"
	ActionFold        Action = "fold"
	ActionQuit        Action = "quit"
)

// AllActions lists every valid action.
var AllActions = []Action{
	ActionUp, ActionDown, ActionTop, ActionBottom,
	ActionHalfUp, ActionHalfDown, ActionPrevSection, ActionNextSection,
	ActionSelect, ActionFold, ActionQuit,
}

// Keymap maps each action to the keys that trigger it. Keys are Bubble Tea
// key.String() values (e.g. "k", "ctrl+d", "enter"); a two-letter value of the
// same rune (e.g. "gg") is a chord: that rune pressed twice in a row.
type Keymap map[Action][]string

// DefaultKeymap returns the built-in bindings. Vim-native, and equal to the
// historic behavior so that absent configuration changes nothing.
func DefaultKeymap() Keymap {
	return Keymap{
		ActionUp:          {"k", "up"},
		ActionDown:        {"j", "down"},
		ActionTop:         {"gg"},
		ActionBottom:      {"G"},
		ActionHalfUp:      {"ctrl+u"},
		ActionHalfDown:    {"ctrl+d"},
		ActionPrevSection: {"{"},
		ActionNextSection: {"}"},
		ActionSelect:      {"enter"},
		ActionFold:        {"tab"},
		ActionQuit:        {"q", "esc", "ctrl+c"},
	}
}

// ResolveKeymap merges user overrides onto the defaults and validates the
// result. An override replaces (not extends) the keys for that action. It fails
// on an unknown action, an action with no keys, or a key bound to two actions.
func ResolveKeymap(overrides map[string][]string) (Keymap, error) {
	km := DefaultKeymap()
	for name, keys := range overrides {
		a := Action(name)
		if !validAction(a) {
			return nil, fmt.Errorf("unknown agents key action %q (valid: %v)", name, AllActions)
		}
		if len(keys) == 0 {
			return nil, fmt.Errorf("agents key action %q has no keys bound", name)
		}
		km[a] = keys
	}
	if _, err := km.buildResolver(); err != nil {
		return nil, err
	}
	return km, nil
}

func validAction(a Action) bool {
	for _, known := range AllActions {
		if known == a {
			return true
		}
	}
	return false
}

// resolver maps pressed keys to actions: single keys directly, and double-rune
// chords by their rune.
type resolver struct {
	single map[string]Action
	double map[rune]Action
}

// buildResolver builds the reverse lookup once and rejects conflicting bindings.
func (k Keymap) buildResolver() (*resolver, error) {
	r := &resolver{single: map[string]Action{}, double: map[rune]Action{}}
	for _, a := range AllActions {
		for _, key := range k[a] {
			if ru, ok := doubleChord(key); ok {
				if existing, dup := r.double[ru]; dup && existing != a {
					return nil, fmt.Errorf("agents key %q is bound to both %q and %q", key, existing, a)
				}
				r.double[ru] = a
				continue
			}
			if existing, dup := r.single[key]; dup && existing != a {
				return nil, fmt.Errorf("agents key %q is bound to both %q and %q", key, existing, a)
			}
			r.single[key] = a
		}
	}
	// A single-rune key cannot also be the prefix of a different chord.
	for key, a := range r.single {
		if ru, ok := singleRune(key); ok {
			if ca, exists := r.double[ru]; exists && ca != a {
				return nil, fmt.Errorf("agents key %q conflicts with chord %q (actions %q and %q)",
					key, string([]rune{ru, ru}), a, ca)
			}
		}
	}
	return r, nil
}

// doubleChord reports whether s is a two-letter chord of one repeated rune.
func doubleChord(s string) (rune, bool) {
	r := []rune(s)
	if len(r) == 2 && r[0] == r[1] && unicode.IsLetter(r[0]) {
		return r[0], true
	}
	return 0, false
}

// singleRune reports whether s is exactly one rune.
func singleRune(s string) (rune, bool) {
	r := []rune(s)
	if len(r) == 1 {
		return r[0], true
	}
	return 0, false
}
