package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type TaskTableRow struct {
	ID        string
	ListID    string
	Title     string
	Status    string
	Priority  string
	DueDate   string
	Assignees string
}

type TaskTableModel struct {
	rows            []TaskTableRow
	idx             int
	x               int
	displayedTaskID string
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

func (m *TaskTableModel) MoveHorizontal(delta int) {
	next := m.x + delta
	if next < 0 {
		next = 0
	}
	m.x = next
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

func (m TaskTableModel) RowByID(taskID string) (TaskTableRow, bool) {
	if taskID == "" {
		return TaskTableRow{}, false
	}
	for _, row := range m.rows {
		if row.ID == taskID {
			return row, true
		}
	}
	return TaskTableRow{}, false
}

func (m *TaskTableModel) SetDisplayedTaskID(taskID string) {
	m.displayedTaskID = taskID
}

func (m TaskTableModel) Render(active bool, width int, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	// Title + header are fixed; data rows fill the remaining height.
	title := "Tasks"
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75"))
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("223"))
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	displayedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("166"))
	selectedDisplayedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("33")).Bold(true)
	_ = active

	const (
		prefixWidth = 2
		sepWidth    = 3
	)
	sepCount := 4
	usable := width - prefixWidth - (sepCount * sepWidth)
	if usable < 5 {
		usable = 5
	}
	// Column widths are proportional to base weights and then balanced to exact width.
	base := []int{34, 14, 10, 10, 18}
	baseTotal := 86
	col := make([]int, len(base))
	sum := 0
	for i := range base {
		col[i] = (base[i] * usable) / baseTotal
		if col[i] < 1 {
			col[i] = 1
		}
		sum += col[i]
	}
	for sum < usable {
		for _, i := range []int{0, 4, 1, 2, 3} {
			if sum >= usable {
				break
			}
			col[i]++
			sum++
		}
	}
	for sum > usable {
		for _, i := range []int{0, 4, 1, 2, 3} {
			if sum <= usable {
				break
			}
			if col[i] > 1 {
				col[i]--
				sum--
			}
		}
	}

	format := func(row TaskTableRow) string {
		return fitCell(row.Title, col[0]) + " | " +
			fitCell(row.Status, col[1]) + " | " +
			fitCell(row.Priority, col[2]) + " | " +
			fitCell(row.DueDate, col[3]) + " | " +
			fitCell(row.Assignees, col[4])
	}

	headerLine := fitCell("Title", col[0]) + " | " +
		fitCell("Status", col[1]) + " | " +
		fitCell("Priority", col[2]) + " | " +
		fitCell("Due Date", col[3]) + " | " +
		fitCell("Assignees", col[4])

	lines := []string{
		titleStyle.Render(truncateToWidth(title, width)),
		headerStyle.Render(lineWindow(headerLine, width, m.x)),
	}

	// Body viewport excludes title/header lines.
	bodySize := height - 2
	if bodySize < 0 {
		bodySize = 0
	}
	if len(m.rows) == 0 {
		lines = append(lines, lineWindow("  No tasks available for selected list", width, 0))
	}
	start, end := visibleWindow(len(m.rows), m.idx, bodySize)
	for i := start; i < end; i++ {
		row := m.rows[i]
		prefix := "  "
		style := lipgloss.NewStyle()
		isSelected := i == m.idx
		isDisplayed := row.ID != "" && row.ID == m.displayedTaskID
		if isSelected {
			prefix = "> "
			style = selectedStyle
		}
		if isDisplayed {
			if isSelected {
				style = selectedDisplayedStyle
			} else {
				style = displayedStyle
			}
		}
		// lineWindow enables horizontal scrolling over wide rows.
		line := prefix + format(row)
		lines = append(lines, style.Render(lineWindow(line, width, m.x)))
	}

	// Fill trailing rows so panel height stays stable when item count is small.
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}
