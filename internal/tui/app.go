package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"lazy-click/internal/cache"
	"lazy-click/internal/provider"
	syncengine "lazy-click/internal/sync"
	"lazy-click/internal/tui/components"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SyncQueuer interface {
	QueueTaskUpdate(taskID string, update provider.TaskUpdate) error
	QueueAddComment(taskID string, text string) error
	Cycle(ctx context.Context) error
}

type RootModel struct {
	width       int
	height      int
	keymap      Keymap
	activePane  int
	repo        *cache.Repository
	sync        SyncQueuer
	provider    string
	statusLine  string
	lists       []cache.ListEntity
	sidebar     components.SidebarModel
	taskTable   components.TaskTableModel
	detailPanel components.DetailModel
	statuses    []string

	statusFilter  string
	searchMode    bool
	searchInput   string
	searchQuery   string
	searchBackup  string
	commentMode   bool
	commentInput  string
	commentTaskID string
	syncing       bool
	syncFrame     int
	syncError     string
}

type dataLoadedMsg struct {
	lists    []cache.ListEntity
	tasks    []cache.TaskEntity
	statuses []string
	err      error
}

type editResultMsg struct {
	err error
}

type syncResultMsg struct {
	err error
}

type commentResultMsg struct {
	err error
}

type pollTickMsg struct{}
type syncTickMsg struct{}

func NewRootModel(repo *cache.Repository, sync *syncengine.Engine, provider string, statusLine string) RootModel {
	if provider == "" {
		provider = "none"
	}
	return RootModel{
		repo:        repo,
		sync:        sync,
		provider:    provider,
		keymap:      DefaultKeymap(),
		statusLine:  statusLine,
		sidebar:     components.NewSidebar(),
		taskTable:   components.NewTaskTable(),
		detailPanel: components.NewDetail(),
	}
}

