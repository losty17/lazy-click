package tui

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"lazy-click/internal/cache"
	"lazy-click/internal/provider"
	syncengine "lazy-click/internal/sync"
	"lazy-click/internal/tui/components"
	"lazy-click/internal/attachment"

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
	appStateTaskSortDirection = "ui.task_sort_direction"
	appStateTaskGroupMode    = "ui.task_group_mode"
	appStateTaskSubtasksMode = "ui.task_subtasks_mode"
	appStateStatusFilter     = "ui.status_filter"
	appStateTaskSearchQuery  = "ui.task_search_query"
	appStateRestoreSession   = "ui.restore.last_session"
	appStateRecentCommands   = "ui.control_center.recent_commands"
	appStateCommandUsage     = "ui.control_center.command_usage"
	appStateActiveProviderID = "ui.active_provider_id"
	appStateClickUpToken     = "provider.clickup.pat"
	appStateMeMode           = "ui.me_mode"
	appStateViewMode         = "ui.view_mode"
	appStateKittyGraphicsEnabled = "ui.kitty_graphics.enabled"

	detailDebounceDelay = 3 * time.Second
	searchDebounceDelay = 250 * time.Millisecond
	pollInterval        = 30 * time.Second
)

type ControlMode string

const (
	ControlModeCommand    ControlMode = ">"
	ControlModeList       ControlMode = "@"
	ControlModeSpace      ControlMode = "#"
	ControlModeTask       ControlMode = "/"
	ControlModeComment    ControlMode = ","
	ControlModeHelp       ControlMode = "?"
	ControlModeAttachment ControlMode = "!"
	ControlModePAT        ControlMode = "$"
)

type controlResult struct {
	Kind       string
	Title      string
	Subtitle   string
	Badge      string
	Shortcut   string
	CommandID  string
	ListID     string
	SpaceID    string
	TaskID     string
	CommentID  string
	TaskTitle  string
	Attachment *provider.Attachment
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
	TaskSortDirection TaskSortDirection `json:"task_sort_direction"`
	TaskGroupMode   TaskGroupMode      `json:"task_group_mode"`
	TaskSubtasks    TaskSubtaskMode    `json:"task_subtasks"`
	StatusFilter    string             `json:"status_filter"`
	TaskSearchQuery string             `json:"task_search_query"`
	MeMode          bool               `json:"me_mode"`
	ViewMode        ViewMode           `json:"view_mode"`
}

type TaskSortMode string

const (
	TaskSortPriority TaskSortMode = "priority"
	TaskSortName     TaskSortMode = "name"
	TaskSortStatus   TaskSortMode = "status"
	TaskSortAssignee TaskSortMode = "assignee"
	TaskSortDueDate  TaskSortMode = "due"
)

type TaskSortDirection string

const (
	TaskSortAsc  TaskSortDirection = "asc"
	TaskSortDesc TaskSortDirection = "desc"
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
	GetCurrentUser(ctx context.Context) (provider.User, error)
	QueueCreateTask(listID string, task provider.Task) error
	QueueTaskUpdate(taskID string, update provider.TaskUpdate) error
	QueueDeleteTask(taskID string) error
	QueueCreateList(spaceID string, name string) error
	QueueUpdateList(listID string, name string) error
	QueueDeleteList(listID string) error
	QueueCreateComment(taskID string, text string) error
	QueueUpdateComment(commentID string, text string) error
	QueueDeleteComment(commentID string) error
	QueueAddComment(taskID string, text string, localCommentID string) error // Deprecated
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

type EditorTarget string

const (
	EditorTargetNone          EditorTarget = ""
	EditorTargetTaskTitle     EditorTarget = "task_title"
	EditorTargetTaskCreate    EditorTarget = "task_create"
	EditorTargetListCreate    EditorTarget = "list_create"
	EditorTargetListName      EditorTarget = "list_name"
	EditorTargetCommentCreate EditorTarget = "comment_create"
)

type ViewMode int

const (
	ViewModeDefault ViewMode = iota
	ViewModeSidebarTaskDetail
	ViewModeFullDetailWithComments
)

type RootModel struct {
	width               int
	height              int
	keymap              Keymap
	activePane          int
	repo                *cache.Repository
	sync                SyncQueuer
	attachments         *attachment.Manager
	provider            string
	viewMode            ViewMode
	debugMode           bool
	activeProviderID    string
	availableProviders  []syncengine.ProviderMeta
	clickUpConnected    bool
	providerSetupPrompt bool
	providerSetupIndex  int
	statusLine          string
	lists               []cache.ListEntity
	spaces              []cache.SpaceEntity
	sidebar             components.SidebarModel
	taskTable           components.TaskTableModel
	detailPanel         components.DetailModel
	commentsSidebar     components.DetailModel
	statuses            []string
	taskSortMode        TaskSortMode
	taskSortDirection   TaskSortDirection
	taskGroupMode       TaskGroupMode
	taskSubtasks        TaskSubtaskMode
	meMode              bool
	currentUser         provider.User

	kittyImageIDs map[string]uint32
	nextKittyID   uint32
	kittyGraphicsEnabled bool

	statusFilter string
	searchQuery  string
	terminalFocused bool

	listSearchQuery       string
	listSortMode          cache.ListSortMode
	favoritesOnly         bool
	stateHydrated         bool
	selectedListID        string
	restoreSnapshot       uiSessionSnapshot
	hasRestoreState       bool

	controlOpen     bool
	controlMode     ControlMode
	controlInput    string
	controlSelected int
	controlResults  []controlResult
	recentCommands  []string
	commandUsage    map[string]commandUsageStat
	loadedTasks     []cache.TaskEntity
	patInput        string

	editorOpen     bool
	editorTarget   EditorTarget
	editorContext  map[string]string
	editorModel    components.TextEditorModel
	confirmOpen    bool
	confirmModel   components.ConfirmModel
	openTaskPrompt bool
	openTaskURL    string
	syncing        bool
	syncFrame      int
	syncError      string
	syncDetail     string

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
	spaces                 []cache.SpaceEntity
	tasks                  []cache.TaskEntity
	statuses               []string
	err                    error
	selectedListID         string
	restored               bool
	restoredListSearch     string
	restoredSortMode       cache.ListSortMode
	restoredFavOnly        bool
	restoredTaskSort       TaskSortMode
	restoredTaskSortDir    TaskSortDirection
	restoredTaskGroup      TaskGroupMode
	restoredSubtasks       TaskSubtaskMode
	restoredStatus         string
	restoredTaskSearch     string
	restoredMeMode         bool
	restoredViewMode       ViewMode
	restoredKittyGraphics  bool
	restoredActiveProvider string
	hasRestoreSnapshot     bool
	restoreSnapshot        uiSessionSnapshot
	recentCommands         []string
	commandUsage           map[string]commandUsageStat
}

type editResultMsg struct{ err error }
type syncResultMsg struct{ err error }
type commentResultMsg struct{ err error }
type copyTaskLinkResultMsg struct {
	url string
	err error
}
type attachmentDownloadResultMsg struct {
	path string
	err  error
	open bool
}
type openTaskInBrowserResultMsg struct {
	err error
}
type kittyUploadResultMsg struct {
	id  uint32
	err error
}
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

type pollTaskMsg struct{}

type manualTaskRefreshResultMsg struct {
	TaskID string
	Err    error
}

type userLoadedMsg struct {
	user provider.User
	err  error
}

func NewRootModel(repo *cache.Repository, sync SyncQueuer, attachments *attachment.Manager, providerName string, statusLine string, clickUpConnected bool, needsProviderSetup bool) RootModel {
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
		attachments:         attachments,
		provider:            providerName,
		activeProviderID:    activeProviderID,
		availableProviders:  availableProviders,
		clickUpConnected:    clickUpConnected,
		providerSetupPrompt: needsProviderSetup,
		providerSetupIndex:  0,
		keymap:              DefaultKeymap(),
		statusLine:          statusLine,
		sidebar:             components.NewSidebar(),
		taskTable:           components.NewTaskTable(),
		detailPanel:         components.NewDetail(),
		commentsSidebar:     components.NewDetail(),
		listSortMode:        cache.ListSortNameAsc,
		taskSortMode:        TaskSortPriority,
		taskSortDirection:   TaskSortAsc,
		taskGroupMode:       TaskGroupNone,
		taskSubtasks:        TaskSubtaskFlat,
		favoritesOnly:       false,
		controlMode:         ControlModeCommand,
		commandUsage:        make(map[string]commandUsageStat),
		kittyImageIDs:       make(map[string]uint32),
		nextKittyID:         1,
		kittyGraphicsEnabled: false,
		terminalFocused:      true,
	}
}

