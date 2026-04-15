package components

import "strings"

type SidebarModel struct {
	items []string
	idx   int
}

func NewSidebar() SidebarModel {
	return SidebarModel{items: []string{"No lists synced yet"}}
}

func (m *SidebarModel) Move(delta int) {
	if len(m.items) == 0 {
		m.idx = 0
		return
	}
	next := m.idx + delta
	if next < 0 {
		next = 0
	}
	if next >= len(m.items) {
		next = len(m.items) - 1
	}
	m.idx = next
}

func (m *SidebarModel) SetItems(items []string) {
	if len(items) == 0 {
		m.items = []string{"No lists synced yet"}
		m.idx = 0
		return
	}
	m.items = items
	if m.idx >= len(m.items) {
		m.idx = len(m.items) - 1
	}
	if m.idx < 0 {
		m.idx = 0
	}
}

func (m SidebarModel) SelectedIndex() int {
	return m.idx
}

func (m SidebarModel) Render(active bool, width int, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	header := "Sidebar (Workspace > Space > List)"
	if active {
		header += " [focused]"
	}
	lines := []string{truncateToWidth(header, width)}

	bodySize := height - 1
	if bodySize < 0 {
		bodySize = 0
	}
	start, end := visibleWindow(len(m.items), m.idx, bodySize)
	for i := start; i < end; i++ {
		item := m.items[i]
		prefix := "  "
		if i == m.idx {
			prefix = "> "
		}
		lines = append(lines, truncateToWidth(prefix+item, width))
	}

	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}
