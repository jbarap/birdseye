package agents

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jbarap/birds-eye/internal/theme"
)

// refreshInterval is how often the open view re-reads agent state and the
// selected agent's preview.
const refreshInterval = time.Second

// Layout constants. Column widths keep rows aligned; the preview is only shown
// when the terminal is at least previewMinWidth columns wide.
const (
	badgeWidth      = 14
	titleColWidth   = 22
	locColWidth     = 16
	previewMinWidth = 92
	minPreviewCols  = 24
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

// statusBadge pairs each status with a glyph + label so statuses stay
// distinguishable on terminals without color.
var statusBadge = map[Status]string{
	StatusNeedsAttention: "● needs-attn",
	StatusWorking:        "◐ working",
	StatusIdle:           "○ idle",
	StatusDone:           "✓ done",
	StatusUnknown:        "· unknown",
}

var (
	titleStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipColor(theme.Coral)).Padding(0, 1)
	frameStyle       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipColor(theme.Border)).Padding(0, 1)
	previewTitle     = lipgloss.NewStyle().Faint(true).Padding(0, 1)
	cursorStyle      = lipgloss.NewStyle().Foreground(lipColor(theme.Coral))
	selectedStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	locStyle         = lipgloss.NewStyle().Faint(true)
	helpStyle        = lipgloss.NewStyle().Faint(true).MarginTop(1)
	placeholderStyle = lipgloss.NewStyle().Faint(true).Italic(true)
)

// helpOrder is the order actions appear in the help line.
var helpOrder = []Action{ActionDown, ActionUp, ActionTop, ActionBottom, ActionSelect, ActionQuit}

var actionLabel = map[Action]string{
	ActionUp:       "up",
	ActionDown:     "down",
	ActionTop:      "top",
	ActionBottom:   "bottom",
	ActionHalfUp:   "half-up",
	ActionHalfDown: "half-down",
	ActionSelect:   "jump",
	ActionQuit:     "quit",
}

// tickMsg drives the periodic live refresh.
type tickMsg time.Time

// model is the Bubble Tea model for the agents view.
type model struct {
	src  Source
	prev Previewer
	keys Keymap
	res  *resolver

	agents  []Agent
	cursor  int
	chosen  *Agent
	pending rune // armed first rune of a chord (0 = none)

	width, height int
	preview       string // cached preview for the selected agent
	err           error
}

// newModel builds the initial model, loading agents and the first preview.
func newModel(src Source, prev Previewer, keys Keymap) (model, error) {
	res, err := keys.buildResolver()
	if err != nil {
		return model{}, err
	}
	m := model{src: src, prev: prev, keys: keys, res: res}
	list, err := src.Agents()
	if err != nil {
		return model{}, err
	}
	m.agents = list
	m.refreshPreview()
	return m, nil
}

// Run renders the agents view and blocks until the user selects an agent or
// quits. It returns the chosen agent (nil when quit without selecting).
func Run(src Source, prev Previewer, keys Keymap) (*Agent, error) {
	m, err := newModel(src, prev, keys)
	if err != nil {
		return nil, err
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
	var selID string
	if m.cursor >= 0 && m.cursor < len(m.agents) {
		selID = m.agents[m.cursor].SessionID
	}
	m.agents = list
	m.cursor = relocate(list, selID, m.cursor)
	m.refreshPreview()
}

// relocate finds selID in the new list, else clamps the old index into bounds.
func relocate(list []Agent, selID string, old int) int {
	if len(list) == 0 {
		return 0
	}
	if selID != "" {
		for i, a := range list {
			if a.SessionID == selID {
				return i
			}
		}
	}
	if old < 0 {
		return 0
	}
	if old >= len(list) {
		return len(list) - 1
	}
	return old
}

// refreshPreview captures the selected agent's preview, caching the result.
func (m *model) refreshPreview() {
	if m.prev == nil || len(m.agents) == 0 || m.cursor < 0 || m.cursor >= len(m.agents) {
		m.preview = ""
		return
	}
	out, err := m.prev.Preview(m.agents[m.cursor])
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
	last := len(m.agents) - 1
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
	case ActionSelect:
		if len(m.agents) > 0 {
			a := m.agents[m.cursor]
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

	title := titleStyle.Render("bird's-eye · agents")
	list := frameStyle.Render(m.renderRows())

	body := list
	if m.showPreview() {
		body = lipgloss.JoinHorizontal(lipgloss.Top, list, "  ", m.renderPreview(list))
	}

	parts := []string{title, body}
	if m.height == 0 || m.height >= 6 {
		parts = append(parts, m.renderHelp())
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m model) emptyView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("bird's-eye · agents"))
	b.WriteString("\n\n")
	b.WriteString(placeholderStyle.Render("No agents tracked yet.\n" +
		"Wire up Claude Code hooks (see README) so sessions report status here."))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("q quit"))
	return b.String()
}

func (m model) renderRows() string {
	rows := make([]string, len(m.agents))
	for i, a := range m.agents {
		rows[i] = m.renderRow(i, a)
	}
	return strings.Join(rows, "\n")
}

func (m model) renderRow(i int, a Agent) string {
	selected := i == m.cursor

	marker := "  "
	if selected {
		marker = cursorStyle.Render("▌ ")
	}

	badge := statusStyle[a.Status].Render(padRight(statusBadge[a.Status], badgeWidth))

	loc := a.TmuxSession
	if a.TmuxWindow != "" {
		loc += ":" + a.TmuxWindow
	}
	loc = padRight(truncate(loc, locColWidth), locColWidth)

	title := padRight(truncate(a.Title, titleColWidth), titleColWidth)
	if selected {
		title = selectedStyle.Render(title)
	}

	return marker + badge + " " + title + " " + locStyle.Render(loc)
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
	// scroll the list out of view). Budget: title (1) + this frame's border (2)
	// + help text and its top margin (2) = 5.
	inner := m.height - 5
	if inner < 1 {
		inner = 1
	}

	avail := max(inner-1, 1) // reserve a line for the "preview" title

	lines := previewLines(m.preview, avail, w)
	if len(lines) == 0 {
		lines = []string{placeholderStyle.Render("(no preview available)")}
	}
	body := previewTitle.Render("preview") + "\n" + strings.Join(lines, "\n")
	// w is the text width; lipgloss Width includes the frame's horizontal
	// padding, so set Width(w+2) to keep the text area exactly w. Otherwise
	// each w-wide line wraps, inflating the frame height past the terminal.
	return frameStyle.Width(w + 2).Height(inner).Render(body)
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
