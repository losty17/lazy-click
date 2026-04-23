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
	"lazy-click/internal/provider/clickup"
	syncengine "lazy-click/internal/sync"
	"lazy-click/internal/tui/components"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
)

const (
	appStateLastOpenedListID = "ui.last_opened_list_id"
	appStateListSortMode     = "ui.list_sort_mode"
	appStateListSearchQuery  = "ui.list_search_query"
	appStateFavoritesOnly    = "ui.list_favorites_only"
	appStateTaskSortMode     = "ui.task_sort_mode"
	appStateTaskGroupMode    = "ui.task_group_mode"
	appStateTaskSubtasksMode = "ui.task_subtasks_mode"
	appStateStatusFilter     = "ui.status_filter"
	appStateTaskSearchQuery  = "ui.task_search_query"
	appStateVimModeEnabled   = "ui.vim_mode.enabled"
	appStateRestorePolicy    = "ui.restore.policy"
	appStateRestoreSession   = "ui.restore.last_session"
	appStateRecentCommands   = "ui.control_center.recent_commands"
	appStateCommandUsage     = "ui.control_center.command_usage"
	appStateActiveProviderID = "ui.active_provider_id"
	appStateClickUpToken     = "provider.clickup.oauth_token"

	detailDebounceDelay = 3 * time.Second
)

type RestorePolicy string

const (
	RestorePolicyAsk    RestorePolicy = "ask"
	RestorePolicyAlways RestorePolicy = "always"
	RestorePolicyNever  RestorePolicy = "never"
)

type ControlMode string

const (
	ControlModeCommand ControlMode = ">"
	ControlModeList    ControlMode = "@"
	ControlModeTask    ControlMode = "#"
	ControlModeHelp    ControlMode = "?"
)

type controlResult struct {
	Kind      string
	Title     string
	Subtitle  string
	Badge     string
	CommandID string
	ListID    string
	TaskID    string
	TaskTitle string
}

type commandUsageStat struct {
	Count        int   `json:"count"`
	LastUsedUnix int64 `json:"last_used_unix"`
}

type uiSessionSnapshot struct {
	SelectedListID  string             `json:"selected_list_id"`
	DisplayedTaskID string             `json:"displayed_task_id"`
	ActivePane      int                `json:"active_pane"`
	ListSearchQuery string             `json:"list_search_query"`
	ListSortMode    cache.ListSortMode `json:"list_sort_mode"`
	FavoritesOnly   bool               `json:"favorites_only"`
	TaskSortMode    TaskSortMode       `json:"task_sort_mode"`
	TaskGroupMode   TaskGroupMode      `json:"task_group_mode"`
	TaskSubtasks    TaskSubtaskMode    `json:"task_subtasks"`
	StatusFilter    string             `json:"status_filter"`
	TaskSearchQuery string             `json:"task_search_query"`
	VimMode         bool               `json:"vim_mode"`
}

type TaskSortMode string

const (
	TaskSortPriority TaskSortMode = "priority"
	TaskSortName     TaskSortMode = "name"
	TaskSortStatus   TaskSortMode = "status"
	TaskSortAssignee TaskSortMode = "assignee"
	TaskSortDueDate  TaskSortMode = "due"
)

type TaskGroupMode string

const (
	TaskGroupNone     TaskGroupMode = "none"
	TaskGroupStatus   TaskGroupMode = "status"
	TaskGroupAssignee TaskGroupMode = "assignee"
)

type TaskSubtaskMode string

const (
	TaskSubtaskFlat    TaskSubtaskMode = "flat"
	TaskSubtaskGrouped TaskSubtaskMode = "grouped"
)

type SyncQueuer interface {
	QueueTaskUpdate(taskID string, update provider.TaskUpdate) error
	QueueAddComment(taskID string, text string) error
	Cycle(ctx context.Context) error
	SyncList(ctx context.Context, listID string) error
	SetActiveListID(listID string)
	SetActiveProvider(providerID string) bool
	ActiveProviderID() string
	Providers() []syncengine.ProviderMeta
	ProviderDisplayName() string
	SetProviderToken(providerID string, token string) bool
	RevalidateTask(ctx context.Context, taskID string) error
	SyncStatus() string
}

type RootModel struct {
	width               int
	height              int
	keymap              Keymap
	activePane          int
	repo                *cache.Repository
	sync                SyncQueuer
	provider            string
	activeProviderID    string
	availableProviders  []syncengine.ProviderMeta
	clickUpClientID     string
	oauthBackendURL     string
	clickUpConnected    bool
	providerSetupPrompt bool
	providerSetupIndex  int
	statusLine          string
	lists               []cache.ListEntity
	sidebar             components.SidebarModel
	taskTable           components.TaskTableModel
	detailPanel         components.DetailModel
	statuses            []string
	taskSortMode        TaskSortMode
	taskGroupMode       TaskGroupMode
	taskSubtasks        TaskSubtaskMode

	statusFilter string
	searchMode   bool
	searchInput  string
	searchQuery  string
	searchBackup string

	listSearchQuery       string
	listSortMode          cache.ListSortMode
	favoritesOnly         bool
	stateHydrated         bool
	selectedListID        string
	restorePolicy         RestorePolicy
	restorePrompt         bool
	restorePromptSelected int
	restoreSnapshot       uiSessionSnapshot
	hasRestoreState       bool
	restorePromptSeen     bool
	vimMode               bool

	controlOpen     bool
	controlMode     ControlMode
	controlInput    string
	controlSelected int
	controlResults  []controlResult
	recentCommands  []string
	commandUsage    map[string]commandUsageStat
	loadedTasks     []cache.TaskEntity

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
	lists                  []cache.ListEntity
	tasks                  []cache.TaskEntity
	statuses               []string
	err                    error
	selectedListID         string
	restored               bool
	restoredListSearch     string
	restoredSortMode       cache.ListSortMode
	restoredFavOnly        bool
	restoredTaskSort       TaskSortMode
	restoredTaskGroup      TaskGroupMode
	restoredSubtasks       TaskSubtaskMode
	restoredStatus         string
	restoredTaskSearch     string
	restoredVimMode        bool
	restoredActiveProvider string
	restorePolicy          RestorePolicy
	hasRestoreSnapshot     bool
	restoreSnapshot        uiSessionSnapshot
	recentCommands         []string
	commandUsage           map[string]commandUsageStat
}