func (m RootModel) Init() tea.Cmd {
	return tea.Batch(m.loadDataCmd(), m.pollCmd())
}

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		if m.commentMode {
			switch msg.Type {
			case tea.KeyEsc:
				m.commentMode = false
				m.commentInput = ""
				m.commentTaskID = ""
				m.statusLine = "Comment canceled"
			case tea.KeyEnter:
				return m, m.submitCommentCmd()
			case tea.KeyBackspace, tea.KeyDelete:
				r := []rune(m.commentInput)
				if len(r) > 0 {
					m.commentInput = string(r[:len(r)-1])
				}
			default:
				if len(msg.Runes) > 0 {
					m.commentInput += string(msg.Runes)
				}
			}
			return m, nil
		}
		if m.searchMode {
			switch msg.Type {
			case tea.KeyEsc:
				m.searchMode = false
				m.searchInput = ""
				m.searchQuery = m.searchBackup
				return m, m.loadDataCmd()
			case tea.KeyEnter:
				m.searchMode = false
				m.searchQuery = strings.TrimSpace(m.searchInput)
				m.searchInput = ""
				if m.searchQuery == "" {
					m.statusLine = "Search cleared"
				} else {
					m.statusLine = "Search: " + m.searchQuery
				}
				return m, m.loadDataCmd()
			case tea.KeyBackspace, tea.KeyDelete:
				r := []rune(m.searchInput)
				if len(r) > 0 {
					m.searchInput = string(r[:len(r)-1])
				}
			default:
				if len(msg.Runes) > 0 {
					m.searchInput += string(msg.Runes)
				}
			}
			m.searchQuery = strings.TrimSpace(m.searchInput)
			return m, m.loadDataCmd()
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.activePane = (m.activePane + 1) % 3
		case "shift+tab", "backtab":
			m.activePane = (m.activePane + 2) % 3
		case m.keymap.Down, "down":
			m.handleMove(1)
			if m.activePane == 0 {
				return m, m.loadDataCmd()
			}
			m.refreshDetail()
		case m.keymap.Up, "up":
			m.handleMove(-1)
			if m.activePane == 0 {
				return m, m.loadDataCmd()
			}
			m.refreshDetail()
		case "h", "left":
			m.handleHorizontalMove(-2)
		case "l", "right":
			m.handleHorizontalMove(2)
		case "r":
			return m, m.loadDataCmd()
		case "s":
			if !m.syncing {
				m.syncing = true
				m.syncFrame = 0
				m.statusLine = "Sync in progress..."
				return m, tea.Batch(m.syncNowCmd(), m.syncTickCmd())
			}
		case m.keymap.Edit:
			if m.activePane == 1 {
				return m, m.editSelectedTaskCmd()
			}
		case m.keymap.AddComment:
			if m.activePane == 1 {
				row, ok := m.taskTable.Selected()
				if !ok {
					m.statusLine = "No task selected for comment"
					return m, nil
				}
				m.commentMode = true
				m.commentInput = ""
				m.commentTaskID = row.ID
				m.statusLine = "Compose comment and press Enter to submit (Esc to cancel)"
				return m, nil
			}
		case m.keymap.Filter:
			m.cycleStatusFilter(1)
			return m, m.loadDataCmd()
		case strings.ToUpper(m.keymap.Filter):
			m.cycleStatusFilter(-1)
			return m, m.loadDataCmd()
		case m.keymap.Search:
			m.searchMode = true
			m.searchInput = m.searchQuery
			m.searchBackup = m.searchQuery
			return m, nil
		}
	case dataLoadedMsg:
		if msg.err != nil {
			m.statusLine = "Load failed: " + msg.err.Error()
			return m, nil
		}
		m.lists = msg.lists
		sidebarItems := make([]string, 0, len(msg.lists))
		for _, list := range msg.lists {
			sidebarItems = append(sidebarItems, "List: "+list.Name)
		}
		m.sidebar.SetItems(sidebarItems)
		m.taskTable.SetRows(mapTasksToRows(msg.tasks))
		m.statuses = msg.statuses
		if m.statusFilter != "" && !containsString(m.statuses, m.statusFilter) {
			m.statusFilter = ""
		}
		m.refreshDetail()
		if len(msg.lists) == 0 {
			m.statusLine = "No lists in cache yet. Press 's' to sync now."
		} else if m.statusLine == "" {
			m.statusLine = "Loaded from local cache"
		}
	case editResultMsg:
		if msg.err != nil {
			m.statusLine = "Edit failed: " + msg.err.Error()
			return m, nil
		}
		m.statusLine = "Task title updated locally and queued for ClickUp push"
		return m, m.loadDataCmd()
	case syncResultMsg:
		m.syncing = false
		if msg.err != nil {
			m.syncError = msg.err.Error()
			m.statusLine = "Sync failed: " + msg.err.Error()
			return m, nil
		}
		m.syncError = ""
		m.statusLine = "Sync completed"
		return m, m.loadDataCmd()
	case commentResultMsg:
		m.commentMode = false
		m.commentInput = ""
		m.commentTaskID = ""
		if msg.err != nil {
			m.statusLine = "Comment failed: " + msg.err.Error()
			return m, nil
		}
		m.statusLine = "Comment saved locally and queued for ClickUp push"
		return m, m.loadDataCmd()
	case pollTickMsg:
		return m, tea.Batch(m.loadDataCmd(), m.pollCmd())
	case syncTickMsg:
		if !m.syncing {
			return m, nil
		}
		m.syncFrame++
		return m, m.syncTickCmd()
	}

	return m, nil
}

func (m *RootModel) handleMove(delta int) {
	switch m.activePane {
	case 0:
		m.sidebar.Move(delta)
	case 1:
		m.taskTable.Move(delta)
	case 2:
		m.detailPanel.Move(delta)
	}
}

func (m *RootModel) handleHorizontalMove(delta int) {
	switch m.activePane {
	case 0:
		m.sidebar.MoveHorizontal(delta)
	case 1:
		m.taskTable.MoveHorizontal(delta)
	case 2:
		m.detailPanel.MoveHorizontal(delta)
	}
}

