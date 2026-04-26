package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type DetailField struct {
	Key       string
	Label     string
	Value     string
	Editable  bool
	MultiLine bool
}

type DetailMode int

const (
	ModeNormal DetailMode = iota // This is "FIELD" label
	ModeInsert
	ModeVisual
)

type DetailModel struct {
	Fields       []DetailField
	Cursor       int
	Scroll       int
	Mode         DetailMode
	Editor       TextEditorModel
	VisualOffset int // Character offset within the current multi-line field
	width        int
	height       int
}

func NewDetail() DetailModel {
	return DetailModel{
		Fields: []DetailField{
			{Key: "title", Label: "Title", Editable: true},
			{Key: "status", Label: "Status", Editable: true},
			{Key: "priority", Label: "Priority", Editable: true},
			{Key: "description", Label: "Description", Editable: true, MultiLine: true},
		},
		Mode: ModeNormal,
	}
}

type FieldUpdateMsg struct {
	Key   string
	Value string
}

func (m *DetailModel) SetFields(fields []DetailField) {
	m.Fields = fields
	if m.Cursor >= len(m.Fields) {
		m.Cursor = max(0, len(m.Fields)-1)
	}
}

func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.Mode == ModeInsert {
			m.Editor.Width = m.width - 2
		}
	}

	if m.Mode == ModeInsert {
		var cmd tea.Cmd
		m.Editor, cmd = m.Editor.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "h", "left":
			if m.Mode == ModeVisual {
				if m.VisualOffset > 0 {
					m.VisualOffset--
				}
			}
		case "l", "right":
			if m.Mode == ModeVisual {
				if m.VisualOffset < len(m.Fields[m.Cursor].Value)-1 {
					m.VisualOffset++
				}
			}
		case "j", "down":
			if m.Mode == ModeVisual {
				val := m.Fields[m.Cursor].Value
				line, col := findLineOfOffset(val, m.VisualOffset, m.width)
				numLines, _ := findLineOfOffset(val, len(val), m.width)
				if line < numLines {
					m.VisualOffset = findOffsetAtCoords(val, line+1, col, m.width)
				} else {
					// Move to next field
					if m.Cursor < len(m.Fields)-1 {
						m.Cursor++
						m.VisualOffset = 0
						if !m.Fields[m.Cursor].MultiLine {
							m.Mode = ModeNormal
						}
					}
				}
			} else {
				if m.Cursor < len(m.Fields)-1 {
					m.Cursor++
					if m.Fields[m.Cursor].MultiLine {
						m.Mode = ModeVisual
						m.VisualOffset = 0
					}
				}
			}
		case "k", "up":
			if m.Mode == ModeVisual {
				val := m.Fields[m.Cursor].Value
				line, col := findLineOfOffset(val, m.VisualOffset, m.width)
				if line > 0 {
					m.VisualOffset = findOffsetAtCoords(val, line-1, col, m.width)
				} else {
					// Move to previous field
					if m.Cursor > 0 {
						m.Cursor--
						if m.Fields[m.Cursor].MultiLine {
							m.Mode = ModeVisual
							m.VisualOffset = len(m.Fields[m.Cursor].Value) - 1
							if m.VisualOffset < 0 {
								m.VisualOffset = 0
							}
						} else {
							m.Mode = ModeNormal
						}
					}
				}
			} else {
				if m.Cursor > 0 {
					m.Cursor--
					if m.Fields[m.Cursor].MultiLine {
						m.Mode = ModeVisual
						m.VisualOffset = len(m.Fields[m.Cursor].Value) - 1
						if m.VisualOffset < 0 {
							m.VisualOffset = 0
						}
					}
				}
			}
		case "i", "enter":
			if m.Cursor < len(m.Fields) && m.Fields[m.Cursor].Editable {
				field := m.Fields[m.Cursor]
				m.Mode = ModeInsert
				m.Editor = NewTextEditor(field.Label)
				m.Editor.Value = field.Value
				m.Editor.Cursor = len(m.Editor.Value)
				m.Editor.Active = true
				m.Editor.MultiLine = field.MultiLine
				m.Editor.Mode = VimModeInsert
				m.Editor.Width = m.width - 2
				
				m.Editor.OnSubmit = func(value string) tea.Cmd {
					return func() tea.Msg {
						return FieldUpdateMsg{Key: field.Key, Value: value}
					}
				}
				m.Editor.OnCancel = func() tea.Cmd {
					return func() tea.Msg {
						return CancelFieldEditMsg{}
					}
				}
			}
		case "esc":
			if m.Mode == ModeVisual {
				m.Mode = ModeNormal
				m.VisualOffset = 0
			}
		}
	}

	return m, nil
}

func (m DetailModel) countFieldLines(idx int, width int) int {
	if idx < 0 || idx >= len(m.Fields) {
		return 0
	}
	field := m.Fields[idx]
	if !field.MultiLine {
		return 1
	}
	
	val := strings.ReplaceAll(field.Value, "\r\n", "\n")
	val = strings.ReplaceAll(val, "\r", "\n")
	parts := strings.Split(val, "\n")
	
	count := 1 // Label line
	for _, part := range parts {
		count += len(breakLines(part, width))
	}
	return count
}

type CancelFieldEditMsg struct{}

