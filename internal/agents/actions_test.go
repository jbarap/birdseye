package agents

import (
	"errors"
	"strings"
	"testing"
)

type spawnCall struct {
	repo   Row
	branch string
	name   string
}
type removeCall struct {
	row   Row
	force bool
}

type fakeOrch struct {
	spawns      []spawnCall
	closes      []Row
	removes     []removeCall
	opens       []Row
	shellOpens  []Row
	openErr     error  // when set, Open/OpenShell returns it
	onOpen      func() // optional: simulate the worktree gaining a window after Open/OpenShell
	dirty       bool   // when true, Remove(force=false) reports ErrWorktreeDirty
	newSessions int    // count of NewSession calls
	newSession  string // session name NewSession returns
}

func (f *fakeOrch) Spawn(repo Row, branch, name string) error {
	f.spawns = append(f.spawns, spawnCall{repo, branch, name})
	return nil
}

func (f *fakeOrch) Open(slot Row) error {
	f.opens = append(f.opens, slot)
	if f.openErr != nil {
		return f.openErr
	}
	if f.onOpen != nil {
		f.onOpen()
	}
	return nil
}

func (f *fakeOrch) OpenShell(r Row) error {
	f.shellOpens = append(f.shellOpens, r)
	if f.openErr != nil {
		return f.openErr
	}
	if f.onOpen != nil {
		f.onOpen()
	}
	return nil
}

func (f *fakeOrch) Close(r Row) error {
	f.closes = append(f.closes, r)
	return nil
}

func (f *fakeOrch) Remove(r Row, force bool) error {
	f.removes = append(f.removes, removeCall{r, force})
	if f.dirty && !force {
		return ErrWorktreeDirty
	}
	return nil
}

func (f *fakeOrch) NewSession() (string, error) {
	f.newSessions++
	return f.newSession, nil
}

// fixedRows is a RowSource over a fixed row list.
type fixedRows struct{ rows []Row }

func (f fixedRows) Rows() ([]Row, error) { return f.rows, nil }

// mutableRows is a RowSource whose rows can change between reloads, used to simulate a
// slot gaining a window after Open.
type mutableRows struct{ rows []Row }

func (m *mutableRows) Rows() ([]Row, error) { return m.rows, nil }

func modelWith(t *testing.T, orch Orchestrator, rows []Row) model {
	t.Helper()
	m, err := newModel(fixedRows{rows}, nil, DefaultKeymap(), 0)
	if err != nil {
		t.Fatal(err)
	}
	m.orch = orch
	m.focus = lensWorkspaces // action tests drive the Workspaces lens; production opens on Agents
	return m
}

func typeRunes(m model, s string) model {
	for _, r := range s {
		nm, _ := m.handleNewAgentKey(key(string(r)))
		m = nm.(model)
	}
	return m
}

func TestNewAgentGatedToManagedRepo(t *testing.T) {
	orch := &fakeOrch{}
	// A plain (non-managed) agent row: new-agent must not start.
	m := modelWith(t, orch, []Row{{Kind: RowAgent, SessionID: "x", TmuxSession: "plain", Title: "x"}})
	nm, _ := m.startNewAgent()
	m = nm.(model)
	if m.mode != modeNormal {
		t.Fatalf("new-agent should not open in a plain session, mode=%v", m.mode)
	}
	if len(orch.spawns) != 0 {
		t.Fatalf("no spawn should occur in a plain session")
	}
}

