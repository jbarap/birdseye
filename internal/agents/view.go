package agents

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jbarap/birds-eye/internal/theme"
)

// refreshInterval is how often the open view re-reads agent state and the
// selected agent's preview.
const refreshInterval = time.Second

// Layout constants. Column widths keep every row's data points at fixed positions;
// the preview is only shown when the terminal is at least previewMinWidth wide.
const (
	cursorColWidth  = 2  // leftmost gutter: cursor glyph on the selected row, else blank
	statusColWidth  = 9  // " G word  " — status glyph + 4-char word, pinned across rows
	agentIndent     = 6  // an agent's columns sit this far right of the session bar's label
	windowColWidth  = 11 // gray window-label column
	nameColWidth    = 22 // white agent-name column
	previewMinWidth = 92
	minPreviewCols  = 24
)

// rowContentWidth is a row's width after the cursor column. Session bars and the
// selected-row highlight span exactly this, so their backgrounds line up.
const rowContentWidth = statusColWidth + agentIndent + windowColWidth + nameColWidth

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
	windowColStyle  = lipgloss.NewStyle().Foreground(lipColor(theme.Gray))
	nameColStyle    = lipgloss.NewStyle().Foreground(lipColor(theme.Text))
	rowHL           = lipColor(theme.RowHL)
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
var helpOrder = []Action{ActionDown, ActionUp, ActionTop, ActionBottom, ActionPrevSection, ActionNextSection, ActionFold, ActionSelect, ActionQuit}

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
	ActionQuit:        "quit",
}

// tickMsg drives the periodic live refresh.
type tickMsg time.Time

// model is the Bubble Tea model for the agents view.
type model struct {
	src  Source
	prev Previewer
	keys Keymap
	res  *resolver

	agents  []Agent         // agent leaves in render order
	items   []renderItem    // full tree: headers + agent rows interleaved
	folded  map[string]bool // session keys whose section is collapsed
	nav     []int           // item indices the cursor can land on, given fold state
	cursor  int             // index into nav (an agent, or a folded session header)
	chosen  *Agent
	pending rune // armed first rune of a chord (0 = none)

	width, height int
	preview       string         // cached preview for the selected agent
	accent        lipgloss.Color // title + cursor color (configurable)
	err           error
}

// newModel builds the initial model, loading agents and the first preview.
func newModel(src Source, prev Previewer, keys Keymap) (model, error) {
	res, err := keys.buildResolver()
	if err != nil {
		return model{}, err
	}
	m := model{src: src, prev: prev, keys: keys, res: res, folded: map[string]bool{}, accent: lipColor(theme.Accent)}
	list, err := src.Agents()
	if err != nil {
		return model{}, err
	}
	m.setAgents(list)
	m.refreshPreview()
	return m, nil
}

