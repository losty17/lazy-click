package tui

import "github.com/charmbracelet/lipgloss"

var (
	HeaderStyle              = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	PanelStyle               = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("63")).Padding(0, 1)
	FocusedPanelStyle        = PanelStyle.Copy().BorderForeground(lipgloss.Color("205"))
	StatusStyle              = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	HelpStyle                = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	SyncIdleStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("70"))
	SyncRunStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	SyncErrorStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	ControlCenterPanelStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("69")).Padding(0, 1)
	ControlCenterTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230"))
	ControlCenterSelectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true)
	ControlActionNameStyle   = lipgloss.NewStyle().Bold(true)
	ControlActionAliasStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	RestorePromptStyle       = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("178")).Foreground(lipgloss.Color("229")).Padding(0, 1)
)
