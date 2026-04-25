package components

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type TaskTableRow struct {
	Type        TaskTableRowType
	ID          string
	ListID      string
	Title       string
	Status      string
	StatusColor string
	Priority    string
	Estimate    string
	DueDate     string
	Assignees   string
	Indent      int
	CollapseKey string
	HiddenBy    []string
	Collapsed   bool
}

type TaskTableRowType string

const (
	TaskTableRowTask  TaskTableRowType = "task"
	TaskTableRowGroup TaskTableRowType = "group"
)

type TaskTableModel struct {
	rows            []TaskTableRow
	allRows         []TaskTableRow
	collapsed       map[string]bool
	idx             int
	x               int
	displayedTaskID string
	NoTasksMessage  string
}

func NewTaskTable() TaskTableModel {
	return TaskTableModel{rows: nil, collapsed: make(map[string]bool)}
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
	for _, row := range m.allRows {
		if row.ID == taskID {
			return row, true
		}
	}
	return TaskTableRow{}, false
}

func (m *TaskTableModel) ToggleSelectedCollapse() bool {
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

func (m *TaskTableModel) CollapseAll() (bool, bool) {
	if len(m.allRows) == 0 {
		return false, false
	}
	if m.collapsed == nil {
		m.collapsed = make(map[string]bool)
	}
	keys := make(map[string]struct{})
	allCollapsed := true
	for _, row := range m.allRows {
		if row.CollapseKey == "" {
			continue
		}
		keys[row.CollapseKey] = struct{}{}
		if !m.collapsed[row.CollapseKey] {
			allCollapsed = false
		}
	}
	if len(keys) == 0 {
		return false, false
	}
	targetCollapsed := true
	if allCollapsed {
		targetCollapsed = false
	}

	changed := false
	for key := range keys {
		if m.collapsed[key] != targetCollapsed {
			m.collapsed[key] = targetCollapsed
			changed = true
		}
	}
	if changed {
		m.applyVisibility()
		if m.idx >= len(m.rows) {
			m.idx = max(len(m.rows)-1, 0)
		}
	}
	return changed, targetCollapsed
}

func (m *TaskTableModel) MoveToTop() {
	m.idx = 0
}

func (m *TaskTableModel) MoveToBottom() {
	if len(m.rows) == 0 {
		m.idx = 0
		return
	}
	m.idx = len(m.rows) - 1
}

func (m *TaskTableModel) applyVisibility() {
	if m.collapsed == nil {
		m.collapsed = make(map[string]bool)
	}
	visible := make([]TaskTableRow, 0, len(m.allRows))
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

func (m *TaskTableModel) SetDisplayedTaskID(taskID string) {
	m.displayedTaskID = taskID
}

func (m *TaskTableModel) JumpToTask(taskID string) {
	for i, row := range m.rows {
		if row.ID == taskID {
			m.idx = i
			return
		}
	}
}

func (m *TaskTableModel) Render(active bool, width int, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	// Title + header are fixed; data rows fill the remaining height.
	title := "Tasks"
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75"))
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("223"))
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	groupStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("111"))
	displayedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("166")).Bold(true)
	selectedDisplayedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true)
	_ = active

	const (
		prefixWidth = 2
		sepWidth    = 3
	)
	sepCount := 5
	// Title / Status / Priority / Due Date / Estimate / Assignees
	weights := []int{75, 9, 1, 3, 1, 11}
	minCols := []int{12, 7, 9, 10, 8, 10}
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
		title := row.Title
		if row.CollapseKey != "" {
			if row.Collapsed {
				title = "[+] " + title
			} else {
				title = "[-] " + title
			}
		}
		if row.Indent > 0 {
			title = strings.Repeat(" ", row.Indent) + title
		}
		parts := []string{
			fitCell(title, col[0]),
			fitCell(strings.ToUpper(row.Status), col[1]),
			fitCell(priorityCellText(row.Priority), col[2]),
			fitCell(row.DueDate, col[3]),
			fitCell(row.Estimate, col[4]),
			fitCell(row.Assignees, col[5]),
		}
		return strings.Join(parts, " | ")
	}

	headerLine := "  " + strings.Join([]string{
		fitCell("Title", col[0]),
		fitCell("Status", col[1]),
		fitCell("Priority", col[2]),
		fitCell("Due Date", col[3]),
		fitCell("Estimate", col[4]),
		fitCell("Assignees", col[5]),
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
		msg := "  No tasks available for selected list"
		if m.NoTasksMessage != "" {
			msg = "  " + m.NoTasksMessage
		}
		lines = append(lines, lineWindow(msg, width, 0))
	}
	start, end := VisibleWindow(len(m.rows), m.idx, bodySize)
	for i := start; i < end; i++ {
		row := m.rows[i]
		prefix := "  "
		style := lipgloss.NewStyle()
		isSelected := i == m.idx
		isDisplayed := row.ID != "" && row.ID == m.displayedTaskID
		
		p1 := " "
		p2 := " "
		prefixStyle := lipgloss.NewStyle()
		
		if isSelected {
			p1 = ">"
			prefixStyle = selectedStyle
		}
		if isDisplayed {
			p2 = "*"
			if !isSelected {
				prefixStyle = displayedStyle
			} else {
				prefixStyle = selectedDisplayedStyle
			}
		}
		prefix = p1 + p2

		if isDisplayed {
			if isSelected {
				style = selectedDisplayedStyle
			} else {
				style = displayedStyle
			}
		}
		if lineWidth > width {
			line := prefixStyle.Render(prefix) + format(row)
			if row.Type == TaskTableRowGroup {
				style = groupStyle.Copy().Inherit(style)
			}
			lines = append(lines, style.Render(lineWindow(line, width, xOffset)))
			continue
		}

		titleValue := row.Title
		if row.CollapseKey != "" {
			if row.Collapsed {
				titleValue = "[+] " + titleValue
			} else {
				titleValue = "[-] " + titleValue
			}
		}
		if row.Indent > 0 {
			titleValue = strings.Repeat(" ", row.Indent) + titleValue
		}
		titleCell := fitCell(titleValue, col[0])
		statusCell := fitCell(strings.ToUpper(formatStatus(row.Status, col[1])), col[1])
		priorityCell := fitCell(priorityCellText(row.Priority), col[2])
		dueDateCell := fitCell(row.DueDate, col[3])
		estimateCell := fitCell(row.Estimate, col[4])
		assigneesCell := fitCell(row.Assignees, col[5])

		statusStyle := statusCellStyle(row.StatusColor)
		priorityStyle := priorityCellStyle(row.Priority)
		if isSelected && isDisplayed {
			statusStyle = statusStyle.Bold(true)
			priorityStyle = priorityStyle.Bold(true)
		}
		if row.Type == TaskTableRowGroup {
			statusStyle = lipgloss.NewStyle()
			priorityStyle = lipgloss.NewStyle()
			if isSelected {
				style = groupStyle.Copy().Inherit(style)
			} else {
				style = groupStyle
			}
		}

		line := strings.Join([]string{
			prefixStyle.Render(prefix),
			style.Render(titleCell + " | "),
			statusStyle.Render(statusCell),
			style.Render(" | "),
			priorityStyle.Render(priorityCell),
			style.Render(" | " + dueDateCell + " | " + estimateCell + " | " + assigneesCell),
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
		for _, i := range []int{0, 5, 2, 1, 3, 4} {
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

func (m *TaskTableModel) xForWidth(lineWidth int, viewportWidth int) int {
	if viewportWidth <= 0 || lineWidth <= viewportWidth {
		m.x = 0
		return 0
	}
	maxOffset := lineWidth - viewportWidth
	if m.x < 0 {
		m.x = 0
		return 0
	}
	if m.x > maxOffset {
		m.x = maxOffset
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

func priorityCellText(label string) string {
	trimmed := strings.TrimSpace(label)
	if trimmed == "" || trimmed == "-" {
		return "-"
	}
	return "⚑ " + strings.ToLower(trimmed)
}

func priorityCellStyle(label string) lipgloss.Style {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "urgent":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	case "high":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	case "normal":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	case "low":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	default:
		return lipgloss.NewStyle()
	}
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

func formatStatus(status string, width int) string {
	// center the text if possible, otherwise left align
	s := strings.ToUpper(strings.TrimSpace(status))
	
	if width <= len(s) {
		return s
	}

	padding := width - len(s)
	
	left := padding / 2
	right := padding - left
	
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}