func (m RootModel) View() string {
	// Compute all pane dimensions up front so every renderer gets a consistent viewport.
	totalWidth, sidebarInnerWidth, rightInnerWidth, sidebarInnerHeight, tableInnerHeight, detailInnerHeight := m.layout()
	header := HeaderStyle.Width(totalWidth).Render(truncateLine("lazy-click", totalWidth))
	const verticalPaneGap = 1
	const horizontalPaneGap = 3

	// Highlight the currently active pane by swapping to the focused border style.
	sidebarStyle := PanelStyle
	if m.activePane == 0 {
		sidebarStyle = FocusedPanelStyle
	}
	tableStyle := PanelStyle
	if m.activePane == 1 {
		tableStyle = FocusedPanelStyle
	}
	detailStyle := PanelStyle
	if m.activePane == 2 {
		detailStyle = FocusedPanelStyle
	}

	// Render each pane using its calculated inner size; PanelStyle adds frame/padding around it.
	sidebar := sidebarStyle.Width(sidebarInnerWidth).Height(sidebarInnerHeight).Render(
		m.sidebar.Render(m.activePane == 0, sidebarInnerWidth, sidebarInnerHeight),
	)
	table := tableStyle.Width(rightInnerWidth).Height(tableInnerHeight).Render(
		m.taskTable.Render(m.activePane == 1, rightInnerWidth, tableInnerHeight),
	)
	detail := detailStyle.Width(rightInnerWidth).Height(detailInnerHeight).Render(
		m.detailPanel.Render(m.activePane == 2, rightInnerWidth, detailInnerHeight),
	)

	// Stack task table over detail on the right, then place sidebar + right column side by side.
	right := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().MarginBottom(verticalPaneGap).Render(table),
		detail,
	)
	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().MarginRight(horizontalPaneGap).Render(sidebar),
		right,
	)

	statusFilter := "all"
	if m.statusFilter != "" {
		statusFilter = m.statusFilter
	}
	searchFilter := "off"
	if m.searchQuery != "" {
		searchFilter = m.searchQuery
	}
	status := fmt.Sprintf("Provider: %s | Status filter: %s | Search: %s", m.provider, statusFilter, searchFilter)
	help := "Keys: hjkl/arrows move, / search, i edit, c comment, f/F filter, r refresh, s sync, q quit"
	if m.commentMode {
		help = "Comment mode: type text, Enter submit, Esc cancel"
	} else if m.searchMode {
		help = fmt.Sprintf("Search mode: %s (type to filter, Enter apply, Esc cancel)", m.searchInput)
	}
	syncLine := m.syncProgressLine(totalWidth)

	// Assemble the full screen in top-to-bottom order.
	screen := strings.Join([]string{
		header,
		body,
		StatusStyle.Render(truncateLine(status, totalWidth)),
		syncLine,
		// StatusStyle.Render(truncateLine(m.statusLine, totalWidth)),
		HelpStyle.Render(truncateLine(help, totalWidth)),
	}, "\n")

	return lipgloss.NewStyle().
		Width(totalWidth).
		MaxWidth(totalWidth).
		MaxHeight(max(totalHeightFromModel(m.height)-1, 8)).
		Render(screen)
}

