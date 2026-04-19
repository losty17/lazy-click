package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"lazy-click/internal/cache"
	"lazy-click/internal/provider"
	syncengine "lazy-click/internal/sync"
	"lazy-click/internal/tui/components"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	appStateLastOpenedListID = "ui.last_opened_list_id"
	appStateListSortMode     = "ui.list_sort_mode"
	appStateListSearchQuery  = "ui.list_search_query"
	appStateFavoritesOnly    = "ui.list_favorites_only"

	detailDebounceDelay = 3 * time.Second
)

type SyncQueuer interface {
	QueueTaskUpdate(taskID string, update provider.TaskUpdate) error
	QueueAddComment(taskID string, text string) error
	Cycle(ctx context.Context) error
	SyncList(ctx context.Context, listID string) error
	SetActiveListID(listID string)
	RevalidateTask(ctx context.Context, taskID string) error
	SyncStatus() string
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

	statusFilter string
	searchMode   bool
	searchInput  string
	searchQuery  string
	searchBackup string

	listSearchMode   bool
	listSearchInput  string
	listSearchQuery  string
	listSearchBackup string
	listSortMode     cache.ListSortMode
	favoritesOnly    bool
	stateHydrated    bool
	selectedListID   string

	commentMode   bool
	commentInput  string
	commentTaskID string
	syncing       bool
	syncFrame     int
	syncError     string
	syncDetail    string

	selectedTaskID   string
	displayedTaskID  string
	detailLoading    bool
	detailError      string
	detailErrorTask  string
	detailReqToken   int64
	detailReqTaskID  string
	detailLoadingMsg string
}

type dataLoadedMsg struct {
	lists              []cache.ListEntity
	tasks              []cache.TaskEntity
	statuses           []string
	err                error
	selectedListID     string
	restored           bool
	restoredListSearch string
	restoredSortMode   cache.ListSortMode
	restoredFavOnly    bool
}

type editResultMsg struct{ err error }
type syncResultMsg struct{ err error }
type commentResultMsg struct{ err error }
type pollTickMsg struct{}
type syncTickMsg struct{}

type detailRevalidateTickMsg struct {
	TaskID string
	Token  int64
}

type detailRevalidateResultMsg struct {
	TaskID string
	Token  int64
	Err    error
}

type manualTaskRefreshResultMsg struct {
	TaskID string
	Err    error
}

