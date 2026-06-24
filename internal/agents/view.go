package agents

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/jbarap/birdseye/internal/theme"
)

// defaultRefresh is the fallback live-refresh interval when none is configured.
const defaultRefresh = time.Second

// noticeTTL is how long a transient notice stays before it auto-dismisses. Long enough
// to read a short line, short enough that a stale error doesn't linger.
const noticeTTL = 4 * time.Second

// Layout constants. Column widths keep every row's data points at fixed positions;
// the preview is only shown when the terminal is at least previewMinWidth wide.
const (
	cursorColWidth  = 2  // leftmost gutter: cursor glyph on the selected row, else blank
	statusColWidth  = 9  // " G word  " — status glyph + 4-char word, pinned across rows
	agentIndent     = 6  // a row's columns sit this far right of the session bar's label
	windowColWidth  = 11 // gray window-label column
	nameColWidth    = 22 // white name column
	previewMinWidth = 92
	minPreviewCols  = 24
)

// rowContentWidth is a row's width after the cursor column. Session bars and the
// selected-row highlight span exactly this, so their backgrounds line up.
const rowContentWidth = statusColWidth + agentIndent + windowColWidth + nameColWidth

// Glyphs that live alongside the status glyphs (colors come from theme; these marks
// are not colors, so they stay here next to statusGlyph by convention).
const (
	anchorGlyph  = "⌂" // a managed repo's default-branch checkout (no agent)
	anchorWord   = "base"
	slotGlyph    = "◌" // a managed worktree with no agent (a spawn target)
	slotWord     = "slot"
	managedGlyph = "\U000f160e" // 󱘎 the managed-repo indicator
	errGlyph     = "✗"          // a rejected/failed action (notice, red)
	infoGlyph    = "•"          // a neutral confirmation (notice, accent)
)

// lipColor adapts a shared-palette color to lipgloss, handing it the hex value;
// lipgloss performs its own profile-based downgrade (truecolor → 256 → 16).
func lipColor(c theme.Color) lipgloss.Color { return lipgloss.Color(c.Hex) }

var statusStyle = map[Status]lipgloss.Style{
	StatusNeedsAttention: lipgloss.NewStyle().Bold(true).Foreground(lipColor(theme.Red)),
	StatusWorking:        lipgloss.NewStyle().Foreground(lipColor(theme.Gold)),
	StatusIdle:           lipgloss.NewStyle().Foreground(lipColor(theme.Blue)),
	StatusDone:           lipgloss.NewStyle().Foreground(lipColor(theme.Gray)),
	StatusUnknown:        lipgloss.NewStyle().Faint(true),
}

// markerStyle renders the anchor/slot gutter tokens — gray, like done/unknown, since
// they are structural markers rather than live statuses.
var markerStyle = lipgloss.NewStyle().Foreground(lipColor(theme.Gray))

// statusGlyph and statusWord render a status in the gutter as a glyph plus a 4-char
// word, so statuses stay distinguishable on terminals without color.
var statusGlyph = map[Status]string{
	StatusNeedsAttention: "●",
	StatusWorking:        "◐",
	StatusIdle:           "○",
	StatusDone:           "✓",
	StatusUnknown:        "·",
}

var statusWord = map[Status]string{
	StatusNeedsAttention: "attn",
	StatusWorking:        "work",
	StatusIdle:           "idle",
	StatusDone:           "done",
	StatusUnknown:        "unkn",
}

var (
	frameStyle       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipColor(theme.Border)).Padding(0, 1)
	helpStyle        = lipgloss.NewStyle().Faint(true).MarginTop(1)
	placeholderStyle = lipgloss.NewStyle().Faint(true).Italic(true)

	sessionBarStyle = lipgloss.NewStyle().Bold(true).Foreground(lipColor(theme.SessionFg)).Background(lipColor(theme.SessionBg))
	// sessionBarCountStyle is the bar's trailing worktree count: same bar background,
	// but faint and unbolded so it reads as a subordinate annotation, not a heading.
	sessionBarCountStyle = lipgloss.NewStyle().Faint(true).Foreground(lipColor(theme.SessionFg)).Background(lipColor(theme.SessionBg))
	windowColStyle       = lipgloss.NewStyle().Foreground(lipColor(theme.Gray))
	nameColStyle         = lipgloss.NewStyle().Foreground(lipColor(theme.Text))
	rowHL                = lipColor(theme.RowHL)
)

// cursorGlyphStyle renders the cursor indicator in the accent color; it takes the
// model's accent so the cursor and the panel titles always match and stay
// user-configurable.
func cursorGlyphStyle(accent lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(accent)
}

// panelTitle rewrites a frame's top border so the title sits embedded in it,
// left-aligned and accent-colored — the tool-wide titled-panel look (see DESIGN.md).
// It takes an already-rendered frameStyle box, measures its outer width, and rebuilds
// only the first line from the rounded-border runes (so it tracks whatever frameStyle
// draws). An over-long title is truncated; a panel too narrow for any title keeps its
// plain top border.
func panelTitle(box, title string, accent lipgloss.Color) string {
	lines := strings.Split(box, "\n")
	if len(lines) == 0 {
		return box
	}
	inner := lipgloss.Width(box) - 2 // cells between the two corners
	const lead = 1                   // dashes before the title
	maxTitle := inner - lead - 2 - 1 // leave the two pad spaces and ≥1 trailing dash
	if maxTitle < 1 {
		return box
	}
	label := " " + truncate(title, maxTitle) + " "
	trail := inner - lead - lipgloss.Width(label)
	if trail < 1 {
		trail = 1
	}
	rb := lipgloss.RoundedBorder()
	bc := lipgloss.NewStyle().Foreground(lipColor(theme.Border))
	tc := lipgloss.NewStyle().Foreground(accent).Bold(true)
	lines[0] = bc.Render(rb.TopLeft+strings.Repeat(rb.Top, lead)) +
		tc.Render(label) +
		bc.Render(strings.Repeat(rb.Top, trail)+rb.TopRight)
	return strings.Join(lines, "\n")
}