func (m RootModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadDataCmd(),
		m.fetchCurrentUserCmd(),
		m.pollTaskCmd(),
		func() tea.Msg {
			fmt.Print("\x1b[?1004h")
			return nil
		},
	)
}

func (m *RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.FocusMsg:
		m.terminalFocused = true
		return m, m.startDetailRevalidate()

	case tea.BlurMsg:
		m.terminalFocused = false
		return m, nil

	case pollTaskMsg:
		var cmd tea.Cmd
		if m.terminalFocused && m.displayedTaskID != "" && !m.detailLoading && !m.editorOpen && !m.confirmOpen {
			cmd = m.revalidateDetailCmd(m.displayedTaskID, m.detailReqToken)
		}
		return m, tea.Batch(cmd, m.pollTaskCmd())

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.editorOpen {
			m.editorModel.Width = max(40, m.width-20)
		}
		var detailCmd, commentsCmd tea.Cmd
		m.detailPanel, detailCmd = m.detailPanel.Update(msg)
		m.commentsSidebar, commentsCmd = m.commentsSidebar.Update(msg)
		return m, tea.Batch(detailCmd, commentsCmd)

	case components.FieldUpdateMsg:
		m.detailPanel.Mode = components.ModeNormal
		return m, m.submitFieldUpdateCmd(msg.Key, msg.Value)

	case components.CancelFieldEditMsg:
		m.detailPanel.Mode = components.ModeNormal
		return m, nil

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
			case tea.KeyEsc:
				m.patInput = ""
				m.statusLine = "PAT entry canceled, switching to local provider"
				if m.switchProvider("local") {
					return m, m.loadDataCmd()
				}
				return m, nil
			case tea.KeyEnter:
				token := strings.TrimSpace(m.patInput)
				if token == "" {
					m.statusLine = "PAT not saved"
					return m, nil
				}
				if err := m.repo.SaveAppState(appStateClickUpToken, token); err != nil {
					m.statusLine = "Failed to save PAT"
					return m, nil
				}
				if m.sync.SetProviderToken("clickup", token) {
					m.clickUpConnected = true
					m.statusLine = "ClickUp PAT saved"
					return m, tea.Batch(m.loadDataCmd(), m.syncNowCmd())
				}
				m.statusLine = "Failed to set PAT on provider"
				return m, nil
			case tea.KeyBackspace, tea.KeyDelete:
				if len(m.patInput) > 0 {
					m.patInput = m.patInput[:len(m.patInput)-1]
				}
				return m, nil
			default:
				if len(msg.Runes) > 0 {
					m.patInput += string(msg.Runes)
				}
				return m, nil
			}
		}

		if m.controlOpen {
			if msg.String() == "ctrl+c" {
				if m.hasUnsavedChanges() {
					return m.openConfirm("Changes not saved will be lost. Quit anyway?", "QUIT", func() tea.Cmd {
						m.persistSessionSnapshot()
						return tea.Quit
					})
				}
				m.persistSessionSnapshot()
				return m, tea.Quit
			}
			return m, m.updateControlCenter(msg)
		}

		if m.editorOpen {
			if msg.String() == "ctrl+c" || (msg.String() == "q" && m.editorModel.Mode == components.VimModeNormal) {
				if m.hasUnsavedChanges() {
					return m.openConfirm("Changes not saved will be lost. Quit anyway?", "QUIT", func() tea.Cmd {
						m.persistSessionSnapshot()
						return tea.Quit
					})
				}
				m.persistSessionSnapshot()
				return m, tea.Quit
			}
			var cmd tea.Cmd
			m.editorModel, cmd = m.editorModel.Update(msg)
			return m, cmd
		}

		if m.confirmOpen {
			var cmd tea.Cmd
			m.confirmModel, cmd = m.confirmModel.Update(msg)
			return m, cmd
		}

		if m.activePane == 2 && m.detailPanel.Mode == components.ModeInsert {
			if msg.String() == "ctrl+c" || (msg.String() == "q" && m.detailPanel.Editor.Mode == components.VimModeNormal) {
				if m.hasUnsavedChanges() {
					return m.openConfirm("Changes not saved will be lost. Quit anyway?", "QUIT", func() tea.Cmd {
						m.persistSessionSnapshot()
						return tea.Quit
					})
				}
				m.persistSessionSnapshot()
				return m, tea.Quit
			}
			var cmd tea.Cmd
			m.detailPanel, cmd = m.detailPanel.Update(msg)
			return m, cmd
		}

		// Global keys and pane-specific delegation
		switch msg.String() {
		case "q", "ctrl+c":
			if m.hasUnsavedChanges() {
				return m.openConfirm("Changes not saved will be lost. Quit anyway?", "QUIT", func() tea.Cmd {
					m.persistSessionSnapshot()
					return tea.Quit
				})
			}
			m.persistSessionSnapshot()
			return m, tea.Quit
		case "ctrl+p", "ctrl+k":
			m.openControlCenter(ControlModeCommand)
			return m, nil
		case ":":
			m.openControlCenter(ControlModeCommand)
			return m, nil
		case "$":
			m.openControlCenter(ControlModePAT)
			return m, nil
		case "tab":
			m.activePane = (m.activePane + 1) % 3
			if m.viewMode == ViewModeSidebarTaskDetail && m.activePane == 0 {
				m.activePane = 1
			} else if m.viewMode == ViewModeFullDetailWithComments && m.activePane == 0 {
				m.activePane = 1
			}
			return m, nil
		case "shift+tab", "backtab":
			m.activePane = (m.activePane + 2) % 3
			if m.viewMode == ViewModeSidebarTaskDetail && m.activePane == 0 {
				m.activePane = 2
			} else if m.viewMode == ViewModeFullDetailWithComments && m.activePane == 0 {
				m.activePane = 2
			}
			return m, nil
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
		case m.keymap.ViewMode:
			m.viewMode = (m.viewMode + 1) % 3
			m.taskTable.Simplified = (m.viewMode == ViewModeSidebarTaskDetail)
			switch m.viewMode {
			case ViewModeDefault:
				m.statusLine = "View Mode: Default"
			case ViewModeSidebarTaskDetail:
				m.statusLine = "View Mode: Sidebar Task List + Full Height Detail"
			case ViewModeFullDetailWithComments:
				m.statusLine = "View Mode: Full Detail + Comments Sidebar"
			}
			return m, m.refreshDetail(m.detailLoading, m.detailLoadingMsg)
		}

		if m.activePane == 2 {
			var cmd tea.Cmd
			m.detailPanel, cmd = m.detailPanel.Update(msg)
			return m, cmd
		}

		if m.activePane == 1 && m.viewMode == ViewModeFullDetailWithComments {
			var cmd tea.Cmd
			m.commentsSidebar, cmd = m.commentsSidebar.Update(msg)
			return m, cmd
		}

		if m.openTaskPrompt {
			switch msg.Type {
			case tea.KeyEnter:
				m.openTaskPrompt = false
				return m, m.openTaskInBrowserCmd(m.openTaskURL)
			case tea.KeyEsc:
				m.openTaskPrompt = false
				m.openTaskURL = ""
				m.statusLine = "Open task canceled"
				return m, nil
			}
			switch strings.ToLower(msg.String()) {
			case "y":
				m.openTaskPrompt = false
				return m, m.openTaskInBrowserCmd(m.openTaskURL)
			case "n":
				m.openTaskPrompt = false
				m.openTaskURL = ""
				m.statusLine = "Open task canceled"
			}
			return m, nil
		}

		if msg.Type == tea.KeyEnter && m.activePane == 0 {
			if m.sidebar.ToggleSelectedCollapse() {
				return m, nil
			}
			newListID := m.selectedListIDFromSidebar()
			if newListID != "" && newListID != m.selectedListID {
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
			} else if newListID != "" && !m.syncing {
				m.syncing = true
				m.syncFrame = 0
				m.statusLine = "Sync in progress..."
				m.syncDetail = "starting manual sync for list"
				return m, tea.Batch(m.syncNowCmd(), m.syncTickCmd())
			}
		}

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

		if msg.Type == tea.KeyEnter && m.activePane == 2 {
			url := m.currentTaskBrowserURL()
			if url != "" {
				m.openTaskURL = url
				m.openTaskPrompt = true
				return m, nil
			}
		}

		switch msg.String() {
		case "home":
			return m.handleMoveToTop()
		case "end":
			return m.handleMoveToBottom()
		case "tab":
			m.activePane = (m.activePane + 1) % 3
		case "shift+tab", "backtab":
			m.activePane = (m.activePane + 2) % 3
		case m.keymap.Down, "down":
			switch m.activePane {
			case 0:
				m.sidebar.Move(1)
			case 1:
				m.taskTable.Move(1)
			}
			return m, nil
		case m.keymap.Up, "up":
			switch m.activePane {
			case 0:
				m.sidebar.Move(-1)
			case 1:
				m.taskTable.Move(-1)
			}
			return m, nil
		case "h", "left":
			return m.handleHorizontalMove(-2)
		case "l", "right":
			return m.handleHorizontalMove(2)
		case m.keymap.RefreshTask:
			if m.activePane == 1 {
				return m, m.refreshCurrentTaskNowCmd()
			}
		case m.keymap.CreateTask:
			if m.selectedListID != "" {
				return m.openEditor(EditorTargetTaskCreate, "", "Create new Task title:", false)
			}
		case m.keymap.DeleteTask:
			if m.activePane == 1 {
				row, ok := m.taskTable.Selected()
				if ok && row.ID != "" {
					return m.openConfirm("Delete task: "+row.Title+"?", "DELETE", func() tea.Cmd {
						return m.deleteTaskCmd(row.ID)
					})
				}
			}
		case m.keymap.CreateList:
			m.openControlCenter(ControlModeSpace)
			return m, nil
		case m.keymap.DeleteList:
			if m.activePane == 0 && m.selectedListID != "" {
				list, _ := m.repo.GetListByID(m.selectedListID)
				if list != nil {
					return m.openConfirm("Delete list: "+list.Name+"?", "DELETE", func() tea.Cmd {
						return m.deleteListCmd(list.ID)
					})
				}
			}
		case m.keymap.AddComment:
			if m.activePane == 1 {
				row, ok := m.taskTable.Selected()
				if ok && row.ID != "" {
					return m.openEditor(EditorTargetCommentCreate, "", "Add Comment:", true)
				}
			}
		case m.keymap.DeleteComment:
			if m.displayedTaskID != "" {
				m.openControlCenter(ControlModeComment)
				return m, nil
			}
		case m.keymap.Edit:
			if m.activePane == 1 {
				m.activePane = 2
				// Also trigger the update so it starts editing immediately
				var cmd tea.Cmd
				m.detailPanel, cmd = m.detailPanel.Update(msg)
				return m, cmd
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
			m.openControlCenter(ControlModeTask)
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
		case m.keymap.CopyTaskLink:
			return m, m.copyTaskLinkCmd()
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
		case m.keymap.SortDirection:
			m.toggleTaskSortDirection()
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
		case m.keymap.MeMode:
			m.meMode = !m.meMode
			m.saveTaskPrefs()
			if m.meMode {
				m.statusLine = "Me Mode enabled (showing only your tasks)"
			} else {
				m.statusLine = "Me Mode disabled"
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
		case m.keymap.DownloadAttachments:
			return m, m.downloadAttachmentsCmd(false)
		case m.keymap.Debug:
			m.debugMode = !m.debugMode
			if m.debugMode {
				m.statusLine = "Debug mode enabled"
			} else {
				m.statusLine = "Debug mode disabled"
			}
			return m, m.refreshDetail(false, "")
		case "A":
			m.openControlCenter(ControlModeAttachment)
			return m, nil
		}

	case userLoadedMsg:
		if msg.err == nil {
			m.currentUser = msg.user
			if m.meMode {
				return m, m.loadDataCmd()
			}
		}
		return m, nil

	case closeEditorMsg:
		m.editorOpen = false
		m.editorTarget = EditorTargetNone
		return m, m.loadDataCmd()

	case closeConfirmMsg:
		m.confirmOpen = false
		return m, m.loadDataCmd()

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
			if msg.restoredTaskSortDir != "" {
				m.taskSortDirection = msg.restoredTaskSortDir
			}
			if msg.restoredTaskGroup != "" {
				m.taskGroupMode = msg.restoredTaskGroup
			}
			if msg.restoredSubtasks != "" {
				m.taskSubtasks = msg.restoredSubtasks
			}
			m.statusFilter = msg.restoredStatus
			m.searchQuery = msg.restoredTaskSearch
			m.meMode = msg.restoredMeMode
			m.viewMode = msg.restoredViewMode
			m.taskTable.Simplified = (m.viewMode == ViewModeSidebarTaskDetail)
			m.kittyGraphicsEnabled = msg.restoredKittyGraphics
			if msg.restoredActiveProvider != "" && m.sync != nil {
				if m.sync.SetActiveProvider(msg.restoredActiveProvider) {
					m.activeProviderID = msg.restoredActiveProvider
					m.provider = m.sync.ProviderDisplayName()
				}
			}
			m.restoreSnapshot = msg.restoreSnapshot
			m.hasRestoreState = msg.hasRestoreSnapshot
			m.recentCommands = append([]string(nil), msg.recentCommands...)
			if msg.commandUsage != nil {
				m.commandUsage = msg.commandUsage
			}
			if m.hasRestoreState {
				m.applySessionSnapshot(m.restoreSnapshot)
			}
		}

		m.lists = msg.lists
		m.spaces = msg.spaces
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

		sidebarRows := mapListsToRows(msg.lists, msg.spaces)
		m.sidebar.SetRows(sidebarRows)
		
		selectedIdx := 0
		if m.selectedListID != "" {
			for i, row := range sidebarRows {
				if row.ID == m.selectedListID {
					selectedIdx = i
					break
				}
			}
		}
		m.sidebar.SetSelectedIndex(selectedIdx)

		m.taskTable.SetRows(mapTasksToRows(msg.tasks, m.taskGroupMode, m.taskSubtasks))
		m.taskTable.NoTasksMessage = ""
		m.loadedTasks = append([]cache.TaskEntity(nil), msg.tasks...)

		// Handle ID remapping for displayed/selected tasks
		if m.displayedTaskID != "" && strings.HasPrefix(m.displayedTaskID, "tmp_") {
			// If our current tmp ID is gone, look for a task that might be its replacement.
			// Since we don't have a direct map, we can check if there's only one task with SyncStateSynced 
			// that was recently updated, or better: just check if the ID is missing and try to recover.
			if _, ok := m.taskTable.RowByID(m.displayedTaskID); !ok {
				// The tmp ID is gone. If there is exactly one new synced task that wasn't there before, 
				// we could guess. But a safer way is to just let the user re-select if it's ambiguous.
				// However, usually, if we just created a task, it's the one we're looking at.
				for _, t := range msg.tasks {
					if !strings.HasPrefix(t.ID, "tmp_") && t.SyncState == cache.SyncStateSynced {
						// This is a heuristic: if we just lost a tmp task and found a new real one, switch to it.
						// This works well for single-user create-then-view flow.
						m.displayedTaskID = t.ID
						m.selectedTaskID = t.ID
						break
					}
				}
			}
		}

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
		cmd := m.refreshDetail(m.detailLoading && m.detailReqTaskID == m.displayedTaskID, m.detailLoadingMsg)
		if len(msg.lists) == 0 {
			m.statusLine = "No lists in cache yet. Press 's' to sync now."
		} else if len(msg.tasks) == 0 {
			m.taskTable.NoTasksMessage = "No tasks for this list in cache. Press Enter to fetch."
		} else if m.statusLine == "" {
			m.statusLine = "Loaded from local cache"
		}
		return m, cmd

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
			return m, m.refreshDetail(false, "")
		}
		m.detailError = ""
		m.detailErrorTask = ""
		m.displayedTaskID = msg.TaskID
		m.taskTable.SetDisplayedTaskID(msg.TaskID)
		m.syncDetail = "task revalidated"
		return m, m.refreshDetail(false, "")

	case manualTaskRefreshResultMsg:
		m.detailLoading = false
		m.detailLoadingMsg = ""
		if msg.Err != nil {
			m.detailError = msg.Err.Error()
			m.detailErrorTask = msg.TaskID
			m.syncDetail = "manual task refresh failed"
			m.statusLine = "Task refresh failed: " + msg.Err.Error()
			return m, m.refreshDetail(false, "")
		}
		m.detailError = ""
		m.detailErrorTask = ""
		m.displayedTaskID = msg.TaskID
		m.taskTable.SetDisplayedTaskID(msg.TaskID)
		m.syncDetail = "manual task refresh complete"
		m.statusLine = "Task refreshed"
		return m, m.refreshDetail(false, "")

	case editResultMsg:
		m.editorOpen = false
		m.editorTarget = EditorTargetNone
		m.confirmOpen = false
		if msg.err != nil {
			m.statusLine = "Operation failed: " + msg.err.Error()
			return m, nil
		}
		m.statusLine = "Success locally and queued for provider push"
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
		m.editorOpen = false
		m.editorTarget = EditorTargetNone
		if msg.err != nil {
			m.statusLine = "Comment failed: " + msg.err.Error()
			return m, nil
		}
		m.statusLine = "Comment saved locally and queued for provider push"
		return m, m.loadDataCmd()

	case copyTaskLinkResultMsg:
		if msg.err != nil {
			m.statusLine = "Copy task link failed: " + msg.err.Error()
			return m, nil
		}
		m.statusLine = "Task link copied: " + msg.url
		return m, nil

	case openTaskInBrowserResultMsg:
		m.openTaskURL = ""
		if msg.err != nil {
			m.statusLine = "Open in browser failed: " + msg.err.Error()
			return m, nil
		}
		m.statusLine = "Opened task in browser"
		return m, nil

	case attachmentDownloadResultMsg:
		if msg.err != nil {
			m.statusLine = "Attachment download failed: " + msg.err.Error()
			return m, nil
		} else {
			if msg.open {
				m.statusLine = "Attachment opened: " + msg.path
			} else {
				m.statusLine = "Attachments downloaded"
			}
			return m, m.refreshDetail(false, "")
		}

	case kittyUploadResultMsg:
		if msg.err != nil {
			m.statusLine = "Kitty image upload failed: " + msg.err.Error()
		}
		return m, nil

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