func NewRootModel(repo *cache.Repository, sync *syncengine.Engine, providerName string, statusLine string) RootModel {
	if providerName == "" {
		providerName = "none"
	}
	var syncer SyncQueuer
	if sync != nil {
		syncer = sync
	}
	return RootModel{
		repo:          repo,
		sync:          syncer,
		provider:      providerName,
		keymap:        DefaultKeymap(),
		statusLine:    statusLine,
		sidebar:       components.NewSidebar(),
		taskTable:     components.NewTaskTable(),
		detailPanel:   components.NewDetail(),
		listSortMode:  cache.ListSortNameAsc,
		favoritesOnly: false,
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
					m.statusLine = "Task search cleared"
				} else {
					m.statusLine = "Task search: " + m.searchQuery
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

		if m.listSearchMode {
			switch msg.Type {
			case tea.KeyEsc:
				m.listSearchMode = false
				m.listSearchInput = ""
				m.listSearchQuery = m.listSearchBackup
				m.saveListPrefs()
				return m, m.loadDataCmd()
			case tea.KeyEnter:
				m.listSearchMode = false
				m.listSearchQuery = strings.TrimSpace(m.listSearchInput)
				m.listSearchInput = ""
				if m.listSearchQuery == "" {
					m.statusLine = "List search cleared"
				} else {
					m.statusLine = "List search: " + m.listSearchQuery
				}
				m.saveListPrefs()
				return m, m.loadDataCmd()
			case tea.KeyBackspace, tea.KeyDelete:
				r := []rune(m.listSearchInput)
				if len(r) > 0 {
					m.listSearchInput = string(r[:len(r)-1])
				}
			default:
				if len(msg.Runes) > 0 {
					m.listSearchInput += string(msg.Runes)
				}
			}
			m.listSearchQuery = strings.TrimSpace(m.listSearchInput)
			m.saveListPrefs()
			return m, m.loadDataCmd()
		}

		prevListID := m.selectedListID
		if msg.Type == tea.KeyEnter && m.activePane == 1 {
			return m, m.selectCursorTaskForDisplayCmd()
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
		case m.keymap.Up, "up":
			m.handleMove(-1)
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
				m.syncDetail = "starting manual sync"
				return m, tea.Batch(m.syncNowCmd(), m.syncTickCmd())
			}
		case m.keymap.Edit:
			if m.activePane == 1 {
				return m, m.editSelectedTaskCmd()
			}
		case m.keymap.RefreshTask:
			if m.activePane == 1 {
				return m, m.refreshCurrentTaskNowCmd()
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
		case m.keymap.ListSearch:
			m.listSearchMode = true
			m.listSearchInput = m.listSearchQuery
			m.listSearchBackup = m.listSearchQuery
			return m, nil
		case m.keymap.SortLists:
			if m.listSortMode == cache.ListSortMostRecentlyOpen {
				m.listSortMode = cache.ListSortNameAsc
			} else {
				m.listSortMode = cache.ListSortMostRecentlyOpen
			}
			m.saveListPrefs()
			m.statusLine = "List sort: " + string(m.listSortMode)
			return m, m.loadDataCmd()
		case m.keymap.FavOnly:
			m.favoritesOnly = !m.favoritesOnly
			m.saveListPrefs()
			if m.favoritesOnly {
				m.statusLine = "Favorites-only lists enabled"
			} else {
				m.statusLine = "Favorites-only lists disabled"
			}
			return m, m.loadDataCmd()
		case m.keymap.Favorite:
			if m.activePane == 0 {
				if err := m.repo.ToggleListFavorite(m.selectedListID); err != nil {
					m.statusLine = "Favorite toggle failed: " + err.Error()
					return m, nil
				}
				m.statusLine = "List favorite updated"
				return m, m.loadDataCmd()
			}
		}

		if m.activePane == 0 {
			newListID := m.selectedListIDFromSidebar()
			if newListID != "" && newListID != prevListID {
				m.selectedListID = newListID
				m.selectedTaskID = ""
				m.displayedTaskID = ""
				m.taskTable.SetDisplayedTaskID("")
				m.detailLoading = false
				m.detailLoadingMsg = ""
				m.markListOpened(newListID)
				m.persistLastOpenedList(newListID)
				if m.sync != nil {
					m.sync.SetActiveListID(newListID)
				}
				return m, m.loadDataCmd()
			}
		}

	case dataLoadedMsg:
		if msg.err != nil {
			m.statusLine = "Load failed: " + msg.err.Error()
			return m, nil
		}
		previousListID := m.selectedListID

		if msg.restored && !m.stateHydrated {
			m.stateHydrated = true
			m.listSearchQuery = msg.restoredListSearch
			if msg.restoredSortMode != "" {
				m.listSortMode = msg.restoredSortMode
			}
			m.favoritesOnly = msg.restoredFavOnly
		}

		m.lists = msg.lists
		m.selectedListID = msg.selectedListID
		if m.selectedListID != "" && m.selectedListID != previousListID {
			m.markListOpened(m.selectedListID)
			m.persistLastOpenedList(m.selectedListID)
		}
		if m.sync != nil {
			m.sync.SetActiveListID(m.selectedListID)
		}

		sidebarItems := make([]string, 0, len(msg.lists))
		selectedIdx := 0
		for i, list := range msg.lists {
			prefix := " "
			if list.Favorite {
				prefix = "*"
			}
			sidebarItems = append(sidebarItems, fmt.Sprintf("%s %s", prefix, list.Name))
			if list.ID == m.selectedListID {
				selectedIdx = i
			}
		}
		m.sidebar.SetItems(sidebarItems)
		m.sidebar.SetSelectedIndex(selectedIdx)

		m.taskTable.SetRows(mapTasksToRows(msg.tasks))
		if m.displayedTaskID != "" {
			if _, ok := m.taskTable.RowByID(m.displayedTaskID); !ok {
				m.displayedTaskID = ""
				m.selectedTaskID = ""
				m.detailLoading = false
				m.detailLoadingMsg = ""
			}
		}
		m.taskTable.SetDisplayedTaskID(m.displayedTaskID)
		m.statuses = msg.statuses
		if m.statusFilter != "" && !containsString(m.statuses, m.statusFilter) {
			m.statusFilter = ""
		}
		m.refreshDetail(m.detailLoading && m.detailReqTaskID == m.displayedTaskID, m.detailLoadingMsg)
		if len(msg.lists) == 0 {
			m.statusLine = "No lists in cache yet. Press 's' to sync now."
		} else if m.statusLine == "" {
			m.statusLine = "Loaded from local cache"
		}
		return m, nil

	case detailRevalidateTickMsg:
		if msg.Token != m.detailReqToken {
			return m, nil
		}
		if msg.TaskID == "" || msg.TaskID != m.selectedTaskID {
			return m, nil
		}
		return m, m.revalidateDetailCmd(msg.TaskID, msg.Token)

	case detailRevalidateResultMsg:
		if msg.Token != m.detailReqToken {
			return m, nil
		}
		if msg.TaskID == "" || msg.TaskID != m.selectedTaskID {
			return m, nil
		}
		m.detailLoading = false
		m.detailLoadingMsg = ""
		if msg.Err != nil {
			m.detailError = msg.Err.Error()
			m.detailErrorTask = msg.TaskID
			m.syncDetail = "task revalidate failed"
			m.refreshDetail(false, "")
			return m, nil
		}
		m.detailError = ""
		m.detailErrorTask = ""
		m.displayedTaskID = msg.TaskID
		m.taskTable.SetDisplayedTaskID(msg.TaskID)
		m.syncDetail = "task revalidated"
		m.refreshDetail(false, "")

	case manualTaskRefreshResultMsg:
		m.detailLoading = false
		m.detailLoadingMsg = ""
		if msg.Err != nil {
			m.detailError = msg.Err.Error()
			m.detailErrorTask = msg.TaskID
			m.syncDetail = "manual task refresh failed"
			m.statusLine = "Task refresh failed: " + msg.Err.Error()
			m.refreshDetail(false, "")
			return m, nil
		}
		m.detailError = ""
		m.detailErrorTask = ""
		m.displayedTaskID = msg.TaskID
		m.taskTable.SetDisplayedTaskID(msg.TaskID)
		m.syncDetail = "manual task refresh complete"
		m.statusLine = "Task refreshed"
		m.refreshDetail(false, "")

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
		if m.sync != nil {
			m.syncDetail = m.sync.SyncStatus()
		}
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
		if m.sync != nil {
			m.syncDetail = m.sync.SyncStatus()
		}
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
	totalWidth, sidebarInnerWidth, rightInnerWidth, sidebarInnerHeight, tableInnerHeight, detailInnerHeight := m.layout()
	header := HeaderStyle.Width(totalWidth).Render(truncateLine("lazy-click", totalWidth))
	const verticalPaneGap = 1
	const horizontalPaneGap = 3

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

	sidebar := sidebarStyle.Width(sidebarInnerWidth).Height(sidebarInnerHeight).Render(
		m.sidebar.Render(m.activePane == 0, sidebarInnerWidth - 4, sidebarInnerHeight),
	)
	table := tableStyle.Width(rightInnerWidth).Height(tableInnerHeight).Render(
		m.taskTable.Render(m.activePane == 1, rightInnerWidth - 2, tableInnerHeight),
	)
	detail := detailStyle.Width(rightInnerWidth).Height(detailInnerHeight).Render(
		m.detailPanel.Render(m.activePane == 2, rightInnerWidth - 2, detailInnerHeight),
	)

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
	taskSearch := "off"
	if m.searchQuery != "" {
		taskSearch = m.searchQuery
	}
	listSearch := "off"
	if m.listSearchQuery != "" {
		listSearch = m.listSearchQuery
	}
	status := fmt.Sprintf(
		"Provider: %s | List sort: %s | Favorites-only: %t | List search: %s | Task status: %s | Task search: %s",
		m.provider,
		m.listSortMode,
		m.favoritesOnly,
		listSearch,
		statusFilter,
		taskSearch,
	)

	help := "Keys: hjkl/arrows move, / task search, ? list search, * favorite list, o sort lists, v favorites-only, i edit, R refresh task, c comment, f/F status, r refresh, s sync, q quit"
	if m.commentMode {
		help = "Comment mode: type text, Enter submit, Esc cancel"
	} else if m.searchMode {
		help = fmt.Sprintf("Task search mode: %s (type to filter, Enter apply, Esc cancel)", m.searchInput)
	} else if m.listSearchMode {
		help = fmt.Sprintf("List search mode: %s (type to filter, Enter apply, Esc cancel)", m.listSearchInput)
	} else {
		help = "Keys: hjkl/arrows move cursor, Enter open task, / task search, ? list search, * favorite list, o sort lists, v favorites-only, i edit, R refresh opened task, c comment, f/F status, r refresh, s sync, q quit"
	}

	syncLine := m.syncProgressLine(totalWidth)

	screen := strings.Join([]string{
		header,
		body,
		StatusStyle.Render(truncateLine(status, totalWidth)),
		StatusStyle.Render(truncateLine(m.statusLine, totalWidth)),
		syncLine,
		HelpStyle.Render(truncateLine(help, totalWidth)),
	}, "\n")

	return lipgloss.NewStyle().
		Width(totalWidth).
		MaxWidth(totalWidth).
		MaxHeight(max(totalHeightFromModel(m.height)-1, 8)).
		Render(screen)
}

func (m RootModel) layout() (int, int, int, int, int, int) {
	hFrame := PanelStyle.GetHorizontalFrameSize()
	vFrame := PanelStyle.GetVerticalFrameSize()
	const verticalPaneGap = 1
	const horizontalPaneGap = 3

	totalWidth := 78
	if m.width > 0 {
		totalWidth = m.width - 2
	}
	totalWidth = max(totalWidth, 20)

	totalHeight := max(totalHeightFromModel(m.height)-1, 8)
	reserved := 6
	bodyOuterHeight := totalHeight - reserved
	minBodyOuter := (2 * vFrame) + 2
	if bodyOuterHeight < minBodyOuter {
		bodyOuterHeight = minBodyOuter
	}

	innerWidthBudget := max(totalWidth-(2*hFrame)-horizontalPaneGap, 2)
	sidebarInnerWidth := innerWidthBudget / 3
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
	rightInnerWidth := innerWidthBudget - sidebarInnerWidth
	if rightInnerWidth < 8 {
		rightInnerWidth = 8
		sidebarInnerWidth = innerWidthBudget - rightInnerWidth
		if sidebarInnerWidth < 1 {
			sidebarInnerWidth = 1
			rightInnerWidth = innerWidthBudget - sidebarInnerWidth
		}
	}

	sidebarInnerHeight := bodyOuterHeight - vFrame
	if sidebarInnerHeight < 1 {
		sidebarInnerHeight = 1
	}

	rightInnerHeightBudget := bodyOuterHeight - (2 * vFrame) - verticalPaneGap
	if rightInnerHeightBudget < 2 {
		rightInnerHeightBudget = 2
	}
	tableInnerHeight := (rightInnerHeightBudget * 2) / 3
	if tableInnerHeight < 1 {
		tableInnerHeight = 1
	}
	detailInnerHeight := rightInnerHeightBudget - tableInnerHeight
	if detailInnerHeight < 1 {
		detailInnerHeight = 1
		tableInnerHeight = rightInnerHeightBudget - 1
		if tableInnerHeight < 1 {
			tableInnerHeight = 1
		}
	}

	return totalWidth, sidebarInnerWidth, rightInnerWidth, sidebarInnerHeight, tableInnerHeight, detailInnerHeight
}

func (m RootModel) loadDataCmd() tea.Cmd {
	if m.repo == nil {
		return func() tea.Msg { return dataLoadedMsg{err: fmt.Errorf("cache repository unavailable")} }
	}

	currentListID := m.selectedListID
	currentSearch := m.listSearchQuery
	currentSort := m.listSortMode
	currentFavOnly := m.favoritesOnly
	hydrated := m.stateHydrated
	statusFilter := m.statusFilter
	taskSearch := m.searchQuery

	return func() tea.Msg {
		msg := dataLoadedMsg{}
		if !hydrated {
			search, err := m.repo.GetAppState(appStateListSearchQuery)
			if err != nil {
				return dataLoadedMsg{err: err}
			}
			sortMode := cache.ListSortNameAsc
			if sortRaw, err := m.repo.GetAppState(appStateListSortMode); err != nil {
				return dataLoadedMsg{err: err}
			} else if sortRaw == string(cache.ListSortMostRecentlyOpen) {
				sortMode = cache.ListSortMostRecentlyOpen
			}
			favOnly := false
			if favRaw, err := m.repo.GetAppState(appStateFavoritesOnly); err != nil {
				return dataLoadedMsg{err: err}
			} else if favRaw != "" {
				if parsed, parseErr := strconv.ParseBool(favRaw); parseErr == nil {
					favOnly = parsed
				}
			}
			lastOpened, err := m.repo.GetAppState(appStateLastOpenedListID)
			if err != nil {
				return dataLoadedMsg{err: err}
			}
			if currentListID == "" {
				currentListID = lastOpened
			}
			if currentListID == "" {
				if recent, err := m.repo.GetMostRecentlyOpenedListID(); err == nil {
					currentListID = recent
				}
			}
			currentSearch = search
			currentSort = sortMode
			currentFavOnly = favOnly
			msg.restored = true
			msg.restoredListSearch = currentSearch
			msg.restoredSortMode = currentSort
			msg.restoredFavOnly = currentFavOnly
		}

		lists, err := m.repo.GetListsByQuery(cache.ListQuery{
			Search:        currentSearch,
			FavoritesOnly: currentFavOnly,
			SortMode:      currentSort,
		})
		if err != nil {
			return dataLoadedMsg{err: err}
		}

		selectedListID := currentListID
		if selectedListID == "" && len(lists) > 0 {
			selectedListID = lists[0].ID
		}
		if selectedListID != "" && !containsListID(lists, selectedListID) {
			if len(lists) > 0 {
				selectedListID = lists[0].ID
			} else {
				selectedListID = ""
			}
		}

		tasks, err := m.repo.GetTasksByQuery(cache.TaskListQuery{
			ListID:        selectedListID,
			Statuses:      selectedStatusFilter(statusFilter),
			IncludeClosed: true,
		})
		if err != nil {
			return dataLoadedMsg{err: err}
		}
		tasks = fuzzyFindTasks(tasks, taskSearch)

		statuses, err := m.repo.GetTaskStatusesByList(selectedListID)
		if err != nil {
			return dataLoadedMsg{err: err}
		}

		msg.lists = lists
		msg.tasks = tasks
		msg.statuses = statuses
		msg.selectedListID = selectedListID
		return msg
	}
}

func (m RootModel) selectedListIDFromSidebar() string {
	if len(m.lists) == 0 {
		return ""
	}
	idx := m.sidebar.SelectedIndex()
	if idx < 0 || idx >= len(m.lists) {
		return m.lists[0].ID
	}
	return m.lists[idx].ID
}

func (m RootModel) currentSelectedTaskID() string {
	row, ok := m.taskTable.Selected()
	if !ok {
		return ""
	}
	return row.ID
}

func (m RootModel) displayedTaskRow() (components.TaskTableRow, bool) {
	if m.displayedTaskID == "" {
		return components.TaskTableRow{}, false
	}
	return m.taskTable.RowByID(m.displayedTaskID)
}

func (m *RootModel) refreshDetail(loading bool, loadingMsg string) {
	selected, ok := m.displayedTaskRow()
	if !ok {
		if loading {
			m.detailPanel.SetSections([]string{"Loading task detail..."})
			return
		}
		cursorTaskID := m.currentSelectedTaskID()
		if cursorTaskID == "" {
			m.detailPanel.SetSections([]string{"No task selected"})
			return
		}
		m.detailPanel.SetSections([]string{"No task opened. Press Enter on a task to open details."})
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

	sections := make([]string, 0, 32)
	if loading {
		if loadingMsg == "" {
			loadingMsg = "Revalidating detail..."
		}
		sections = append(sections, loadingMsg, "")
	}

	if task == nil {
		sections = append(sections,
			"Title: "+selected.Title,
			"Status: "+selected.Status,
			"Assignees: "+selected.Assignees,
		)
		if m.detailError != "" && m.detailErrorTask == selected.ID {
			sections = append(sections, "", "Last revalidate error: "+m.detailError)
		}
		sections = append(sections,
			"",
			"Loading task detail from provider...",
		)
		m.detailPanel.SetSections(sections)
		return
	}

	comments, err := m.repo.GetTaskComments(selected.ID, 50)
	if err != nil {
		comments = nil
	}

	priority := "-"
	if task.PriorityLabel != "" {
		priority = task.PriorityLabel
	}
	assignees := formatAssignees(task.AssigneesJSON)
	dueDate := "-"
	if task.DueAtUnixMS != nil {
		dueDate = time.UnixMilli(*task.DueAtUnixMS).Format("2006-01-02")
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

	sections = append(sections,
		"Title: "+task.Title,
		"Status: "+task.Status,
		"Priority: "+priority,
		"Due Date: "+dueDate,
		"Assignees: "+assignees,
	)
	if m.detailError != "" && m.detailErrorTask == selected.ID {
		sections = append(sections, "", "Last revalidate error: "+m.detailError)
	}
	sections = append(sections, "", "Description:")
	sections = append(sections, descriptionLines...)
	sections = append(sections, "", "Comments:")
	sections = append(sections, commentLines...)
	m.detailPanel.SetSections(sections)
}

func (m *RootModel) startDetailRevalidate() tea.Cmd {
	taskID := m.currentSelectedTaskID()
	m.selectedTaskID = taskID
	m.displayedTaskID = taskID
	m.taskTable.SetDisplayedTaskID(taskID)
	if taskID == "" {
		m.detailLoading = false
		m.detailLoadingMsg = ""
		m.refreshDetail(false, "")
		return nil
	}

	m.detailLoading = true
	m.detailReqTaskID = taskID
	m.detailReqToken++
	token := m.detailReqToken
	m.detailLoadingMsg = "Revalidating detail..."
	m.syncDetail = "waiting for detail revalidate debounce"
	m.refreshDetail(true, m.detailLoadingMsg)

	return tea.Tick(detailDebounceDelay, func(time.Time) tea.Msg {
		return detailRevalidateTickMsg{TaskID: taskID, Token: token}
	})
}

func (m *RootModel) selectCursorTaskForDisplayCmd() tea.Cmd {
	taskID := m.currentSelectedTaskID()
	if taskID == "" {
		m.displayedTaskID = ""
		m.taskTable.SetDisplayedTaskID("")
		m.refreshDetail(false, "")
		return nil
	}
	if taskID == m.displayedTaskID && !m.detailLoading {
		m.refreshDetail(false, "")
		return nil
	}
	return m.startDetailRevalidate()
}

func (m RootModel) revalidateDetailCmd(taskID string, token int64) tea.Cmd {
	if m.sync == nil {
		return func() tea.Msg {
			return detailRevalidateResultMsg{TaskID: taskID, Token: token, Err: fmt.Errorf("sync service unavailable")}
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		err := m.sync.RevalidateTask(ctx, taskID)
		return detailRevalidateResultMsg{TaskID: taskID, Token: token, Err: err}
	}
}

func (m *RootModel) refreshCurrentTaskNowCmd() tea.Cmd {
	taskID := m.displayedTaskID
	if taskID == "" {
		taskID = m.currentSelectedTaskID()
	}
	if taskID == "" {
		return nil
	}
	if m.sync == nil {
		return func() tea.Msg {
			return manualTaskRefreshResultMsg{TaskID: taskID, Err: fmt.Errorf("sync service unavailable")}
		}
	}
	m.selectedTaskID = taskID
	m.displayedTaskID = taskID
	m.taskTable.SetDisplayedTaskID(taskID)
	m.detailLoading = true
	m.detailReqTaskID = taskID
	m.detailReqToken++
	m.detailLoadingMsg = "Refreshing task now..."
	m.syncDetail = "manual task refresh in progress"
	m.refreshDetail(true, m.detailLoadingMsg)

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		err := m.sync.RevalidateTask(ctx, taskID)
		return manualTaskRefreshResultMsg{TaskID: taskID, Err: err}
	}
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
	listID := m.selectedListID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*60*time.Second)
		defer cancel()
		err := m.sync.SyncList(ctx, listID)
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
		if err := m.repo.SaveComments([]cache.CommentEntity{{
			ID:            fmt.Sprintf("local-%d", now),
			TaskID:        taskID,
			AuthorName:    "you",
			BodyMD:        text,
			CreatedAtUnix: now,
		}}); err != nil {
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
	detail := ""
	if m.sync != nil {
		detail = strings.TrimSpace(m.sync.SyncStatus())
	} else {
		detail = strings.TrimSpace(m.syncDetail)
	}
	if detail == "" {
		detail = "idle"
	}
	if !m.syncing {
		if m.syncError != "" {
			return SyncErrorStyle.Render(prefix + "failed: " + m.syncError + " | " + detail)
		}
		return SyncIdleStyle.Render(prefix + "idle | " + detail)
	}
	barWidth := max(10, min(20, width-len(prefix)-18))
	pos := m.syncFrame % barWidth
	cells := make([]rune, barWidth)
	for i := range cells {
		cells[i] = '░'
	}
	cells[pos] = '█'
	return SyncRunStyle.Render(fmt.Sprintf("%s[%s] %s", prefix, string(cells), detail))
}

func (m RootModel) pollCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return pollTickMsg{}
	})
}

