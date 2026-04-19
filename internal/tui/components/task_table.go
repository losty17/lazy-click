package components

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type TaskTableRow struct {
	ID          string
	ListID      string
	Title       string
	Status      string
	StatusColor string
	Priority    string
	DueDate     string
	Assignees   string
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
	weights := []int{42, 14, 10, 12, 22}
	minCols := []int{12, 7, 7, 10, 10}
	usable := max(width-prefixWidth-(sepCount*sepWidth), 1)
	minUsable := 0
	for _, w := range minCols {
		minUsable += w
	}

	col := fitColumns(usable, minCols, weights)
	lineWidth := width
	if usable < minUsable {
		col = append([]int(nil), minCols...)
		lineWidth = prefixWidth + (sepCount * sepWidth) + minUsable
	}

	format := func(row TaskTableRow) string {
		parts := []string{
			fitCell(row.Title, col[0]),
			fitCell(strings.ToUpper(row.Status), col[1]),
			fitCell(row.Priority, col[2]),
			fitCell(row.DueDate, col[3]),
			fitCell(row.Assignees, col[4]),
		}
		return strings.Join(parts, " | ")
	}

	headerLine := "  " + strings.Join([]string{
		fitCell("Title", col[0]),
		fitCell("Status", col[1]),
		fitCell("Priority", col[2]),
		fitCell("Due Date", col[3]),
		fitCell("Assignees", col[4]),
	}, " | ")

	xOffset := m.xForWidth(lineWidth, width)
	lines := []string{titleStyle.Render(lineWindow(title, width, 0))}
	if lineWidth > width {
		lines = append(lines, headerStyle.Render(lineWindow(headerLine, width, xOffset)))
	} else {
		lines = append(lines, headerStyle.Render(headerLine))
	}

	// Body viewport excludes title/header lines.
	bodySize := max(height-2, 0)
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
		if lineWidth > width {
			line := prefix + format(row)
			lines = append(lines, style.Render(lineWindow(line, width, xOffset)))
			continue
		}

		titleCell := fitCell(row.Title, col[0])
		statusCell := fitCell(strings.ToUpper(row.Status), col[1])
		priorityCell := fitCell(row.Priority, col[2])
		dueDateCell := fitCell(row.DueDate, col[3])
		assigneesCell := fitCell(row.Assignees, col[4])

		statusStyle := statusCellStyle(row.StatusColor)
		if isSelected && isDisplayed {
			statusStyle = statusStyle.Bold(true)
		}

		line := strings.Join([]string{
			style.Render(prefix + titleCell + " | "),
			statusStyle.Render(statusCell),
			style.Render(" | " + priorityCell + " | " + dueDateCell + " | " + assigneesCell),
		}, "")
		lines = append(lines, line)
	}

	// Fill trailing rows so panel height stays stable when item count is small.
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func fitColumns(usable int, minCols []int, weights []int) []int {
	cols := append([]int(nil), minCols...)
	if usable <= 0 || len(cols) == 0 {
		return cols
	}
	minTotal := 0
	for _, w := range minCols {
		minTotal += w
	}
	if usable <= minTotal {
		return cols
	}

	extra := usable - minTotal
	weightTotal := 0
	for _, w := range weights {
		if w > 0 {
			weightTotal += w
		}
	}
	if weightTotal <= 0 {
		weightTotal = len(cols)
		weights = make([]int, len(cols))
		for i := range weights {
			weights[i] = 1
		}
	}

	usedExtra := 0
	for i := range cols {
		gain := (weights[i] * extra) / weightTotal
		cols[i] += gain
		usedExtra += gain
	}

	for usedExtra < extra {
		for _, i := range []int{0, 4, 1, 3, 2} {
			if usedExtra >= extra {
				break
			}
			cols[i]++
			usedExtra++
		}
	}

	return cols
}

func fitCell(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) > width {
		if width <= 3 {
			return strings.Repeat(".", width)
		}
		value = string(runes[:width-3]) + "..."
	}
	cell := value
	cellRunes := []rune(cell)
	if len(cellRunes) < width {
		cell += strings.Repeat(" ", width-len(cellRunes))
	}
	return cell
}

func (m TaskTableModel) xForWidth(lineWidth int, viewportWidth int) int {
	if viewportWidth <= 0 || lineWidth <= viewportWidth {
		return 0
	}
	maxOffset := lineWidth - viewportWidth
	if m.x < 0 {
		return 0
	}
	if m.x > maxOffset {
		return maxOffset
	}
	return m.x
}

func statusCellStyle(rawHex string) lipgloss.Style {
	bg, ok := normalizeHexColor(rawHex)
	if !ok {
		return lipgloss.NewStyle()
	}
	fg := contrastColor(bg)
	return lipgloss.NewStyle().Foreground(lipgloss.Color(fg)).Background(lipgloss.Color(bg))
}

func normalizeHexColor(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}
	if strings.HasPrefix(s, "#") {
		s = s[1:]
	}
	if len(s) == 3 {
		s = strings.Repeat(string(s[0]), 1) + strings.Repeat(string(s[0]), 1) +
			strings.Repeat(string(s[1]), 1) + strings.Repeat(string(s[1]), 1) +
			strings.Repeat(string(s[2]), 1) + strings.Repeat(string(s[2]), 1)
	}
	if len(s) != 6 {
		return "", false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return "", false
		}
	}
	return "#" + strings.ToUpper(s), true
}

func contrastColor(hex string) string {
	s := strings.TrimPrefix(hex, "#")
	r, _ := strconv.ParseUint(s[0:2], 16, 8)
	g, _ := strconv.ParseUint(s[2:4], 16, 8)
	b, _ := strconv.ParseUint(s[4:6], 16, 8)
	brightness := (299*int(r) + 587*int(g) + 114*int(b)) / 1000
	if brightness >= 140 {
		return "#111111"
	}
	return "#FFFFFF"
}