func TestNewAgentPromptAndSpawn(t *testing.T) {
	orch := &fakeOrch{}
	repo := Row{Kind: RowAgent, SessionID: "feat", TmuxSession: "proj", Title: "feat", Dir: "/code/proj/feat", Worktree: "feat", GitDir: "/code/proj/.git"}
	m := modelWith(t, orch, []Row{repo})

	nm, _ := m.startNewAgent()
	m = nm.(model)
	if m.mode != modeNewAgent {
		t.Fatalf("new-agent should open the name prompt in a managed repo, mode=%v", m.mode)
	}
	m = typeRunes(m, "wip")
	nm, _ = m.handleNewAgentKey(key("enter"))
	m = nm.(model)
	if m.mode != modeNormal {
		t.Fatalf("submitting should return to normal mode, got %v", m.mode)
	}
	// Typing into the branch field auto-derives the worktree, so both arrive as "wip".
	if len(orch.spawns) != 1 || orch.spawns[0].branch != "wip" || orch.spawns[0].name != "wip" || orch.spawns[0].repo.Dir != "/code/proj/feat" {
		t.Fatalf("spawn not invoked with the typed branch/name and repo dir: %+v", orch.spawns)
	}
}

// TestNewAgentWorktreeSlugsBranch checks the form's two-field behavior: typing a branch
// with a slash auto-fills a slugified worktree name, and tabbing to the worktree field to
// edit it stops the auto-derivation so the branch and worktree decouple.
func TestNewAgentWorktreeSlugsBranch(t *testing.T) {
	orch := &fakeOrch{}
	repo := Row{Kind: RowAgent, SessionID: "feat", TmuxSession: "proj", Title: "feat", Dir: "/code/proj/feat", Worktree: "feat", GitDir: "/code/proj/.git"}
	m := modelWith(t, orch, []Row{repo})

	nm, _ := m.startNewAgent()
	m = nm.(model)
	m = typeRunes(m, "feature/login")
	if m.worktreeInput != "feature-login" {
		t.Fatalf("worktree should auto-slug the branch, got %q", m.worktreeInput)
	}

	// Tab to the worktree field and append: the manual edit stops auto-derivation.
	nm, _ = m.handleNewAgentKey(key("tab"))
	m = nm.(model)
	if m.focusField != fieldWorktree {
		t.Fatalf("tab should focus the worktree field, got %v", m.focusField)
	}
	m = typeRunes(m, "-wt")
	if m.worktreeInput != "feature-login-wt" {
		t.Fatalf("worktree manual edit not applied, got %q", m.worktreeInput)
	}

	// Editing the branch again must no longer overwrite the hand-edited worktree.
	nm, _ = m.handleNewAgentKey(key("shift+tab"))
	m = nm.(model)
	m = typeRunes(m, "X")
	if m.worktreeInput != "feature-login-wt" {
		t.Fatalf("worktree should stay hand-edited, got %q", m.worktreeInput)
	}

	nm, _ = m.handleNewAgentKey(key("enter"))
	m = nm.(model)
	if len(orch.spawns) != 1 || orch.spawns[0].branch != "feature/loginX" || orch.spawns[0].name != "feature-login-wt" {
		t.Fatalf("spawn branch/name wrong: %+v", orch.spawns)
	}
}

func TestNewAgentEscCancels(t *testing.T) {
	orch := &fakeOrch{}
	m := modelWith(t, orch, []Row{{Kind: RowAgent, Dir: "/d", TmuxSession: "proj", Title: "t", GitDir: "/code/proj/.git"}})
	nm, _ := m.startNewAgent()
	m = nm.(model)
	m = typeRunes(m, "abc")
	nm, _ = m.handleNewAgentKey(key("esc"))
	m = nm.(model)
	if m.mode != modeNormal || len(orch.spawns) != 0 {
		t.Fatalf("esc should cancel without spawning, mode=%v spawns=%v", m.mode, orch.spawns)
	}
}