func (m *DetailModel) Render(active bool, width int, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	m.width = width
	m.height = height

	headerStyle := lipgloss.NewStyle().Bold(true).Padding(0, 1)
	modeStr := " FIELD "
	if m.Mode == ModeNormal {
		headerStyle = headerStyle.Background(lipgloss.Color("42")).Foreground(lipgloss.Color("0"))
	} else if m.Mode == ModeVisual {
		modeStr = " VISUAL "
		headerStyle = headerStyle.Background(lipgloss.Color("214")).Foreground(lipgloss.Color("0"))
	} else {
		if m.Editor.Mode == VimModeInsert {
			modeStr = " INSERT "
			headerStyle = headerStyle.Background(lipgloss.Color("13")).Foreground(lipgloss.Color("0"))
		} else {
			modeStr = " NORMAL "
			headerStyle = headerStyle.Background(lipgloss.Color("62")).Foreground(lipgloss.Color("255"))
		}
	}
	
	header := headerStyle.Render(modeStr) + " " + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75")).Render("Detail")
	lines := []string{header}

	contentHeight := height - 1
	var bodyLines []string
	fieldStartLines := make([]int, len(m.Fields))
	fieldEndLines := make([]int, len(m.Fields))
	cursorLineOffset := 0

	visualCursorStyle := lipgloss.NewStyle().Background(lipgloss.Color("255")).Foreground(lipgloss.Color("0"))

	for i, field := range m.Fields {
		fieldStartLines[i] = len(bodyLines)
		labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		
		if active && i == m.Cursor {
			labelStyle = labelStyle.Foreground(lipgloss.Color("12")).Bold(true)
		}

		label := labelStyle.Render(field.Label + ":")
		
		if m.Mode == ModeInsert && i == m.Cursor {
			bodyLines = append(bodyLines, label)
			
			// Value with cursor visualization
			valWithCursor := m.Editor.Render(width)
			
			// Handle newlines in the value and wrap them
			val := strings.ReplaceAll(valWithCursor, "\r\n", "\n")
			val = strings.ReplaceAll(val, "\r", "\n")
			parts := strings.Split(val, "\n")
			
			editorValueStyle := lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(lipgloss.Color("62")).
				PaddingLeft(1)

			// Find which line the cursor is on for scrolling
			rawVal := m.Editor.Value
			cursorPos := m.Editor.Cursor
			targetWrappedLineIdx, _ := findLineOfOffset(rawVal, cursorPos, width-2)

			currentWrappedLineCounter := 0
			for _, part := range parts {
				wrapped := breakLines(part, width-2)
				for _, wLine := range wrapped {
					if currentWrappedLineCounter == targetWrappedLineIdx { 
						cursorLineOffset = len(bodyLines) - fieldStartLines[i]
					}
					bodyLines = append(bodyLines, editorValueStyle.Render(wLine))
					currentWrappedLineCounter++
				}
			}
		} else {
			valStyle := lipgloss.NewStyle()
			
			if field.MultiLine {
				bodyLines = append(bodyLines, label)
				
				val := field.Value
				cursorPartIdx := -1
				cursorLineInPartIdx := -1
				cursorColInLineIdx := -1
				
				if m.Mode == ModeVisual && i == m.Cursor {
					cursorLine, cursorCol := findLineOfOffset(val, m.VisualOffset, width)
					cursorLineOffset = cursorLine + 1 // +1 because of the label line
					
					// We need to find which part and which wrapped line it is
					parts := strings.Split(val, "\n")
					accLines := 0
					for pi, p := range parts {
						w := breakLines(p, width)
						if accLines + len(w) > cursorLine {
							cursorPartIdx = pi
							cursorLineInPartIdx = cursorLine - accLines
							cursorColInLineIdx = cursorCol
							break
						}
						accLines += len(w)
					}
				}

				parts := strings.Split(val, "\n")
				for pi, part := range parts {
					wrapped := breakLines(part, width)
					for li, l := range wrapped {
						renderedLine := valStyle.Render(l)
						if pi == cursorPartIdx && li == cursorLineInPartIdx {
							// Apply single character cursor
							runes := []rune(l)
							if cursorColInLineIdx < len(runes) {
								char := string(runes[cursorColInLineIdx])
								renderedLine = valStyle.Render(string(runes[:cursorColInLineIdx])) + 
									visualCursorStyle.Render(char) + 
									valStyle.Render(string(runes[cursorColInLineIdx+1:]))
							} else {
								renderedLine = valStyle.Render(l) + visualCursorStyle.Render(" ")
							}
						}
						bodyLines = append(bodyLines, renderedLine)
					}
				}
			} else {
				value := valStyle.Render(field.Value)
				if field.Value == "" {
					value = lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("240")).Render("empty")
				}
				bodyLines = append(bodyLines, label+" "+value)
			}
		}
		fieldEndLines[i] = len(bodyLines) - 1
	}

	// Improved scroll management: ensure the selected field's relevant part is visible
	if len(m.Fields) > 0 && m.Cursor < len(m.Fields) {
		startLine := fieldStartLines[m.Cursor]
		
		targetLine := startLine
		if m.Mode == ModeInsert || m.Mode == ModeVisual {
			targetLine = startLine + cursorLineOffset
		}

		if targetLine < m.Scroll {
			m.Scroll = targetLine
		} else if targetLine >= m.Scroll+contentHeight {
			m.Scroll = targetLine - contentHeight + 1
		}
		
		// Ensure the field header is also visible if possible
		if startLine < m.Scroll && m.Mode == ModeInsert {
			// If we are deep in a multiline field, don't force header visibility if it hides the cursor
		}
	}

	start := m.Scroll
	end := min(start+contentHeight, len(bodyLines))
	if start < 0 {
		start = 0
	}
	if end > len(bodyLines) {
		end = len(bodyLines)
	}

	for i := start; i < end; i++ {
		lines = append(lines, Truncate(bodyLines[i], width, "..."))
	}

	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}
