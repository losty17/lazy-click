package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type SidebarRowType string

const (
	SidebarRowList  SidebarRowType = "list"
	SidebarRowGroup SidebarRowType = "group"
)

type SidebarRow struct {
	Type        SidebarRowType
	ID          string
	Title       string
	Indent      int
	CollapseKey string
	HiddenBy    []string
	Collapsed   bool
}

type SidebarModel struct {
	rows      []SidebarRow
	allRows   []SidebarRow
	collapsed map[string]bool
	idx       int
	x         int
}

func NewSidebar() SidebarModel {
	return SidebarModel{collapsed: make(map[string]bool)}
}

func (m *SidebarModel) Move(delta int) {
	if len(m.rows) == 0 {
		m.idx = 0
		return
	}
	next := m.idx + delta
	if next < 0 {
		next = 0
	}
	if next >= len(m.rows) {
		next = len(m.rows) - 1
	}
	m.idx = next
}

func (m *SidebarModel) SetRows(rows []SidebarRow) {
	m.allRows = rows
	m.applyVisibility()
	if len(m.rows) == 0 {
		m.idx = 0
		return
	}
	if m.idx >= len(m.rows) {
		m.idx = len(m.rows) - 1
	}
	if m.idx < 0 {
		m.idx = 0
	}
}

func (m *SidebarModel) applyVisibility() {
	if m.collapsed == nil {
		m.collapsed = make(map[string]bool)
	}
	visible := make([]SidebarRow, 0, len(m.allRows))
	for _, row := range m.allRows {
		hidden := false
		for _, key := range row.HiddenBy {
			if m.collapsed[key] {
				hidden = true
				break
			}
		}
		if hidden {
			continue
		}
		copyRow := row
		if copyRow.CollapseKey != "" {
			copyRow.Collapsed = m.collapsed[copyRow.CollapseKey]
		}
		visible = append(visible, copyRow)
	}
	m.rows = visible
}

func (m *SidebarModel) ToggleSelectedCollapse() bool {
	row, ok := m.Selected()
	if !ok || row.CollapseKey == "" {
		return false
	}
	if m.collapsed == nil {
		m.collapsed = make(map[string]bool)
	}
	m.collapsed[row.CollapseKey] = !m.collapsed[row.CollapseKey]
	m.applyVisibility()
	if m.idx >= len(m.rows) {
		m.idx = max(len(m.rows)-1, 0)
	}
	return true
}

func (m SidebarModel) SelectedIndex() int {
	return m.idx
}

func (m *SidebarModel) SetSelectedIndex(idx int) {
	// Since rows can be hidden, we need to find the index in m.rows
	// based on ID if it's a list. This is handled by the caller usually.
	if len(m.rows) == 0 {
		m.idx = 0
		return
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(m.rows) {
		idx = len(m.rows) - 1
	}
	m.idx = idx
}

func (m SidebarModel) Selected() (SidebarRow, bool) {
	if len(m.rows) == 0 || m.idx < 0 || m.idx >= len(m.rows) {
		return SidebarRow{}, false
	}
	return m.rows[m.idx], true
}

func (m *SidebarModel) MoveHorizontal(delta int) {
	next := max(m.x+delta, 0)
	m.x = next
}

func (m *SidebarModel) MoveToTop() {
	m.idx = 0
}

func (m *SidebarModel) MoveToBottom() {
	if len(m.rows) == 0 {
		m.idx = 0
		return
	}
	m.idx = len(m.rows) - 1
}

func (m *SidebarModel) Render(active bool, width int, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	header := "Sidebar (Workspace > Space > List)"
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75"))
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	groupStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("111"))
	_ = active

	maxLineWidth := len(header)
	for _, row := range m.rows {
		w := len(row.Title) + 6 + row.Indent
		if w > maxLineWidth {
			maxLineWidth = w
		}
	}
	maxOffset := max(maxLineWidth-width, 0)
	if m.x > maxOffset {
		m.x = maxOffset
	}
	if m.x < 0 {
		m.x = 0
	}

	lines := []string{headerStyle.Render(lineWindow(header, width, m.x))}

	if len(m.rows) == 0 {
		lines = append(lines, "  No lists synced yet")
	}

	bodySize := max(height-1, 0)
	start, end := VisibleWindow(len(m.rows), m.idx, bodySize)
	for i := start; i < end; i++ {
		row := m.rows[i]
		prefix := "  "
		style := lipgloss.NewStyle()
		if i == m.idx {
			prefix = "> "
			style = selectedStyle
		}

		if row.Type == SidebarRowGroup {
			style = groupStyle.Copy().Inherit(style)
		}

		title := row.Title
		if row.CollapseKey != "" {
			if row.Collapsed {
				title = "[+] " + title
			} else {
				title = "[-] " + title
			}
		}

		text := prefix + strings.Repeat(" ", row.Indent) + title
		lines = append(lines, style.Render(lineWindow(text, width, m.x)))
	}

	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}

	return strings.Join(lines, "\n")
}
