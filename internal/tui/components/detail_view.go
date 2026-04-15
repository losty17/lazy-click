package components

import "strings"

type DetailModel struct {
	sections []string
	idx      int
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
	if len(m.sections) == 0 {
		m.idx = 0
		return
	}
	next := m.idx + delta
	if next < 0 {
		next = 0
	}
	if next >= len(m.sections) {
		next = len(m.sections) - 1
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

func (m DetailModel) Render(active bool, width int, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	title := "Detail"
	if active {
		title += " [focused]"
	}
	lines := []string{truncateToWidth(title, width)}

	bodySize := height - 1
	if bodySize < 0 {
		bodySize = 0
	}
	start, end := visibleWindow(len(m.sections), m.idx, bodySize)
	for i := start; i < end; i++ {
		section := m.sections[i]
		prefix := "  "
		if i == m.idx {
			prefix = "> "
		}
		lines = append(lines, truncateToWidth(prefix+section, width))
	}

	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}
