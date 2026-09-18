package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// observeFile holds the pane screen samples backing quiescence detection, at the state root
// (view/runtime state, not per-session hook data). Both the dash refresh and every hook
// invocation fold their samples into it, so a short-lived hook process can still tell how long a
// pane has been still - a measurement that needs two samples separated in time and therefore
// cannot live in process memory.
const observeFile = "panes.json"

// quiescenceWindow is how long a pane's screen must be byte-identical before we treat a
// "working" hook record as disproved. It must comfortably exceed the period of Claude's working
// animation - a spinner glyph and a 1Hz elapsed-seconds counter on its status line - plus polling
// jitter, so a working agent can never look still. Observed animation is ~1Hz; 12s is an order of
// magnitude of headroom, at the cost of a finished agent lingering as "working" for that long.
//
// Claude renders full-screen (alternate screen), so a user scrolling or typing repaints the pane
// and reads here as change, masking a genuinely still screen. That only ever *delays* disproving
// "working" - a detector that can report stillness and nothing else cannot manufacture activity -
// and it clears once the user stops touching the pane. It is also the cheapest case to lose: a
// pane someone is actively driving is one they are already watching.
const quiescenceWindow = 12 * time.Second

// backgroundQuiescenceWindow is the stillness window for a session parked on background work
// (see ClaudeSource.detectorFor). Such a session is legitimately silent: it has yielded its turn
// to a subagent and draws nothing until that work reports back, so the ordinary window would
// demote it within one refresh. This is a backstop, not a signal - it bounds how long a record
// can stay pinned at "working" when the work never reports back - so it is set far above any
// plausible repaint interval: a pane that has not changed one byte in this long has no live
// session behind it, whatever the last hook claimed.
const backgroundQuiescenceWindow = 10 * time.Minute

// subagentClaimExpiry bounds how long an idle row keeps naming a subagent when the SubagentStop
// that should have retired it never arrives - hooks installed by an older build, or a crash
// between dispatch and completion. It is a stop-gap for a missing announcement rather than a
// judgement about the work, so it sits far above how long a subagent plausibly runs; the ordinary
// correction is the event itself, which lands the moment the subagent finishes.
const subagentClaimExpiry = 2 * time.Hour

// observeFreshness bounds how old a sample may be and still support a quiescence claim. Without
// it, a pane sampled once and then ignored for an hour would look still for an hour, when in
// truth nobody was watching. Samples are taken immediately before they are read, so this only
// ever rejects panes that fell out of the sampled set.
const observeFreshness = 30 * time.Second

// observeRetention prunes samples for panes that have not been seen in a day, bounding the file
// against panes that no longer exist.
const observeRetention = 24 * time.Hour

// paneObservation is what one pane's screen looked like when last sampled. Since is the earliest
// sample at which the current Hash was seen, so now-Since is how long the screen has been still;
// Seen is the most recent sample, which says whether that stillness was actually being watched.
type paneObservation struct {
	Hash  string    `json:"hash"`
	Since time.Time `json:"since"`
	Seen  time.Time `json:"seen"`
}

// hashScreen reduces a captured screen to a comparison token. Only equality matters, so the
// digest is stored rather than the screen itself - keeping the file small and out of the business
// of holding terminal content.
func hashScreen(screen string) string {
	sum := sha256.Sum256([]byte(screen))
	return hex.EncodeToString(sum[:8])
}

// readObservations loads the sample store under dir. Any problem (missing, unreadable, corrupt)
// yields an empty store: quiescence is then simply unknown, and status falls back to the hook
// record.
func readObservations(dir string) map[string]paneObservation {
	data, err := os.ReadFile(filepath.Join(dir, observeFile))
	if err != nil {
		return map[string]paneObservation{}
	}
	obs := map[string]paneObservation{}
	if err := json.Unmarshal(data, &obs); err != nil {
		return map[string]paneObservation{}
	}
	return obs
}

// writeObservations replaces the sample store atomically. Writers race by design - the dash and
// any number of concurrent hooks - and last-writer-wins is safe here: a lost update only resets a
// pane's Since to a later sample, which delays disproving "working" rather than asserting it.
func writeObservations(dir string, obs map[string]paneObservation) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(obs)
	if err != nil {
		return err
	}
	final := filepath.Join(dir, observeFile)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

// updateObservations folds this round's screens into the previous store. A pane whose screen
// still hashes the same keeps its Since (its stillness accrues); any change restarts the clock. A
// first sighting starts at now, so a newly seen pane is never immediately called still. Panes
// absent from this round keep their entry until retention expires, so dropping in and out of the
// sampled set does not lose their history.
func updateObservations(prev map[string]paneObservation, screens map[string]string, now time.Time) map[string]paneObservation {
	next := make(map[string]paneObservation, len(prev)+len(screens))
	for id, o := range prev {
		if now.Sub(o.Seen) < observeRetention {
			next[id] = o
		}
	}
	for id, screen := range screens {
		h := hashScreen(screen)
		o, ok := next[id]
		if !ok || o.Hash != h {
			o = paneObservation{Hash: h, Since: now}
		}
		o.Seen = now
		next[id] = o
	}
	return next
}

// observePanes samples the given panes, folds the result into the store under dir, and returns
// the updated store. Sampling is best-effort throughout: a capture failure simply contributes no
// screens, leaving the affected panes without a quiescence signal.
func observePanes(dir string, ids []string, capture func([]string) (map[string]string, error), now time.Time) map[string]paneObservation {
	prev := readObservations(dir)
	screens := map[string]string{}
	if capture != nil && len(ids) > 0 {
		if s, err := capture(ids); err == nil && s != nil {
			screens = s
		}
	}
	next := updateObservations(prev, screens, now)
	_ = writeObservations(dir, next)
	return next
}

// workingPaneIDs returns the distinct panes worth sampling: those of records claiming to be
// working. They are the only records a level can change, so sampling anything else would capture
// screens no decision reads.
func workingPaneIDs(recs []record) []string {
	var ids []string
	seen := map[string]bool{}
	for _, r := range recs {
		if r.Status != StatusWorking || r.TmuxPane == "" || seen[r.TmuxPane] {
			continue
		}
		seen[r.TmuxPane] = true
		ids = append(ids, r.TmuxPane)
	}
	return ids
}