// ResolveAccent turns a configured accent (a #rrggbb hex, or "" for the built-in
// default) into the color used for the title and cursor. A malformed value is an
// error, mirroring how an invalid keymap is reported rather than silently ignored.
func ResolveAccent(hex string) (lipgloss.Color, error) {
	if hex == "" {
		return lipColor(theme.Accent), nil
	}
	if !validHex(hex) {
		return "", fmt.Errorf("invalid agents.accent %q: want a #rrggbb hex color", hex)
	}
	return lipgloss.Color(hex), nil
}

// validHex reports whether s is a #rrggbb color literal.
func validHex(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

// helpOrder is the order actions appear in the help line.
var helpOrder = []Action{ActionDown, ActionUp, ActionTop, ActionBottom, ActionPrevSection, ActionNextSection, ActionFold, ActionNewSession, ActionNewAgent, ActionClose, ActionDelete, ActionSelect, ActionQuit}

var actionLabel = map[Action]string{
	ActionUp:          "up",
	ActionDown:        "down",
	ActionTop:         "top",
	ActionBottom:      "bottom",
	ActionHalfUp:      "half-up",
	ActionHalfDown:    "half-down",
	ActionPrevSection: "prev",
	ActionNextSection: "next session",
	ActionSelect:      "jump",
	ActionFold:        "fold",
	ActionNewSession:  "open",
	ActionNewAgent:    "new",
	ActionClose:       "close",
	ActionDelete:      "delete",
	ActionQuit:        "quit",
}

// tickMsg drives the periodic live refresh.
type tickMsg time.Time

// mode is the model's interaction mode: normal navigation, the new-agent form, the
// delete confirmation, or the dirty-worktree force escalation.
type mode int

const (
	modeNormal mode = iota
	modeNewAgent
	modeConfirmDelete // confirm any delete before it happens
	modeConfirmForce  // a worktree was dirty: confirm a forced (changes-discarding) remove
)

// formField identifies a field in the new-agent form. Tab cycles between them.
type formField int

const (
	fieldBranch formField = iota
	fieldWorktree
	numFormFields
)

// noticeLevel selects how a transient notice is styled. The zero value is an
// error/rejection (the common case), so a notice set without a level still reads as
// notable rather than silently neutral.
type noticeLevel int

const (
	noticeError noticeLevel = iota // a rejection or failure: red, "✗"
	noticeInfo                     // a neutral confirmation: accent, "•"
)

// model is the Bubble Tea model for the agents view.
type model struct {
	src     RowSource
	prev    Previewer
	keys    Keymap
	res     *resolver
	refresh time.Duration
	orch    Orchestrator // optional; nil disables n/d

	rows    []Row           // leaf rows in render order
	items   []renderItem    // full tree: headers + leaf rows interleaved
	folded  map[string]bool // session keys whose section is collapsed
	nav     []int           // item indices the cursor can land on, given fold state
	cursor  int             // index into nav (a leaf, or a folded session header)
	top     int             // first visible item (scroll offset into the rendered list)
	chosen  *Row
	pending string // armed first key of a chord ("" = none)

	mode           mode
	branchInput    string      // new-agent branch field (modeNewAgent)
	worktreeInput  string      // new-agent worktree-dir field; auto-derived from the branch
	worktreeEdited bool        // the user edited the worktree field, so stop auto-deriving it
	focusField     formField   // which new-agent field has focus (fieldBranch / fieldWorktree)
	target         Row         // the row the active modal acts on (new-agent repo / delete confirm)
	notice         string      // transient status line (feedback from actions)
	noticeLevel    noticeLevel // how to style the notice (error vs neutral info)
	noticeGen      int         // bumped each time a notice is set, so a stale auto-dismiss no-ops

	width, height int
	preview       string         // cached preview for the selected row
	accent        lipgloss.Color // title + cursor color (configurable)
	err           error
}

// newModel builds the initial model, loading rows and the first preview.
func newModel(src RowSource, prev Previewer, keys Keymap, refresh time.Duration) (model, error) {
	res, err := keys.buildResolver()
	if err != nil {
		return model{}, err
	}
	if refresh <= 0 {
		refresh = defaultRefresh
	}
	m := model{src: src, prev: prev, keys: keys, res: res, refresh: refresh, folded: map[string]bool{}, accent: lipColor(theme.Accent)}
	list, err := src.Rows()
	if err != nil {
		return model{}, err
	}
	m.setRows(list)
	m.refreshPreview()
	return m, nil
}

// setRows arranges the source's rows into the session render order, caches the
// interleaved items, drops fold state for sessions that are gone, and recomputes the
// navigable rows.
func (m *model) setRows(list []Row) {
	m.rows, m.items = groupRows(list)
	m.pruneFolded()
	m.recomputeNav()
}

// pruneFolded keeps fold state only for sessions still present, so the map does
// not grow without bound as sessions come and go.
func (m *model) pruneFolded() {
	if len(m.folded) == 0 {
		return
	}
	present := map[string]bool{}
	for _, it := range m.items {
		if it.kind == kindSession {
			present[it.sessionKey] = true
		}
	}
	for key := range m.folded {
		if !present[key] {
			delete(m.folded, key)
		}
	}
}

// recomputeNav rebuilds the list of item indices the cursor may land on: every leaf
// of an unfolded session, plus the header of each folded session (its single
// navigable stand-in). Window headers are never navigable.
func (m *model) recomputeNav() {
	m.nav = m.nav[:0]
	for i, it := range m.items {
		switch it.kind {
		case kindSession:
			if m.folded[it.sessionKey] {
				m.nav = append(m.nav, i)
			}
		case kindRow:
			if !m.folded[it.sessionKey] {
				m.nav = append(m.nav, i)
			}
		}
	}
}

// Run renders the agents view and blocks until the user selects a row or quits. It
// returns the chosen row (nil when quit without selecting). orch may be nil to disable
// the create/delete actions.
func Run(src RowSource, prev Previewer, keys Keymap, accent lipgloss.Color, refresh time.Duration, orch Orchestrator) (*Row, error) {
	m, err := newModel(src, prev, keys, refresh)
	if err != nil {
		return nil, err
	}
	if accent != "" {
		m.accent = accent
	}
	m.orch = orch
	p := tea.NewProgram(m, tea.WithAltScreen())
	out, err := p.Run()
	if err != nil {
		return nil, err
	}
	return out.(model).chosen, nil
}

func (m model) Init() tea.Cmd { return m.tick() }

func (m model) tick() tea.Cmd {
	d := m.refresh
	if d <= 0 {
		d = defaultRefresh
	}
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Update funnels every message through update, then re-syncs the scroll offset so
// the selected row is always within the visible window before the next View. Keeping
// the sync in one place (rather than at each cursor mutation) is what makes scrolling
// rock solid: no message can leave the viewport pointing off-screen.
// setError and setInfo set the transient status line; clearNotice removes it. Each set
// bumps noticeGen so a pending auto-dismiss for an older notice no-ops once a newer one
// (or a clear) supersedes it. These are the single way to touch the notice, so its
// text, level, and generation never drift apart.
func (m *model) setError(msg string) {
	m.notice, m.noticeLevel, m.noticeGen = msg, noticeError, m.noticeGen+1
}
func (m *model) setInfo(msg string) {
	m.notice, m.noticeLevel, m.noticeGen = msg, noticeInfo, m.noticeGen+1
}
func (m *model) clearNotice() { m.notice, m.noticeLevel, m.noticeGen = "", noticeError, m.noticeGen+1 }

// noticeExpireMsg auto-dismisses the notice of a given generation after noticeTTL.
type noticeExpireMsg struct{ gen int }

// expireNotice schedules the auto-dismiss for the notice generation gen.
func expireNotice(gen int) tea.Cmd {
	return tea.Tick(noticeTTL, func(time.Time) tea.Msg { return noticeExpireMsg{gen} })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	prevGen := m.noticeGen
	next, cmd := m.update(msg)
	mm, ok := next.(model)
	if !ok {
		return next, cmd
	}
	// A freshly-set notice arms its own auto-dismiss; the gen tag means a later notice
	// (or a clear) supersedes this timer instead of blanking the new message.
	if mm.notice != "" && mm.noticeGen != prevGen {
		cmd = tea.Batch(cmd, expireNotice(mm.noticeGen))
	}
	mm.syncViewport()
	return mm, cmd
}

func (m model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		m.reload()
		return m, m.tick()
	case noticeExpireMsg:
		if msg.gen == m.noticeGen {
			m.clearNotice()
		}
		return m, nil
	case sessionCreatedMsg:
		if msg.err != nil {
			m.setError("open session: " + msg.err.Error())
			return m, nil
		}
		if msg.name != "" {
			m.reload()
			m.focusSession(msg.name)
			m.refreshPreview()
			m.setInfo("opened " + msg.name)
		}
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// reloadAfterAction drops any cached git facts (so a just-created or just-removed
// worktree is seen) and then reloads. Used after spawn/remove; the periodic tick uses
// the plain reload so it stays a cache lookup rather than a git call.
func (m *model) reloadAfterAction() {
	if inv, ok := m.src.(Invalidator); ok {
		inv.Invalidate()
	}
	m.reload()
}

// reload re-reads rows through the source, preserving the selected row by identity,
// and refreshes the preview.
func (m *model) reload() {
	list, err := m.src.Rows()
	if err != nil {
		m.err = err
		return
	}
	m.err = nil
	selKey := m.currentRowKey()
	m.setRows(list)
	m.cursor = relocate(m, selKey, m.cursor)
	m.refreshPreview()
}

// rowKey identifies a navigable row by what it represents — a leaf (by its row id) or
// a folded session header (by session key) — so selection can follow that row across a
// refresh even as ordering or fold state shifts.
func (m model) rowKey(itemIdx int) string {
	it := m.items[itemIdx]
	switch it.kind {
	case kindRow:
		return "r:" + m.rows[it.rowIdx].SessionID
	case kindSession:
		return "s:" + it.sessionKey
	}
	return ""
}

// currentRowKey is the rowKey of the row under the cursor, or "" when none.
func (m model) currentRowKey() string {
	if m.cursor < 0 || m.cursor >= len(m.nav) {
		return ""
	}
	return m.rowKey(m.nav[m.cursor])
}

// relocate finds the row matching selKey in the rebuilt nav, else clamps the old
// cursor into bounds.
func relocate(m *model, selKey string, old int) int {
	if len(m.nav) == 0 {
		return 0
	}
	if selKey != "" {
		for i, itemIdx := range m.nav {
			if m.rowKey(itemIdx) == selKey {
				return i
			}
		}
	}
	if old < 0 {
		return 0
	}
	if old >= len(m.nav) {
		return len(m.nav) - 1
	}
	return old
}

// currentItem returns the item under the cursor.
func (m model) currentItem() (renderItem, bool) {
	if m.cursor < 0 || m.cursor >= len(m.nav) {
		return renderItem{}, false
	}
	return m.items[m.nav[m.cursor]], true
}

// currentRow returns the row the cursor points at: the selected leaf, or the
// first row of a folded session whose header is selected. The second case lets the
// preview and jump still target something useful while folded.
func (m model) currentRow() (Row, bool) {
	it, ok := m.currentItem()
	if !ok {
		return Row{}, false
	}
	switch it.kind {
	case kindRow:
		return m.rows[it.rowIdx], true
	case kindSession:
		return m.firstRowOfSession(it.sessionKey)
	}
	return Row{}, false
}

// firstRowOfSession returns the first row (in render order) of a session.
func (m model) firstRowOfSession(key string) (Row, bool) {
	for _, it := range m.items {
		if it.kind == kindRow && it.sessionKey == key {
			return m.rows[it.rowIdx], true
		}
	}
	return Row{}, false
}

// refreshPreview captures the selected row's pane preview, caching the result. Any row
// with a tmux location (agent, anchor, or slot) can be previewed.
func (m *model) refreshPreview() {
	if m.prev == nil {
		m.preview = ""
		return
	}
	r, ok := m.currentRow()
	if !ok || r.TmuxSession == "" {
		m.preview = ""
		return
	}
	out, err := m.prev.Preview(Agent{
		SessionID:   r.SessionID,
		TmuxSession: r.TmuxSession,
		TmuxWindow:  r.TmuxWindow,
		TmuxPane:    r.TmuxPane,
	})
	if err != nil {
		m.preview = ""
		return
	}
	m.preview = out
}

func (m model) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeNewAgent:
		return m.handleNewAgentKey(key)
	case modeConfirmDelete:
		return m.handleConfirmKey(key)
	case modeConfirmForce:
		return m.handleConfirmForceKey(key)
	}

	k := key.String()

	// Complete a chord (e.g. the second key of gg or dD).
	if m.pending != "" {
		armed := m.pending
		m.pending = ""
		if a, ok := m.res.chords[[2]string{armed, k}]; ok {
			return m.applyAction(a)
		}
		// Not a completing key: fall through and handle k as its own binding.
	}

	// Arm a chord on its first key.
	if m.res.prefix[k] {
		m.pending = k
		return m, nil
	}

	if a, ok := m.res.single[k]; ok {
		return m.applyAction(a)
	}
	return m, nil
}

func (m model) applyAction(a Action) (tea.Model, tea.Cmd) {
	old := m.cursor
	last := len(m.nav) - 1
	switch a {
	case ActionUp:
		if m.cursor > 0 {
			m.cursor--
		}
	case ActionDown:
		if m.cursor < last {
			m.cursor++
		}
	case ActionTop:
		m.cursor = 0
	case ActionBottom:
		if last >= 0 {
			m.cursor = last
		}
	case ActionHalfUp:
		m.cursor = clamp(m.cursor-m.halfPage(), 0, max(last, 0))
	case ActionHalfDown:
		m.cursor = clamp(m.cursor+m.halfPage(), 0, max(last, 0))
	case ActionPrevSection:
		m.cursor = m.prevSection()
	case ActionNextSection:
		m.cursor = m.nextSection()
	case ActionFold:
		return m.toggleFold()
	case ActionNewSession:
		return m.startNewSession()
	case ActionNewAgent:
		return m.startNewAgent()
	case ActionClose:
		return m.startClose()
	case ActionDelete:
		return m.startDelete()
	case ActionSelect:
		if r, ok := m.currentRow(); ok {
			m.chosen = &r
		}
		return m, tea.Quit
	case ActionQuit:
		return m, tea.Quit
	}
	if m.cursor != old {
		m.dismissNotice()
		m.refreshPreview()
	}
	return m, nil
}

// dismissNotice clears a showing notice when the user moves on (navigation, fold), so
// feedback fades the moment it is acknowledged — without churning the generation when
// there is nothing to clear.
func (m *model) dismissNotice() {
	if m.notice != "" {
		m.clearNotice()
	}
}

// toggleFold collapses or expands the section under the cursor. Folding a section
// from one of its rows leaves the cursor on the now-collapsed session header;
// unfolding from that header drops the cursor onto the section's first row.
func (m model) toggleFold() (tea.Model, tea.Cmd) {
	it, ok := m.currentItem()
	if !ok {
		return m, nil
	}
	key := it.sessionKey
	m.folded[key] = !m.folded[key]
	m.recomputeNav()
	m.cursor = m.navIndexForSession(key)
	m.dismissNotice()
	m.refreshPreview()
	return m, nil
}

// focusSession moves the cursor onto the named session (its header when folded, else its
// first row) when it is present, and leaves the cursor put otherwise. Used after opening a
// new session so the eye lands on what was just created.
func (m *model) focusSession(name string) {
	for i, itemIdx := range m.nav {
		if m.items[itemIdx].sessionKey == name {
			m.cursor = i
			return
		}
	}
}

// navIndexForSession returns the cursor position of a session's navigable row: its
// header when folded, otherwise its first row.
func (m model) navIndexForSession(key string) int {
	for i, itemIdx := range m.nav {
		it := m.items[itemIdx]
		if it.sessionKey == key && (it.kind == kindSession || it.kind == kindRow) {
			return i
		}
	}
	return clamp(m.cursor, 0, max(len(m.nav)-1, 0))
}

// sectionStarts returns the nav indices at which each session section begins.
// Sections are contiguous runs of navigable rows sharing a session key.
func (m model) sectionStarts() []int {
	var starts []int
	prev := ""
	for i, itemIdx := range m.nav {
		key := m.items[itemIdx].sessionKey
		if i == 0 || key != prev {
			starts = append(starts, i)
		}
		prev = key
	}
	return starts
}

// sectionStart is the nav index where the cursor's current section begins.
func (m model) sectionStart() int {
	cur := 0
	for _, s := range m.sectionStarts() {
		if s <= m.cursor {
			cur = s
		} else {
			break
		}
	}
	return cur
}

// nextSection moves to the first row of the following section, or the last row
// when already in the final section (mirroring vim's `}` at end of buffer).
func (m model) nextSection() int {
	if len(m.nav) == 0 {
		return 0
	}
	cur := m.sectionStart()
	for _, s := range m.sectionStarts() {
		if s > cur {
			return s
		}
	}
	return len(m.nav) - 1
}

// prevSection moves to the top of the current section, or — when already there —
// to the top of the previous section (mirroring vim's `{`).
func (m model) prevSection() int {
	if len(m.nav) == 0 {
		return 0
	}
	cur := m.sectionStart()
	if m.cursor > cur {
		return cur
	}
	prev := 0
	for _, s := range m.sectionStarts() {
		if s >= cur {
			break
		}
		prev = s
	}
	return prev
}

// halfPage is half the visible list height, at least 1.
func (m model) halfPage() int {
	page := m.contentRows()
	if page < 2 {
		page = 10
	}
	return max(page/2, 1)
}

// chromeLines counts the rendered lines stacked below the body (list/preview): the
// help line and the transient notice/modal line, each two rows (a top margin plus
// its text). The body must shrink by this much so the whole view never exceeds the
// terminal height — the overflow that scrolls the top off-screen.
func (m model) chromeLines() int {
	n := 0
	if m.modalLineActive() {
		n += 2
	}
	if m.height == 0 || m.height >= 6 {
		n += 2 // the help line (same condition as View)
	}
	return n
}

// modalLineActive reports whether View will render a notice line below the body. The
// new-agent prompt and the delete confirmation are excluded: each renders as a centered
// modal *over* the body (same height), so they reserve no extra chrome.
func (m model) modalLineActive() bool {
	return m.notice != ""
}

// contentRows is the number of body rows that fit inside the panel border given the
// terminal height and the chrome below it. Both the list window and the preview pane
// size to this, so they always line up and never overflow.
func (m model) contentRows() int {
	h := m.height - m.chromeLines() - 2 // the panel's top+bottom border
	if h < 1 {
		h = 1
	}
	return h
}

// listCap is the maximum number of list rows to draw. When the terminal size is not
// yet known (height 0, e.g. before the first WindowSizeMsg or in tests) the list is
// not windowed.
func (m model) listCap() int {
	if m.height <= 0 {
		return 1 << 30
	}
	return m.contentRows()
}

// visibleItems is the item indices that render, in order: every item except rows
// hidden beneath a folded session. This is the sequence the scroll window slides over.
func (m model) visibleItems() []int {
	out := make([]int, 0, len(m.items))
	for i, it := range m.items {
		if it.kind == kindRow && m.folded[it.sessionKey] {
			continue
		}
		out = append(out, i)
	}
	return out
}

// selVisiblePos is the position of the selected item within visibleItems (0 when none).
func (m model) selVisiblePos(vis []int) int {
	if m.cursor < 0 || m.cursor >= len(m.nav) {
		return 0
	}
	target := m.nav[m.cursor]
	for p, i := range vis {
		if i == target {
			return p
		}
	}
	return 0
}

// syncViewport pins the scroll offset so the selected row stays visible, scrolling the
// minimum amount needed (selection at an edge nudges the window by one, not a jump).
func (m *model) syncViewport() {
	vis := m.visibleItems()
	m.top = windowTop(m.top, m.selVisiblePos(vis), m.listCap(), len(vis))
}

// windowTop returns the first-visible index for a list of n items with a window of h
// rows, given the previous top and the selected position: it keeps sel inside
// [top, top+h) with minimal movement and clamps to the valid range.
func windowTop(top, sel, h, n int) int {
	if h <= 0 || n <= h {
		return 0
	}
	if top < 0 {
		top = 0
	}
	if sel < top {
		top = sel
	}
	if sel >= top+h {
		top = sel - h + 1
	}
	if top > n-h {
		top = n - h
	}
	if top < 0 {
		top = 0
	}
	return top
}

func (m model) View() string {
	if len(m.rows) == 0 {
		return m.emptyView()
	}

	list := panelTitle(frameStyle.Render(m.renderRows()), "agents", m.accent)

	body := list
	if m.showPreview() {
		body = lipgloss.JoinHorizontal(lipgloss.Top, list, "  ", m.renderPreview(list))
	}
	if m.mode == modeNewAgent {
		body = overlayCenter(body, m.newAgentModal())
	}
	if m.mode == modeConfirmDelete || m.mode == modeConfirmForce {
		body = overlayCenter(body, m.confirmModal())
	}

	parts := []string{body}
	if line, ok := m.statusLine(); ok {
		parts = append(parts, line)
	}
	if m.height == 0 || m.height >= 6 {
		parts = append(parts, m.renderHelp())
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// statusLine renders the transient notice below the body. The delete confirmation now
// floats as a centered popup over the body (see confirmModal), not as an inline line,
// so only the notice remains here. The notice is bold and color-coded — a rejected
// action or an error can't be missed — and is exactly one row plus its top margin,
// matching the two lines chromeLines reserves so the view never overflows.
func (m model) statusLine() (string, bool) {
	if m.notice != "" {
		return m.renderNotice(), true
	}
	return "", false
}

// newAgentModal renders the new-agent form as a titled, bordered card: it echoes the
// target repo for context and presents two labeled fields — branch and worktree — with the
// focused field carrying the accent and a cursor block. It floats centered over the list
// (see overlayCenter) so the form reads as a focused popup rather than a crowded status
// line.
func (m model) newAgentModal() string {
	width := clamp(m.width/2, 32, 52)
	if m.width > 0 && width > m.width-4 {
		width = m.width - 4
	}
	inner := width - 2 // inside the border's left/right padding (frameStyle pads 0,1)

	label := lipgloss.NewStyle().Foreground(lipColor(theme.Gray))
	hint := lipgloss.NewStyle().Faint(true)

	repo := m.target.TmuxSession
	if repo == "" {
		repo = m.target.Title
	}
	rows := []string{
		label.Render("repo  ") + lipgloss.NewStyle().Foreground(lipColor(theme.Text)).Render(truncate(repo, inner-6)),
		"",
		label.Render("branch"),
		m.formField(m.branchInput, m.focusField == fieldBranch, inner),
		"",
		label.Render("worktree"),
		m.formField(m.worktreeInput, m.focusField == fieldWorktree, inner),
		"",
		hint.Render("⏎ create   ⇥ field   esc cancel"),
	}
	box := frameStyle.Width(width).Render(strings.Join(rows, "\n"))
	return panelTitle(box, "new agent", m.accent)
}

// confirmModal renders the delete confirmation as a centered popup, consistent with the
// new-agent modal (DESIGN.md's single modal style) and replacing the old inline (y/n)
// line. modeConfirmDelete asks before removing a worktree; modeConfirmForce folds the
// dirty-worktree force choice into the same popup, defaulting to cancel.
func (m model) confirmModal() string {
	width := clamp(m.width/2, 32, 52)
	if m.width > 0 && width > m.width-4 {
		width = m.width - 4
	}
	inner := width - 2 // inside the border's left/right padding (frameStyle pads 0,1)

	text := lipgloss.NewStyle().Foreground(lipColor(theme.Text))
	hint := lipgloss.NewStyle().Faint(true)

	var title, prompt, choices string
	switch m.mode {
	case modeConfirmForce:
		title = "force delete"
		prompt = lipgloss.NewStyle().Foreground(lipColor(theme.Red)).Render(
			truncate(fmt.Sprintf("worktree %q has uncommitted changes.", m.target.Worktree), inner))
		choices = hint.Render("y force-remove (discards changes)   n cancel")
	default:
		title = "delete"
		prompt = text.Render(truncate(m.deletePrompt(), inner))
		choices = hint.Render("y confirm   n cancel")
	}
	box := frameStyle.Width(width).Render(strings.Join([]string{prompt, "", choices}, "\n"))
	return panelTitle(box, title, m.accent)
}

// formField renders one input line of the new-agent form. The focused field is accented
// and bold with a trailing cursor block; an unfocused field is plain gray text. An empty
// unfocused field shows a faint dash so the row never looks broken.
func (m model) formField(val string, focused bool, width int) string {
	if focused {
		s := lipgloss.NewStyle().Foreground(m.accent).Bold(true)
		return s.Render(truncate("› "+val+"▌", width))
	}
	if val == "" {
		return lipgloss.NewStyle().Faint(true).Render("  —")
	}
	return lipgloss.NewStyle().Foreground(lipColor(theme.Text)).Render(truncate("  "+val, width))
}

// overlayCenter composites fg centered over bg, splicing fg's cells in place of the
// underlying ones so the modal floats above the list instead of replacing it. bg defines
// the canvas size; each spliced row is cut ANSI-aware (x/ansi) so the background styling
// on either side of the modal survives.
func overlayCenter(bg, fg string) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")
	bgH, bgW := len(bgLines), lipgloss.Width(bg)
	fgH, fgW := len(fgLines), lipgloss.Width(fg)

	top := max((bgH-fgH)/2, 0)
	left := max((bgW-fgW)/2, 0)

	for i, fl := range fgLines {
		row := top + i
		if row < 0 || row >= bgH {
			continue
		}
		bgLine := bgLines[row]
		leftPart := ansi.Truncate(bgLine, left, "")
		if w := lipgloss.Width(leftPart); w < left { // bg line shorter than the cut
			leftPart += strings.Repeat(" ", left-w)
		}
		rightPart := ansi.TruncateLeft(bgLine, left+fgW, "")
		bgLines[row] = leftPart + fl + rightPart
	}
	return strings.Join(bgLines, "\n")
}

// deletePrompt phrases the delete confirmation for the target row: removing a managed
// worktree (which also drops its branch's checkout) reads differently from closing an
// incidental agent window, so the user knows exactly what `y` will do.
func (m model) deletePrompt() string {
	if m.target.isManagedWorktree() {
		return fmt.Sprintf("remove worktree %q and its window?", m.target.Worktree)
	}
	name := displayTitle(m.target.Title, m.target.TmuxSession)
	if name == "" {
		name = "this agent"
	}
	return fmt.Sprintf("close %s's window?", name)
}

// renderNotice styles the transient notice by level: a red "✗" for an error or
// rejection, an accent "•" for neutral info — the glyph carries the meaning so it reads
// without color too (DESIGN.md: never rely on color alone).
func (m model) renderNotice() string {
	glyph, color := errGlyph, lipColor(theme.Red)
	if m.noticeLevel == noticeInfo {
		glyph, color = infoGlyph, m.accent
	}
	s := lipgloss.NewStyle().Bold(true).Foreground(color).MarginTop(1)
	return s.Render(m.fitLine(glyph + " " + m.notice))
}

// fitLine truncates a status line to the terminal width so it stays a single row; a
// wrapped line would add a row the height budget did not reserve and scroll the top
// off-screen.
func (m model) fitLine(s string) string {
	if m.width > 1 {
		return truncate(s, m.width-1)
	}
	return s
}

func (m model) emptyView() string {
	body := placeholderStyle.Render("No agents tracked yet.\n" +
		"Wire up Claude Code hooks (see README) so sessions report status here.")
	panel := panelTitle(frameStyle.Render(body), "agents", m.accent)
	help := "q quit"
	// With orchestration, opening a session is the natural next step from an empty view.
	if m.orch != nil {
		if keys := m.keys[ActionNewSession]; len(keys) > 0 {
			help = prettyKeys(keys) + " open · " + help
		}
	}
	return panel + "\n" + helpStyle.Render(help)
}

// renderKind distinguishes the line types in the grouped list.
type renderKind int

const (
	kindSession renderKind = iota // tmux session header, drawn as a full-width section bar
	kindRow                       // a selectable leaf row (agent, anchor, or slot)
)

// renderItem is one line of the grouped list. A session item is a header (not
// selectable); a leaf item points at an entry in the ordered rows slice and carries
// its window label for the gray window column.
type renderItem struct {
	kind       renderKind
	label      string // session name or "ungrouped" (session header)
	sessionKey string // owning tmux session (its own key for a session header)
	window     string // the row's window/worktree label (gray column)
	count      int    // leaves in the session, for the folded header's count
	managed    bool   // the session is a recognized managed repo (session header)
	worktrees  int    // worktree count, for the managed indicator (session header)
	rowIdx     int    // index into model.rows, for kindRow
}

// windowLabel is how a row's window is shown in the gray column: its tmux window name
// when set, else "win <index>". With neither it is empty.
func windowLabel(index, name string) string {
	if name != "" {
		return name
	}
	if index != "" {
		return "win " + index
	}
	return ""
}

// sessionGroup is one tmux session's rows ("" key is the ungrouped bucket).
type sessionGroup struct {
	key       string
	rows      []Row
	managed   bool
	worktrees int
}

// groupRows arranges rows into a flat session→row list and returns them in render
// order alongside the interleaved header/row items the view draws. Sessions are
// ordered by their most-urgent agent (lowest status rank), then by name. Within a
// session the anchor (if any) is pinned first, agents follow by status rank then
// title, and empty worktree slots sort last. Rows lacking a tmux session collect under
// an "ungrouped" heading.
func groupRows(in []Row) ([]Row, []renderItem) {
	if len(in) == 0 {
		return nil, nil
	}

	var sessions []*sessionGroup
	idx := map[string]*sessionGroup{}
	for _, r := range in {
		sg := idx[r.TmuxSession]
		if sg == nil {
			sg = &sessionGroup{key: r.TmuxSession}
			idx[r.TmuxSession] = sg
			sessions = append(sessions, sg)
		}
		sg.rows = append(sg.rows, r)
		if r.Managed {
			sg.managed = true
		}
	}

	for _, sg := range sessions {
		sort.SliceStable(sg.rows, func(i, j int) bool { return rowLess(sg.rows[i], sg.rows[j]) })
		seen := map[string]bool{}
		for _, r := range sg.rows {
			if r.Worktree != "" && !seen[r.Worktree] {
				seen[r.Worktree] = true
				sg.worktrees++
			}
		}
	}
	sort.SliceStable(sessions, func(i, j int) bool {
		if ri, rj := groupRank(sessions[i].rows), groupRank(sessions[j].rows); ri != rj {
			return ri < rj
		}
		return sessions[i].key < sessions[j].key
	})

	ordered := make([]Row, 0, len(in))
	items := make([]renderItem, 0, len(in)+len(sessions))
	for _, sg := range sessions {
		label := sg.key
		if label == "" {
			label = "ungrouped"
		}
		items = append(items, renderItem{kind: kindSession, label: label, sessionKey: sg.key, count: len(sg.rows), managed: sg.managed, worktrees: sg.worktrees})
		for _, r := range sg.rows {
			ordered = append(ordered, r)
			items = append(items, renderItem{
				kind:       kindRow,
				sessionKey: sg.key,
				window:     rowLabel(r),
				rowIdx:     len(ordered) - 1,
			})
		}
	}
	return ordered, items
}

// rowLabel is the gray-column label for a leaf row: the worktree name when the row is
// a managed worktree (or anchor), else the tmux window label.
func rowLabel(r Row) string {
	if r.Worktree != "" {
		return r.Worktree
	}
	return windowLabel(r.TmuxWindow, r.TmuxWindowName)
}

// rowLess orders rows within a session: the anchor first, then by status rank, with
// empty slots last; ties break by title.
func rowLess(a, b Row) bool {
	if ra, rb := rowRank(a), rowRank(b); ra != rb {
		return ra < rb
	}
	return a.Title < b.Title
}

// rowRank ranks a row for within-session ordering: anchor before everything, slots
// after everything, agents by their status rank in between.
func rowRank(r Row) int {
	switch r.Kind {
	case RowAnchor:
		return -1
	case RowSlot:
		return 100
	default:
		return r.Status.rank()
	}
}

// groupRank is the urgency of a session: the lowest (most-urgent) status rank among
// its live agents. A session with no live agents (e.g. a managed repo of only an
// anchor and slots) sorts as if idle so it sits among quiet sessions.
func groupRank(rows []Row) int {
	r := int(^uint(0) >> 1) // max int
	has := false
	for _, row := range rows {
		if row.Kind != RowAgent {
			continue
		}
		has = true
		if rk := row.Status.rank(); rk < r {
			r = rk
		}
	}
	if !has {
		return StatusIdle.rank()
	}
	return r
}

// renderRows draws the visible window of items: a leftmost cursor column, then the
// session bar or the fixed-column leaf row. Rows under a folded session are skipped,
// and only the slice [top, top+cap) is drawn so the list never overflows the panel.
// The window top is recomputed here (not just trusted from the model) so a View called
// without a prior Update — e.g. directly in a test — still keeps the cursor on-screen.
func (m model) renderRows() string {
	vis := m.visibleItems()
	h := m.listCap()
	top := windowTop(m.top, m.selVisiblePos(vis), h, len(vis))
	end := min(top+h, len(vis))

	selItem := -1
	if m.cursor >= 0 && m.cursor < len(m.nav) {
		selItem = m.nav[m.cursor]
	}
	rows := make([]string, 0, end-top)
	for _, i := range vis[top:end] {
		it := m.items[i]
		selected := i == selItem
		rows = append(rows, m.cursorCol(selected)+m.rowBody(it, selected))
	}
	return strings.Join(rows, "\n")
}

// cursorCol is the leftmost column: the cursor glyph on the selected row, else blank.
// It applies to whatever row the cursor is on, including a folded session header.
func (m model) cursorCol(selected bool) string {
	if selected {
		return cursorGlyphStyle(m.accent).Render(theme.CursorGlyph + " ")
	}
	return strings.Repeat(" ", cursorColWidth)
}

// rowBody renders an item's content to the right of the cursor column.
func (m model) rowBody(it renderItem, selected bool) string {
	switch it.kind {
	case kindSession:
		return m.sessionBar(it)
	case kindRow:
		return m.leafRow(it, selected)
	}
	return ""
}

// sessionBar renders a session header as a full-width bar, left-aligned to the start
// of the list. A managed repo carries the worktree-source indicator and its worktree
// count; a folded section shows a collapsed glyph and its hidden-row count.
func (m model) sessionBar(it renderItem) string {
	name := it.label
	if it.managed {
		name = managedGlyph + " " + it.label
	}
	switch {
	case m.folded[it.sessionKey]:
		return sessionBarStyle.Width(rowContentWidth).Render(truncate(fmt.Sprintf("▸ %s  (%d)", name, it.count), rowContentWidth))
	case it.managed && it.worktrees > 0:
		return m.barWithCount("▾ "+name, fmt.Sprintf("%d wt", it.worktrees))
	default:
		return sessionBarStyle.Width(rowContentWidth).Render(truncate("▾ "+name, rowContentWidth))
	}
}

// barWithCount renders a full-width session bar with left pinned to the start and a
// subtle count pinned to the right edge, over a continuous bar background. When the
// two would not fit, it degrades to a single left-aligned, truncated label.
func (m model) barWithCount(left, count string) string {
	gap := rowContentWidth - lipgloss.Width(left) - lipgloss.Width(count)
	if gap < 1 {
		return sessionBarStyle.Width(rowContentWidth).Render(truncate(left+"  "+count, rowContentWidth))
	}
	return sessionBarStyle.Render(left) +
		sessionBarStyle.Render(strings.Repeat(" ", gap)) +
		sessionBarCountStyle.Render(count)
}

// leafRow renders one row at fixed columns: the status/marker gutter (glyph + word), an
// indent, the gray window/worktree label, then the white name. The selected row carries
// a full-row highlight spanning gutter→name in addition to the cursor glyph.
func (m model) leafRow(it renderItem, selected bool) string {
	r := m.rows[it.rowIdx]
	gutter, st := gutterFor(r)
	indent := strings.Repeat(" ", agentIndent)
	win := padRight(truncate(it.window, windowColWidth-1), windowColWidth)
	name := padRight(truncate(displayTitle(r.Title, it.sessionKey), nameColWidth), nameColWidth)
	if !selected {
		return st.Render(gutter) + indent + windowColStyle.Render(win) + nameColStyle.Render(name)
	}
	hl := lipgloss.NewStyle().Background(rowHL)
	return st.Background(rowHL).Render(gutter) +
		hl.Render(indent) +
		windowColStyle.Background(rowHL).Render(win) +
		nameColStyle.Background(rowHL).Bold(true).Render(name)
}

// gutterFor returns the pinned status/marker gutter for a row and the style to render
// it: a per-status glyph+word for agents, or the gray anchor/slot marker otherwise.
func gutterFor(r Row) (string, lipgloss.Style) {
	switch r.Kind {
	case RowAnchor:
		return " " + anchorGlyph + " " + padRight(anchorWord, 4) + "  ", markerStyle
	case RowSlot:
		return " " + slotGlyph + " " + padRight(slotWord, 4) + "  ", markerStyle
	default:
		return " " + statusGlyph[r.Status] + " " + padRight(statusWord[r.Status], 4) + "  ", statusStyle[r.Status]
	}
}

// displayTitle drops a leading "<session>:" from a row title, since the session is
// already shown by its header — "arewa:birds_eye" under session "arewa" becomes
// "birds_eye". Titles without that prefix (or ungrouped rows) are unchanged.
func displayTitle(title, sessionKey string) string {
	if sessionKey == "" {
		return title
	}
	if rest := strings.TrimPrefix(title, sessionKey+":"); rest != title && rest != "" {
		return rest
	}
	return title
}

func (m model) showPreview() bool {
	if m.prev == nil {
		return false
	}
	if _, isNoop := m.prev.(NoopPreviewer); isNoop {
		return false
	}
	return m.width >= previewMinWidth
}

// renderPreview renders the preview pane beside the list. It fills the leftover
// width and is sized to the available terminal height (not the often-short
// list) so it shows as much of the session's output as the popup allows.
func (m model) renderPreview(list string) string {
	w := m.width - lipgloss.Width(list) - 2 /*gap*/ - 4 /*border+padding*/
	if w < minPreviewCols {
		w = minPreviewCols
	}

	// Inner content height: fill the terminal but leave room for everything
	// stacked around it, so the whole view never exceeds m.height (which would
	// scroll the list out of view). contentRows already nets out this frame's
	// border, the help line, and any active notice/modal line — the list window
	// uses the same budget, so the two panes line up.
	inner := m.contentRows()

	lines := previewLines(m.preview, inner, w)
	if len(lines) == 0 {
		lines = []string{placeholderStyle.Render("(no preview available)")}
	}
	// w is the text width; lipgloss Width includes the frame's horizontal
	// padding, so set Width(w+2) to keep the text area exactly w. Otherwise
	// each w-wide line wraps, inflating the frame height past the terminal.
	box := frameStyle.Width(w + 2).Height(inner).Render(strings.Join(lines, "\n"))
	return panelTitle(box, "preview", m.accent)
}

// previewLines turns captured pane text into at most h display lines of width w.
// Trailing blank lines are dropped first: a pane showing only a shell prompt at
// the top would otherwise present an all-blank tail. The last h non-empty-tail
// lines are kept so a full-screen TUI shows its most recent (bottom) content.
func previewLines(content string, h, w int) []string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return nil
	}
	if len(lines) > h {
		lines = lines[len(lines)-h:]
	}
	out := make([]string, len(lines))
	for i, ln := range lines {
		out[i] = truncate(ln, w)
	}
	return out
}

func (m model) renderHelp() string {
	var parts []string
	for _, a := range helpOrder {
		if (a == ActionNewSession || a == ActionNewAgent || a == ActionClose || a == ActionDelete) && m.orch == nil {
			continue // create/close/delete unavailable without an orchestrator
		}
		keys := m.keys[a]
		if len(keys) == 0 {
			continue
		}
		parts = append(parts, prettyKeys(keys)+" "+actionLabel[a])
	}
	return helpStyle.Render(strings.Join(parts, " · "))
}

// prettyKeys renders up to the first two bound keys for a help hint.
func prettyKeys(keys []string) string {
	out := make([]string, 0, 2)
	for _, k := range keys {
		if len(out) == 2 {
			break
		}
		out = append(out, prettyKey(k))
	}
	return strings.Join(out, "/")
}

func prettyKey(k string) string {
	switch k {
	case "up":
		return "↑"
	case "down":
		return "↓"
	case "enter":
		return "⏎"
	case "tab":
		return "⇥"
	default:
		return k
	}
}

func padRight(s string, n int) string {
	r := []rune(s)
	if len(r) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(r))
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:max(n, 0)])
	}
	return string(r[:n-1]) + "…"
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
