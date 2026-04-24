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

func (m *DetailModel) Render(active bool, width int, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	// Detail renders as a title row plus a vertically scrollable text body.
	title := "Detail"
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75"))
	_ = active
	lines := []string{titleStyle.Render(Truncate(title, width, "..."))}

	// Expand logical sections into one flat list of renderable lines.
	bodyLines := m.expandedLines(width)

	bodySize := max(height-1, 0)

	// Clamp vertical scroll
	maxStart := max(len(bodyLines)-bodySize, 0)
	if m.idx > maxStart {
		m.idx = maxStart
	}
	if m.idx < 0 {
		m.idx = 0
	}
	start := m.idx

	// Find max line width for horizontal clamping
	maxLineWidth := 0
	for _, l := range bodyLines {
		dw := DisplayWidth(l)
		if dw > maxLineWidth {
			maxLineWidth = dw
		}
	}
	maxHorizontal := max(maxLineWidth-width, 0)
	if m.x > maxHorizontal {
		m.x = maxHorizontal
	}
	if m.x < 0 {
		m.x = 0
	}

	end := min(start+bodySize, len(bodyLines))

	for i := start; i < end; i++ {
		line := bodyLines[i]
		if strings.Contains(line, "\x1b") {
			// Don't truncate or scroll lines with escape sequences (Kitty images)
			lines = append(lines, line)
			continue
		}
		// Apply horizontal scrolling offset m.x
		if m.x > 0 {
			if m.x < len(line) {
				line = line[m.x:]
			} else {
				line = ""
			}
		}
		
		padded := Truncate(line, width, "...")
		dw := DisplayWidth(padded)
		if dw < width {
			padded += strings.Repeat(" ", width-dw)
		}
		lines = append(lines, padded)
	}

	// Pad to fixed height to avoid panel jitter when content is short.
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
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
			if strings.Contains(part, "\x1b") {
				lines = append(lines, part)
				continue
			}
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