func (m *RootModel) handleHorizontalMove(delta int) (tea.Model, tea.Cmd) {
	switch m.activePane {
	case 0:
		m.sidebar.MoveHorizontal(delta)
	case 1:
		m.taskTable.MoveHorizontal(delta)
	}
	return m, nil
}

func (m *RootModel) handleMoveToTop() (tea.Model, tea.Cmd) {
	switch m.activePane {
	case 0:
		m.sidebar.MoveToTop()
	case 1:
		m.taskTable.MoveToTop()
	}
	return m, nil
}

func (m *RootModel) handleMoveToBottom() (tea.Model, tea.Cmd) {
	switch m.activePane {
	case 0:
		m.sidebar.MoveToBottom()
	case 1:
		m.taskTable.MoveToBottom()
	}
	return m, nil
}

func (m RootModel) View() string {
	totalWidth, sidebarWidth, tableWidth, _, sidebarHeight, tableHeight, detailHeight := m.layout()

	wsInfo := m.currentWorkspaceInfo()
	headerText := fmt.Sprintf("lazy-click [%s]", m.provider)
	if wsInfo != "" {
		headerText = fmt.Sprintf("lazy-click [%s] %s", m.provider, wsInfo)
	}
	title := HeaderStyle.Render(headerText)
	syncLine := m.syncProgressLine(totalWidth)
	header := lipgloss.JoinHorizontal(lipgloss.Top, title, lipgloss.NewStyle().Width(totalWidth-lipgloss.Width(title)).Align(lipgloss.Right).Render(syncLine))

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

	var body string
	switch m.viewMode {
	case ViewModeDefault:
		sidebar := sidebarStyle.Width(sidebarWidth).Height(sidebarHeight).Render(
			m.sidebar.Render(m.activePane == 0, sidebarWidth-4, sidebarHeight),
		)
		table := tableStyle.Width(tableWidth).Height(tableHeight).Render(
			m.taskTable.Render(m.activePane == 1, tableWidth-2, tableHeight),
		)
		detail := detailStyle.Width(tableWidth).Height(detailHeight).Render(
			m.detailPanel.Render(m.activePane == 2, tableWidth-2, detailHeight),
		)
		right := lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.NewStyle().MarginBottom(verticalPaneGap).Render(table),
			detail,
		)
		body = lipgloss.JoinHorizontal(
			lipgloss.Top,
			lipgloss.NewStyle().MarginRight(horizontalPaneGap).Render(sidebar),
			right,
		)
	case ViewModeSidebarTaskDetail:
		table := tableStyle.Width(sidebarWidth).Height(sidebarHeight).Render(
			m.taskTable.Render(m.activePane == 1, sidebarWidth-2, sidebarHeight),
		)
		detail := detailStyle.Width(tableWidth).Height(sidebarHeight).Render(
			m.detailPanel.Render(m.activePane == 2, tableWidth-2, sidebarHeight),
		)
		body = lipgloss.JoinHorizontal(
			lipgloss.Top,
			lipgloss.NewStyle().MarginRight(horizontalPaneGap).Render(table),
			detail,
		)
	case ViewModeFullDetailWithComments:
		// In this mode, we show detail full height and comments on a sidebar
		// sidebarWidth here is used for the comments sidebar
		detail := detailStyle.Width(tableWidth).Height(sidebarHeight).Render(
			m.detailPanel.Render(m.activePane == 2, tableWidth-2, sidebarHeight),
		)
		
		commentsStyle := PanelStyle
		if m.activePane == 1 {
			commentsStyle = FocusedPanelStyle
		}

		commentsView := commentsStyle.
			Width(sidebarWidth).
			Height(sidebarHeight).
			Render(m.commentsSidebar.Render(m.activePane == 1, sidebarWidth-2, sidebarHeight))

		body = lipgloss.JoinHorizontal(
			lipgloss.Top,
			lipgloss.NewStyle().MarginRight(horizontalPaneGap).Render(detail),
			commentsView,
		)
	}

	statusFilter := "all"
	if m.statusFilter != "" {
		statusFilter = m.statusFilter
	}
	taskSearch := "off"
	if m.searchQuery != "" {
		taskSearch = m.searchQuery
	}
	status := fmt.Sprintf(
		"List sort: %s | Task sort: %s (%s) | Task group: %s | Subtasks: %s | Favorites-only: %t | Me Mode: %t | Task status: %s | Task search: %s",
		m.listSortMode,
		m.taskSortMode,
		m.taskSortDirection,
		m.taskGroupMode,
		m.taskSubtasks,
		m.favoritesOnly,
		m.meMode,
		statusFilter,
		taskSearch,
	)

	screen := strings.Join([]string{
		header,
		body,
		StatusStyle.Render(components.Truncate(status, totalWidth, "...")),
		StatusStyle.Render(components.Truncate(m.statusLine, totalWidth, "...")),
	}, "\n")

	if m.controlOpen {
		overlay := m.renderControlCenter(totalWidth)
		y := centeredOverlayY(screen, overlay, 0)
		screen = overlayCentered(screen, overlay, totalWidth, y)
	}
	if m.editorOpen {
		overlay := m.editorModel.RenderOverlay(totalWidth)
		y := centeredOverlayY(screen, overlay, 0)
		screen = overlayCentered(screen, overlay, totalWidth, y)
	}
	if m.confirmOpen {
		overlay := m.confirmModel.Render(totalWidth)
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
	if m.openTaskPrompt {
		overlay := m.renderOpenTaskPromptOverlay(totalWidth)
		y := centeredOverlayY(screen, overlay, -1)
		screen = overlayCentered(screen, overlay, totalWidth, y)
	}

	return lipgloss.NewStyle().
		Width(totalWidth).
		MaxWidth(totalWidth).
		MaxHeight(max(totalHeightFromModel(m.height)-1, 8)).
		Render(screen)
}

