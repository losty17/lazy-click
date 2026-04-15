package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"command-task/internal/cache"
	"command-task/internal/provider"
	syncengine "command-task/internal/sync"
	"command-task/internal/tui/components"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SyncQueuer interface {
	QueueTaskUpdate(taskID string, update provider.TaskUpdate) error
	Cycle(ctx context.Context) error
}

type RootModel struct {
	width       int
	height      int
	keymap      Keymap
	activePane  int
	repo        *cache.Repository
	sync        SyncQueuer
	statusLine  string
	lists       []cache.ListEntity
	sidebar     components.SidebarModel
	taskTable   components.TaskTableModel
	detailPanel components.DetailModel
}

type dataLoadedMsg struct {
	lists []cache.ListEntity
	tasks []cache.TaskEntity
	err   error
}

type editResultMsg struct {
	err error
}

type syncResultMsg struct {
	err error
}

type pollTickMsg struct{}

func NewRootModel(repo *cache.Repository, sync *syncengine.Engine, statusLine string) RootModel {
	return RootModel{
		repo:        repo,
		sync:        sync,
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
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.activePane = (m.activePane + 1) % 3
		case m.keymap.Down:
			m.handleMove(1)
			if m.activePane == 0 {
				return m, m.loadDataCmd()
			}
			m.refreshDetail()
		case m.keymap.Up:
			m.handleMove(-1)
			if m.activePane == 0 {
				return m, m.loadDataCmd()
			}
			m.refreshDetail()
		case "r":
			return m, m.loadDataCmd()
		case "s":
			return m, m.syncNowCmd()
		case m.keymap.Edit:
			if m.activePane == 1 {
				return m, m.editSelectedTaskCmd()
			}
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
		if msg.err != nil {
			m.statusLine = "Sync failed: " + msg.err.Error()
			return m, nil
		}
		m.statusLine = "Sync completed"
		return m, m.loadDataCmd()
	case pollTickMsg:
		return m, tea.Batch(m.loadDataCmd(), m.pollCmd())
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

func (m RootModel) View() string {
	totalWidth, sidebarInnerWidth, rightInnerWidth, sidebarInnerHeight, tableInnerHeight, detailInnerHeight := m.layout()
	header := HeaderStyle.Width(totalWidth).Render(truncateLine("Command-Task", totalWidth))

	sidebar := PanelStyle.Width(sidebarInnerWidth).Height(sidebarInnerHeight).Render(
		m.sidebar.Render(m.activePane == 0, sidebarInnerWidth, sidebarInnerHeight),
	)
	table := PanelStyle.Width(rightInnerWidth).Height(tableInnerHeight).Render(
		m.taskTable.Render(m.activePane == 1, rightInnerWidth, tableInnerHeight),
	)
	detail := PanelStyle.Width(rightInnerWidth).Height(detailInnerHeight).Render(
		m.detailPanel.Render(m.activePane == 2, rightInnerWidth, detailInnerHeight),
	)

	right := lipgloss.JoinVertical(lipgloss.Left, table, detail)
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, right)

	status := fmt.Sprintf("Active pane: %d (tab to switch)", m.activePane+1)
	help := "Keys: j/k navigate, / search, i edit task title, c comment, r refresh, s sync now, tab switch, q quit"

	screen := strings.Join([]string{
		header,
		body,
		truncateLine(status, totalWidth),
		truncateLine(m.statusLine, totalWidth),
		truncateLine(help, totalWidth),
	}, "\n")

	return lipgloss.NewStyle().
		Width(totalWidth).
		MaxWidth(totalWidth).
		MaxHeight(max(totalHeightFromModel(m.height)-1, 8)).
		Render(screen)
}

func (m RootModel) layout() (totalWidth int, sidebarInnerWidth int, rightInnerWidth int, sidebarInnerHeight int, tableInnerHeight int, detailInnerHeight int) {
	hFrame := PanelStyle.GetHorizontalFrameSize()
	vFrame := PanelStyle.GetVerticalFrameSize()

	if m.width > 0 {
		totalWidth = m.width - 2
	} else {
		totalWidth = 78
	}
	totalWidth = max(totalWidth, 20)

	totalHeight := totalHeightFromModel(m.height) - 1
	if totalHeight < 8 {
		totalHeight = 8
	}
	reserved := 4 // header + status + statusLine + help
	bodyOuterHeight := totalHeight - reserved
	minBodyOuter := (2 * vFrame) + 2
	if bodyOuterHeight < minBodyOuter {
		bodyOuterHeight = minBodyOuter
	}

	innerWidthBudget := totalWidth - (2 * hFrame)
	if innerWidthBudget < 2 {
		innerWidthBudget = 2
	}

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

	sidebarInnerHeight = bodyOuterHeight - vFrame
	if sidebarInnerHeight < 1 {
		sidebarInnerHeight = 1
	}

	rightInnerHeightBudget := bodyOuterHeight - (2 * vFrame)
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

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
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
			IncludeClosed: true,
		})
		if err != nil {
			return dataLoadedMsg{err: err}
		}

		return dataLoadedMsg{lists: lists, tasks: tasks}
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
	sections := []string{
		"Title: " + selected.Title,
		"Status: " + selected.Status,
		"Priority: " + selected.Priority,
		"Due Date: " + selected.DueDate,
		"Tags: " + selected.Tags,
	}
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