// TestCloseAndDeleteChords drives the dd/dD teardown chords through the dispatcher: dd
// closes the row's window instantly (no confirmation, no worktree removal), while dD —
// sharing the first key — opens the delete confirmation instead.
func TestCloseAndDeleteChords(t *testing.T) {
	orch := &fakeOrch{}
	row := Row{Kind: RowAgent, SessionID: "feat", TmuxSession: "proj", Worktree: "feat", Dir: "/d", TmuxWindow: "1", TmuxPane: "%2"}
	m := modelWith(t, orch, []Row{row})

	// First d arms the shared chord prefix; it must not act yet.
	nm, _ := m.handleKey(key("d"))
	m = nm.(model)
	if m.pending != "d" {
		t.Fatalf("first d should arm the chord, pending=%q", m.pending)
	}
	// Second d completes dd → close: one Close, no confirm, no removes.
	nm, _ = m.handleKey(key("d"))
	m = nm.(model)
	if len(orch.closes) != 1 {
		t.Fatalf("dd should close once, got %d", len(orch.closes))
	}
	if m.mode != modeNormal || len(orch.removes) != 0 {
		t.Fatalf("dd must not confirm or remove, mode=%v removes=%v", m.mode, orch.removes)
	}

	// dD shares the first key but completes to delete → the confirmation popup opens.
	nm, _ = m.handleKey(key("d"))
	m = nm.(model)
	nm, _ = m.handleKey(key("D"))
	m = nm.(model)
	if m.mode != modeConfirmDelete {
		t.Fatalf("dD should open the delete confirm, mode=%v", m.mode)
	}
	if len(orch.removes) != 0 {
		t.Fatalf("dD must not remove before confirmation, got %+v", orch.removes)
	}
}

// TestCloseWindowlessSlotIsNoop checks the close action no-ops on a windowless slot:
// there is no tmux window to close, so the orchestrator is never invoked — but the user
// still gets feedback rather than a swallowed keystroke.
func TestCloseWindowlessSlotIsNoop(t *testing.T) {
	orch := &fakeOrch{}
	m := modelWith(t, orch, []Row{{Kind: RowSlot, SessionID: "slot:proj:spike", TmuxSession: "proj", Worktree: "spike", Dir: "/d"}})
	nm, _ := m.startClose()
	m = nm.(model)
	if len(orch.closes) != 0 {
		t.Fatalf("closing a windowless slot should be a no-op, got %+v", orch.closes)
	}
	if m.notice == "" || m.noticeLevel != noticeInfo {
		t.Fatalf("a windowless-slot close should report feedback, notice=%q level=%v", m.notice, m.noticeLevel)
	}
	if strings.Contains(m.notice, "base") {
		t.Fatalf("a slot close must not be phrased as the base, notice=%q", m.notice)
	}
}

// TestCloseWindowlessBaseFeedback covers the bug where closing a base worktree's window
// (when other worktrees keep the session alive) left a windowless base that a second `dd`
// mislabeled as an empty slot — and wrongly suggested `dD`, which is blocked on the anchor.
// A windowless base must read as the base, never as a removable slot.
func TestCloseWindowlessBaseFeedback(t *testing.T) {
	orch := &fakeOrch{}
	m := modelWith(t, orch, []Row{{Kind: RowAnchor, SessionID: "anchor:proj", TmuxSession: "proj", Worktree: "main", Repo: "proj", Dir: "/d", IsPrimary: true}})
	nm, _ := m.startClose()
	m = nm.(model)
	if len(orch.closes) != 0 {
		t.Fatalf("a windowless base has no window to close, got %+v", orch.closes)
	}
	if m.notice == "" || m.noticeLevel != noticeInfo {
		t.Fatalf("a windowless-base close should report feedback, notice=%q level=%v", m.notice, m.noticeLevel)
	}
	if strings.Contains(m.notice, "slot") || strings.Contains(m.notice, "dD") {
		t.Fatalf("a base must not be called a slot nor suggest dD, notice=%q", m.notice)
	}
}

