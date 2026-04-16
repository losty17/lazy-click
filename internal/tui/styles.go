package tui

import "github.com/charmbracelet/lipgloss"

var (
	HeaderStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	PanelStyle        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("63")).Padding(0, 1)
	FocusedPanelStyle = PanelStyle.Copy().BorderForeground(lipgloss.Color("205"))
	StatusStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	HelpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	SyncIdleStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("70"))
	SyncRunStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	SyncErrorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
)
