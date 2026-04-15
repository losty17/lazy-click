package components

import "strings"

type TaskTableRow struct {
	ID       string
	ListID   string
	Title    string
	Status   string
	Priority string
	DueDate  string
	Tags     string
}

type TaskTableModel struct {
	rows []TaskTableRow
	idx  int
}

func NewTaskTable() TaskTableModel {
	return TaskTableModel{rows: nil}
}

func (m *TaskTableModel) Move(delta int) {
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

func (m *TaskTableModel) SetRows(rows []TaskTableRow) {
	m.rows = rows
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

func (m TaskTableModel) Selected() (TaskTableRow, bool) {
	if len(m.rows) == 0 || m.idx < 0 || m.idx >= len(m.rows) {
		return TaskTableRow{}, false
	}
	return m.rows[m.idx], true
}

func (m TaskTableModel) Render(active bool, width int, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	title := "Tasks"
	if active {
		title += " [focused]"
	}
	lines := []string{
		truncateToWidth(title, width),
		truncateToWidth("Title | Status | Priority | Due Date | Tags", width),
	}

	bodySize := height - 2
	if bodySize < 0 {
		bodySize = 0
	}
	if len(m.rows) == 0 {
		lines = append(lines, truncateToWidth("  No tasks available for selected list", width))
	}
	start, end := visibleWindow(len(m.rows), m.idx, bodySize)
	for i := start; i < end; i++ {
		row := m.rows[i]
		prefix := "  "
		if i == m.idx {
			prefix = "> "
		}
		line := prefix + row.Title + " | " + row.Status + " | " + row.Priority + " | " + row.DueDate + " | " + row.Tags
		lines = append(lines, truncateToWidth(line, width))
	}

	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}