// TestEnterOnWindowlessSlotSpawnsAndAttaches drives the slot spawn-target path: Enter on a
// windowless slot opens an agent in its existing worktree (never recreating it), then
// attaches to the now-windowed row rather than jumping to an unrelated window.
func TestEnterOnWindowlessSlotSpawnsAndAttaches(t *testing.T) {
	slot := Row{Kind: RowSlot, SessionID: "slot:proj:spike", TmuxSession: "proj", Repo: "proj", Worktree: "spike", Dir: "/d"}
	src := &mutableRows{rows: []Row{slot}}
	orch := &fakeOrch{onOpen: func() {
		// Open spawned a window in the slot's worktree; the reload now sees it windowed.
		windowed := slot
		windowed.TmuxWindow, windowed.TmuxPane = "2", "%9"
		src.rows = []Row{windowed}
	}}
	m, err := newModel(src, nil, DefaultKeymap(), 0)
	if err != nil {
		t.Fatal(err)
	}
	m.orch = orch
	m.focus = lensWorkspaces // slot/base actions are Workspaces-lens motions

	nm, cmd := m.applyAction(ActionSelect)
	m = nm.(model)
	if len(orch.opens) != 1 || orch.opens[0].Worktree != "spike" {
		t.Fatalf("Enter on a windowless slot should Open it once, got %+v", orch.opens)
	}
	if len(orch.spawns) != 0 {
		t.Fatalf("opening a slot must reuse it, not Spawn a new worktree, got %+v", orch.spawns)
	}
	if m.chosen == nil || m.chosen.TmuxPane != "%9" {
		t.Fatalf("Enter should attach to the freshly-opened window, chosen=%+v", m.chosen)
	}
	if cmd == nil {
		t.Fatal("attaching should return a quit command")
	}
}

// TestEnterOnWindowlessBaseOpensShellAndAttaches covers the base path: Enter on a
// windowless base opens a plain shell (never an agent) in the base worktree and attaches to
// it, rather than dropping the user onto an unrelated agent's window in the session.
func TestEnterOnWindowlessBaseOpensShellAndAttaches(t *testing.T) {
	base := Row{Kind: RowAnchor, SessionID: "anchor:proj", TmuxSession: "proj", Repo: "proj", Worktree: "main", Dir: "/code/proj", IsPrimary: true}
	src := &mutableRows{rows: []Row{base}}
	orch := &fakeOrch{onOpen: func() {
		windowed := base
		windowed.TmuxWindow, windowed.TmuxPane = "1", "%3"
		src.rows = []Row{windowed}
	}}
	m, err := newModel(src, nil, DefaultKeymap(), 0)
	if err != nil {
		t.Fatal(err)
	}
	m.orch = orch
	m.focus = lensWorkspaces // slot/base actions are Workspaces-lens motions

	nm, cmd := m.applyAction(ActionSelect)
	m = nm.(model)
	if len(orch.shellOpens) != 1 || orch.shellOpens[0].Dir != "/code/proj" {
		t.Fatalf("Enter on a windowless base should OpenShell it once, got %+v", orch.shellOpens)
	}
	if len(orch.opens) != 0 || len(orch.spawns) != 0 {
		t.Fatalf("the base must get a shell, not an agent, opens=%+v spawns=%+v", orch.opens, orch.spawns)
	}
	if m.chosen == nil || m.chosen.TmuxPane != "%3" {
		t.Fatalf("Enter should attach to the freshly-opened base window, chosen=%+v", m.chosen)
	}
	if cmd == nil {
		t.Fatal("attaching should return a quit command")
	}
}

// TestEnterOnWindowlessSlotOpenError keeps the view alive on failure: a failed Open reports
// the error and attaches to nothing.
func TestEnterOnWindowlessSlotOpenError(t *testing.T) {
	slot := Row{Kind: RowSlot, SessionID: "slot:proj:spike", TmuxSession: "proj", Repo: "proj", Worktree: "spike", Dir: "/d"}
	orch := &fakeOrch{openErr: errors.New("boom")}
	m := modelWith(t, orch, []Row{slot})

	nm, cmd := m.applyAction(ActionSelect)
	m = nm.(model)
	if m.chosen != nil {
		t.Fatalf("a failed open must not attach, chosen=%+v", m.chosen)
	}
	if m.notice == "" || m.noticeLevel != noticeError {
		t.Fatalf("a failed open should report an error notice, notice=%q level=%v", m.notice, m.noticeLevel)
	}
	if cmd != nil {
		t.Fatal("a failed open should not quit")
	}
}