type editResultMsg struct{ err error }
type syncResultMsg struct{ err error }
type commentResultMsg struct{ err error }
type oauthResultMsg struct {
	providerID string
	token      string
	err        error
}
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

func NewRootModel(repo *cache.Repository, sync SyncQueuer, providerName string, statusLine string, clickUpClientID string, oauthBackendURL string, clickUpConnected bool, needsProviderSetup bool) RootModel {
	if providerName == "" {
		providerName = "none"
	}
	var syncer SyncQueuer
	if sync != nil {
		syncer = sync
		providerName = sync.ProviderDisplayName()
	}
	activeProviderID := ""
	availableProviders := []syncengine.ProviderMeta{}
	if syncer != nil {
		activeProviderID = syncer.ActiveProviderID()
		availableProviders = syncer.Providers()
	}
	return RootModel{
		repo:                repo,
		sync:                syncer,
		provider:            providerName,
		activeProviderID:    activeProviderID,
		availableProviders:  availableProviders,
		clickUpClientID:     clickUpClientID,
		oauthBackendURL:     oauthBackendURL,
		clickUpConnected:    clickUpConnected,
		providerSetupPrompt: needsProviderSetup,
		providerSetupIndex:  0,
		keymap:              DefaultKeymap(),
		statusLine:          statusLine,
		sidebar:             components.NewSidebar(),
		taskTable:           components.NewTaskTable(),
		detailPanel:         components.NewDetail(),
		listSortMode:        cache.ListSortNameAsc,
		taskSortMode:        TaskSortPriority,
		taskGroupMode:       TaskGroupNone,
		taskSubtasks:        TaskSubtaskFlat,
		favoritesOnly:       false,
		restorePolicy:       RestorePolicyAsk,
		controlMode:         ControlModeCommand,
		commandUsage:        make(map[string]commandUsageStat),
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
		if m.providerSetupPrompt {
			switch msg.Type {
			case tea.KeyUp:
				if m.providerSetupIndex > 0 {
					m.providerSetupIndex--
				}
				return m, nil
			case tea.KeyDown:
				if m.providerSetupIndex+1 < len(m.availableProviders) {
					m.providerSetupIndex++
				}
				return m, nil
			case tea.KeyEnter:
				if len(m.availableProviders) == 0 {
					return m, nil
				}
				selected := m.availableProviders[m.providerSetupIndex]
				m.providerSetupPrompt = false
				if m.switchProvider(selected.ID) {
					m.statusLine = "Default provider set to " + m.provider
					if m.providerNeedsConnectionOverlay() {
						return m, nil
					}
					return m, m.loadDataCmd()
				}
				m.providerSetupPrompt = true
				return m, nil
			}
			return m, nil
		}

		if m.providerNeedsConnectionOverlay() {
			switch msg.Type {
			case tea.KeyEnter:
				m.statusLine = "Starting ClickUp OAuth..."
				return m, m.startClickUpOAuthCmd()
			}
			switch msg.String() {
			case "q", "ctrl+c":
				m.persistSessionSnapshot()
				return m, tea.Quit
			}
			return m, nil
		}

		if m.restorePrompt {
			switch msg.Type {
			case tea.KeyUp:
				if m.restorePromptSelected > 0 {
					m.restorePromptSelected--
				}
				return m, nil
			case tea.KeyDown:
				if m.restorePromptSelected+1 < len(restorePromptOptions()) {
					m.restorePromptSelected++
				}
				return m, nil
			case tea.KeyEnter:
				options := restorePromptOptions()
				if len(options) == 0 {
					return m, nil
				}
				if m.restorePromptSelected < 0 || m.restorePromptSelected >= len(options) {
					m.restorePromptSelected = 0
				}
				return m, m.applyRestorePromptAction(options[m.restorePromptSelected].Action)
			}

			switch msg.String() {
			case "r", "R":
				return m, m.applyRestorePromptAction("restore")
			case "n", "N", "esc":
				return m, m.applyRestorePromptAction("fresh")
			case "a", "A":
				return m, m.applyRestorePromptAction("always")
			case "v", "V":
				return m, m.applyRestorePromptAction("never")
			}
			return m, nil
		}

		if m.controlOpen {
			return m, m.updateControlCenter(msg)
		}

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
				m.saveTaskPrefs()
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
				m.saveTaskPrefs()
				m.persistSessionSnapshot()
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
			m.saveTaskPrefs()
			return m, m.loadDataCmd()
		}

		prevListID := m.selectedListID
		if msg.Type == tea.KeyEnter && m.activePane == 1 {
			selectedRow, ok := m.taskTable.Selected()
			if !ok {
				return m, nil
			}
			if m.taskTable.ToggleSelectedCollapse() {
				if selectedRow.Type == components.TaskTableRowTask && selectedRow.ID != "" {
					return m, m.selectCursorTaskForDisplayCmd()
				}
				return m, nil
			}
			return m, m.selectCursorTaskForDisplayCmd()
		}

		switch msg.String() {
		case "q", "ctrl+c":
			m.persistSessionSnapshot()
			return m, tea.Quit
		case "ctrl+p", "ctrl+k":
			m.openControlCenter(ControlModeCommand)
			return m, nil
		case ":":
			m.openControlCenter(ControlModeCommand)
			return m, nil
		case "?":
			m.openControlCenter(ControlModeHelp)
			return m, nil
		case "home":
			m.handleMoveToTop()
		case "end":
			m.handleMoveToBottom()
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
		case "ctrl+d":
			if m.vimMode {
				m.handleMove(10)
			}
		case "ctrl+u":
			if m.vimMode {
				m.handleMove(-10)
			}
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
			m.saveTaskPrefs()
			return m, m.loadDataCmd()
		case strings.ToUpper(m.keymap.Filter):
			m.cycleStatusFilter(-1)
			m.saveTaskPrefs()
			return m, m.loadDataCmd()
		case m.keymap.Search:
			m.searchMode = true
			m.searchInput = m.searchQuery
			m.searchBackup = m.searchQuery
			return m, nil
		case m.keymap.TaskTitle:
			if m.activePane == 1 {
				row, ok := m.taskTable.Selected()
				if !ok {
					m.statusLine = "No task selected"
					return m, nil
				}
				m.statusLine = "Task title: " + row.Title
				return m, nil
			}
		case m.keymap.SortLists:
			if m.listSortMode == cache.ListSortMostRecentlyOpen {
				m.listSortMode = cache.ListSortNameAsc
			} else {
				m.listSortMode = cache.ListSortMostRecentlyOpen
			}
			m.saveListPrefs()
			m.statusLine = "List sort: " + string(m.listSortMode)
			return m, m.loadDataCmd()
		case m.keymap.SortTasks:
			m.cycleTaskSort(1)
			m.saveTaskPrefs()
			return m, m.loadDataCmd()
		case m.keymap.GroupTasks:
			m.cycleTaskGroup(1)
			m.saveTaskPrefs()
			return m, m.loadDataCmd()
		case m.keymap.Subtasks:
			m.cycleTaskSubtasks(1)
			m.saveTaskPrefs()
			return m, m.loadDataCmd()
		case m.keymap.CollapseAll:
			if m.activePane == 1 {
				changed, collapsed := m.taskTable.CollapseAll()
				if changed {
					if collapsed {
						m.statusLine = "Collapsed all groups"
					} else {
						m.statusLine = "Expanded all groups"
					}
				} else {
					m.statusLine = "No groups available"
				}
				return m, nil
			}
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
			if msg.restoredTaskSort != "" {
				m.taskSortMode = msg.restoredTaskSort
			}
			if msg.restoredTaskGroup != "" {
				m.taskGroupMode = msg.restoredTaskGroup
			}
			if msg.restoredSubtasks != "" {
				m.taskSubtasks = msg.restoredSubtasks
			}
			m.statusFilter = msg.restoredStatus
			m.searchQuery = msg.restoredTaskSearch
			m.vimMode = msg.restoredVimMode
			if msg.restoredActiveProvider != "" && m.sync != nil {
				if m.sync.SetActiveProvider(msg.restoredActiveProvider) {
					m.activeProviderID = msg.restoredActiveProvider
					m.provider = m.sync.ProviderDisplayName()
				}
			}
			if msg.restorePolicy != "" {
				m.restorePolicy = msg.restorePolicy
			}
			m.restoreSnapshot = msg.restoreSnapshot
			m.hasRestoreState = msg.hasRestoreSnapshot
			m.recentCommands = append([]string(nil), msg.recentCommands...)
			if msg.commandUsage != nil {
				m.commandUsage = msg.commandUsage
			}
			if m.restorePolicy == RestorePolicyAlways && m.hasRestoreState {
				m.applySessionSnapshot(m.restoreSnapshot)
			} else if m.restorePolicy == RestorePolicyAsk && m.hasRestoreState {
				m.restorePrompt = true
				m.restorePromptSelected = 0
			}
		}

		m.lists = msg.lists
		m.selectedListID = msg.selectedListID
		if m.selectedListID != "" && m.selectedListID != previousListID {
			m.markListOpened(m.selectedListID)
			m.persistLastOpenedList(m.selectedListID)
		}
		if m.sync != nil {
			m.sync.SetActiveListID(m.selectedListID)
			m.availableProviders = m.sync.Providers()
			m.activeProviderID = m.sync.ActiveProviderID()
			m.provider = m.sync.ProviderDisplayName()
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

		m.taskTable.SetRows(mapTasksToRows(msg.tasks, m.taskGroupMode, m.taskSubtasks))
		m.loadedTasks = append([]cache.TaskEntity(nil), msg.tasks...)
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
		m.statusLine = "Task title updated locally and queued for provider push"
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
		m.statusLine = "Comment saved locally and queued for provider push"
		return m, m.loadDataCmd()

	case oauthResultMsg:
		if msg.err != nil {
			m.statusLine = "OAuth failed: " + msg.err.Error()
			return m, nil
		}
		if m.repo != nil {
			_ = m.repo.SaveAppState(appStateClickUpToken, msg.token)
		}
		if m.sync != nil {
			if !m.sync.SetProviderToken(msg.providerID, msg.token) {
				m.statusLine = "OAuth completed but provider token update failed"
				return m, nil
			}
			m.availableProviders = m.sync.Providers()
		}
		if msg.providerID == "clickup" {
			m.clickUpConnected = true
		}
		m.statusLine = "ClickUp connected successfully"
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

func (m *RootModel) handleMoveToTop() {
	switch m.activePane {
	case 0:
		m.sidebar.MoveToTop()
	case 1:
		m.taskTable.MoveToTop()
	case 2:
		m.detailPanel.MoveToTop()
	}
}

func (m *RootModel) handleMoveToBottom() {
	switch m.activePane {
	case 0:
		m.sidebar.MoveToBottom()
	case 1:
		m.taskTable.MoveToBottom()
	case 2:
		m.detailPanel.MoveToBottom()
	}
}

func (m RootModel) View() string {
	totalWidth, sidebarInnerWidth, rightInnerWidth, sidebarInnerHeight, tableInnerHeight, detailInnerHeight := m.layout()
	header := HeaderStyle.Width(totalWidth).Render(truncateLine("lazy-click", totalWidth))
	const verticalPaneGap = 0
	const horizontalPaneGap = 1

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
		m.sidebar.Render(m.activePane == 0, sidebarInnerWidth-4, sidebarInnerHeight),
	)
	table := tableStyle.Width(rightInnerWidth).Height(tableInnerHeight).Render(
		m.taskTable.Render(m.activePane == 1, rightInnerWidth-2, tableInnerHeight),
	)
	detail := detailStyle.Width(rightInnerWidth).Height(detailInnerHeight).Render(
		m.detailPanel.Render(m.activePane == 2, rightInnerWidth-2, detailInnerHeight),
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
	status := fmt.Sprintf(
		"Provider: %s (%s) | List sort: %s | Task sort: %s | Task group: %s | Subtasks: %s | Favorites-only: %t | Task status: %s | Task search: %s | Vim mode: %t",
		m.provider,
		m.activeProviderID,
		m.listSortMode,
		m.taskSortMode,
		m.taskGroupMode,
		m.taskSubtasks,
		m.favoritesOnly,
		statusFilter,
		taskSearch,
		m.vimMode,
	)

	help := "Keys: hjkl/arrows move, home/end jump, / task search, ? list search, t show task title, * favorite list, o sort lists, O sort tasks, g group tasks, G subtasks mode, X collapse all groups, v favorites-only, i edit, R refresh task, c comment, f/F status, r refresh, s sync, q quit"
	if m.commentMode {
		help = "Comment mode: type text, Enter submit, Esc cancel"
	} else if m.searchMode {
		help = fmt.Sprintf("Task search mode: %s (type to filter, Enter apply, Esc cancel)", m.searchInput)
	} else if m.controlOpen {
		help = "Control center: Enter run/select, Esc close, @ lists, # tasks, ? help, > commands, provider switch commands"
	} else if m.restorePrompt {
		help = "Session restore: r restore, n fresh, a always, v never"
	} else {
		help = "Keys: ctrl+p/ctrl+k/: control center, hjkl/arrows move cursor, Enter open row, / task search, * favorite list, o sort lists, O sort tasks, g group tasks, G subtasks mode, X collapse groups, v favorites-only, i edit, R refresh task, c comment, f/F status, r refresh, s sync, q quit"
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

	if m.restorePrompt {
		overlay := m.renderRestorePrompt(totalWidth)
		y := centeredOverlayY(screen, overlay, -2)
		screen = overlayCentered(screen, overlay, totalWidth, y)
	}
	if m.controlOpen {
		overlay := m.renderControlCenter(totalWidth)
		y := centeredOverlayY(screen, overlay, 0)
		screen = overlayCentered(screen, overlay, totalWidth, y)
	}
	if m.providerSetupPrompt {
		overlay := m.renderProviderSetupOverlay(totalWidth)
		y := centeredOverlayY(screen, overlay, -1)
		screen = overlayCentered(screen, overlay, totalWidth, y)
	}
	if m.providerNeedsConnectionOverlay() {
		overlay := m.renderProviderConnectOverlay(totalWidth)
		y := centeredOverlayY(screen, overlay, -1)
		screen = overlayCentered(screen, overlay, totalWidth, y)
	}

	return lipgloss.NewStyle().
		Width(totalWidth).
		MaxWidth(totalWidth).
		MaxHeight(max(totalHeightFromModel(m.height)-1, 8)).
		Render(screen)
}

func (m RootModel) layout() (int, int, int, int, int, int) {
	hFrame := PanelStyle.GetHorizontalFrameSize()
	vFrame := PanelStyle.GetVerticalFrameSize()
	const verticalPaneGap = 0
	const horizontalPaneGap = 1

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
	sidebarInnerWidth := innerWidthBudget / 5
	minSidebar := 8
	maxSidebar := max(innerWidthBudget-8, minSidebar)
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

	sidebarInnerHeight := max(bodyOuterHeight-vFrame, 1)

	rightInnerHeightBudget := max(bodyOuterHeight-(2*vFrame)-verticalPaneGap, 2)
	tableInnerHeight := max((rightInnerHeightBudget*2)/3, 1)
	detailInnerHeight := rightInnerHeightBudget - tableInnerHeight

	if detailInnerHeight < 1 {
		detailInnerHeight = 1
		tableInnerHeight = max(rightInnerHeightBudget-1, 1)
	}

	return totalWidth, sidebarInnerWidth, rightInnerWidth, sidebarInnerHeight, tableInnerHeight, detailInnerHeight
}

func (m RootModel) loadDataCmd() tea.Cmd {
	if m.repo == nil {
		return func() tea.Msg { return dataLoadedMsg{err: fmt.Errorf("cache repository unavailable")} }
	}

	currentListID := m.selectedListID
	currentSearch := ""
	currentSort := m.listSortMode
	currentFavOnly := m.favoritesOnly
	hydrated := m.stateHydrated
	statusFilter := m.statusFilter
	taskSearch := m.searchQuery
	taskSortMode := m.taskSortMode
	taskGroupMode := m.taskGroupMode
	taskSubtaskMode := m.taskSubtasks

	return func() tea.Msg {
		msg := dataLoadedMsg{}
		if !hydrated {
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

			taskSortMode = TaskSortPriority
			if sortRaw, err := m.repo.GetAppState(appStateTaskSortMode); err != nil {
				return dataLoadedMsg{err: err}
			} else if parsed := TaskSortMode(sortRaw); parsed != "" {
				taskSortMode = parsed
			}

			taskGroupMode = TaskGroupNone
			if groupRaw, err := m.repo.GetAppState(appStateTaskGroupMode); err != nil {
				return dataLoadedMsg{err: err}
			} else if parsed := TaskGroupMode(groupRaw); parsed != "" {
				taskGroupMode = parsed
			}

			taskSubtaskMode = TaskSubtaskFlat
			if subRaw, err := m.repo.GetAppState(appStateTaskSubtasksMode); err != nil {
				return dataLoadedMsg{err: err}
			} else if parsed := TaskSubtaskMode(subRaw); parsed != "" {
				taskSubtaskMode = parsed
			}

			if statusRaw, err := m.repo.GetAppState(appStateStatusFilter); err != nil {
				return dataLoadedMsg{err: err}
			} else {
				statusFilter = statusRaw
			}

			if taskSearchRaw, err := m.repo.GetAppState(appStateTaskSearchQuery); err != nil {
				return dataLoadedMsg{err: err}
			} else {
				taskSearch = taskSearchRaw
			}

			vimMode := false
			if vimRaw, err := m.repo.GetAppState(appStateVimModeEnabled); err != nil {
				return dataLoadedMsg{err: err}
			} else if vimRaw != "" {
				if parsed, parseErr := strconv.ParseBool(vimRaw); parseErr == nil {
					vimMode = parsed
				}
			}

			restorePolicy := RestorePolicyAsk
			if policyRaw, err := m.repo.GetAppState(appStateRestorePolicy); err != nil {
				return dataLoadedMsg{err: err}
			} else {
				restorePolicy = normalizeRestorePolicy(policyRaw)
			}

			hasSnapshot := false
			snapshot := uiSessionSnapshot{}
			if snapshotRaw, err := m.repo.GetAppState(appStateRestoreSession); err != nil {
				return dataLoadedMsg{err: err}
			} else if strings.TrimSpace(snapshotRaw) != "" {
				if unmarshalErr := json.Unmarshal([]byte(snapshotRaw), &snapshot); unmarshalErr == nil {
					hasSnapshot = true
				}
			}

			recentCommands := []string{}
			if recentRaw, err := m.repo.GetAppState(appStateRecentCommands); err != nil {
				return dataLoadedMsg{err: err}
			} else if strings.TrimSpace(recentRaw) != "" {
				_ = json.Unmarshal([]byte(recentRaw), &recentCommands)
			}

			usage := make(map[string]commandUsageStat)
			if usageRaw, err := m.repo.GetAppState(appStateCommandUsage); err != nil {
				return dataLoadedMsg{err: err}
			} else if strings.TrimSpace(usageRaw) != "" {
				_ = json.Unmarshal([]byte(usageRaw), &usage)
			}

			activeProvider := ""
			if providerRaw, err := m.repo.GetAppState(appStateActiveProviderID); err != nil {
				return dataLoadedMsg{err: err}
			} else {
				activeProvider = strings.TrimSpace(providerRaw)
			}

			currentSort = sortMode
			currentFavOnly = favOnly
			msg.restored = true
			msg.restoredListSearch = currentSearch
			msg.restoredSortMode = currentSort
			msg.restoredFavOnly = currentFavOnly
			msg.restoredTaskSort = taskSortMode
			msg.restoredTaskGroup = taskGroupMode
			msg.restoredSubtasks = taskSubtaskMode
			msg.restoredStatus = statusFilter
			msg.restoredTaskSearch = taskSearch
			msg.restoredVimMode = vimMode
			msg.restorePolicy = restorePolicy
			msg.hasRestoreSnapshot = hasSnapshot
			msg.restoreSnapshot = snapshot
			msg.recentCommands = recentCommands
			msg.commandUsage = usage
			msg.restoredActiveProvider = activeProvider
		}

		activeProviderID := m.activeProviderID
		if msg.restoredActiveProvider != "" {
			activeProviderID = msg.restoredActiveProvider
		}
		if m.sync != nil && activeProviderID != "" {
			_ = m.sync.SetActiveProvider(activeProviderID)
		}

		lists, err := m.repo.GetListsByQuery(cache.ListQuery{
			Provider:      activeProviderID,
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
			Provider:      activeProviderID,
			ListID:        selectedListID,
			Statuses:      selectedStatusFilter(statusFilter),
			IncludeClosed: true,
		})
		if err != nil {
			return dataLoadedMsg{err: err}
		}
		tasks = fuzzyFindTasks(tasks, taskSearch)
		tasks = organizeTasks(tasks, taskSortMode, taskGroupMode, taskSubtaskMode)

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

func (m RootModel) activeProviderMeta() (syncengine.ProviderMeta, bool) {
	for _, p := range m.availableProviders {
		if p.ID == m.activeProviderID {
			return p, true
		}
	}
	return syncengine.ProviderMeta{}, false
}

func (m RootModel) providerNeedsConnectionOverlay() bool {
	meta, ok := m.activeProviderMeta()
	if !ok {
		return false
	}
	if meta.Kind != "clickup" {
		return false
	}
	return !m.clickUpConnected
}

func (m RootModel) renderProviderSetupOverlay(width int) string {
	panelWidth := min(max(width-6, 46), 96)
	lines := []string{
		ControlCenterTitleStyle.Render(truncateLine("Choose your default provider", panelWidth-2)),
		"",
	}
	if len(m.availableProviders) == 0 {
		lines = append(lines, "  No providers available")
	} else {
		for i, p := range m.availableProviders {
			prefix := "  "
			style := lipgloss.NewStyle()
			if i == m.providerSetupIndex {
				prefix = "> "
				style = ControlCenterSelectStyle
			}
			label := p.DisplayName
			if strings.TrimSpace(label) == "" {
				label = p.ID
			}
			line := fmt.Sprintf("%s [%s]", label, p.Kind)
			lines = append(lines, style.Render(truncateLine(prefix+line, panelWidth-2)))
		}
	}
	lines = append(lines, "", HelpStyle.Render("Use arrows and press Enter to continue"))
	return RestorePromptStyle.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func (m RootModel) renderProviderConnectOverlay(width int) string {
	panelWidth := min(max(width-6, 52), 104)
	lines := []string{
		ControlCenterTitleStyle.Render(truncateLine("Provider requires connection", panelWidth-2)),
		"",
		truncateLine("The active provider is not connected yet.", panelWidth-2),
		truncateLine("Press Enter to connect with OAuth.", panelWidth-2),
		"",
		ControlCenterSelectStyle.Render(truncateLine("> Connect ClickUp OAuth (Enter)", panelWidth-2)),
	}
	return RestorePromptStyle.Width(panelWidth).Render(strings.Join(lines, "\n"))
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

func (m *RootModel) cycleTaskSort(step int) {
	options := []TaskSortMode{TaskSortPriority, TaskSortName, TaskSortStatus, TaskSortAssignee, TaskSortDueDate}
	current := 0
	for i, mode := range options {
		if mode == m.taskSortMode {
			current = i
			break
		}
	}
	next := (current + step) % len(options)
	if next < 0 {
		next += len(options)
	}
	m.taskSortMode = options[next]
	m.statusLine = "Task sort: " + string(m.taskSortMode)
}

func (m *RootModel) cycleTaskGroup(step int) {
	options := []TaskGroupMode{TaskGroupNone, TaskGroupStatus, TaskGroupAssignee}
	current := 0
	for i, mode := range options {
		if mode == m.taskGroupMode {
			current = i
			break
		}
	}
	next := (current + step) % len(options)
	if next < 0 {
		next += len(options)
	}
	m.taskGroupMode = options[next]
	m.statusLine = "Task group: " + string(m.taskGroupMode)
}

func (m *RootModel) cycleTaskSubtasks(step int) {
	options := []TaskSubtaskMode{TaskSubtaskFlat, TaskSubtaskGrouped}
	current := 0
	for i, mode := range options {
		if mode == m.taskSubtasks {
			current = i
			break
		}
	}
	next := (current + step) % len(options)
	if next < 0 {
		next += len(options)
	}
	m.taskSubtasks = options[next]
	m.statusLine = "Subtasks: " + string(m.taskSubtasks)
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
	_ = m.repo.SaveAppState(appStateFavoritesOnly, strconv.FormatBool(m.favoritesOnly))
}

func (m *RootModel) switchProvider(providerID string) bool {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" || m.sync == nil {
		return false
	}
	if !m.sync.SetActiveProvider(providerID) {
		m.statusLine = "Provider switch failed: " + providerID
		return false
	}
	m.activeProviderID = m.sync.ActiveProviderID()
	m.provider = m.sync.ProviderDisplayName()
	m.availableProviders = m.sync.Providers()
	m.selectedListID = ""
	m.displayedTaskID = ""
	m.selectedTaskID = ""
	m.taskTable.SetDisplayedTaskID("")
	m.detailLoading = false
	m.detailLoadingMsg = ""
	if m.repo != nil {
		_ = m.repo.SaveAppState(appStateActiveProviderID, m.activeProviderID)
	}
	m.statusLine = "Provider switched to " + m.provider
	m.persistSessionSnapshot()
	return true
}

func (m *RootModel) switchToNextProviderCmd() tea.Cmd {
	if len(m.availableProviders) == 0 {
		m.statusLine = "No providers configured"
		return nil
	}
	idx := -1
	for i, p := range m.availableProviders {
		if p.ID == m.activeProviderID {
			idx = i
			break
		}
	}
	next := 0
	if idx >= 0 {
		next = (idx + 1) % len(m.availableProviders)
	}
	if m.switchProvider(m.availableProviders[next].ID) {
		return m.loadDataCmd()
	}
	return nil
}

func (m RootModel) startClickUpOAuthCmd() tea.Cmd {
	if strings.TrimSpace(m.clickUpClientID) == "" {
		return func() tea.Msg {
			return oauthResultMsg{providerID: "clickup", err: fmt.Errorf("CLICKUP_CLIENT_ID is required")}
		}
	}
	if strings.TrimSpace(m.oauthBackendURL) == "" {
		return func() tea.Msg {
			return oauthResultMsg{providerID: "clickup", err: fmt.Errorf("LAZY_CLICK_OAUTH_BACKEND_URL is required")}
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		token, err := clickup.OAuthTokenViaBackend(ctx, clickup.BackendOAuthConfig{
			BackendURL: m.oauthBackendURL,
			ClientID:   m.clickUpClientID,
		})
		return oauthResultMsg{providerID: "clickup", token: token, err: err}
	}
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

func overlayCentered(base string, overlay string, width int, y int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")
	if y < 0 {
		y = 0
	}

	for i, overlayLine := range overlayLines {
		baseY := y + i
		if baseY < 0 || baseY >= len(baseLines) {
			continue
		}
		line := overlayLine
		lineWidth := xansi.StringWidth(line)
		if lineWidth <= 0 {
			continue
		}
		if lineWidth > width {
			line = xansi.Cut(line, 0, width)
			lineWidth = xansi.StringWidth(line)
		}

		baseLine := lipgloss.Place(width, 1, lipgloss.Left, lipgloss.Top, baseLines[baseY])
		x := max((width-lineWidth)/2, 0)
		left := xansi.Cut(baseLine, 0, x)
		right := xansi.Cut(baseLine, x+lineWidth, width)
		baseLines[baseY] = left + line + right
	}

	return strings.Join(baseLines, "\n")
}

func centeredOverlayY(base string, overlay string, offset int) int {
	baseH := len(strings.Split(base, "\n"))
	overlayH := len(strings.Split(overlay, "\n"))
	if overlayH <= 0 {
		return 0
	}
	y := (baseH-overlayH)/2 + offset
	if y < 0 {
		return 0
	}
	maxY := max(baseH-overlayH, 0)
	if y > maxY {
		return maxY
	}
	return y
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

func mapTasksToRows(tasks []cache.TaskEntity, groupMode TaskGroupMode, subtaskMode TaskSubtaskMode) []components.TaskTableRow {
	hasChildren := make(map[string]bool, len(tasks))
	presentTaskIDs := make(map[string]struct{}, len(tasks))
	tasksByID := make(map[string]cache.TaskEntity, len(tasks))
	for _, task := range tasks {
		if task.ID != "" {
			presentTaskIDs[task.ID] = struct{}{}
			tasksByID[task.ID] = task
		}
		if task.IsSubtask && task.ParentTaskID != "" && task.ParentTaskID != task.ID {
			hasChildren[task.ParentTaskID] = true
		}
	}

	rows := make([]components.TaskTableRow, 0, len(tasks)+(len(tasks)/4)+4)
	currentGroupKey := ""
	currentGroupCollapseKey := ""

	for _, task := range tasks {
		groupAnchor := taskGroupingAnchor(task, tasksByID, subtaskMode)
		groupKey := taskGroupKey(groupAnchor, groupMode)
		if groupMode != TaskGroupNone && groupKey != currentGroupKey {
			currentGroupKey = groupKey
			currentGroupCollapseKey = "group:" + string(groupMode) + ":" + groupKey
			rows = append(rows, components.TaskTableRow{
				Type:        components.TaskTableRowGroup,
				Title:       taskGroupLabel(groupAnchor, groupMode),
				Status:      "",
				Priority:    "",
				Estimate:    "",
				DueDate:     "",
				Assignees:   "",
				CollapseKey: currentGroupCollapseKey,
			})
		}

		priority := "-"
		if task.PriorityLabel != "" {
			priority = task.PriorityLabel
		}
		due := "-"
		if task.DueAtUnixMS != nil {
			due = time.UnixMilli(*task.DueAtUnixMS).Format("2006-01-02")
		}

		indent := 0
		hiddenBy := make([]string, 0, 2)
		if groupMode != TaskGroupNone {
			indent += 2
			hiddenBy = append(hiddenBy, currentGroupCollapseKey)
		}

		title := task.Title
		depth, ancestorIDs := taskNesting(task, presentTaskIDs, tasksByID)
		if subtaskMode == TaskSubtaskGrouped && depth > 0 {
			indent += 2 * depth
		}

		collapseKey := ""
		if subtaskMode == TaskSubtaskGrouped {
			if hasChildren[task.ID] {
				collapseKey = "subtask:" + task.ID
			}
			for _, ancestorID := range ancestorIDs {
				hiddenBy = append(hiddenBy, "subtask:"+ancestorID)
			}
		}

		rows = append(rows, components.TaskTableRow{
			Type:        components.TaskTableRowTask,
			ID:          task.ID,
			ListID:      task.ListID,
			Title:       title,
			Status:      task.Status,
			StatusColor: task.StatusColor,
			Priority:    priority,
			Estimate:    formatEstimate(task.EstimateMS),
			DueDate:     due,
			Assignees:   formatAssignees(task.AssigneesJSON),
			Indent:      indent,
			CollapseKey: collapseKey,
			HiddenBy:    hiddenBy,
		})
	}
	return rows
}

func formatEstimate(estimateMS *int64) string {
	if estimateMS == nil || *estimateMS <= 0 {
		return "-"
	}
	seconds := *estimateMS / 1000
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	if hours > 0 && minutes > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return "<1m"
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

func organizeTasks(tasks []cache.TaskEntity, sortMode TaskSortMode, groupMode TaskGroupMode, subtaskMode TaskSubtaskMode) []cache.TaskEntity {
	if len(tasks) <= 1 {
		return tasks
	}

	ordered := append([]cache.TaskEntity(nil), tasks...)
	tasksByID := make(map[string]cache.TaskEntity, len(ordered))
	for _, task := range ordered {
		if task.ID != "" {
			tasksByID[task.ID] = task
		}
	}
	sort.SliceStable(ordered, func(i int, j int) bool {
		groupI := taskGroupKey(taskGroupingAnchor(ordered[i], tasksByID, subtaskMode), groupMode)
		groupJ := taskGroupKey(taskGroupingAnchor(ordered[j], tasksByID, subtaskMode), groupMode)
		if groupI != groupJ {
			return groupI < groupJ
		}
		return taskLess(ordered[i], ordered[j], sortMode)
	})

	if subtaskMode == TaskSubtaskGrouped {
		return placeSubtasksAfterParents(ordered)
	}

	return ordered
}

func taskGroupingAnchor(task cache.TaskEntity, tasksByID map[string]cache.TaskEntity, subtaskMode TaskSubtaskMode) cache.TaskEntity {
	if subtaskMode != TaskSubtaskGrouped {
		return task
	}
	if !task.IsSubtask || task.ParentTaskID == "" || task.ParentTaskID == task.ID {
		return task
	}
	parent, ok := tasksByID[task.ParentTaskID]
	if !ok {
		return task
	}
	return parent
}

func taskNesting(task cache.TaskEntity, presentTaskIDs map[string]struct{}, tasksByID map[string]cache.TaskEntity) (int, []string) {
	if !task.IsSubtask || task.ParentTaskID == "" || task.ParentTaskID == task.ID {
		return 0, nil
	}

	depth := 0
	ancestors := make([]string, 0, 4)
	currentParentID := task.ParentTaskID
	visited := make(map[string]struct{}, 4)

	for currentParentID != "" {
		if _, seen := visited[currentParentID]; seen {
			break
		}
		visited[currentParentID] = struct{}{}

		if _, ok := presentTaskIDs[currentParentID]; !ok {
			break
		}

		ancestors = append(ancestors, currentParentID)
		depth++

		parent, ok := tasksByID[currentParentID]
		if !ok || !parent.IsSubtask || parent.ParentTaskID == "" || parent.ParentTaskID == parent.ID {
			break
		}
		currentParentID = parent.ParentTaskID
	}

	return depth, ancestors
}

func taskGroupLabel(task cache.TaskEntity, mode TaskGroupMode) string {
	switch mode {
	case TaskGroupStatus:
		label := strings.TrimSpace(task.Status)
		if label == "" {
			return "Status: (none)"
		}
		return "Status: " + strings.ToUpper(label)
	case TaskGroupAssignee:
		assignee := strings.TrimSpace(primaryAssignee(task.AssigneesJSON))
		if assignee == "" {
			return "Assignee: unassigned"
		}
		return "Assignee: " + assignee
	default:
		return "Group"
	}
}

func taskGroupKey(task cache.TaskEntity, mode TaskGroupMode) string {
	switch mode {
	case TaskGroupStatus:
		status := strings.TrimSpace(strings.ToLower(task.Status))
		if status == "" {
			return "~"
		}
		return status
	case TaskGroupAssignee:
		assignee := strings.TrimSpace(strings.ToLower(primaryAssignee(task.AssigneesJSON)))
		if assignee == "" {
			return "~"
		}
		return assignee
	default:
		return ""
	}
}

func taskLess(a cache.TaskEntity, b cache.TaskEntity, mode TaskSortMode) bool {
	switch mode {
	case TaskSortName:
		return compareStrings(a.Title, b.Title)
	case TaskSortStatus:
		if compareStrings(a.Status, b.Status) {
			return true
		}
		if compareStrings(b.Status, a.Status) {
			return false
		}
		return compareStrings(a.Title, b.Title)
	case TaskSortAssignee:
		assigneeA := primaryAssignee(a.AssigneesJSON)
		assigneeB := primaryAssignee(b.AssigneesJSON)
		if compareStrings(assigneeA, assigneeB) {
			return true
		}
		if compareStrings(assigneeB, assigneeA) {
			return false
		}
		return compareStrings(a.Title, b.Title)
	case TaskSortDueDate:
		return dueDateLess(a.DueAtUnixMS, b.DueAtUnixMS, a.Title, b.Title)
	default:
		return priorityLess(a, b)
	}
}

func compareStrings(a string, b string) bool {
	aNorm := strings.ToLower(strings.TrimSpace(a))
	bNorm := strings.ToLower(strings.TrimSpace(b))
	if aNorm == "" && bNorm != "" {
		return false
	}
	if bNorm == "" && aNorm != "" {
		return true
	}
	return aNorm < bNorm
}

func dueDateLess(a *int64, b *int64, aTitle string, bTitle string) bool {
	if a == nil && b != nil {
		return false
	}
	if b == nil && a != nil {
		return true
	}
	if a != nil && b != nil {
		if *a != *b {
			return *a < *b
		}
	}
	return compareStrings(aTitle, bTitle)
}

func priorityLess(a cache.TaskEntity, b cache.TaskEntity) bool {
	rankA := prioritySortRank(a)
	rankB := prioritySortRank(b)
	if rankA != rankB {
		return rankA < rankB
	}
	return compareStrings(a.Title, b.Title)
}

func prioritySortRank(task cache.TaskEntity) int {
	label := strings.ToLower(strings.TrimSpace(task.PriorityLabel))
	switch label {
	case "urgent":
		return 1
	case "high":
		return 2
	case "normal":
		return 3
	case "low":
		return 4
	}
	if task.PriorityRank > 0 {
		return task.PriorityRank
	}
	return 99
}

func primaryAssignee(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	var assignees []provider.User
	if err := json.Unmarshal([]byte(raw), &assignees); err != nil || len(assignees) == 0 {
		return ""
	}
	for _, a := range assignees {
		if strings.TrimSpace(a.Username) != "" {
			return a.Username
		}
		if strings.TrimSpace(a.Email) != "" {
			return a.Email
		}
	}
	return ""
}

func placeSubtasksAfterParents(tasks []cache.TaskEntity) []cache.TaskEntity {
	if len(tasks) <= 1 {
		return tasks
	}

	byID := make(map[string]cache.TaskEntity, len(tasks))
	children := make(map[string][]cache.TaskEntity)
	roots := make([]cache.TaskEntity, 0, len(tasks))

	for _, task := range tasks {
		if task.ID != "" {
			byID[task.ID] = task
		}
	}

	for _, task := range tasks {
		if task.IsSubtask && task.ParentTaskID != "" && task.ParentTaskID != task.ID {
			if _, ok := byID[task.ParentTaskID]; ok {
				children[task.ParentTaskID] = append(children[task.ParentTaskID], task)
				continue
			}
		}
		roots = append(roots, task)
	}

	ordered := make([]cache.TaskEntity, 0, len(tasks))
	visited := make(map[string]bool, len(tasks))

	var appendWithChildren func(task cache.TaskEntity)
	appendWithChildren = func(task cache.TaskEntity) {
		if task.ID != "" && visited[task.ID] {
			return
		}
		ordered = append(ordered, task)
		if task.ID != "" {
			visited[task.ID] = true
		}
		for _, child := range children[task.ID] {
			appendWithChildren(child)
		}
	}

	for _, root := range roots {
		appendWithChildren(root)
	}
	for _, task := range tasks {
		if task.ID == "" || !visited[task.ID] {
			appendWithChildren(task)
		}
	}

	return ordered
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
