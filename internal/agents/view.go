package agents

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/jbarap/birdseye/internal/providers/dir"
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
	windowColMin    = 11 // worktree/window-label column: baseline, flexes to fit content
	windowColMax    = 28 // ... but never past this
	nameColMin      = 8  // title column: baseline, flexes to fit content
	nameColMax      = 48 // ... but never past this
	previewMinWidth = 92
	minPreviewCols  = 24
)

// defaultSplit mirrors config.DefaultSplit: the agents-list pane's share of the width when
// none is configured. Kept here too so a model built without Run (tests) still lays out
// sensibly.
const defaultSplit = 0.4

// Dual-lens layout. The Agents lens is a fixed-ish column carved off the terminal before
// the Workspaces+preview layout is computed; below dualLensMinWidth only the focused lens
// shows (the narrow-terminal fallback), still toggled by the focus keys.
const (
	agentsNameMin    = 14  // agents-lens title column: baseline, flexes to content
	agentsNameMax    = 32  // ... but never past this
	dualLensMinWidth = 140 // below this, show one lens at a time; at/above it, both fit with a preview
)

// dualLens reports whether both lenses render side by side. Below the threshold (or before
// the first size message) the view shows a single lens - the focused one - plus the preview.
func (m model) dualLens() bool { return m.width >= dualLensMinWidth }

// agentNameWidth is the agents-lens title column: the widest agent title (and band label),
// clamped to a sane band so a long title never starves the rest of the layout.
func (m model) agentNameWidth() int {
	w := agentsNameMin
	for i := range m.agentRows {
		if t := lipgloss.Width(displayTitle(m.agentRows[i].Title, m.agentRows[i].TmuxSession)); t > w {
			w = t
		}
	}
	for _, lbl := range bandLabels {
		if l := lipgloss.Width(lbl) + 2; l > w {
			w = l
		}
	}
	if w > agentsNameMax {
		w = agentsNameMax
	}
	return w
}

// agentsContentWidth is the agents-lens body width (status gutter + title column); the
// outer panel adds the cursor column and the frame.
func (m model) agentsContentWidth() int { return statusColWidth + m.agentNameWidth() }

// agentsPaneOuter is the rendered width of the Agents lens panel (cursor column + body +
// frame border/padding).
func (m model) agentsPaneOuter() int { return cursorColWidth + m.agentsContentWidth() + 4 }

// agentsReserve is the terminal width the Agents lens claims (its panel plus the inter-pane
// gap), removed from the terminal before the Workspaces+preview split is computed. Zero when
// the Agents lens is not shown alongside Workspaces.
func (m model) agentsReserve() int {
	if !m.dualLens() {
		return 0
	}
	return m.agentsPaneOuter() + 2 // inter-pane gap
}

// mainWidth is the terminal width available to the Workspaces lens and the preview, after
// the Agents lens has claimed its column.
func (m model) mainWidth() int {
	if w := m.width - m.agentsReserve(); w > 0 {
		return w
	}
	return m.width
}

// listCapWidth is the largest the list content (after the cursor column and the frame) may
// grow to while still leaving the preview its minimum width. Zero means no preview is shown
// (or the size isn't known yet), so the list is uncapped. The shrink loop in colWidths trims
// columns to this so a long label can never starve the preview.
func (m model) listCapWidth() int {
	if m.width <= 0 || !m.showPreview() {
		return 0
	}
	return m.mainWidth() - 2 /*inter-pane gap*/ - (minPreviewCols + 4) /*preview frame*/ - cursorColWidth - 4 /*list frame*/
}

// listTargetWidth is the list content width the configured split asks for: the agents pane's
// share of the terminal, clamped so the preview keeps its minimum. The list fills out to this
// (padding past its content) so the pane occupies its configured portion rather than collapsing
// to a narrow column. Zero means no preview / unknown size, so there is nothing to fill toward.
func (m model) listTargetWidth() int {
	if m.width <= 0 || m.split <= 0 || !m.showPreview() {
		return 0
	}
	t := int(float64(m.mainWidth())*m.split+0.5) - cursorColWidth - 4
	if cap := m.listCapWidth(); cap > 0 && t > cap {
		t = cap
	}
	return t
}

// rawRowWidth is the width the columns actually need: the gutter, indent, the worktree and
// title columns, and (when any row carries one) a one-space gap plus the locator-hint column.
func rawRowWidth(windowW, nameW, hintW int) int {
	w := statusColWidth + agentIndent + windowW + nameW
	if hintW > 0 {
		w += 1 + hintW
	}
	return w
}

// colWidths returns the worktree-label, title, and locator-hint column widths for the current
// rows. Each flexes to fit the widest value present — so a label like "algorithms2" shows in
// full rather than clipped — clamped to a sane band, then narrowed (title first, then worktree)
// so the columns never starve the preview. The hint is its own fixed-width column so every
// "[in: …]" lines up at the same x, rather than floating behind each title's trailing edge.
func (m model) colWidths() (windowW, nameW, hintW int) {
	windowW, nameW = windowColMin, nameColMin
	for i := range m.rows {
		r := m.rows[i]
		if w := lipgloss.Width(rowLabel(r)) + 1; w > windowW { // +1 keeps a trailing space
			windowW = w
		}
		if w := lipgloss.Width(displayTitle(r.Title, r.TmuxSession)); w > nameW {
			nameW = w
		}
		if w := lipgloss.Width(locatorHint(r)); w > hintW {
			hintW = w
		}
	}
	if windowW > windowColMax {
		windowW = windowColMax
	}
	if nameW > nameColMax {
		nameW = nameColMax
	}
	for cap := m.listCapWidth(); cap > 0 && rawRowWidth(windowW, nameW, hintW) > cap; {
		if nameW > nameColMin {
			nameW--
		} else if windowW > windowColMin {
			windowW--
		} else {
			break
		}
	}
	return
}

