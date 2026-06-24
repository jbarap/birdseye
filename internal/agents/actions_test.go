package agents

import "testing"

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
	dirty       bool   // when true, Remove(force=false) reports ErrWorktreeDirty
	newSessions int    // count of NewSession calls
	newSession  string // session name NewSession returns
}

func (f *fakeOrch) Spawn(repo Row, branch, name string) error {
	f.spawns = append(f.spawns, spawnCall{repo, branch, name})
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

func modelWith(t *testing.T, orch Orchestrator, rows []Row) model {
	t.Helper()
	m, err := newModel(fixedRows{rows}, nil, DefaultKeymap(), 0)
	if err != nil {
		t.Fatal(err)
	}
	m.orch = orch
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
	repo := Row{Kind: RowAgent, SessionID: "feat", TmuxSession: "proj", Title: "feat", Managed: true, Dir: "/code/proj/feat", Worktree: "feat"}
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
	repo := Row{Kind: RowAgent, SessionID: "feat", TmuxSession: "proj", Title: "feat", Managed: true, Dir: "/code/proj/feat", Worktree: "feat"}
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
	m := modelWith(t, orch, []Row{{Kind: RowAgent, Managed: true, Dir: "/d", TmuxSession: "proj", Title: "t"}})
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
	row := Row{Kind: RowAgent, SessionID: "feat", TmuxSession: "proj", Worktree: "feat", Managed: true, Dir: "/d", TmuxWindow: "1", TmuxPane: "%2"}
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
// there is no tmux window to close, so the orchestrator is never invoked.
func TestCloseWindowlessSlotIsNoop(t *testing.T) {
	orch := &fakeOrch{}
	m := modelWith(t, orch, []Row{{Kind: RowSlot, SessionID: "slot:proj:spike", TmuxSession: "proj", Worktree: "spike", Managed: true, Dir: "/d"}})
	nm, _ := m.startClose()
	m = nm.(model)
	if len(orch.closes) != 0 {
		t.Fatalf("closing a windowless slot should be a no-op, got %+v", orch.closes)
	}
}

func TestDeleteAnchorBlocked(t *testing.T) {
	orch := &fakeOrch{}
	m := modelWith(t, orch, []Row{{Kind: RowAnchor, SessionID: "anchor:proj", TmuxSession: "proj", Worktree: "main", Managed: true}})
	nm, _ := m.startDelete()
	m = nm.(model)
	if len(orch.removes) != 0 {
		t.Fatalf("the anchor must not be removable, removes=%v", orch.removes)
	}
}

func TestDeleteAlwaysConfirms(t *testing.T) {
	orch := &fakeOrch{dirty: false}
	m := modelWith(t, orch, []Row{{Kind: RowAgent, SessionID: "feat", TmuxSession: "proj", Worktree: "feat", Managed: true, Dir: "/d"}})

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

func TestDeleteConfirmCancelDoesNothing(t *testing.T) {
	orch := &fakeOrch{dirty: false}
	m := modelWith(t, orch, []Row{{Kind: RowAgent, SessionID: "feat", TmuxSession: "proj", Worktree: "feat", Managed: true, Dir: "/d"}})
	nm, _ := m.startDelete()
	m = nm.(model)
	nm, _ = m.handleConfirmKey(key("n"))
	m = nm.(model)
	if m.mode != modeNormal || len(orch.removes) != 0 {
		t.Fatalf("cancel must not remove anything, mode=%v removes=%v", m.mode, orch.removes)
	}
}

func TestDeleteDirtyWorktreeEscalatesToForce(t *testing.T) {
	orch := &fakeOrch{dirty: true}
	m := modelWith(t, orch, []Row{{Kind: RowAgent, SessionID: "feat", TmuxSession: "proj", Worktree: "feat", Managed: true, Dir: "/d"}})

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
