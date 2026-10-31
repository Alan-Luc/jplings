package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"jplings/internal/doctor"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true)
	dimStyle    = lipgloss.NewStyle().Faint(true)
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	failStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	headerStyle = lipgloss.NewStyle().MarginBottom(1)
)

type model struct {
	checks []doctor.Check
}

// Run starts the TUI, showing host prerequisites until the learner has content to
// work through.
func Run() error {
	_, err := tea.NewProgram(model{checks: doctor.Run(doctor.Default())}).Run()
	return err
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("jplings") + "\n")
	b.WriteString(dimStyle.Render("Japanese learning — scaffold") + "\n\n")
	b.WriteString(headerStyle.Render("Host prerequisites") + "\n")

	for _, c := range m.checks {
		b.WriteString(fmt.Sprintf("%s %-22s %s\n", marker(c), c.Name, dimStyle.Render(c.Detail)))
	}

	if failed := doctor.Failures(m.checks); len(failed) > 0 {
		b.WriteString("\n" + failStyle.Render(fmt.Sprintf("%d required prerequisite(s) missing", len(failed))) + "\n")
	}
	b.WriteString("\n" + dimStyle.Render("press q to quit") + "\n")
	return b.String()
}

func marker(c doctor.Check) string {
	switch {
	case c.Status == doctor.OK:
		return okStyle.Render("[ok]  ")
	case c.Required:
		return failStyle.Render("[fail]")
	default:
		return warnStyle.Render("[warn]")
	}
}