func (m RootModel) layout() (int, int, int, int, int, int, int) {
	hFrame := PanelStyle.GetHorizontalFrameSize()
	vFrame := PanelStyle.GetVerticalFrameSize()
	const horizontalPaneGap = 1

	totalWidth := hFrame
	if m.width > 0 {
		totalWidth = m.width - 2
	}
	totalWidth = max(totalWidth, 20)

	totalHeight := max(totalHeightFromModel(m.height)-1, 8)
	reserved := 4
	bodyOuterHeight := totalHeight - reserved
	minBodyOuter := (2 * vFrame) + 2
	if bodyOuterHeight < minBodyOuter {
		bodyOuterHeight = minBodyOuter
	}

	innerWidthBudget := max(totalWidth-(2*hFrame)-horizontalPaneGap, 2)
	
	var sidebarWidth, tableWidth, detailWidth int
	var sidebarHeight, tableHeight, detailHeight int

	sidebarHeight = max(bodyOuterHeight-vFrame, 1)

	switch m.viewMode {
	case ViewModeDefault:
		sidebarWidth = innerWidthBudget / 5
		minSidebar := 8
		maxSidebar := max(innerWidthBudget-8, minSidebar)
		if sidebarWidth < minSidebar {
			sidebarWidth = minSidebar
		}
		if sidebarWidth > maxSidebar {
			sidebarWidth = maxSidebar
		}
		tableWidth = innerWidthBudget - sidebarWidth + (totalWidth - innerWidthBudget - (2 * hFrame) - horizontalPaneGap) + 4
		detailWidth = tableWidth

		rightInnerHeightBudget := max(bodyOuterHeight-(2*vFrame), 2)
		tableHeight = max((rightInnerHeightBudget*2)/3, 1)
		detailHeight = rightInnerHeightBudget - tableHeight
		if detailHeight < 1 {
			detailHeight = 1
			tableHeight = max(rightInnerHeightBudget-1, 1)
		}

	case ViewModeSidebarTaskDetail:
		sidebarWidth = innerWidthBudget / 4
		if sidebarWidth < 15 {
			sidebarWidth = 15
		}
		tableWidth = innerWidthBudget - sidebarWidth + 4
		detailWidth = tableWidth
		tableHeight = sidebarHeight
		detailHeight = sidebarHeight

	case ViewModeFullDetailWithComments:
		sidebarWidth = innerWidthBudget / 3
		if sidebarWidth < 20 {
			sidebarWidth = 20
		}
		tableWidth = innerWidthBudget - sidebarWidth + 4
		detailWidth = tableWidth
		tableHeight = sidebarHeight
		detailHeight = sidebarHeight
	}

	return totalWidth, sidebarWidth, tableWidth, detailWidth, sidebarHeight, tableHeight, detailHeight
}