func (m RootModel) persistLastOpenedList(listID string) {
	if listID == "" || m.repo == nil {
		return
	}
	_ = m.repo.SaveAppState(appStateLastOpenedListID, listID)
}

func (m RootModel) markListOpened(listID string) {
	if listID == "" || m.repo == nil {
		return
	}
	_ = m.repo.MarkListOpened(listID)
}

func (m RootModel) saveListPrefs() {
	if m.repo == nil {
		return
	}
	_ = m.repo.SaveAppState(appStateListSortMode, string(m.listSortMode))
	_ = m.repo.SaveAppState(appStateListSearchQuery, m.listSearchQuery)
	_ = m.repo.SaveAppState(appStateFavoritesOnly, strconv.FormatBool(m.favoritesOnly))
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

func containsListID(lists []cache.ListEntity, listID string) bool {
	for _, list := range lists {
		if list.ID == listID {
			return true
		}
	}
	return false
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
			ID:        task.ID,
			ListID:    task.ListID,
			Title:     formatTaskTitle(task),
			Status:    task.Status,
			Priority:  priority,
			DueDate:   due,
			Assignees: formatAssignees(task.AssigneesJSON),
		})
	}
	return rows
}

func formatTaskTitle(task cache.TaskEntity) string {
	if task.IsSubtask {
		return "~> " + task.Title
	}
	return task.Title
}

func formatAssignees(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "-"
	}
	var assignees []provider.User
	if err := json.Unmarshal([]byte(raw), &assignees); err != nil {
		return "-"
	}
	if len(assignees) == 0 {
		return "-"
	}
	names := make([]string, 0, len(assignees))
	for _, a := range assignees {
		if a.Username != "" {
			names = append(names, a.Username)
			continue
		}
		if a.Email != "" {
			names = append(names, a.Email)
		}
	}
	if len(names) == 0 {
		return "-"
	}
	return strings.Join(names, ", ")
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