func (m RootModel) layout() (totalWidth int, sidebarInnerWidth int, rightInnerWidth int, sidebarInnerHeight int, tableInnerHeight int, detailInnerHeight int) {
	// Frame sizes are used to translate between outer panel size and inner content size.
	hFrame := PanelStyle.GetHorizontalFrameSize()
	vFrame := PanelStyle.GetVerticalFrameSize()
	const verticalPaneGap = 1
	const horizontalPaneGap = 3

	// Derive total drawable width from terminal size, with a fallback for initial render.
	if m.width > 0 {
		totalWidth = m.width - 2
	} else {
		totalWidth = 78
	}
	totalWidth = max(totalWidth, 20)

	// Reserve fixed lines for non-body UI elements (header/status/help), then allocate body height.
	totalHeight := max(totalHeightFromModel(m.height)-1, 8)
	reserved := 5 // header + status + sync + statusLine + help
	bodyOuterHeight := totalHeight - reserved
	minBodyOuter := (2 * vFrame) + 2
	if bodyOuterHeight < minBodyOuter {
		bodyOuterHeight = minBodyOuter
	}

	// Split horizontal space into sidebar (left) and content column (right).
	innerWidthBudget := max(totalWidth-(2*hFrame)-horizontalPaneGap, 2)

	sidebarInnerWidth = innerWidthBudget / 3
	minSidebar := 8
	maxSidebar := innerWidthBudget - 8
	if maxSidebar < minSidebar {
		maxSidebar = minSidebar
	}
	if sidebarInnerWidth < minSidebar {
		sidebarInnerWidth = minSidebar
	}
	if sidebarInnerWidth > maxSidebar {
		sidebarInnerWidth = maxSidebar
	}
	rightInnerWidth = innerWidthBudget - sidebarInnerWidth
	if rightInnerWidth < 8 {
		rightInnerWidth = 8
		sidebarInnerWidth = innerWidthBudget - rightInnerWidth
		if sidebarInnerWidth < 1 {
			sidebarInnerWidth = 1
			rightInnerWidth = innerWidthBudget - sidebarInnerWidth
		}
	}

	// Sidebar uses full body height; right column is split into table (top) and detail (bottom).
	sidebarInnerHeight = bodyOuterHeight - vFrame
	if sidebarInnerHeight < 1 {
		sidebarInnerHeight = 1
	}

	rightInnerHeightBudget := bodyOuterHeight - (2 * vFrame) - verticalPaneGap
	if rightInnerHeightBudget < 2 {
		rightInnerHeightBudget = 2
	}
	tableInnerHeight = (rightInnerHeightBudget * 2) / 3
	if tableInnerHeight < 1 {
		tableInnerHeight = 1
	}
	detailInnerHeight = rightInnerHeightBudget - tableInnerHeight
	if detailInnerHeight < 1 {
		detailInnerHeight = 1
		tableInnerHeight = rightInnerHeightBudget - 1
		if tableInnerHeight < 1 {
			tableInnerHeight = 1
		}
	}

	return totalWidth, sidebarInnerWidth, rightInnerWidth, sidebarInnerHeight, tableInnerHeight, detailInnerHeight
}

func totalHeightFromModel(height int) int {
	if height > 0 {
		return height
	}
	return 24
}

func truncateLine(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	return string(r[:width-1]) + "…"
}

func (m RootModel) loadDataCmd() tea.Cmd {
	if m.repo == nil {
		return func() tea.Msg { return dataLoadedMsg{err: fmt.Errorf("cache repository unavailable")} }
	}
	selectedListID := m.selectedListID()
	return func() tea.Msg {
		lists, err := m.repo.GetAllLists()
		if err != nil {
			return dataLoadedMsg{err: err}
		}

		if selectedListID == "" && len(lists) > 0 {
			selectedListID = lists[0].ID
		}

		tasks, err := m.repo.GetTasksByQuery(cache.TaskListQuery{
			ListID:        selectedListID,
			Statuses:      selectedStatusFilter(m.statusFilter),
			IncludeClosed: true,
		})
		if err != nil {
			return dataLoadedMsg{err: err}
		}
		tasks = fuzzyFindTasks(tasks, m.searchQuery)

		statuses, err := m.repo.GetTaskStatusesByList(selectedListID)
		if err != nil {
			return dataLoadedMsg{err: err}
		}

		return dataLoadedMsg{lists: lists, tasks: tasks, statuses: statuses}
	}
}

func (m RootModel) selectedListID() string {
	if len(m.lists) == 0 {
		return ""
	}
	idx := m.sidebar.SelectedIndex()
	if idx < 0 || idx >= len(m.lists) {
		return m.lists[0].ID
	}
	return m.lists[idx].ID
}

func (m *RootModel) refreshDetail() {
	selected, ok := m.taskTable.Selected()
	if !ok {
		m.detailPanel.SetSections([]string{"No task selected"})
		return
	}

	task, err := m.repo.GetTaskByID(selected.ID)
	if err != nil {
		m.detailPanel.SetSections([]string{
			"Title: " + selected.Title,
			"Failed to load task detail: " + err.Error(),
		})
		return
	}
	if task == nil {
		m.detailPanel.SetSections([]string{"Task not found in cache"})
		return
	}
	comments, err := m.repo.GetTaskComments(selected.ID, 50)
	if err != nil {
		comments = nil
	}

	descriptionLines := components.RenderMarkdownLines(task.DescriptionMD)
	commentLines := make([]string, 0, len(comments)+1)
	if len(comments) == 0 {
		commentLines = append(commentLines, "(no comments in cache)")
	} else {
		for _, c := range comments {
			author := c.AuthorName
			if author == "" {
				author = "unknown"
			}
			commentLines = append(commentLines, fmt.Sprintf("- %s: %s", author, strings.TrimSpace(c.BodyMD)))
		}
	}

	sections := []string{
		"Title: " + task.Title,
		"Status: " + task.Status,
		"Priority: " + selected.Priority,
		"Due Date: " + selected.DueDate,
		"Tags: " + selected.Tags,
		"",
		"Description:",
	}
	sections = append(sections, descriptionLines...)
	sections = append(sections, "", "Comments:")
	sections = append(sections, commentLines...)
	m.detailPanel.SetSections(sections)
}