func (m RootModel) fetchCurrentUserCmd() tea.Cmd {
	return func() tea.Msg {
		if m.sync == nil {
			return userLoadedMsg{err: fmt.Errorf("sync not available")}
		}
		user, err := m.sync.GetCurrentUser(context.Background())
		return userLoadedMsg{user: user, err: err}
	}
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
	taskSortDirection := m.taskSortDirection
	taskGroupMode := m.taskGroupMode
	taskSubtaskMode := m.taskSubtasks
	meMode := m.meMode
	currentUserID := m.currentUser.ID

	return func() tea.Msg {
		msg := dataLoadedMsg{}
		if !hydrated {
			sortMode := cache.ListSortNameAsc
			// ... (lines omitted for brevity, I will match carefully)

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

			taskSortDirection = TaskSortAsc
			if dirRaw, err := m.repo.GetAppState(appStateTaskSortDirection); err != nil {
				return dataLoadedMsg{err: err}
			} else if parsed := TaskSortDirection(dirRaw); parsed != "" {
				taskSortDirection = parsed
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

			if meModeRaw, err := m.repo.GetAppState(appStateMeMode); err != nil {
				return dataLoadedMsg{err: err}
			} else if meModeRaw != "" {
				if parsed, parseErr := strconv.ParseBool(meModeRaw); parseErr == nil {
					meMode = parsed
				}
			}

			viewMode := ViewModeDefault
			if viewModeRaw, err := m.repo.GetAppState(appStateViewMode); err == nil && viewModeRaw != "" {
				if parsed, err := strconv.Atoi(viewModeRaw); err == nil {
					viewMode = ViewMode(parsed)
				}
			}

			kittyEnabled := false
			if kittyRaw, err := m.repo.GetAppState(appStateKittyGraphicsEnabled); err == nil && kittyRaw != "" {
				if parsed, parseErr := strconv.ParseBool(kittyRaw); parseErr == nil {
					kittyEnabled = parsed
				}
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
			msg.restoredTaskSortDir = taskSortDirection
			msg.restoredTaskGroup = taskGroupMode
			msg.restoredSubtasks = taskSubtaskMode
			msg.restoredStatus = statusFilter
			msg.restoredTaskSearch = taskSearch
			msg.restoredMeMode = meMode
			msg.restoredViewMode = viewMode
			msg.restoredKittyGraphics = kittyEnabled
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

		spaces, err := m.repo.GetSpacesByProvider(activeProviderID)
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

		assigneeIDs := []string{}
		if meMode && currentUserID != "" {
			assigneeIDs = append(assigneeIDs, currentUserID)
		}

		tasks, err := m.repo.GetTasksByQuery(cache.TaskListQuery{
			Provider:      activeProviderID,
			ListID:        selectedListID,
			Statuses:      selectedStatusFilter(statusFilter),
			AssigneeIDs:   assigneeIDs,
			IncludeClosed: true,
		})
		if err != nil {
			return dataLoadedMsg{err: err}
		}
		tasks = fuzzyFindTasks(tasks, taskSearch)
		tasks = organizeTasks(tasks, taskSortMode, taskSortDirection, taskGroupMode, taskSubtaskMode)

		statuses, err := m.repo.GetTaskStatusesByList(selectedListID)
		if err != nil {
			return dataLoadedMsg{err: err}
		}

		msg.lists = lists
		msg.spaces = spaces
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
		ControlCenterTitleStyle.Render(components.Truncate("Choose your default provider", panelWidth-2, "...")),
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
			lines = append(lines, style.Render(components.Truncate(prefix+line, panelWidth-2, "...")))
		}
	}
	lines = append(lines, "", HelpStyle.Render("Use arrows and press Enter to continue"))
	return RestorePromptStyle.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func (m RootModel) renderProviderConnectOverlay(width int) string {
	panelWidth := min(max(width-6, 52), 104)
	meta, _ := m.activeProviderMeta()
	lines := []string{
		ControlCenterTitleStyle.Render(components.Truncate("Provider requires connection", panelWidth-2, "...")),
		"",
		components.Truncate("Enter your Personal Access Token for "+meta.DisplayName, panelWidth-2, "..."),
		"",
		HelpStyle.Render(meta.TokenInstructions),
		"",
		m.patInput,
		"",
		HelpStyle.Render("Press Enter to save, Esc to cancel"),
	}
	return RestorePromptStyle.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func (m RootModel) renderOpenTaskPromptOverlay(width int) string {
	panelWidth := min(max(width-6, 56), 110)
	urlLine := components.Truncate(m.openTaskURL, panelWidth-2, "...")
	lines := []string{
		ControlCenterTitleStyle.Render(components.Truncate("Open task in browser?", panelWidth-2, "...")),
		"",
		components.Truncate("This will open your default browser for the selected task.", panelWidth-2, "..."),
		"",
		components.Truncate(urlLine, panelWidth-2, "..."),
		"",
		HelpStyle.Render("Enter/Y confirm, N/Esc cancel"),
	}
	return RestorePromptStyle.Width(panelWidth).Render(strings.Join(lines, "\n"))
}

func (m RootModel) currentTaskBrowserURL() string {
	row, ok := m.displayedTaskRow()
	if !ok {
		row, ok = m.taskTable.Selected()
		if !ok {
			return ""
		}
	}
	if strings.TrimSpace(row.ID) == "" {
		return ""
	}
	return "https://app.clickup.com/t/" + row.ID
}

func (m RootModel) openAttachmentCmd(a provider.Attachment) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		path, err := m.attachments.Download(ctx, a.ID, a.Filename, a.URL)
		if err != nil {
			return attachmentDownloadResultMsg{err: err}
		}
		if err := openFile(path); err != nil {
			return attachmentDownloadResultMsg{err: err}
		}
		return attachmentDownloadResultMsg{path: path, open: true}
	}
}

func (m RootModel) uploadToKittyCmd(id uint32, path string) tea.Cmd {
	return func() tea.Msg {
		data, err := os.ReadFile(path)
		if err != nil {
			return kittyUploadResultMsg{id, err}
		}

		encoded := base64.StdEncoding.EncodeToString(data)
		const chunkSize = 4096

		// Use APC sequence to transmit image without displaying it immediately
		// a=t: transmit only
		// i: ID
		// t=d: direct data
		// f=100: auto-detect format
		// q=2: quiet mode
		for i := 0; i < len(encoded); i += chunkSize {
			end := i + chunkSize
			more := 1
			if end >= len(encoded) {
				end = len(encoded)
				more = 0
			}
			payload := encoded[i:end]
			var sequence string
			if i == 0 {
				sequence = fmt.Sprintf("\x1b_Ga=t,i=%d,t=d,f=100,q=2,m=%d;%s\x1b\\", id, more, payload)
			} else {
				sequence = fmt.Sprintf("\x1b_Gm=%d;%s\x1b\\", more, payload)
			}
			// Write directly to stdout to bypass any Bubbletea buffering if possible
			// but being careful not to block.
			_, _ = os.Stdout.WriteString(sequence)
		}
		return kittyUploadResultMsg{id, nil}
	}
}

func (m RootModel) downloadAttachmentsCmd(open bool) tea.Cmd {
	taskID := m.displayedTaskID
	if taskID == "" {
		return func() tea.Msg { return attachmentDownloadResultMsg{err: fmt.Errorf("no task selected")} }
	}

	return func() tea.Msg {
		task, err := m.repo.GetTaskByID(taskID)
		if err != nil {
			return attachmentDownloadResultMsg{err: err}
		}
		if task == nil || task.AttachmentsJSON == "" {
			return attachmentDownloadResultMsg{err: fmt.Errorf("no attachments found")}
		}

		var attachments []provider.Attachment
		if err := json.Unmarshal([]byte(task.AttachmentsJSON), &attachments); err != nil {
			return attachmentDownloadResultMsg{err: err}
		}

		if len(attachments) == 0 {
			return attachmentDownloadResultMsg{err: fmt.Errorf("no attachments found")}
		}

		ctx := context.Background()
		var lastPath string
		for _, a := range attachments {
			path, err := m.attachments.Download(ctx, a.ID, a.Filename, a.URL)
			if err != nil {
				return attachmentDownloadResultMsg{err: err}
			}
			lastPath = path
		}

		if open && lastPath != "" {
			if err := openFile(lastPath); err != nil {
				return attachmentDownloadResultMsg{err: err}
			}
			return attachmentDownloadResultMsg{path: lastPath, open: true}
		}

		return attachmentDownloadResultMsg{}
	}
}

func openFile(path string) error {
	return openBrowser(path) // openBrowser uses xdg-open/open/start which works for files too
}

func (m RootModel) copyTaskLinkCmd() tea.Cmd {
	url := m.currentTaskBrowserURL()
	if strings.TrimSpace(url) == "" {
		return func() tea.Msg {
			return copyTaskLinkResultMsg{err: fmt.Errorf("no task selected")}
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		if err := copyToClipboard(ctx, url); err != nil {
			return copyTaskLinkResultMsg{url: url, err: err}
		}
		return copyTaskLinkResultMsg{url: url}
	}
}

func (m RootModel) openTaskInBrowserCmd(url string) tea.Cmd {
	url = strings.TrimSpace(url)
	if url == "" {
		return func() tea.Msg { return openTaskInBrowserResultMsg{err: fmt.Errorf("task URL is empty")} }
	}
	return func() tea.Msg {
		if err := openBrowser(url); err != nil {
			return openTaskInBrowserResultMsg{err: err}
		}
		return openTaskInBrowserResultMsg{}
	}
}

func (m RootModel) selectedListIDFromSidebar() string {
	row, ok := m.sidebar.Selected()
	if !ok || row.Type != components.SidebarRowList {
		return ""
	}
	return row.ID
}

func (m RootModel) currentWorkspaceInfo() string {
	if m.selectedListID == "" {
		return ""
	}
	var listName, spaceName, workspaceName string
	for _, l := range m.lists {
		if l.ID == m.selectedListID {
			listName = l.Name
			for _, s := range m.spaces {
				if s.ID == l.SpaceID {
					spaceName = s.Name
					workspaceName = s.WorkspaceName
					break
				}
			}
			break
		}
	}
	if workspaceName != "" && spaceName != "" && listName != "" {
		return fmt.Sprintf("%s > %s > %s", workspaceName, spaceName, listName)
	}
	if spaceName != "" && listName != "" {
		return fmt.Sprintf("%s > %s", spaceName, listName)
	}
	return listName
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

func (m *RootModel) refreshDetail(loading bool, loadingMsg string) tea.Cmd {
	selected, ok := m.displayedTaskRow()
	if !ok {
		if loading {
			m.detailPanel.SetFields([]components.DetailField{{Label: "Loading", Value: "Loading task detail..."}})
			return nil
		}
		cursorTaskID := m.currentSelectedTaskID()
		if cursorTaskID == "" {
			m.detailPanel.SetFields([]components.DetailField{{Label: "Status", Value: "No task selected"}})
			return nil
		}
		m.detailPanel.SetFields([]components.DetailField{{Label: "Status", Value: "No task opened. Press Enter on a task to open details."}})
		return nil
	}

	task, err := m.repo.GetTaskByID(selected.ID)
	if err != nil {
		m.detailPanel.SetFields([]components.DetailField{
			{Label: "Title", Value: selected.Title},
			{Label: "Error", Value: "Failed to load task detail: " + err.Error()},
		})
		return nil
	}

	fields := make([]components.DetailField, 0, 10)
	if loading {
		if loadingMsg == "" {
			loadingMsg = "Revalidating detail..."
		}
		fields = append(fields, components.DetailField{Label: "Loading", Value: loadingMsg})
	}

	if task == nil {
		fields = append(fields,
			components.DetailField{Key: "title", Label: "Title", Value: selected.Title, Editable: true},
			components.DetailField{Key: "status", Label: "Status", Value: selected.Status, Editable: true},
			components.DetailField{Label: "Assignees", Value: selected.Assignees},
		)
		if m.detailError != "" && m.detailErrorTask == selected.ID {
			fields = append(fields, components.DetailField{Label: "Last Error", Value: m.detailError})
		}
		fields = append(fields, components.DetailField{Label: "Sync", Value: "Loading task detail from provider..."})
		m.detailPanel.SetFields(fields)
		return nil
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

	fields = append(fields,
		components.DetailField{Key: "title", Label: "Title", Value: task.Title, Editable: true},
		components.DetailField{Key: "status", Label: "Status", Value: task.Status, Editable: true},
		components.DetailField{Key: "priority", Label: "Priority", Value: priority, Editable: true},
		components.DetailField{Label: "Due Date", Value: dueDate},
		components.DetailField{Label: "Assignees", Value: assignees},
	)

	if m.detailError != "" && m.detailErrorTask == selected.ID {
		fields = append(fields, components.DetailField{Label: "Last Error", Value: m.detailError})
	}

	fields = append(fields, components.DetailField{Key: "description", Label: "Description", Value: task.DescriptionMD, Editable: true, MultiLine: true})

	var cmds []tea.Cmd
	var taskAttachments []provider.Attachment
	if task.AttachmentsJSON != "" {
		_ = json.Unmarshal([]byte(task.AttachmentsJSON), &taskAttachments)
	}

	if len(taskAttachments) > 0 {
		var sb strings.Builder
		isKitty := components.IsKittyTerminal() && m.kittyGraphicsEnabled
		_, _, _, rightInnerWidth, _, _, _ := m.layout()
		imageMaxWidth := rightInnerWidth - 4
		for _, a := range taskAttachments {
			sb.WriteString(fmt.Sprintf("- %s (%s)\n", a.Filename, formatSize(a.Size)))
			if isKitty && isImage(a.Filename) {
				localPath := m.attachments.GetLocalPath(a.ID, a.Filename)
				if _, err := os.Stat(localPath); err == nil {
					id, exists := m.kittyImageIDs[localPath]
					if !exists {
						id = m.nextKittyID
						m.nextKittyID++
						m.kittyImageIDs[localPath] = id
						cmds = append(cmds, m.uploadToKittyCmd(id, localPath))
					}
					w, h, err := components.GetImageDimensions(localPath)
					if err == nil {
						cols, rows := components.CalculateRenderSize(w, h, imageMaxWidth)
						sb.WriteString(components.RenderKittyPlacement(id, cols, rows) + "\n")
					}
				}
			}
		}
		fields = append(fields, components.DetailField{Label: "Attachments", Value: sb.String(), MultiLine: true})
	}

	// Add comments as a non-editable multiline field for now, or handle separately
	comments, err := m.repo.GetTaskComments(selected.ID, 50)
	if err == nil && len(comments) > 0 {
		var sb strings.Builder
		commentFields := make([]components.DetailField, 0, len(comments))
		for _, c := range comments {
			author := c.AuthorName
			if author == "" {
				author = "unknown"
			}
			body := strings.TrimSpace(c.BodyMD)
			sb.WriteString(fmt.Sprintf("- %s: %s\n", author, body))
			commentFields = append(commentFields, components.DetailField{
				Label: author,
				Value: body,
				MultiLine: true,
			})
		}
		
		// Only add comments to main detail panel if we are NOT in the mode that has a dedicated sidebar for them
		if m.viewMode != ViewModeFullDetailWithComments {
			fields = append(fields, components.DetailField{Label: "Comments", Value: sb.String(), MultiLine: true})
		}
		
		m.commentsSidebar.SetFields(commentFields)
	} else {
		m.commentsSidebar.SetFields([]components.DetailField{{Label: "Comments", Value: "No comments"}})
	}

	m.detailPanel.SetFields(fields)
	return tea.Batch(cmds...)
}

func isImage(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp"
}

func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
}

func (m *RootModel) startDetailRevalidate() tea.Cmd {
	taskID := m.currentSelectedTaskID()
	m.selectedTaskID = taskID
	m.displayedTaskID = taskID
	m.taskTable.SetDisplayedTaskID(taskID)
	if taskID == "" {
		m.detailLoading = false
		m.detailLoadingMsg = ""
		return m.refreshDetail(false, "")
	}

	m.detailLoading = true
	m.detailReqTaskID = taskID
	m.detailReqToken++
	token := m.detailReqToken
	m.detailLoadingMsg = "Revalidating detail..."
	m.syncDetail = "waiting for detail revalidate debounce"
	cmd := m.refreshDetail(true, m.detailLoadingMsg)

	return tea.Batch(
		cmd,
		tea.Tick(detailDebounceDelay, func(time.Time) tea.Msg {
			return detailRevalidateTickMsg{TaskID: taskID, Token: token}
		}),
	)
}

func (m *RootModel) selectCursorTaskForDisplayCmd() tea.Cmd {
	taskID := m.currentSelectedTaskID()
	if taskID == "" {
		m.displayedTaskID = ""
		m.taskTable.SetDisplayedTaskID("")
		return m.refreshDetail(false, "")
	}
	if taskID == m.displayedTaskID && !m.detailLoading {
		return m.refreshDetail(false, "")
	}
	return m.startDetailRevalidate()
}

func (m RootModel) pollTaskCmd() tea.Cmd {
	return tea.Tick(pollInterval, func(time.Time) tea.Msg {
		return pollTaskMsg{}
	})
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
	cmd := m.refreshDetail(true, m.detailLoadingMsg)

	return tea.Batch(
		cmd,
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			err := m.sync.RevalidateTask(ctx, taskID)
			return manualTaskRefreshResultMsg{TaskID: taskID, Err: err}
		},
	)
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
	m.statusLine = "Task sort: " + string(m.taskSortMode) + " (" + string(m.taskSortDirection) + ")"
}

func (m *RootModel) toggleTaskSortDirection() {
	if m.taskSortDirection == TaskSortAsc {
		m.taskSortDirection = TaskSortDesc
	} else {
		m.taskSortDirection = TaskSortAsc
	}
	m.statusLine = "Task sort: " + string(m.taskSortMode) + " (" + string(m.taskSortDirection) + ")"
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
		return tea.Batch(m.loadDataCmd(), m.fetchCurrentUserCmd())
	}
	return nil
}



func totalHeightFromModel(height int) int {
	if height > 0 {
		return height
	}
	return 24
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

func mapListsToRows(lists []cache.ListEntity, spaces []cache.SpaceEntity) []components.SidebarRow {
	spacesByID := make(map[string]cache.SpaceEntity)
	for _, s := range spaces {
		spacesByID[s.ID] = s
	}

	listsBySpace := make(map[string][]cache.ListEntity)
	for _, l := range lists {
		listsBySpace[l.SpaceID] = append(listsBySpace[l.SpaceID], l)
	}

	// Group spaces by workspace
	spacesByWorkspace := make(map[string][]cache.SpaceEntity)
	workspaceNames := make(map[string]string)
	for _, s := range spaces {
		wsID := s.WorkspaceID
		if wsID == "" {
			wsID = "unknown"
		}
		spacesByWorkspace[wsID] = append(spacesByWorkspace[wsID], s)
		if s.WorkspaceName != "" {
			workspaceNames[wsID] = s.WorkspaceName
		} else if workspaceNames[wsID] == "" {
			workspaceNames[wsID] = "Workspace: " + wsID
		}
	}

	// Sort workspaces
	wsIDs := make([]string, 0, len(spacesByWorkspace))
	for id := range spacesByWorkspace {
		wsIDs = append(wsIDs, id)
	}
	sort.Strings(wsIDs)

	rows := make([]components.SidebarRow, 0, len(lists)+len(spaces)+len(wsIDs))

	for _, wsID := range wsIDs {
		wsCollapseKey := "ws:" + wsID
		rows = append(rows, components.SidebarRow{
			Type:        components.SidebarRowGroup,
			Title:       workspaceNames[wsID],
			CollapseKey: wsCollapseKey,
		})

		wsSpaces := spacesByWorkspace[wsID]
		sort.Slice(wsSpaces, func(i, j int) bool {
			return wsSpaces[i].Name < wsSpaces[j].Name
		})

		for _, s := range wsSpaces {
			sCollapseKey := "space:" + s.ID
			rows = append(rows, components.SidebarRow{
				Type:        components.SidebarRowGroup,
				Title:       s.Name,
				Indent:      2,
				CollapseKey: sCollapseKey,
				HiddenBy:    []string{wsCollapseKey},
			})

			sLists := listsBySpace[s.ID]
			sort.Slice(sLists, func(i, j int) bool {
				return sLists[i].Name < sLists[j].Name
			})

			for _, l := range sLists {
				prefix := "  "
				if l.Favorite {
					prefix = "* "
				}
				rows = append(rows, components.SidebarRow{
					Type:     components.SidebarRowList,
					ID:       l.ID,
					Title:    prefix + l.Name,
					Indent:   4,
					HiddenBy: []string{wsCollapseKey, sCollapseKey},
				})
			}
		}
	}

	return rows
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

func organizeTasks(tasks []cache.TaskEntity, sortMode TaskSortMode, sortDir TaskSortDirection, groupMode TaskGroupMode, subtaskMode TaskSubtaskMode) []cache.TaskEntity {
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
		if sortDir == TaskSortDesc {
			return taskLess(ordered[j], ordered[i], sortMode)
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

	// High-priority fields
	fields := []string{
		task.Title,
		task.Status,
		task.PriorityLabel,
		due,
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

	// Low-priority / potentially large fields - only search if no good match yet or for long queries
	// Limit search in large fields to avoid extreme memory pressure
	if len(task.DescriptionMD) > 0 && len(task.DescriptionMD) < 5000 {
		if score, ok := fuzzyScoreField(query, task.DescriptionMD); ok {
			if !matched || score > bestScore {
				bestScore = score
			}
			matched = true
		}
	} else if len(task.DescriptionMD) >= 5000 {
		// For very large descriptions, use a simple case-insensitive check instead of full fuzzy
		if strings.Contains(strings.ToLower(task.DescriptionMD), query) {
			score := 500 // Base score for large field match
			if !matched || score > bestScore {
				bestScore = score
			}
			matched = true
		}
	}

	if len(task.CustomFieldsJSON) > 0 && len(task.CustomFieldsJSON) < 2000 {
		if score, ok := fuzzyScoreField(query, task.CustomFieldsJSON); ok {
			if !matched || score > bestScore {
				bestScore = score
			}
			matched = true
		}
	}

	return bestScore, matched
}

func fuzzyScoreField(query string, candidate string) (int, bool) {
	if query == "" {
		return 0, true
	}
	if candidate == "" {
		return 0, false
	}
	
	candidate = normalizeSearchText(candidate)
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
	if s == "" {
		return ""
	}
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

func (m *RootModel) openEditor(target EditorTarget, initialValue string, prompt string, multiline bool) (tea.Model, tea.Cmd) {
	m.editorOpen = true
	m.editorTarget = target
	m.editorContext = make(map[string]string)
	m.editorModel = components.NewTextEditor(prompt)
	m.editorModel.MultiLine = multiline
	m.editorModel.Width = max(40, m.width-20)
	m.editorModel.Value = initialValue
	m.editorModel.Cursor = len(initialValue)
	m.editorModel.Active = true
	m.editorModel.OnSubmit = func(value string) tea.Cmd {
		return m.submitEditorCmd(target, value)
	}
	m.editorModel.OnCancel = func() tea.Cmd {
		return m.closeEditorCmd()
	}
	return m, nil
}

func (m *RootModel) openConfirm(prompt string, expected string, onConfirm func() tea.Cmd) (tea.Model, tea.Cmd) {
	m.confirmOpen = true
	m.confirmModel = components.NewConfirm(prompt, expected)
	m.confirmModel.Active = true
	m.confirmModel.OnConfirm = onConfirm
	m.confirmModel.OnCancel = func() tea.Cmd {
		return m.closeConfirmCmd()
	}
	return m, nil
}

func (m RootModel) closeEditorCmd() tea.Cmd {
	return func() tea.Msg { return closeEditorMsg{} }
}

func (m RootModel) closeConfirmCmd() tea.Cmd {
	return func() tea.Msg { return closeConfirmMsg{} }
}

type closeEditorMsg struct{}
type closeConfirmMsg struct{}

func (m RootModel) submitEditorCmd(target EditorTarget, value string) tea.Cmd {
	return func() tea.Msg {
		switch target {
		case EditorTargetTaskTitle:
			row, ok := m.taskTable.Selected()
			if !ok {
				return editResultMsg{err: fmt.Errorf("no task selected")}
			}
			if err := m.repo.UpdateTaskTitle(row.ID, value); err != nil {
				return editResultMsg{err: err}
			}
			if err := m.sync.QueueTaskUpdate(row.ID, provider.TaskUpdate{Title: &value}); err != nil {
				return editResultMsg{err: err}
			}
			return editResultMsg{}

		case EditorTargetTaskCreate:
			if m.selectedListID == "" {
				return editResultMsg{err: fmt.Errorf("no list selected")}
			}
			if err := m.sync.QueueCreateTask(m.selectedListID, provider.Task{Title: value}); err != nil {
				return editResultMsg{err: err}
			}
			return editResultMsg{}

		case EditorTargetListCreate:
			spaceID := m.editorContext["spaceID"]
			if spaceID == "" {
				// Fallback to selected list's space if somehow it's missing
				list, _ := m.repo.GetListByID(m.selectedListID)
				if list != nil {
					spaceID = list.SpaceID
				}
			}
			if spaceID == "" {
				return editResultMsg{err: fmt.Errorf("could not determine space for list creation")}
			}
			if err := m.sync.QueueCreateList(spaceID, value); err != nil {
				return editResultMsg{err: err}
			}
			return editResultMsg{}

		case EditorTargetCommentCreate:
			if m.displayedTaskID == "" {
				return commentResultMsg{err: fmt.Errorf("no task opened")}
			}
			if err := m.sync.QueueCreateComment(m.displayedTaskID, value); err != nil {
				return commentResultMsg{err: err}
			}
			return commentResultMsg{}

		default:
			return editResultMsg{err: fmt.Errorf("unknown editor target: %s", target)}
		}
	}
}

func (m RootModel) submitFieldUpdateCmd(key string, value string) tea.Cmd {
	return func() tea.Msg {
		row, ok := m.displayedTaskRow()
		if !ok {
			return editResultMsg{err: fmt.Errorf("no task selected")}
		}
		
		var update provider.TaskUpdate
		switch key {
		case "title":
			if err := m.repo.UpdateTaskTitle(row.ID, value); err != nil {
				return editResultMsg{err: err}
			}
			update.Title = &value
		case "status":
			if err := m.repo.UpdateTaskStatus(row.ID, value); err != nil {
				return editResultMsg{err: err}
			}
			update.Status = &value
		case "description":
			if err := m.repo.UpdateTaskDescription(row.ID, value); err != nil {
				return editResultMsg{err: err}
			}
			update.DescriptionMD = &value
		case "priority":
			prio := ""
			switch strings.ToLower(value) {
			case "urgent":
				prio = "1"
			case "high":
				prio = "2"
			case "normal":
				prio = "3"
			case "low":
				prio = "4"
			default:
				// try numeric directly
				if _, err := strconv.Atoi(value); err == nil {
					prio = value
				}
			}
			if prio != "" {
				update.PriorityKey = &prio
			} else {
				return editResultMsg{err: fmt.Errorf("invalid priority: %s", value)}
			}
		default:
			return editResultMsg{err: fmt.Errorf("unsupported field: %s", key)}
		}

		if err := m.sync.QueueTaskUpdate(row.ID, update); err != nil {
			return editResultMsg{err: err}
		}
		return editResultMsg{}
	}
}

func (m RootModel) deleteTaskCmd(taskID string) tea.Cmd {
	return func() tea.Msg {
		if err := m.sync.QueueDeleteTask(taskID); err != nil {
			return editResultMsg{err: err}
		}
		// Optimistically remove from view or let re-load handle it?
		// Better to mark as pending delete in repo first.
		_ = m.repo.UpdateTaskSyncState(taskID, cache.SyncStatePendingDelete, "")
		return editResultMsg{}
	}
}

func (m RootModel) deleteListCmd(listID string) tea.Cmd {
	return func() tea.Msg {
		if err := m.sync.QueueDeleteList(listID); err != nil {
			return editResultMsg{err: err}
		}
		_ = m.repo.UpdateListSyncState(listID, cache.SyncStatePendingDelete, "")
		return editResultMsg{}
	}
}

func (m RootModel) hasUnsavedChanges() bool {
	if m.editorOpen && m.editorModel.Mode == components.VimModeNormal {
		return true
	}
	if m.activePane == 2 && m.detailPanel.Mode == components.ModeInsert && m.detailPanel.Editor.Mode == components.VimModeNormal {
		return true
	}
	return false
}
