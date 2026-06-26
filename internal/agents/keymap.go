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
	ActionFocusLeft   Action = "focus_left"
	ActionFocusRight  Action = "focus_right"
	ActionSelect      Action = "select"
	ActionFold        Action = "fold"
	ActionNewSession  Action = "new_session"
	ActionNewAgent    Action = "new_agent"
	ActionClose       Action = "close"
	ActionDelete      Action = "delete"
	ActionMute        Action = "mute"
	ActionQuit        Action = "quit"
)

// AllActions lists every valid action.
var AllActions = []Action{
	ActionUp, ActionDown, ActionTop, ActionBottom,
	ActionHalfUp, ActionHalfDown, ActionPrevSection, ActionNextSection,
	ActionFocusLeft, ActionFocusRight,
	ActionSelect, ActionFold, ActionNewSession, ActionNewAgent, ActionClose, ActionDelete, ActionMute, ActionQuit,
}

// Keymap maps each action to the keys that trigger it. Keys are Bubble Tea
// key.String() values (e.g. "k", "ctrl+d", "enter"); a two-rune value of two
// typeable keys (e.g. "gg", "dd", "dD") is a chord: those two keys pressed in
// sequence.
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
		ActionFocusLeft:   {"h", "left"},
		ActionFocusRight:  {"l", "right"},
		ActionSelect:      {"enter"},
		ActionFold:        {"tab"},
		ActionNewSession:  {"s"},
		ActionNewAgent:    {"n"},
		ActionClose:       {"dd"},
		ActionDelete:      {"dD"},
		ActionMute:        {"m"},
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

// resolver maps pressed keys to actions: single keys directly, and two-key chords
// by their (firstKey, secondKey) pair. prefix holds the first keys that begin some
// chord, so the view knows when to arm rather than dispatch a key immediately.
type resolver struct {
	single map[string]Action
	chords map[[2]string]Action
	prefix map[string]bool
}

// buildResolver builds the reverse lookup once and rejects conflicting bindings: a
// key bound to two actions, or a single key that is also the first key of a chord.
func (k Keymap) buildResolver() (*resolver, error) {
	r := &resolver{single: map[string]Action{}, chords: map[[2]string]Action{}, prefix: map[string]bool{}}
	for _, a := range AllActions {
		for _, key := range k[a] {
			if chord, ok := parseChord(key); ok {
				if existing, dup := r.chords[chord]; dup && existing != a {
					return nil, fmt.Errorf("agents key %q is bound to both %q and %q", key, existing, a)
				}
				r.chords[chord] = a
				r.prefix[chord[0]] = true
				continue
			}
			if existing, dup := r.single[key]; dup && existing != a {
				return nil, fmt.Errorf("agents key %q is bound to both %q and %q", key, existing, a)
			}
			r.single[key] = a
		}
	}
	// A single key cannot also be the first key of a chord: pressing it would arm
	// the chord rather than fire the single binding.
	for key, a := range r.single {
		if r.prefix[key] {
			return nil, fmt.Errorf("agents key %q conflicts with chord %q (actions %q and %q)",
				key, key, a, chordWithPrefix(r, key))
		}
	}
	return r, nil
}

// chordWithPrefix returns an action of some chord whose first key is prefix, for a
// clearer conflict message; "" if none (should not happen when prefix is set).
func chordWithPrefix(r *resolver, prefix string) Action {
	for chord, a := range r.chords {
		if chord[0] == prefix {
			return a
		}
	}
	return ""
}

// nonChordKeys are Bubble Tea key.String() values that are two runes long yet name a
// single key (not two typed keys), so they must not be parsed as chords. "up" is the
// only such named key; the rest ("down", "enter", "esc", "tab", …) are longer.
var nonChordKeys = map[string]bool{"up": true}

// parseChord reports whether s is a two-key chord like "dd", "dD", or "gg" and
// returns its two single-key strings. A value of exactly two typeable runes is a
// chord; a named key such as "up" is not.
func parseChord(s string) ([2]string, bool) {
	if nonChordKeys[s] {
		return [2]string{}, false
	}
	r := []rune(s)
	if len(r) == 2 && isChordRune(r[0]) && isChordRune(r[1]) {
		return [2]string{string(r[0]), string(r[1])}, true
	}
	return [2]string{}, false
}

// isChordRune reports whether r is a key that may take part in a chord: a letter or
// digit (so "dd"/"gg"/"dD" qualify, but "{"/"}" or whitespace do not).
func isChordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