func (m RootModel) editSelectedTaskCmd() tea.Cmd {
	row, ok := m.taskTable.Selected()
	if !ok {
		return func() tea.Msg { return editResultMsg{err: fmt.Errorf("no task selected")} }
	}
	if m.repo == nil || m.sync == nil {
		return func() tea.Msg { return editResultMsg{err: fmt.Errorf("sync/edit services unavailable")} }
	}

	return func() tea.Msg {
		newTitle := row.Title
		if strings.Contains(newTitle, " (edited)") {
			newTitle = strings.TrimSuffix(newTitle, " (edited)")
		} else {
			newTitle += " (edited)"
		}

		if err := m.repo.UpdateTaskTitle(row.ID, newTitle); err != nil {
			return editResultMsg{err: err}
		}
		if err := m.sync.QueueTaskUpdate(row.ID, provider.TaskUpdate{Title: &newTitle}); err != nil {
			return editResultMsg{err: err}
		}
		return editResultMsg{}
	}
}

func (m RootModel) syncNowCmd() tea.Cmd {
	if m.sync == nil {
		return func() tea.Msg { return syncResultMsg{err: fmt.Errorf("sync service unavailable")} }
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		err := m.sync.Cycle(ctx)
		return syncResultMsg{err: err}
	}
}

func (m RootModel) submitCommentCmd() tea.Cmd {
	taskID := m.commentTaskID
	text := strings.TrimSpace(m.commentInput)
	if taskID == "" {
		return func() tea.Msg { return commentResultMsg{err: fmt.Errorf("comment target not set")} }
	}
	if text == "" {
		return func() tea.Msg { return commentResultMsg{err: fmt.Errorf("comment cannot be empty")} }
	}
	if m.repo == nil || m.sync == nil {
		return func() tea.Msg { return commentResultMsg{err: fmt.Errorf("sync/comment services unavailable")} }
	}

	return func() tea.Msg {
		now := time.Now().UnixMilli()
		if err := m.repo.SaveComments([]cache.CommentEntity{
			{
				ID:            fmt.Sprintf("local-%d", now),
				TaskID:        taskID,
				AuthorName:    "you",
				BodyMD:        text,
				CreatedAtUnix: now,
			},
		}); err != nil {
			return commentResultMsg{err: err}
		}
		if err := m.sync.QueueAddComment(taskID, text); err != nil {
			return commentResultMsg{err: err}
		}
		return commentResultMsg{}
	}
}

func (m *RootModel) cycleStatusFilter(step int) {
	statusOptions := append([]string{""}, m.statuses...)
	if len(statusOptions) == 1 {
		m.statusFilter = ""
		m.statusLine = "No status values available for selected list"
		return
	}

	current := 0
	for i, status := range statusOptions {
		if status == m.statusFilter {
			current = i
			break
		}
	}
	next := (current + step) % len(statusOptions)
	if next < 0 {
		next += len(statusOptions)
	}
	m.statusFilter = statusOptions[next]
	if m.statusFilter == "" {
		m.statusLine = "Status filter: all"
	} else {
		m.statusLine = "Status filter: " + m.statusFilter
	}
}

func (m RootModel) syncTickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg {
		return syncTickMsg{}
	})
}

