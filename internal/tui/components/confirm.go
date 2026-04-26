package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ConfirmModel struct {
	Prompt   string
	OnConfirm func() tea.Cmd
	OnCancel  func() tea.Cmd
	Active   bool
	Input    string
	Expected string
}

func NewConfirm(prompt string, expected string) ConfirmModel {
	return ConfirmModel{
		Prompt:   prompt,
		Expected: expected,
	}
}

func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	if !m.Active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			if m.OnCancel != nil {
				return m, m.OnCancel()
			}
		case tea.KeyEnter:
			if m.Input == m.Expected {
				if m.OnConfirm != nil {
					return m, m.OnConfirm()
				}
			} else {
				if m.OnCancel != nil {
					return m, m.OnCancel()
				}
			}
		case tea.KeyBackspace:
			if len(m.Input) > 0 {
				m.Input = m.Input[:len(m.Input)-1]
			}
		default:
			if len(msg.Runes) > 0 {
				m.Input += string(msg.Runes)
			}
		}
	}
	return m, nil
}

func (m ConfirmModel) Render(width int) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("196")).
		Padding(1, 2).
		Width(width - 4)

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	
	expectedStyle := lipgloss.NewStyle().Bold(true).Underline(true)
	
	content := lipgloss.JoinVertical(lipgloss.Center,
		titleStyle.Render("CONFIRM DESTRUCTIVE ACTION"),
		"",
		m.Prompt,
		"",
		"Type "+expectedStyle.Render(m.Expected)+" to confirm:",
		"",
		lipgloss.NewStyle().Background(lipgloss.Color("235")).Padding(0, 1).Render(m.Input),
		"",
		HelpStyle.Render("Enter to confirm, Esc to cancel"),
	)

	return style.Render(content)
}
