package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type VimMode string

const (
	VimModeNormal VimMode = "NORMAL"
	VimModeInsert VimMode = "INSERT"
)

type TextEditorModel struct {
	Value     string
	Cursor    int
	Mode      VimMode
	Active    bool
	Width     int
	Prompt    string
	MultiLine bool
	OnSubmit  func(string) tea.Cmd
	OnCancel  func() tea.Cmd
}

func NewTextEditor(prompt string) TextEditorModel {
	return TextEditorModel{
		Mode:   VimModeNormal,
		Prompt: prompt,
	}
}

func (m TextEditorModel) Update(msg tea.Msg) (TextEditorModel, tea.Cmd) {
	if !m.Active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.Mode {
		case VimModeNormal:
			switch msg.String() {
			case "i":
				m.Mode = VimModeInsert
			case "a":
				m.Mode = VimModeInsert
				if len(m.Value) > 0 {
					m.Cursor++
				}
			case "h", "left":
				if m.Cursor > 0 {
					m.Cursor--
				}
			case "l", "right":
				if m.Cursor < len(m.Value) {
					m.Cursor++
				}
			case "j", "down":
				if m.MultiLine {
					line, col := findLineOfOffset(m.Value, m.Cursor, m.Width)
					m.Cursor = findOffsetAtCoords(m.Value, line+1, col, m.Width)
				}
			case "k", "up":
				if m.MultiLine {
					line, col := findLineOfOffset(m.Value, m.Cursor, m.Width)
					if line > 0 {
						m.Cursor = findOffsetAtCoords(m.Value, line-1, col, m.Width)
					}
				}
			case "x":
				if len(m.Value) > 0 && m.Cursor < len(m.Value) {
					m.Value = m.Value[:m.Cursor] + m.Value[m.Cursor+1:]
				}
			case "esc":
				if m.OnCancel != nil {
					return m, m.OnCancel()
				}
			case "enter":
				if m.OnSubmit != nil {
					return m, m.OnSubmit(m.Value)
				}
			}

		case VimModeInsert:
			switch msg.Type {
			case tea.KeyEsc:
				m.Mode = VimModeNormal
				if m.Cursor > 0 {
					m.Cursor--
				}
			case tea.KeyEnter:
				if m.MultiLine {
					m.Value = m.Value[:m.Cursor] + "\n" + m.Value[m.Cursor:]
					m.Cursor++
				} else if m.OnSubmit != nil {
					return m, m.OnSubmit(m.Value)
				}
			case tea.KeyBackspace:
				if m.Cursor > 0 {
					m.Value = m.Value[:m.Cursor-1] + m.Value[m.Cursor:]
					m.Cursor--
				}
			case tea.KeyDelete:
				if m.Cursor < len(m.Value) {
					m.Value = m.Value[:m.Cursor] + m.Value[m.Cursor+1:]
				}
			case tea.KeyUp:
				if m.MultiLine {
					line, col := findLineOfOffset(m.Value, m.Cursor, m.Width)
					if line > 0 {
						m.Cursor = findOffsetAtCoords(m.Value, line-1, col, m.Width)
					}
				}
			case tea.KeyDown:
				if m.MultiLine {
					line, col := findLineOfOffset(m.Value, m.Cursor, m.Width)
					m.Cursor = findOffsetAtCoords(m.Value, line+1, col, m.Width)
				}
			case tea.KeyLeft:
				if m.Cursor > 0 {
					m.Cursor--
				}
			case tea.KeyRight:
				if m.Cursor < len(m.Value) {
					m.Cursor++
				}
			default:
				if len(msg.Runes) > 0 {
					m.Value = m.Value[:m.Cursor] + string(msg.Runes) + m.Value[m.Cursor:]
					m.Cursor += len(msg.Runes)
				}
			}
		}
	}

	// Clamp cursor
	if m.Cursor < 0 {
		m.Cursor = 0
	}
	if m.Cursor > len(m.Value) {
		m.Cursor = len(m.Value)
	}

	return m, nil
}

func (m TextEditorModel) Render(width int) string {
	if width < 5 {
		return ""
	}

	displayValue := m.Value
	if m.Active {
		if m.Cursor < len(displayValue) {
			cursorChar := string(displayValue[m.Cursor])
			// If it's a newline, we need to visualize it as a space before the newline
			if cursorChar == "\n" {
				cursorChar = " \n"
				cursorStyle := lipgloss.NewStyle().Reverse(true)
				displayValue = displayValue[:m.Cursor] + cursorStyle.Render(" ") + displayValue[m.Cursor:]
			} else {
				cursorStyle := lipgloss.NewStyle().Reverse(true)
				displayValue = displayValue[:m.Cursor] + cursorStyle.Render(cursorChar) + displayValue[m.Cursor+1:]
			}
		} else {
			displayValue += lipgloss.NewStyle().Reverse(true).Render(" ")
		}
	}

	return displayValue
}

func (m TextEditorModel) RenderOverlay(width int) string {
	overlayWidth := min(max(width-10, 40), 100)
	editorWidth := overlayWidth - 6
	
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2).
		Width(overlayWidth)

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	
	valWithCursor := m.Render(editorWidth)
	parts := strings.Split(valWithCursor, "\n")
	var wrappedLines []string
	for _, part := range parts {
		wrappedLines = append(wrappedLines, breakLines(part, editorWidth)...)
	}
	displayValue := strings.Join(wrappedLines, "\n")

	content := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(m.Prompt),
		"",
		displayValue,
		"",
		HelpStyle.Render("Enter to submit, Esc to cancel"),
	)

	if m.Mode == VimModeNormal {
		content = lipgloss.JoinVertical(lipgloss.Left,
			content,
			HelpStyle.Render("Mode: NORMAL (press 'i' to insert)"),
		)
	} else {
		content = lipgloss.JoinVertical(lipgloss.Left,
			content,
			HelpStyle.Render("Mode: INSERT (press 'Esc' for normal)"),
		)
	}

	return style.Render(content)
}