func TestDeleteAnchorBlocked(t *testing.T) {
	orch := &fakeOrch{}
	m := modelWith(t, orch, []Row{{Kind: RowAnchor, SessionID: "anchor:proj", TmuxSession: "proj", Worktree: "main", IsPrimary: true}})
	nm, _ := m.startDelete()
	m = nm.(model)
	if m.mode == modeConfirmDelete {
		t.Fatal("deleting the base must not even open the confirm")
	}
	if len(orch.removes) != 0 {
		t.Fatalf("the anchor must not be removable, removes=%v", orch.removes)
	}
}

// TestDeletePrimaryWithAgentBlocked is the regression for the conflation bug: a recognized
// agent running in the primary worktree is a RowAgent that is still IsPrimary, and must be
// refused at the gate (cleanly) rather than slipping through to a git error.
func TestDeletePrimaryWithAgentBlocked(t *testing.T) {
	orch := &fakeOrch{}
	primaryAgent := Row{Kind: RowAgent, SessionID: "a", TmuxSession: "proj", Repo: "proj", Worktree: "proj", Dir: "/code/proj", IsPrimary: true, TmuxWindow: "0", TmuxPane: "%1"}
	m := modelWith(t, orch, []Row{primaryAgent})
	nm, _ := m.startDelete()
	m = nm.(model)
	if m.mode == modeConfirmDelete {
		t.Fatal("an agent in the base is still the base; delete must not open the confirm")
	}
	if m.notice == "" || m.noticeLevel != noticeError {
		t.Fatalf("blocking the base delete should report why, notice=%q level=%v", m.notice, m.noticeLevel)
	}
	if len(orch.removes) != 0 {
		t.Fatalf("the primary worktree must never be removed, removes=%v", orch.removes)
	}
}

func TestDeleteAlwaysConfirms(t *testing.T) {
	orch := &fakeOrch{dirty: false}
	m := modelWith(t, orch, []Row{{Kind: RowAgent, SessionID: "feat", TmuxSession: "proj", Worktree: "feat", Dir: "/d"}})

	// dd opens a confirmation and removes nothing yet — the safety gate.
	nm, _ := m.startDelete()
	m = nm.(model)
	if m.mode != modeConfirmDelete {
		t.Fatalf("delete should open a confirm before acting, mode=%v", m.mode)
	}
	if len(orch.removes) != 0 {
		t.Fatalf("no removal should happen before confirming, got %+v", orch.removes)
	}

	// Confirming a clean worktree removes it once, unforced.
	nm, _ = m.handleConfirmKey(key("y"))
	m = nm.(model)
	if m.mode != modeNormal {
		t.Fatalf("confirm should return to normal, mode=%v", m.mode)
	}
	if len(orch.removes) != 1 || orch.removes[0].force {
		t.Fatalf("confirm should call Remove(force=false) once, got %+v", orch.removes)
	}
}

// TestDeleteRefusedWithCoTenant pins the sole-occupant guard: invoking delete on a row
// whose worktree also hosts another agent is refused before the confirmation, removing
// nothing, so a worktree is never deleted out from under a co-located agent. A sole
// occupant of the same worktree proceeds to the confirmation as normal.
func TestDeleteRefusedWithCoTenant(t *testing.T) {
	const gd = "/code/proj/.git"
	// Two agents share the feat worktree.
	coTenants := []Row{
		{Kind: RowAgent, SessionID: "a1", TmuxSession: "proj-x", Worktree: "feat", Dir: "/code/proj.worktrees/feat", GitDir: gd, TmuxWindow: "1", TmuxPane: "%2"},
		{Kind: RowAgent, SessionID: "a2", TmuxSession: "proj-x", Worktree: "feat", Dir: "/code/proj.worktrees/feat", GitDir: gd, TmuxWindow: "2", TmuxPane: "%3"},
	}
	orch := &fakeOrch{}
	m := modelWith(t, orch, coTenants)
	nm, _ := m.startDelete() // cursor on the first co-tenant
	m = nm.(model)
	if m.mode != modeNormal {
		t.Fatalf("delete with a co-tenant must not open the confirmation, mode=%v", m.mode)
	}
	if m.notice == "" || m.noticeLevel != noticeError {
		t.Fatalf("delete with a co-tenant should report a refusal, notice=%q level=%v", m.notice, m.noticeLevel)
	}
	if len(orch.removes) != 0 {
		t.Fatalf("delete with a co-tenant must remove nothing, got %+v", orch.removes)
	}

	// A sole occupant of its worktree proceeds to the confirmation.
	sole := []Row{{Kind: RowAgent, SessionID: "s", TmuxSession: "proj-x", Worktree: "solo", Dir: "/code/proj.worktrees/solo", GitDir: gd, TmuxWindow: "1", TmuxPane: "%9"}}
	m2 := modelWith(t, orch, sole)
	nm2, _ := m2.startDelete()
	m2 = nm2.(model)
	if m2.mode != modeConfirmDelete {
		t.Fatalf("delete of a sole occupant should open the confirmation, mode=%v", m2.mode)
	}
}