// setAgents arranges the source's agents into the session→window render order,
// caches the interleaved items, drops fold state for sessions that are gone, and
// recomputes the navigable rows.
func (m *model) setAgents(list []Agent) {
	m.agents, m.items = groupAgents(list)
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

// recomputeNav rebuilds the list of item indices the cursor may land on: every
// agent of an unfolded session, plus the header of each folded session (its single
// navigable stand-in). Window headers are never navigable.
func (m *model) recomputeNav() {
	m.nav = m.nav[:0]
	for i, it := range m.items {
		switch it.kind {
		case kindSession:
			if m.folded[it.sessionKey] {
				m.nav = append(m.nav, i)
			}
		case kindAgent:
			if !m.folded[it.sessionKey] {
				m.nav = append(m.nav, i)
			}
		}
	}
}

// Run renders the agents view and blocks until the user selects an agent or
// quits. It returns the chosen agent (nil when quit without selecting).
func Run(src Source, prev Previewer, keys Keymap, accent lipgloss.Color) (*Agent, error) {
	m, err := newModel(src, prev, keys)
	if err != nil {
		return nil, err
	}
	if accent != "" {
		m.accent = accent
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	out, err := p.Run()
	if err != nil {
		return nil, err
	}
	return out.(model).chosen, nil
}

func (m model) Init() tea.Cmd { return tick() }

func tick() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		m.reload()
		return m, tick()
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// reload re-reads agent state through the Source, preserving the selected agent
// by identity, and refreshes the preview.
func (m *model) reload() {
	list, err := m.src.Agents()
	if err != nil {
		m.err = err
		return
	}
	m.err = nil
	selKey := m.currentRowKey()
	m.setAgents(list)
	m.cursor = relocate(m, selKey, m.cursor)
	m.refreshPreview()
}

// rowKey identifies a navigable row by what it represents — an agent (by session
// id) or a folded session header (by session key) — so selection can follow that
// row across a refresh even as ordering or fold state shifts.
func (m model) rowKey(itemIdx int) string {
	it := m.items[itemIdx]
	switch it.kind {
	case kindAgent:
		return "a:" + m.agents[it.agentIdx].SessionID
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

// currentAgent returns the agent the cursor points at: the selected agent leaf, or
// the most-urgent agent of a folded session whose header is selected. The second
// case lets the preview and jump still target something useful while folded.
func (m model) currentAgent() (Agent, bool) {
	it, ok := m.currentItem()
	if !ok {
		return Agent{}, false
	}
	switch it.kind {
	case kindAgent:
		return m.agents[it.agentIdx], true
	case kindSession:
		return m.firstAgentOfSession(it.sessionKey)
	}
	return Agent{}, false
}

// firstAgentOfSession returns the first agent (in render order) of a session.
func (m model) firstAgentOfSession(key string) (Agent, bool) {
	for _, it := range m.items {
		if it.kind == kindAgent && it.sessionKey == key {
			return m.agents[it.agentIdx], true
		}
	}
	return Agent{}, false
}

// refreshPreview captures the selected agent's preview, caching the result.
func (m *model) refreshPreview() {
	if m.prev == nil {
		m.preview = ""
		return
	}
	a, ok := m.currentAgent()
	if !ok {
		m.preview = ""
		return
	}
	out, err := m.prev.Preview(a)
	if err != nil {
		m.preview = ""
		return
	}
	m.preview = out
}

func (m model) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := key.String()
	r, isRune := singleRune(k)

	// Complete a chord (e.g. the second g of gg).
	if m.pending != 0 {
		armed := m.pending
		m.pending = 0
		if isRune && r == armed {
			if a, ok := m.res.double[armed]; ok {
				return m.applyAction(a)
			}
		}
		// Not the completing key: fall through and handle k normally.
	}

	// Arm a chord on its first rune.
	if isRune {
		if _, ok := m.res.double[r]; ok {
			m.pending = r
			return m, nil
		}
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
	case ActionSelect:
		if a, ok := m.currentAgent(); ok {
			m.chosen = &a
		}
		return m, tea.Quit
	case ActionQuit:
		return m, tea.Quit
	}
	if m.cursor != old {
		m.refreshPreview()
	}
	return m, nil
}

// toggleFold collapses or expands the section under the cursor. Folding a section
// from one of its agents leaves the cursor on the now-collapsed session header;
// unfolding from that header drops the cursor onto the section's first agent.
func (m model) toggleFold() (tea.Model, tea.Cmd) {
	it, ok := m.currentItem()
	if !ok {
		return m, nil
	}
	key := it.sessionKey
	m.folded[key] = !m.folded[key]
	m.recomputeNav()
	m.cursor = m.navIndexForSession(key)
	m.refreshPreview()
	return m, nil
}

// navIndexForSession returns the cursor position of a session's navigable row: its
// header when folded, otherwise its first agent.
func (m model) navIndexForSession(key string) int {
	for i, itemIdx := range m.nav {
		it := m.items[itemIdx]
		if it.sessionKey == key && (it.kind == kindSession || it.kind == kindAgent) {
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
	page := m.height - 4 // title + frame + help chrome
	if page < 2 {
		page = 10
	}
	return max(page/2, 1)
}

func (m model) View() string {
	if len(m.agents) == 0 {
		return m.emptyView()
	}

	list := panelTitle(frameStyle.Render(m.renderRows()), "agents", m.accent)

	body := list
	if m.showPreview() {
		body = lipgloss.JoinHorizontal(lipgloss.Top, list, "  ", m.renderPreview(list))
	}

	parts := []string{body}
	if m.height == 0 || m.height >= 6 {
		parts = append(parts, m.renderHelp())
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m model) emptyView() string {
	body := placeholderStyle.Render("No agents tracked yet.\n" +
		"Wire up Claude Code hooks (see README) so sessions report status here.")
	panel := panelTitle(frameStyle.Render(body), "agents", m.accent)
	return panel + "\n" + helpStyle.Render("q quit")
}

// renderKind distinguishes the line types in the grouped list.
type renderKind int

const (
	kindSession renderKind = iota // tmux session header, drawn as a full-width section bar
	kindAgent                     // a selectable agent row
)

// renderItem is one line of the grouped list. A session item is a header (not
// selectable); an agent item points at an entry in the ordered agents slice and
// carries its window label for the gray window column.
type renderItem struct {
	kind       renderKind
	label      string // session name or "ungrouped" (session header)
	sessionKey string // owning tmux session (its own key for a session header)
	window     string // the agent's window label (gray column)
	count      int    // agents in the session, for the folded header's count
	agentIdx   int    // index into model.agents, for kindAgent
}

// windowLabel is how an agent's window is shown in the gray column: its tmux window
// name when set, else "win <index>". With neither (an agent outside tmux) it is empty.
func windowLabel(index, name string) string {
	if name != "" {
		return name
	}
	if index != "" {
		return "win " + index
	}
	return ""
}

// sessionGroup is one tmux session's agents ("" key is the ungrouped bucket).
type sessionGroup struct {
	key    string
	agents []Agent
}

// groupAgents arranges agents into a flat session→agent list and returns them in
// render order alongside the interleaved header/agent items the view draws. Sessions
// are ordered by their most-urgent member (lowest status rank), then by name; agents
// within a session by status rank then title. Windows are not a grouping level, so
// agents that share a window are not forced adjacent — each agent simply carries its
// window as a label. Agents lacking a tmux session collect under an "ungrouped" heading.
func groupAgents(in []Agent) ([]Agent, []renderItem) {
	if len(in) == 0 {
		return nil, nil
	}

	var sessions []*sessionGroup
	sessIdx := map[string]*sessionGroup{}
	for _, a := range in {
		sg := sessIdx[a.TmuxSession]
		if sg == nil {
			sg = &sessionGroup{key: a.TmuxSession}
			sessIdx[a.TmuxSession] = sg
			sessions = append(sessions, sg)
		}
		sg.agents = append(sg.agents, a)
	}

	for _, sg := range sessions {
		sort.SliceStable(sg.agents, func(i, j int) bool { return agentLess(sg.agents[i], sg.agents[j]) })
	}
	sort.SliceStable(sessions, func(i, j int) bool {
		if ri, rj := groupRank(sessions[i].agents), groupRank(sessions[j].agents); ri != rj {
			return ri < rj
		}
		return sessions[i].key < sessions[j].key
	})

	ordered := make([]Agent, 0, len(in))
	items := make([]renderItem, 0, len(in)+len(sessions))
	for _, sg := range sessions {
		label := sg.key
		if label == "" {
			label = "ungrouped"
		}
		items = append(items, renderItem{kind: kindSession, label: label, sessionKey: sg.key, count: len(sg.agents)})
		for _, a := range sg.agents {
			ordered = append(ordered, a)
			items = append(items, renderItem{
				kind:       kindAgent,
				sessionKey: sg.key,
				window:     windowLabel(a.TmuxWindow, a.TmuxWindowName),
				agentIdx:   len(ordered) - 1,
			})
		}
	}
	return ordered, items
}

// agentLess orders agents within a session: most-urgent status first, then title.
func agentLess(a, b Agent) bool {
	if a.Status.rank() != b.Status.rank() {
		return a.Status.rank() < b.Status.rank()
	}
	return a.Title < b.Title
}

// groupRank is the urgency of a set of agents: the lowest (most-urgent) status rank.
func groupRank(agents []Agent) int {
	r := int(^uint(0) >> 1) // max int
	for _, a := range agents {
		if rk := a.Status.rank(); rk < r {
			r = rk
		}
	}
	return r
}

// renderRows draws each visible item: a leftmost cursor column, then the session bar
// or the fixed-column agent row. Agents under a folded session are skipped.
func (m model) renderRows() string {
	selItem := -1
	if m.cursor >= 0 && m.cursor < len(m.nav) {
		selItem = m.nav[m.cursor]
	}
	var rows []string
	for i, it := range m.items {
		if it.kind == kindAgent && m.folded[it.sessionKey] {
			continue // hidden beneath a folded session
		}
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
	case kindAgent:
		return m.agentRow(it, selected)
	}
	return ""
}

// sessionBar renders a session header as a full-width bar, left-aligned to the start
// of the list. A folded section shows a collapsed glyph and its hidden-agent count.
func (m model) sessionBar(it renderItem) string {
	label := "▾ " + it.label
	if m.folded[it.sessionKey] {
		label = fmt.Sprintf("▸ %s  (%d)", it.label, it.count)
	}
	return sessionBarStyle.Width(rowContentWidth).Render(truncate(label, rowContentWidth))
}

// agentRow renders one agent at fixed columns: the status gutter (glyph + word), an
// indent, the gray window label, then the white name. The selected row carries a
// full-row highlight spanning gutter→name in addition to the cursor glyph.
func (m model) agentRow(it renderItem, selected bool) string {
	a := m.agents[it.agentIdx]
	gutter := " " + statusGlyph[a.Status] + " " + padRight(statusWord[a.Status], 4) + "  "
	indent := strings.Repeat(" ", agentIndent)
	win := padRight(truncate(it.window, windowColWidth-1), windowColWidth)
	name := padRight(truncate(displayTitle(a.Title, it.sessionKey), nameColWidth), nameColWidth)
	st := statusStyle[a.Status]
	if !selected {
		return st.Render(gutter) + indent + windowColStyle.Render(win) + nameColStyle.Render(name)
	}
	hl := lipgloss.NewStyle().Background(rowHL)
	return st.Background(rowHL).Render(gutter) +
		hl.Render(indent) +
		windowColStyle.Background(rowHL).Render(win) +
		nameColStyle.Background(rowHL).Bold(true).Render(name)
}

// displayTitle drops a leading "<session>:" from an agent title, since the session
// is already shown by its header — "arewa:birds_eye" under session "arewa" becomes
// "birds_eye". Titles without that prefix (or ungrouped agents) are unchanged.
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
	// scroll the list out of view). Budget: this frame's border (2) + help text
	// and its top margin (2) = 4. The title now lives in the border, not a body
	// line, so it costs no inner row.
	inner := m.height - 4
	if inner < 1 {
		inner = 1
	}

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
