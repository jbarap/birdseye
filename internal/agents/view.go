package agents

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var statusStyle = map[Status]lipgloss.Style{
	StatusNeedsAttention: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9")),  // bright red
	StatusWorking:        lipgloss.NewStyle().Foreground(lipgloss.Color("11")),             // yellow
	StatusIdle:           lipgloss.NewStyle().Foreground(lipgloss.Color("12")),             // blue
	StatusDone:           lipgloss.NewStyle().Foreground(lipgloss.Color("8")),              // gray
	StatusUnknown:        lipgloss.NewStyle().Faint(true),
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).MarginBottom(1)
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	selectedStyle = lipgloss.NewStyle().Bold(true)
	helpStyle     = lipgloss.NewStyle().Faint(true).MarginTop(1)
)

// model is the Bubble Tea model for the agents view.
type model struct {
	agents []Agent
	cursor int
	chosen *Agent
}

// Run renders the agents view and blocks until the user selects an agent or
// quits. It returns the chosen agent (nil when quit without selecting).
func Run(src Source) (*Agent, error) {
	list, err := src.Agents()
	if err != nil {
		return nil, err
	}
	m := model{agents: list}
	p := tea.NewProgram(m, tea.WithAltScreen())
	out, err := p.Run()
	if err != nil {
		return nil, err
	}
	final := out.(model)
	return final.chosen, nil
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.agents)-1 {
			m.cursor++
		}
	case "enter":
		if len(m.agents) > 0 {
			a := m.agents[m.cursor]
			m.chosen = &a
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("bird's-eye · agents"))
	b.WriteByte('\n')

	if len(m.agents) == 0 {
		b.WriteString(helpStyle.Render("No agents tracked yet.\n" +
			"Wire up Claude Code hooks (see README) so sessions report status here."))
		b.WriteByte('\n')
		b.WriteString(helpStyle.Render("q quit"))
		return b.String()
	}

	for i, a := range m.agents {
		cursor := "  "
		render := func(s string) string { return s }
		if i == m.cursor {
			cursor = cursorStyle.Render("➤ ")
			render = func(s string) string { return selectedStyle.Render(s) }
		}
		badge := statusStyle[a.Status].Render(fmt.Sprintf("%-15s", a.Status))
		loc := a.TmuxSession
		if a.TmuxWindow != "" {
			loc += ":" + a.TmuxWindow
		}
		line := fmt.Sprintf("%s%s %s  %s", cursor, badge, render(pad(a.Title, 24)), helpStyle.Render(loc))
		b.WriteString(line)
		b.WriteByte('\n')
	}

	b.WriteString(helpStyle.Render("↑/↓ move · enter jump · q quit"))
	return b.String()
}

func pad(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
