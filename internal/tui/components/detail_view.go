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
	next := m.idx + delta
	if next < 0 {
		next = 0
	}
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
	next := m.x + delta
	if next < 0 {
		next = 0
	}
	m.x = next
}

func (m DetailModel) Render(active bool, width int, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	title := "Detail"
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75"))
	lines := []string{titleStyle.Render(truncateToWidth(title, width))}

	bodyLines := m.expandedLines()
	bodySize := height - 1
	if bodySize < 0 {
		bodySize = 0
	}
	start := m.idx
	if start < 0 {
		start = 0
	}
	maxStart := len(bodyLines) - bodySize
	if maxStart < 0 {
		maxStart = 0
	}
	if start > maxStart {
		start = maxStart
	}
	end := start + bodySize
	if end > len(bodyLines) {
		end = len(bodyLines)
	}
	for i := start; i < end; i++ {
		lines = append(lines, lineWindow(bodyLines[i], width, m.x))
	}

	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (m DetailModel) expandedLines() []string {
	if len(m.sections) == 0 {
		return []string{"  No task selected"}
	}
	lines := make([]string, 0, len(m.sections))
	for _, section := range m.sections {
		normalized := strings.ReplaceAll(section, "\r\n", "\n")
		normalized = strings.ReplaceAll(normalized, "\r", "\n")
		parts := strings.Split(normalized, "\n")
		if len(parts) == 0 {
			lines = append(lines, "  ")
			continue
		}
		for _, part := range parts {
			lines = append(lines, "  "+part)
		}
	}
	return lines
}
