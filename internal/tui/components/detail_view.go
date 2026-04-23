package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type DetailModel struct {
	sections []string
	idx      int
	x        int
}

func NewDetail() DetailModel {
	return DetailModel{
		sections: []string{
			"Description (markdown rendering planned)",
			"Comments (threaded view planned)",
			"Metadata (status/dates editing planned)",
		},
	}
}

func (m *DetailModel) Move(delta int) {
	next := max(m.idx+delta, 0)
	m.idx = next
}

func (m *DetailModel) SetSections(sections []string) {
	if len(sections) == 0 {
		m.sections = []string{"No task selected"}
		m.idx = 0
		return
	}
	m.sections = sections
	if m.idx >= len(m.sections) {
		m.idx = len(m.sections) - 1
	}
	if m.idx < 0 {
		m.idx = 0
	}
}

func (m *DetailModel) MoveHorizontal(delta int) {
	next := max(m.x+delta, 0)
	m.x = next
}

func (m *DetailModel) MoveToTop() {
	m.idx = 0
}

func (m *DetailModel) MoveToBottom() {
	m.idx = len(m.sections)
	if m.idx < 0 {
		m.idx = 0
	}
}

func (m DetailModel) Render(active bool, width int, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	// Detail renders as a title row plus a vertically scrollable text body.
	title := "Detail"
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75"))
	_ = active
	lines := []string{titleStyle.Render(truncateText(title, width))}

	// Expand logical sections into one flat list of renderable lines.
	bodyLines := m.expandedLines(width)

	bodySize := max(height-1, 0)
	start := max(m.idx, 0)
	maxStart := max(len(bodyLines)-bodySize, 0)

	if start > maxStart {
		start = maxStart
	}

	end := min(start+bodySize, len(bodyLines))

	for i := start; i < end; i++ {
		// lineWindow applies horizontal scrolling offset m.x.
		lines = append(lines, bodyLines[i])
	}

	// Pad to fixed height to avoid panel jitter when content is short.
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (m DetailModel) expandedLines(width int) []string {
	if len(m.sections) == 0 {
		return []string{"No task selected"}
	}

	// Normalize mixed newline styles from cached markdown/comments.
	lines := make([]string, 0, len(m.sections))
	for _, section := range m.sections {
		normalized := strings.ReplaceAll(section, "\r\n", "\n")
		normalized = strings.ReplaceAll(normalized, "\r", "\n")

		parts := strings.Split(normalized, "\n")

		if len(parts) == 0 {
			lines = append(lines, "")
			continue
		}

		for _, part := range parts {
			formattedLines := breakLines(part, width)
			lines = append(lines, formattedLines...)
		}
	}

	return lines
}

func breakLines(s string, width int) []string {
	if width <= 0 {
		return []string{}
	}

	lines := []string{}
	for len(s) > width {
		breakPoint := strings.LastIndex(s[:width], " ")
		if breakPoint == -1 {
			breakPoint = width
		}
		if breakPoint == 0 {
			breakPoint = 1
		}
		lines = append(lines, strings.Trim(s[:breakPoint], " "))
		s = s[breakPoint:]
	}

	s = strings.Trim(s, " ")
	lines = append(lines, s)

	return lines
}