// rowContentWidth is a row's width after the cursor column: the larger of what the columns need
// and the split's target share, so session bars and the selected-row highlight span the whole
// pane and the pane fills its configured portion. The columns pack left; any surplus is trailing
// space on the right (see leafRow), not a gap wedged between a title and its hint.
func (m model) rowContentWidth() int {
	windowW, nameW, hintW := m.colWidths()
	w := rawRowWidth(windowW, nameW, hintW)
	if t := m.listTargetWidth(); t > w {
		w = t
	}
	return w
}

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
	hintStyle            = lipgloss.NewStyle().Faint(true).Foreground(lipColor(theme.Gray))
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
	return panelTitleBordered(box, title, accent, lipColor(theme.Border))
}

// panelTitleBordered is panelTitle with the top-border runes drawn in a caller-chosen
// color rather than the default theme.Border. The confirm modals use it to tint the
// whole frame (a warning hue), so the rebuilt title line matches the body's border.
func panelTitleBordered(box, title string, titleColor, borderColor lipgloss.Color) string {
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
	bc := lipgloss.NewStyle().Foreground(borderColor)
	tc := lipgloss.NewStyle().Foreground(titleColor).Bold(true)
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

// helpOrder is the order actions appear in the help line. Focus-switch comes right after
// navigation; the two focus keys collapse to one "h/l lens" hint (see renderHelp).
var helpOrder = []Action{ActionDown, ActionUp, ActionTop, ActionBottom, ActionPrevSection, ActionNextSection, ActionFocusLeft, ActionFold, ActionNewSession, ActionNewAgent, ActionClose, ActionDelete, ActionSelect, ActionQuit}

var actionLabel = map[Action]string{
	ActionUp:          "up",
	ActionDown:        "down",
	ActionTop:         "top",
	ActionBottom:      "bottom",
	ActionHalfUp:      "half-up",
	ActionHalfDown:    "half-down",
	ActionPrevSection: "prev",
	ActionNextSection: "next session",
	ActionFocusLeft:   "lens",
	ActionFocusRight:  "lens",
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

	// Workspaces lens: the repository→row tree, stable-ordered, foldable.
	rows   []Row           // leaf rows in render order
	items  []renderItem    // full tree: headers + leaf rows interleaved
	folded map[string]bool // session keys whose section is collapsed
	nav    []int           // item indices the cursor can land on, given fold state
	cursor int             // index into nav (a leaf, or a folded session header)
	top    int             // first visible item (scroll offset into the rendered list)

	// Agents lens: a flat triage list of agents in fixed status bands. It is a second
	// projection of the same rows. Band headers are always rendered; a populated band folds
	// like a Workspaces section, collapsing its agents to a navigable header stand-in.
	focus       lensID         // which lens the cursor and actions act on
	agentRows   []Row          // agent rows in band order (the Agents lens leaves)
	agentItems  []agentItem    // band headers + agent rows interleaved
	agentNav    []int          // agentItems indices the agents cursor can land on
	agentCursor int            // index into agentNav (an agent, or a folded band header)
	agentTop    int            // scroll offset into the visible agent items
	bandFolded  [numBands]bool // bands whose agents are collapsed under the header

	chosen  *Row
	pending string // armed first key of a chord ("" = none)

	mode           mode
	branchInput    string      // new-agent branch field (modeNewAgent)
	worktreeInput  string      // new-agent worktree-dir field; auto-derived from the branch
	worktreeEdited bool        // the user edited the worktree field, so stop auto-deriving it
	focusField     formField   // which new-agent field has focus (fieldBranch / fieldWorktree)
	target         Row         // the row the active modal acts on (new-agent repo / delete confirm)
	confirmChoice  int         // selected button in a confirm modal: 0 cancel (safe default), 1 confirm
	notice         string      // transient status line (feedback from actions)
	noticeLevel    noticeLevel // how to style the notice (error vs neutral info)
	noticeGen      int         // bumped each time a notice is set, so a stale auto-dismiss no-ops

	width, height int
	preview       string         // cached preview for the selected row
	accent        lipgloss.Color // title + cursor color (configurable)
	split         float64        // agents-list pane's share of the width (configurable)
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
	m := model{src: src, prev: prev, keys: keys, res: res, refresh: refresh, folded: map[string]bool{}, accent: lipColor(theme.Accent), split: defaultSplit, focus: lensAgents}
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
	m.buildAgentsLens(list)
}

// buildAgentsLens projects the rows into the Agents lens: band headers (always present)
// interleaved with their agents in band order. agentRows holds the agent leaves; agentItems
// is the rendered sequence; agentNav lists the navigable (agent) positions.
func (m *model) buildAgentsLens(list []Row) {
	bands := agentsByBand(list)
	m.agentRows = m.agentRows[:0]
	m.agentItems = m.agentItems[:0]
	for b := agentBand(0); b < numBands; b++ {
		m.agentItems = append(m.agentItems, agentItem{band: b, header: true, count: len(bands[b])})
		for _, r := range bands[b] {
			m.agentRows = append(m.agentRows, r)
			m.agentItems = append(m.agentItems, agentItem{band: b, rowIdx: len(m.agentRows) - 1})
		}
	}
	m.recomputeAgentNav()
}

// recomputeAgentNav lists the agentItems indices the agents cursor can land on: every agent
// of an unfolded band, plus the header of each folded band (its single navigable stand-in,
// mirroring the Workspaces fold). An empty band has no agents and is never folded, so its
// header stays unselectable.
func (m *model) recomputeAgentNav() {
	m.agentNav = m.agentNav[:0]
	for i, it := range m.agentItems {
		if it.header {
			if m.bandFolded[it.band] && it.count > 0 {
				m.agentNav = append(m.agentNav, i)
			}
			continue
		}
		if !m.bandFolded[it.band] {
			m.agentNav = append(m.agentNav, i)
		}
	}
	if m.agentCursor > len(m.agentNav)-1 {
		m.agentCursor = max(len(m.agentNav)-1, 0)
	}
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
func Run(src RowSource, prev Previewer, keys Keymap, accent lipgloss.Color, refresh time.Duration, split float64, orch Orchestrator) (*Row, error) {
	m, err := newModel(src, prev, keys, refresh)
	if err != nil {
		return nil, err
	}
	if accent != "" {
		m.accent = accent
	}
	if split > 0 {
		m.split = split
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
	agentSel := m.agentSelID()
	m.setRows(list)
	m.cursor = relocate(m, selKey, m.cursor)
	m.agentCursor = m.relocateAgent(agentSel, m.agentCursor)
	m.refreshPreview()
}

// agentSelID is the SessionID of the agent under the Agents lens cursor, or "" - used to
// follow that selection across a refresh by identity rather than by index.
func (m model) agentSelID() string {
	if m.agentCursor < 0 || m.agentCursor >= len(m.agentNav) {
		return ""
	}
	return m.agentRows[m.agentItems[m.agentNav[m.agentCursor]].rowIdx].SessionID
}

// relocateAgent finds the agents-cursor position for the agent with the given SessionID in
// the rebuilt Agents lens, else clamps the old cursor into bounds.
func (m model) relocateAgent(id string, old int) int {
	if len(m.agentNav) == 0 {
		return 0
	}
	if id != "" {
		for i, itemIdx := range m.agentNav {
			if m.agentRows[m.agentItems[itemIdx].rowIdx].SessionID == id {
				return i
			}
		}
	}
	return clamp(old, 0, len(m.agentNav)-1)
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

// currentRow returns the row the focused lens's cursor points at. In the Workspaces lens
// that is the selected leaf, or the first row of a folded session whose header is selected
// (so preview and jump still target something useful while folded). In the Agents lens it
// is the selected agent. Routing every action through this one accessor is what makes the
// actions (jump, close, delete, new agent) work from whichever lens holds focus.
func (m model) currentRow() (Row, bool) {
	if m.focus == lensAgents {
		return m.agentsCurrentRow()
	}
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

// agentsCurrentRow returns the agent under the Agents lens cursor. A folded band header is
// a stand-in for many agents, not one row, so it reports no current row (like a folded
// Workspaces section header).
func (m model) agentsCurrentRow() (Row, bool) {
	if m.agentCursor < 0 || m.agentCursor >= len(m.agentNav) {
		return Row{}, false
	}
	it := m.agentItems[m.agentNav[m.agentCursor]]
	if it.header {
		return Row{}, false
	}
	return m.agentRows[it.rowIdx], true
}

// selectedAgentID is the SessionID of the focused lens's current selection when it is a
// live agent, else "". It is the link between the lenses: the counterpart to highlight in
// the other lens, and the agent to land on when focus switches.
func (m model) selectedAgentID() string {
	if r, ok := m.currentRow(); ok && r.Kind == RowAgent {
		return r.SessionID
	}
	return ""
}

// activeCursor returns a pointer to the focused lens's cursor and that lens's nav length,
// so the shared navigation actions move whichever lens holds focus.
func (m *model) activeCursor() (*int, int) {
	if m.focus == lensAgents {
		return &m.agentCursor, len(m.agentNav)
	}
	return &m.cursor, len(m.nav)
}

// setFocus switches the focused lens, carrying the selected agent across when it has a
// counterpart (so the switch feels linked, not like jumping to a disjoint list); when there
// is no counterpart the target lens keeps its own cursor (a sensible last-position default).
func (m *model) setFocus(to lensID) {
	if m.focus == to {
		return
	}
	id := m.selectedAgentID()
	m.focus = to
	if id != "" {
		m.selectAgentInFocusedLens(id)
	}
	m.dismissNotice()
	m.refreshPreview()
}

// selectAgentInFocusedLens moves the focused lens's cursor onto the agent with the given
// SessionID when present; otherwise it leaves the cursor where it is. In either lens a
// counterpart inside a folded section/band is reached by selecting that header (the fold is
// left collapsed), per the resolved design question.
func (m *model) selectAgentInFocusedLens(id string) {
	if m.focus == lensAgents {
		for i, itemIdx := range m.agentNav {
			it := m.agentItems[itemIdx]
			if !it.header && m.agentRows[it.rowIdx].SessionID == id {
				m.agentCursor = i
				return
			}
			// Folded band: its header stands in for the agents it hides, so land there.
			if it.header && m.bandFolded[it.band] && m.bandHasAgent(it.band, id) {
				m.agentCursor = i
				return
			}
		}
		return
	}
	for i, itemIdx := range m.nav {
		it := m.items[itemIdx]
		if it.kind == kindRow && m.rows[it.rowIdx].SessionID == id {
			m.cursor = i
			return
		}
		// Folded section: its header stands in for the rows it hides, so land there.
		if it.kind == kindSession && m.folded[it.sessionKey] && m.sessionHasAgent(it.sessionKey, id) {
			m.cursor = i
			return
		}
	}
}

// sessionHasAgent reports whether a (folded) section contains the agent with the given id.
func (m model) sessionHasAgent(key, id string) bool {
	for _, it := range m.items {
		if it.kind == kindRow && it.sessionKey == key && m.rows[it.rowIdx].SessionID == id {
			return true
		}
	}
	return false
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

// refreshPreview captures the selected row's pane preview, caching the result. A row is
// previewed by its own pane/window; a structural row (a base or slot) with no live window
// of its own shows nothing — capturing its session would surface an unrelated window
// (e.g. a sibling worktree's), which reads as if the empty slot were running something.
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
	if r.isWindowlessStructural() {
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
	cur, n := m.activeCursor()
	old := *cur
	last := n - 1
	switch a {
	case ActionUp:
		if *cur > 0 {
			*cur--
		}
	case ActionDown:
		if *cur < last {
			*cur++
		}
	case ActionTop:
		*cur = 0
	case ActionBottom:
		if last >= 0 {
			*cur = last
		}
	case ActionHalfUp:
		*cur = clamp(*cur-m.halfPage(), 0, max(last, 0))
	case ActionHalfDown:
		*cur = clamp(*cur+m.halfPage(), 0, max(last, 0))
	case ActionPrevSection:
		// Section jumps are a Workspaces-lens motion; the Agents lens is a flat list.
		if m.focus == lensWorkspaces {
			m.cursor = m.prevSection()
		}
	case ActionNextSection:
		if m.focus == lensWorkspaces {
			m.cursor = m.nextSection()
		}
	case ActionFocusLeft:
		m.setFocus(lensAgents)
		return m, nil
	case ActionFocusRight:
		m.setFocus(lensWorkspaces)
		return m, nil
	case ActionFold:
		// Both lenses fold: a Workspaces section collapses its rows, an Agents band collapses
		// its agents. Each leaves the cursor on the now-collapsed header.
		if m.focus == lensAgents {
			return m.toggleAgentFold()
		}
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
			// A windowless base or slot has no window to jump to; jumping would land on
			// an unrelated window in the session. Instead Enter wakes it — gives it a
			// window of its own and attaches (wake picks a shell for the base, an agent
			// for a slot) — the inverse of the `dd` that closed it.
			if m.orch != nil && r.isWindowlessStructural() {
				return m.wake(r)
			}
			m.chosen = &r
		}
		return m, tea.Quit
	case ActionQuit:
		return m, tea.Quit
	}
	if *cur != old {
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

// toggleAgentFold collapses or expands the band under the agents cursor, mirroring
// toggleFold: folding from one of a band's agents leaves the cursor on the now-collapsed
// band header; unfolding from that header drops the cursor onto the band's first agent.
func (m model) toggleAgentFold() (tea.Model, tea.Cmd) {
	if m.agentCursor < 0 || m.agentCursor >= len(m.agentNav) {
		return m, nil
	}
	b := m.agentItems[m.agentNav[m.agentCursor]].band
	m.bandFolded[b] = !m.bandFolded[b]
	m.recomputeAgentNav()
	m.agentCursor = m.agentNavIndexForBand(b)
	m.dismissNotice()
	m.refreshPreview()
	return m, nil
}

// agentNavIndexForBand returns the agentNav index of band b's navigable stand-in: its header
// when folded, otherwise its first agent. agentNav is in band order, so the first nav entry
// in band b is exactly that stand-in. Falls back to the clamped cursor when b has none.
func (m model) agentNavIndexForBand(b agentBand) int {
	for ci, ii := range m.agentNav {
		if m.agentItems[ii].band == b {
			return ci
		}
	}
	return clamp(m.agentCursor, 0, max(len(m.agentNav)-1, 0))
}

// focusSession moves the cursor onto the first navigable row whose pane lives in the
// named tmux session, and leaves the cursor put otherwise. Used after opening a new
// session so the eye lands on what was just created. Sections group by repository now, so
// this matches the row's own tmux session rather than the section key.
func (m *model) focusSession(name string) {
	for i, itemIdx := range m.nav {
		it := m.items[itemIdx]
		if it.kind == kindRow && m.rows[it.rowIdx].TmuxSession == name {
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
	avis := m.visibleAgentItems()
	m.agentTop = windowTop(m.agentTop, m.agentSelVisiblePos(avis), m.listCap(), len(avis))
}

// visibleAgentItems is the agentItems indices that render, in order: every item except
// agents hidden beneath a folded band. This is the sequence the agents scroll window slides
// over.
func (m model) visibleAgentItems() []int {
	out := make([]int, 0, len(m.agentItems))
	for i, it := range m.agentItems {
		if !it.header && m.bandFolded[it.band] {
			continue
		}
		out = append(out, i)
	}
	return out
}

// agentSelVisiblePos is the position of the selected agent item within visibleAgentItems
// (0 when none).
func (m model) agentSelVisiblePos(vis []int) int {
	if m.agentCursor < 0 || m.agentCursor >= len(m.agentNav) {
		return 0
	}
	target := m.agentNav[m.agentCursor]
	for p, i := range vis {
		if i == target {
			return p
		}
	}
	return 0
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

	left := m.renderLenses()

	body := left
	if m.showPreview() {
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", m.renderPreview(left))
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

	repo := m.target.Repo
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

// confirmModal renders a destructive-action confirmation as a centered popup (DESIGN.md's
// single modal style). The frame is tinted a warning hue so it reads as a caution before
// any color is parsed: Gold for a plain delete, the more severe Red for the dirty-worktree
// force escalation. The prompt wraps to the modal width (it is a popup, not the
// single-row status line, so a long worktree name no longer gets sheared off), and the
// y/n choices render as selectable buttons defaulting to cancel.
func (m model) confirmModal() string {
	width := clamp(m.width/2, 32, 52)
	if m.width > 0 && width > m.width-4 {
		width = m.width - 4
	}
	inner := width - 2 // inside the border's left/right padding (frameStyle pads 0,1)

	wrap := lipgloss.NewStyle().Width(inner)
	text := wrap.Foreground(lipColor(theme.Text))

	var title, prompt, confirmLabel string
	var warn lipgloss.Color
	switch m.mode {
	case modeConfirmForce:
		title, warn = "force delete", lipColor(theme.Red)
		prompt = wrap.Foreground(warn).Render(
			fmt.Sprintf("worktree %q has uncommitted changes. Force-remove and discard them?", m.target.Worktree))
		confirmLabel = "discard & remove"
	default:
		title, warn = "delete", lipColor(theme.Gold) // Gold is the warning hue here (context-scoped, like its dir/working uses)
		prompt = text.Render(m.deletePrompt())
		confirmLabel = "confirm"
	}

	choices := confirmButtons(m.confirmChoice, confirmLabel, m.accent)
	box := frameStyle.BorderForeground(warn).Width(width).Render(strings.Join([]string{prompt, "", choices}, "\n"))
	return panelTitleBordered(box, title, warn, warn)
}

// confirmButtons renders the cancel/confirm pair for a confirm modal. The selected button
// carries the tool-wide selection treatment (the accent cursor glyph, accent text, RowHL
// background); the other stays faint. Each button's hotkey letter (n/y) is bold-accented in
// both so the shortcut reads even on the unselected side — never color alone, per DESIGN.md.
func confirmButtons(selected int, confirmLabel string, accent lipgloss.Color) string {
	hotkey := lipgloss.NewStyle().Foreground(accent).Bold(true)
	faint := lipgloss.NewStyle().Faint(true)
	button := func(idx int, key, label string) string {
		if idx == selected {
			sel := lipgloss.NewStyle().Foreground(accent).Background(rowHL).Bold(true)
			return sel.Render(theme.CursorGlyph + " " + key + " " + label + " ")
		}
		return faint.Render("  ") + hotkey.Render(key) + " " + label + faint.Render(" ")
	}
	return button(0, "n", "cancel") + "   " + button(1, "y", confirmLabel)
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
	label      string // session name or "(no-repo)" (session header)
	note       string // an explanatory annotation for the bar (e.g. the incidental reason)
	sessionKey string // owning tmux session (its own key for a session header)
	window     string // the row's window/worktree label (gray column)
	count      int    // leaves in the session, for the folded header's count
	managed    bool   // the session is a recognized managed repo (session header)
	worktrees  int    // worktree count, for the managed indicator (session header)
	rowIdx     int    // index into model.rows, for kindRow
	// badgeStatus/badgeCount summarize a section's most-urgent live agent status for the
	// bar's non-positional status badge. badgeStatus is "" when the section has no agents.
	badgeStatus Status
	badgeCount  int
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

// repoGroup is one repository's rows, keyed by git-common-dir ("" is the ungrouped
// bucket for incidental agents). label is the repository's display name; isRepo marks a
// recognized repository (so its section bar carries the indicator and `n` is available),
// as opposed to the ungrouped bucket.
type repoGroup struct {
	key       string // git-common-dir (section identity / fold key)
	label     string // repository name
	rows      []Row
	isRepo    bool
	worktrees int
}

// groupRows arranges rows into a flat repository→row list and returns them in render
// order alongside the interleaved header/row items the view draws. Grouping is by
// repository (git-common-dir), not tmux session: a repository's rows may come from
// panes in several sessions and still collect under one section. Sections are ordered by
// a stable key (repository name), never by status, so a watched section holds position;
// the incidental "(no-repo)" bucket sorts last. Within a section the anchor (if any) is
// pinned first, agents follow by worktree name then title, and empty worktree slots sort
// last. Status is conveyed by a per-row indicator, not by position.
func groupRows(in []Row) ([]Row, []renderItem) {
	if len(in) == 0 {
		return nil, nil
	}

	var groups []*repoGroup
	idx := map[string]*repoGroup{}
	for _, r := range in {
		g := idx[r.GitDir]
		if g == nil {
			g = &repoGroup{key: r.GitDir, label: r.Repo, isRepo: r.GitDir != ""}
			idx[r.GitDir] = g
			groups = append(groups, g)
		}
		g.rows = append(g.rows, r)
	}

	for _, g := range groups {
		sort.SliceStable(g.rows, func(i, j int) bool { return rowLess(g.rows[i], g.rows[j]) })
		seen := map[string]bool{}
		for _, r := range g.rows {
			if r.Worktree != "" && !seen[r.Worktree] {
				seen[r.Worktree] = true
				g.worktrees++
			}
		}
	}
	sort.SliceStable(groups, func(i, j int) bool {
		// Stable identity order: named repositories by name, with the incidental
		// (no-repo) bucket last. A section's position never depends on its agents'
		// status, so a section a user is watching holds still when its agent changes
		// state; cross-group urgency lives in the Agents lens instead (see dash-lenses).
		li, lj := groups[i].label, groups[j].label
		if ei, ej := li == "", lj == ""; ei != ej {
			return ej // a named section sorts before the empty (incidental) bucket
		}
		return li < lj
	})

	ordered := make([]Row, 0, len(in))
	items := make([]renderItem, 0, len(in)+len(groups))
	for _, g := range groups {
		label := g.label
		note := ""
		if label == "" {
			// The incidental bucket: every row here failed git-native recognition, so it
			// states why it offers no worktrees or slots rather than an opaque label.
			label = "(no-repo)"
			if len(g.rows) > 0 {
				note = incidentalReason(g.rows[0])
			}
		}
		badgeStatus, badgeCount := sectionBadge(g.rows)
		items = append(items, renderItem{kind: kindSession, label: label, note: note, sessionKey: g.key, count: len(g.rows), managed: g.isRepo, worktrees: g.worktrees, badgeStatus: badgeStatus, badgeCount: badgeCount})
		for _, r := range g.rows {
			ordered = append(ordered, r)
			items = append(items, renderItem{
				kind:       kindRow,
				sessionKey: g.key,
				window:     rowLabel(r),
				rowIdx:     len(ordered) - 1,
			})
		}
	}
	return ordered, items
}

// agentBand is a fixed triage band in the Agents lens. Bands render in this order, are
// always shown (dimmed when empty), and an agent's band follows only its status, so a
// status change moves an agent between bands without reordering its band peers.
type agentBand int

const (
	bandNeedsYou agentBand = iota
	bandWorking
	bandIdle
	bandDone
	numBands
)

// bandLabels are the band headers, in render order.
var bandLabels = [numBands]string{
	bandNeedsYou: "NEEDS YOU",
	bandWorking:  "WORKING",
	bandIdle:     "IDLE",
	bandDone:     "DONE",
}

// lensID names the two always-visible lenses over the same row set.
type lensID int

const (
	lensWorkspaces lensID = iota // right: repository→row tree, stable-ordered
	lensAgents                   // left: flat triage list in status bands
)

// agentItem is one line of the Agents lens: a band header (always shown, not navigable)
// or an agent leaf pointing into model.agentRows.
type agentItem struct {
	band   agentBand
	header bool // a band header line rather than an agent row
	count  int  // agents in the band, for a header line
	rowIdx int  // index into model.agentRows when !header
}

// bandOf maps a status to its triage band. needs-attention, working, and done map to
// their own bands; idle and unknown (no fresh state) both read as quiet and fall in IDLE.
func bandOf(s Status) agentBand {
	switch s {
	case StatusNeedsAttention:
		return bandNeedsYou
	case StatusWorking:
		return bandWorking
	case StatusDone:
		return bandDone
	default:
		return bandIdle
	}
}

// agentsByBand projects rows into the Agents lens: only RowAgent rows, bucketed by band
// and ordered within each band by most-recent status change (newest first), falling back
// to worktree then title when the change time is absent (zero Updated). The DONE band's
// membership is governed by the existing process-liveness GC upstream - a done agent is
// present here only while its process lives - so this projection adds no time window.
func agentsByBand(in []Row) [numBands][]Row {
	var bands [numBands][]Row
	for _, r := range in {
		if r.Kind != RowAgent {
			continue
		}
		b := bandOf(r.Status)
		bands[b] = append(bands[b], r)
	}
	for b := range bands {
		rows := bands[b]
		sort.SliceStable(rows, func(i, j int) bool {
			if !rows[i].Updated.Equal(rows[j].Updated) {
				return rows[i].Updated.After(rows[j].Updated) // newest first
			}
			if rows[i].Worktree != rows[j].Worktree {
				return rows[i].Worktree < rows[j].Worktree
			}
			return rows[i].Title < rows[j].Title
		})
	}
	return bands
}

// sectionBadge summarizes a section's live agents for the bar's status badge: the
// most-urgent status present and how many agents share it. It returns ("", 0) when the
// section has no live agents, so a quiet section shows no badge and is not ranked by it.
func sectionBadge(rows []Row) (Status, int) {
	best := Status("")
	bestRank := int(^uint(0) >> 1)
	count := 0
	for _, r := range rows {
		if r.Kind != RowAgent {
			continue
		}
		if rk := r.Status.rank(); rk < bestRank {
			best, bestRank, count = r.Status, rk, 1
		} else if r.Status == best {
			count++
		}
	}
	return best, count
}

// incidentalReason explains why a row is not grouped under a recognized repository, shown
// on the incidental (no-repo) section. Recognition is git-native (any pane or agent path
// resolving to a git worktree is recognized), so the only reason a row lands here is that
// its directory is not a git repository. Empty when the row belongs to a recognized repo.
func incidentalReason(r Row) string {
	if r.GitDir != "" {
		return ""
	}
	return "not a git repo"
}

// locatorHint returns the `[in: <session>]` suffix for a row whose pane lives outside its
// repository's home session, so a pane be recognized in one of the user's own sessions is
// visibly flagged rather than silently drawn under a section it does not share a session
// with. It is empty for rows in the home, rows with no live window, and incidental agents.
func locatorHint(r Row) string {
	if r.GitDir == "" || r.TmuxSession == "" {
		return ""
	}
	if r.TmuxSession == dir.HomeSession(r.GitDir) {
		return ""
	}
	return "[in: " + r.TmuxSession + "]"
}

// rowLabel is the gray-column label for a leaf row: the worktree name when the row is
// a managed worktree (or anchor), else the tmux window label.
func rowLabel(r Row) string {
	if r.Worktree != "" {
		return r.Worktree
	}
	return windowLabel(r.TmuxWindow, r.TmuxWindowName)
}

// rowLess orders rows within a section stably: the anchor first, empty slots last, and
// agents in between by worktree name then title. Status is deliberately absent from the
// key - it is shown as a per-row indicator, so a row does not move when its agent's state
// changes. Ordering by worktree (before title) keeps a worktree's co-located agents
// adjacent at that worktree's name position.
func rowLess(a, b Row) bool {
	if ra, rb := rowRank(a), rowRank(b); ra != rb {
		return ra < rb
	}
	if a.Worktree != b.Worktree {
		return a.Worktree < b.Worktree
	}
	return a.Title < b.Title
}

// rowRank ranks a row for within-section ordering: anchor before everything, slots after
// everything, agents in between. Agents share one rank so they sort among themselves by
// name (see rowLess), never by status.
func rowRank(r Row) int {
	switch r.Kind {
	case RowAnchor:
		return -1
	case RowSlot:
		return 100
	default:
		return 0
	}
}

// renderRows draws the visible window of items: a leftmost cursor column, then the
// session bar or the fixed-column leaf row. Rows under a folded session are skipped,
// and only the slice [top, top+cap) is drawn so the list never overflows the panel.
// The window top is recomputed here (not just trusted from the model) so a View called
// without a prior Update — e.g. directly in a test — still keeps the cursor on-screen.
// hlState is a row's highlight in a lens: the focused cursor, the dimmer mirror of the
// focused selection shown in the *other* lens, or none.
type hlState int

const (
	hlNone   hlState = iota
	hlCursor         // this lens holds focus and this is its selected row
	hlMirror         // the focused lens's selection, shown in the non-focused lens
)

// renderLenses composes the left side of the body: both lenses side by side when the
// terminal is wide enough, otherwise just the focused lens (the narrow-terminal fallback,
// still toggled with the focus keys). The focused lens's panel title carries the accent; the
// other reads gray, so focus is visible without color alone via the cursor glyph too.
func (m model) renderLenses() string {
	if !m.dualLens() {
		if m.focus == lensAgents {
			return m.lensPanel(lensAgents, "agents", m.renderAgents())
		}
		return m.lensPanel(lensWorkspaces, "workspaces", m.renderRows())
	}
	ws := m.lensPanel(lensWorkspaces, "workspaces", m.renderRows())
	ag := m.lensPanel(lensAgents, "agents", m.renderAgents())
	return lipgloss.JoinHorizontal(lipgloss.Top, ag, "  ", ws)
}

// lensPanel frames a lens's body with its title, accenting the title when the lens is
// focused and graying it otherwise.
func (m model) lensPanel(lens lensID, title, body string) string {
	color := lipColor(theme.Gray)
	if m.focus == lens {
		color = m.accent
	}
	return panelTitle(frameStyle.Render(body), title, color)
}

func (m model) renderRows() string {
	vis := m.visibleItems()
	h := m.listCap()
	top := windowTop(m.top, m.selVisiblePos(vis), h, len(vis))
	end := min(top+h, len(vis))

	selItem := -1
	if m.focus == lensWorkspaces && m.cursor >= 0 && m.cursor < len(m.nav) {
		selItem = m.nav[m.cursor]
	}
	mirrorID := ""
	if m.focus == lensAgents {
		mirrorID = m.selectedAgentID()
	}
	rows := make([]string, 0, end-top)
	for _, i := range vis[top:end] {
		it := m.items[i]
		hl := hlNone
		switch {
		case i == selItem:
			hl = hlCursor
		case mirrorID != "" && it.kind == kindRow && m.rows[it.rowIdx].SessionID == mirrorID:
			hl = hlMirror
		}
		if it.kind == kindSession {
			rows = append(rows, m.sessionBar(it, hl))
			continue
		}
		rows = append(rows, m.cursorCol(hl)+m.leafRow(it, hl == hlCursor))
	}
	return strings.Join(rows, "\n")
}

// renderAgents draws the Agents lens: always all four band headers (a section bar when
// populated, faint when empty), with each unfolded band's agents beneath, windowed to the
// visible height. A folded band collapses to its header. The focused cursor highlights the
// selected agent (or folded band header); when the Workspaces lens is focused instead, its
// selected agent shows here with the dimmer mirror highlight (on the band header if that
// band is folded).
func (m model) renderAgents() string {
	cw := m.agentsContentWidth()
	if !m.dualLens() {
		if t := m.listTargetWidth(); t > cw {
			cw = t // sole panel in the fallback: fill like the workspaces list would
		}
	}
	h := m.listCap()
	vis := m.visibleAgentItems()
	top := windowTop(m.agentTop, m.agentSelVisiblePos(vis), h, len(vis))
	end := min(top+h, len(vis))

	selItem := -1
	if m.focus == lensAgents && m.agentCursor >= 0 && m.agentCursor < len(m.agentNav) {
		selItem = m.agentNav[m.agentCursor]
	}
	mirrorID := ""
	if m.focus == lensWorkspaces {
		mirrorID = m.selectedAgentID()
	}
	rows := make([]string, 0, end-top)
	for _, i := range vis[top:end] {
		it := m.agentItems[i]
		if it.header {
			folded := m.bandFolded[it.band] && it.count > 0
			hl := hlNone
			switch {
			case folded && i == selItem:
				hl = hlCursor
			case folded && mirrorID != "" && m.bandHasAgent(it.band, mirrorID):
				hl = hlMirror
			}
			rows = append(rows, m.bandBar(it.band, it.count, cw, folded, hl))
			continue
		}
		r := m.agentRows[it.rowIdx]
		hl := hlNone
		switch {
		case i == selItem:
			hl = hlCursor
		case mirrorID != "" && r.SessionID == mirrorID:
			hl = hlMirror
		}
		rows = append(rows, m.cursorCol(hl)+m.agentLeaf(r, cw, hl == hlCursor))
	}
	return strings.Join(rows, "\n")
}

// bandHasAgent reports whether band b currently holds the agent with the given session id
// (used to mirror a Workspaces selection onto a folded band's header).
func (m model) bandHasAgent(b agentBand, id string) bool {
	for _, it := range m.agentItems {
		if !it.header && it.band == b && m.agentRows[it.rowIdx].SessionID == id {
			return true
		}
	}
	return false
}

// barGutter renders the cursor-column cell over the section-bar background: the accent
// cursor glyph on the focused selection, a faint accent glyph on its mirror, else a blank
// cell - so a section bar's distinct background runs unbroken across the gutter.
func (m model) barGutter(hl hlState) string {
	switch hl {
	case hlCursor:
		return cursorGlyphStyle(m.accent).Background(lipColor(theme.SessionBg)).Render(theme.CursorGlyph + " ")
	case hlMirror:
		return cursorGlyphStyle(m.accent).Faint(true).Background(lipColor(theme.SessionBg)).Render(theme.CursorGlyph + " ")
	default:
		return sessionBarStyle.Render(strings.Repeat(" ", cursorColWidth))
	}
}

// sectionBar renders an edge-to-edge section header over the full list width (cursor gutter
// included), shared by both lenses so a repository header and a status band read as the same
// kind of thing. left is pinned to the start (after the gutter), right (a count or badge, may
// be "") is pinned to the right edge, and the cursor/mirror glyph sits in the gutter over the
// bar. A faint bar (an empty band) drops to dim text with a blank gutter.
func (m model) sectionBar(left, right string, contentW int, hl hlState, faint bool) string {
	fullW := cursorColWidth + contentW
	if faint {
		body := strings.Repeat(" ", cursorColWidth) + left
		return sessionBarStyle.Bold(false).Faint(true).Width(fullW).Render(truncate(body, fullW))
	}
	leftSeg := m.barGutter(hl) + sessionBarStyle.Render(left)
	gap := fullW - lipgloss.Width(leftSeg) - lipgloss.Width(right)
	if gap < 1 {
		body := strings.Repeat(" ", cursorColWidth) + left
		if right != "" {
			body += "  " + right
		}
		return sessionBarStyle.Width(fullW).Render(truncate(body, fullW))
	}
	out := leftSeg + sessionBarStyle.Render(strings.Repeat(" ", gap))
	if right != "" {
		out += sessionBarCountStyle.Render(right)
	}
	return out
}

// bandBar renders a triage band's header as a section bar. A populated band is bold, carries
// the ▾/▸ fold caret, and pins its count to the right edge (a folded band shows the collapsed
// caret and its hidden-agent count). An empty band keeps the bar but goes faint with no caret
// or count - still clearly a section, just a quiet "all clear". It is not foldable.
func (m model) bandBar(b agentBand, count, contentW int, folded bool, hl hlState) string {
	label := bandLabels[b]
	if count == 0 {
		// Indent by the caret's width ("▾ ") so an empty band's label lines up with the
		// populated bands' labels rather than sitting two columns to its left.
		return m.sectionBar("  "+label, "", contentW, hlNone, true)
	}
	caret := "▾"
	if folded {
		caret = "▸"
	}
	return m.sectionBar(caret+" "+label, fmt.Sprintf("%d", count), contentW, hl, false)
}

// agentLeaf renders one agent row in the Agents lens: the status gutter (glyph + word) then
// the title, padded to the panel width. The focused selection carries the full-row
// highlight; the cursor/mirror glyph is drawn separately in the cursor column.
func (m model) agentLeaf(r Row, w int, selected bool) string {
	gutter, st := gutterFor(r)
	nameW := max(w-statusColWidth, 1)
	title := padRight(truncate(displayTitle(r.Title, r.TmuxSession), nameW), nameW)
	gutterSt, nameSt := st, nameColStyle
	if selected {
		gutterSt = gutterSt.Background(rowHL)
		nameSt = nameSt.Background(rowHL).Bold(true)
	}
	return gutterSt.Render(gutter) + nameSt.Render(title)
}

// cursorCol is the leftmost column: the accent cursor glyph on the focused selection, a
// faint accent glyph on its mirror in the other lens, else blank.
func (m model) cursorCol(hl hlState) string {
	switch hl {
	case hlCursor:
		return cursorGlyphStyle(m.accent).Render(theme.CursorGlyph + " ")
	case hlMirror:
		return cursorGlyphStyle(m.accent).Faint(true).Render(theme.CursorGlyph + " ")
	default:
		return strings.Repeat(" ", cursorColWidth)
	}
}

// sessionBar renders a session header as an edge-to-edge section bar (see sectionBar),
// left-aligned to the start of the list. A managed repo carries the worktree-source
// indicator and its right-pinned badge/worktree count; a folded section shows a collapsed
// glyph and its hidden-row count. The cursor/mirror glyph sits in the gutter over the bar.
func (m model) sessionBar(it renderItem, hl hlState) string {
	name := it.label
	if it.managed {
		name = managedGlyph + " " + it.label
	}
	w := m.rowContentWidth()
	if m.folded[it.sessionKey] {
		return m.sectionBar("▸ "+name, fmt.Sprintf("%d", it.count), w, hl, false)
	}
	return m.sectionBar("▾ "+name, m.sessionBarRight(it), w, hl, false)
}

// sessionBarRight builds the bar's right-pinned annotation: the most-urgent-status badge
// (a status glyph + count, paired so it never relies on color alone) followed by the
// worktree count for a managed repo. Either part may be absent.
func (m model) sessionBarRight(it renderItem) string {
	var parts []string
	if it.badgeStatus != "" {
		parts = append(parts, statusGlyph[it.badgeStatus]+" "+fmt.Sprintf("%d", it.badgeCount))
	}
	if it.note != "" {
		parts = append(parts, it.note)
	}
	if it.managed && it.worktrees > 0 {
		parts = append(parts, fmt.Sprintf("%d wt", it.worktrees))
	}
	return strings.Join(parts, "  ")
}

// leafRow renders one row at fixed columns: the status/marker gutter (glyph + word), an
// indent, the gray window/worktree label, then the white name. The selected row carries
// a full-row highlight spanning gutter→name in addition to the cursor glyph.
func (m model) leafRow(it renderItem, selected bool) string {
	r := m.rows[it.rowIdx]
	gutter, st := gutterFor(r)
	indent := strings.Repeat(" ", agentIndent)
	windowW, nameW, hintW := m.colWidths()
	win := padRight(truncate(it.window, windowW-1), windowW)
	title := padRight(truncate(displayTitle(r.Title, r.TmuxSession), nameW), nameW)

	winSt, nameSt, hintSt := windowColStyle, nameColStyle, hintStyle
	gutterSt := st
	if selected {
		hl := func(s lipgloss.Style) lipgloss.Style { return s.Background(rowHL) }
		winSt, hintSt, gutterSt = hl(winSt), hl(hintSt), hl(gutterSt)
		nameSt = nameSt.Background(rowHL).Bold(true)
	}

	// Columns pack left: gutter, indent, worktree, title. The "[in: …]" locator is an exception
	// annotation, so it rides a rightmost column flush to the pane's target width — every hint
	// lines up at the same right edge, and any surplus is the gap between the title and that
	// column, never trailing space past the hint.
	row := gutterSt.Render(gutter) + nameSt.Render(indent) + winSt.Render(win) + nameSt.Render(title)
	used := statusColWidth + agentIndent + windowW + nameW
	if hintW > 0 {
		// Reserve one space before the hint column, then push the rest of the surplus into it so
		// the locator sits at the right edge.
		gap := 1
		if pad := m.rowContentWidth() - used - hintW; pad > gap {
			gap = pad
		}
		row += nameSt.Render(strings.Repeat(" ", gap)) + hintSt.Render(padRight(truncate(locatorHint(r), hintW), hintW))
	} else if pad := m.rowContentWidth() - used; pad > 0 {
		row += nameSt.Render(strings.Repeat(" ", pad))
	}
	return row
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
	return m.mainWidth() >= previewMinWidth
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
		lines = strings.Split(m.previewPlaceholder(), "\n")
	}
	// w is the text width; lipgloss Width includes the frame's horizontal
	// padding, so set Width(w+2) to keep the text area exactly w. Otherwise
	// each w-wide line wraps, inflating the frame height past the terminal.
	box := frameStyle.Width(w + 2).Height(inner).Render(strings.Join(lines, "\n"))
	return panelTitle(box, "preview", m.accent)
}

// previewPlaceholder is shown when the preview pane has nothing to capture. A windowless
// base or slot is not a failed capture — it's a worktree with no open window — so it reads
// as an accented, actionable notice (an accent glyph + words, never color alone per
// DESIGN.md) that says what the row is and how to wake it, rather than the faint
// "(no preview available)" reserved for a real window that simply produced no output.
func (m model) previewPlaceholder() string {
	r, ok := m.currentRow()
	if ok && r.isWindowlessStructural() {
		head := lipgloss.NewStyle().Foreground(m.accent).Bold(true)
		hint := lipgloss.NewStyle().Foreground(m.accent)
		// Match what ⏎ (wake) will do: a shell for the base, an agent for a slot.
		if r.IsPrimary {
			return head.Render(anchorGlyph+" No window to preview.") + "\n" +
				hint.Render("⏎ opens a shell here.")
		}
		return head.Render(slotGlyph+" No window to preview.") + "\n" +
			hint.Render("⏎ spawns an agent here.")
	}
	return placeholderStyle.Render("(no preview available)")
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
		if a == ActionFocusLeft {
			// Show both directions as one hint: the primary left/right keys joined.
			left, right := firstKey(m.keys[ActionFocusLeft]), firstKey(m.keys[ActionFocusRight])
			parts = append(parts, prettyKey(left)+"/"+prettyKey(right)+" "+actionLabel[a])
			continue
		}
		parts = append(parts, prettyKeys(keys)+" "+actionLabel[a])
	}
	return helpStyle.Render(strings.Join(parts, " · "))
}

// firstKey returns the first bound key, or "" when none.
func firstKey(keys []string) string {
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
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