func TestDeleteConfirmCancelDoesNothing(t *testing.T) {
	orch := &fakeOrch{dirty: false}
	m := modelWith(t, orch, []Row{{Kind: RowAgent, SessionID: "feat", TmuxSession: "proj", Worktree: "feat", Dir: "/d"}})
	nm, _ := m.startDelete()
	m = nm.(model)
	nm, _ = m.handleConfirmKey(key("n"))
	m = nm.(model)
	if m.mode != modeNormal || len(orch.removes) != 0 {
		t.Fatalf("cancel must not remove anything, mode=%v removes=%v", m.mode, orch.removes)
	}
}

// TestDeleteConfirmDefaultsToCancel verifies the selectable buttons start on cancel (the
// safe default) and that ⏎ there cancels without removing anything.
func TestDeleteConfirmDefaultsToCancel(t *testing.T) {
	orch := &fakeOrch{dirty: false}
	m := modelWith(t, orch, []Row{{Kind: RowAgent, SessionID: "feat", TmuxSession: "proj", Worktree: "feat", Dir: "/d"}})
	nm, _ := m.startDelete()
	m = nm.(model)
	if m.confirmChoice != 0 {
		t.Fatalf("confirm should default to cancel (0), got %d", m.confirmChoice)
	}
	nm, _ = m.handleConfirmKey(key("enter"))
	m = nm.(model)
	if m.mode != modeNormal || len(orch.removes) != 0 {
		t.Fatalf("enter on the cancel button must not remove, mode=%v removes=%v", m.mode, orch.removes)
	}
}

// TestDeleteConfirmSelectThenEnter moves the selection onto the confirm button and
// activates it with ⏎ (no y keystroke), exercising the navigable-button path.
func TestDeleteConfirmSelectThenEnter(t *testing.T) {
	orch := &fakeOrch{dirty: false}
	m := modelWith(t, orch, []Row{{Kind: RowAgent, SessionID: "feat", TmuxSession: "proj", Worktree: "feat", Dir: "/d"}})
	nm, _ := m.startDelete()
	m = nm.(model)
	nm, _ = m.handleConfirmKey(key("right"))
	m = nm.(model)
	if m.confirmChoice != 1 {
		t.Fatalf("right should move to the confirm button, got %d", m.confirmChoice)
	}
	nm, _ = m.handleConfirmKey(key("enter"))
	m = nm.(model)
	if m.mode != modeNormal || len(orch.removes) != 1 || orch.removes[0].force {
		t.Fatalf("enter on confirm should Remove(force=false) once, mode=%v removes=%+v", m.mode, orch.removes)
	}
}

