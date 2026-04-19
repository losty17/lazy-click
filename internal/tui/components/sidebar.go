package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type SidebarModel struct {
	items []string
	idx   int
	x     int
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

func (m *SidebarModel) SetSelectedIndex(idx int) {
	if len(m.items) == 0 {
		m.idx = 0
		return
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(m.items) {
		idx = len(m.items) - 1
	}
	m.idx = idx
}

func (m *SidebarModel) MoveHorizontal(delta int) {
	next := max(m.x + delta, 0)
	m.x = next
}

func (m SidebarModel) Render(active bool, width int, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	// First line is a static header; remaining lines are list rows.
	header := "Sidebar (Workspace > Space > List)"
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75"))
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	_ = active
	lines := []string{headerStyle.Render(lineWindow(header, width, m.x))} 

	// Keep the selected item centered when possible and only render the visible slice.
	bodySize := max(height - 1, 0)
	start, end := visibleWindow(len(m.items), m.idx, bodySize)
	for i := start; i < end; i++ {
		item := m.items[i]
		prefix := "  "
		style := lipgloss.NewStyle()
		if i == m.idx {
			prefix = "> "
			style = selectedStyle
		}

		// lineWindow applies horizontal scrolling (m.x) while preserving fixed row width.
		lines = append(lines, style.Render(lineWindow(prefix+item, width, m.x))) 
	}

	// Pad to the requested height so the panel always renders as a full rectangle.
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func truncateText(s string, width int) string {
	if len(s) <= width {
		return s
	}
	if width <= 3 {
		return s[:width]
	}
	return s[:width-3] + "..."
}