func (m RootModel) syncProgressLine(width int) string {
	if width < 10 {
		return ""
	}
	prefix := fmt.Sprintf("Sync [%s]: ", m.provider)
	if m.sync == nil {
		return SyncIdleStyle.Render(prefix + "disabled")
	}
	if !m.syncing {
		if m.syncError != "" {
			return SyncErrorStyle.Render(prefix + "failed: " + m.syncError)
		}
		return SyncIdleStyle.Render(prefix + "idle")
	}
	barWidth := max(10, min(24, width-len(prefix)-10))
	pos := m.syncFrame % barWidth
	cells := make([]rune, barWidth)
	for i := range cells {
		cells[i] = '░'
	}
	cells[pos] = '█'
	return SyncRunStyle.Render(fmt.Sprintf("%s[%s] running", prefix, string(cells)))
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func selectedStatusFilter(status string) []string {
	if status == "" {
		return nil
	}
	return []string{status}
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func (m RootModel) pollCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return pollTickMsg{}
	})
}

func mapTasksToRows(tasks []cache.TaskEntity) []components.TaskTableRow {
	rows := make([]components.TaskTableRow, 0, len(tasks))
	for _, task := range tasks {
		priority := "-"
		if task.PriorityLabel != "" {
			priority = task.PriorityLabel
		}
		due := "-"
		if task.DueAtUnixMS != nil {
			due = time.UnixMilli(*task.DueAtUnixMS).Format("2006-01-02")
		}
		rows = append(rows, components.TaskTableRow{
			ID:       task.ID,
			ListID:   task.ListID,
			Title:    task.Title,
			Status:   task.Status,
			Priority: priority,
			DueDate:  due,
			Tags:     "-",
		})
	}
	return rows
}

func fuzzyFindTasks(tasks []cache.TaskEntity, query string) []cache.TaskEntity {
	normalizedQuery := normalizeSearchText(query)
	if normalizedQuery == "" {
		return tasks
	}

	type scoredTask struct {
		task  cache.TaskEntity
		score int
		idx   int
	}

	scored := make([]scoredTask, 0, len(tasks))
	for i, task := range tasks {
		if score, ok := fuzzyScoreTask(task, normalizedQuery); ok {
			scored = append(scored, scoredTask{task: task, score: score, idx: i})
		}
	}
	sort.SliceStable(scored, func(i int, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].idx < scored[j].idx
		}
		return scored[i].score > scored[j].score
	})

	out := make([]cache.TaskEntity, 0, len(scored))
	for _, item := range scored {
		out = append(out, item.task)
	}
	return out
}

func fuzzyScoreTask(task cache.TaskEntity, query string) (int, bool) {
	due := "-"
	if task.DueAtUnixMS != nil {
		due = time.UnixMilli(*task.DueAtUnixMS).Format("2006-01-02")
	}

	fields := []string{
		task.Title,
		task.Status,
		task.PriorityLabel,
		due,
		task.DescriptionMD,
		task.CustomFieldsJSON,
	}
	bestScore := 0
	matched := false
	for _, field := range fields {
		score, ok := fuzzyScoreField(query, field)
		if !ok {
			continue
		}
		if !matched || score > bestScore {
			bestScore = score
		}
		matched = true
	}
	return bestScore, matched
}

func fuzzyScoreField(query string, candidate string) (int, bool) {
	candidate = normalizeSearchText(candidate)
	if query == "" {
		return 0, true
	}
	if candidate == "" {
		return 0, false
	}

	if idx := strings.Index(candidate, query); idx >= 0 {
		score := 1200 - (idx * 4) - (len(candidate) - len(query))
		return score, true
	}

	qRunes := []rune(query)
	cRunes := []rune(candidate)
	positions := make([]int, len(qRunes))
	qi := 0
	for ci, r := range cRunes {
		if r == qRunes[qi] {
			positions[qi] = ci
			qi++
			if qi == len(qRunes) {
				break
			}
		}
	}
	if qi != len(qRunes) {
		return 0, false
	}

	first := positions[0]
	last := positions[len(positions)-1]
	span := last - first + 1
	gaps := span - len(qRunes)
	score := 900 - (gaps * 10) - (first * 3) - (len(cRunes) - len(qRunes))
	if first == 0 {
		score += 60
	} else if cRunes[first-1] == ' ' {
		score += 30
	}
	return score, true
}

func normalizeSearchText(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return strings.Join(strings.Fields(s), " ")
}