func TestDeleteDirtyWorktreeEscalatesToForce(t *testing.T) {
	orch := &fakeOrch{dirty: true}
	m := modelWith(t, orch, []Row{{Kind: RowAgent, SessionID: "feat", TmuxSession: "proj", Worktree: "feat", Dir: "/d"}})

	nm, _ := m.startDelete()
	m = nm.(model)
	// Confirming a dirty worktree declines the unforced remove and escalates.
	nm, _ = m.handleConfirmKey(key("y"))
	m = nm.(model)
	if m.mode != modeConfirmForce {
		t.Fatalf("a dirty worktree should escalate to the force confirm, mode=%v", m.mode)
	}
	if len(orch.removes) != 1 || orch.removes[0].force {
		t.Fatalf("the escalation should come from a single unforced probe, got %+v", orch.removes)
	}

	// Cancelling the force confirm leaves the worktree intact.
	cancel, _ := m.handleConfirmForceKey(key("n"))
	mc := cancel.(model)
	if mc.mode != modeNormal || len(orch.removes) != 1 {
		t.Fatalf("cancel should not force-remove, mode=%v removes=%v", mc.mode, orch.removes)
	}

	// Confirming the force confirm forces removal.
	confirm, _ := m.handleConfirmForceKey(key("y"))
	mf := confirm.(model)
	if mf.mode != modeNormal {
		t.Fatalf("force confirm should return to normal, mode=%v", mf.mode)
	}
	last := orch.removes[len(orch.removes)-1]
	if !last.force {
		t.Fatalf("force confirm should call Remove(force=true), got %+v", orch.removes)
	}
}

// TestNewSessionFocusesCreatedSession drives the create-session flow: pressing the action
// runs the picker (a command, since it must release the terminal), and the resulting
// sessionCreatedMsg reloads and moves the cursor onto the just-opened session.
func TestNewSessionFocusesCreatedSession(t *testing.T) {
	orch := &fakeOrch{newSession: "proj2"}
	rows := []Row{
		{Kind: RowAgent, SessionID: "a", TmuxSession: "proj1", Title: "a"},
		{Kind: RowAgent, SessionID: "b", TmuxSession: "proj2", Title: "b"},
	}
	m := modelWith(t, orch, rows)

	// Cursor starts on the first session; opening must hand off to the picker via a command.
	nm, cmd := m.startNewSession()
	m = nm.(model)
	if cmd == nil {
		t.Fatal("startNewSession should return a command to run the picker")
	}

	// The picker's completion message lands in update; it should focus proj2.
	out, _ := m.update(sessionCreatedMsg{name: "proj2"})
	m = out.(model)
	if got, _ := m.currentRow(); got.TmuxSession != "proj2" {
		t.Fatalf("cursor should land on the opened session, got %q", got.TmuxSession)
	}
	if m.notice == "" || m.noticeLevel != noticeInfo {
		t.Fatalf("opening a session should set an info notice, got %q level=%v", m.notice, m.noticeLevel)
	}
}

// TestNewSessionCancelIsQuiet checks that a cancelled picker (empty name) leaves the view
// untouched: no cursor move, no notice.
func TestNewSessionCancelIsQuiet(t *testing.T) {
	orch := &fakeOrch{newSession: ""}
	rows := []Row{
		{Kind: RowAgent, SessionID: "a", TmuxSession: "proj1", Title: "a"},
		{Kind: RowAgent, SessionID: "b", TmuxSession: "proj2", Title: "b"},
	}
	m := modelWith(t, orch, rows)
	m.cursor = 1

	out, _ := m.update(sessionCreatedMsg{name: ""})
	m = out.(model)
	if m.cursor != 1 {
		t.Fatalf("a cancelled open should not move the cursor, got %d", m.cursor)
	}
	if m.notice != "" {
		t.Fatalf("a cancelled open should be quiet, got notice %q", m.notice)
	}
}

// TestNewSessionDisabledWithoutOrch verifies the action is inert without an orchestrator.
func TestNewSessionDisabledWithoutOrch(t *testing.T) {
	m, err := newModel(fixedRows{[]Row{{Kind: RowAgent, SessionID: "a", TmuxSession: "p", Title: "a"}}}, nil, DefaultKeymap(), 0)
	if err != nil {
		t.Fatal(err)
	}
	_, cmd := m.startNewSession()
	if cmd != nil {
		t.Fatal("startNewSession should be a no-op without an orchestrator")
	}
}
