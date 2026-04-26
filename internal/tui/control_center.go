package tui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"lazy-click/internal/cache"
	"lazy-click/internal/provider"
	"lazy-click/internal/tui/components"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
)

const maxRecentCommands = 20

type controlCommand struct {
	ID          string
	Title       string
	Subtitle    string
	Badge       string
	Shortcut    string
	Aliases     []string
}

func (m *RootModel) openControlCenter(mode ControlMode) {
	m.controlOpen = true
	m.controlMode = mode
	m.controlInput = ""
	m.controlSelected = 0
	m.refreshControlResults()
}

func (m *RootModel) closeControlCenter() {
	m.controlOpen = false
	m.controlInput = ""
	m.controlResults = nil
	m.controlSelected = 0
}

func (m *RootModel) updateControlCenter(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+p", "ctrl+k":
		m.closeControlCenter()
		return nil
	}

	if m.controlMode == ControlModePAT {
		switch msg.Type {
		case tea.KeyEsc:
			m.closeControlCenter()
			return nil
		case tea.KeyEnter:
			m.closeControlCenter()
			token := strings.TrimSpace(m.patInput)
			if token == "" {
				m.statusLine = "PAT not saved"
				return nil
			}
			if err := m.repo.SaveAppState(appStateClickUpToken, token); err != nil {
				m.statusLine = "Failed to save PAT"
				return nil
			}
			if m.sync.SetProviderToken("clickup", token) {
				m.clickUpConnected = true
				m.statusLine = "ClickUp PAT saved"
				return m.loadDataCmd()
			}
			m.statusLine = "Failed to set PAT on provider"
			return nil
		case tea.KeyBackspace, tea.KeyDelete:
			if len(m.patInput) > 0 {
				m.patInput = m.patInput[:len(m.patInput)-1]
			}
			return nil
		default:
			if len(msg.Runes) > 0 {
				m.patInput += string(msg.Runes)
			}
			return nil
		}
	}

	switch msg.Type {
	case tea.KeyEsc:
		m.closeControlCenter()
		return nil
	case tea.KeyEnter:
		if len(m.controlResults) == 0 {
			m.closeControlCenter()
			return nil
		}
		selected := m.controlResults[m.controlSelected]
		m.closeControlCenter()
		return m.runControlResult(selected)
	case tea.KeyUp:
		if m.controlSelected > 0 {
			m.controlSelected--
		}
		return nil
	case tea.KeyDown:
		if m.controlSelected+1 < len(m.controlResults) {
			m.controlSelected++
		}
		return nil
	case tea.KeyBackspace, tea.KeyDelete:
		r := []rune(m.controlInput)
		if len(r) > 0 {
			m.controlInput = string(r[:len(r)-1])
		}
		m.syncControlModeByPrefix()
		m.refreshControlResults()
		return nil
	default:
		if len(msg.Runes) == 0 {
			return nil
		}
		m.controlInput += string(msg.Runes)
		m.syncControlModeByPrefix()
		m.refreshControlResults()
		return nil
	}
}

func (m *RootModel) syncControlModeByPrefix() {
	trimmed := strings.TrimSpace(m.controlInput)
	if trimmed == "" {
		return
	}
	switch string([]rune(trimmed)[0]) {
	case string(ControlModeCommand):
		m.controlMode = ControlModeCommand
		m.controlInput = strings.TrimSpace(trimmed[1:])
	case string(ControlModeList):
		m.controlMode = ControlModeList
		m.controlInput = strings.TrimSpace(trimmed[1:])
	case string(ControlModeSpace):
		m.controlMode = ControlModeSpace
		m.controlInput = strings.TrimSpace(trimmed[1:])
	case string(ControlModeTask):
		m.controlMode = ControlModeTask
		m.controlInput = strings.TrimSpace(trimmed[1:])
	case string(ControlModeComment):
		m.controlMode = ControlModeComment
		m.controlInput = strings.TrimSpace(trimmed[1:])
	case string(ControlModeHelp):
		m.controlMode = ControlModeHelp
		m.controlInput = strings.TrimSpace(trimmed[1:])
	case string(ControlModeAttachment):
		m.controlMode = ControlModeAttachment
		m.controlInput = strings.TrimSpace(trimmed[1:])
	case string(ControlModePAT):
		m.controlMode = ControlModePAT
		m.controlInput = ""
	}
}

func (m *RootModel) refreshControlResults() {
	switch m.controlMode {
	case ControlModeList:
		m.controlResults = m.buildListResults(m.controlInput)
	case ControlModeSpace:
		m.controlResults = m.buildSpaceResults(m.controlInput)
	case ControlModeTask:
		m.controlResults = m.buildTaskResults(m.controlInput)
	case ControlModeComment:
		m.controlResults = m.buildCommentResults(m.controlInput)
	case ControlModeHelp:
		m.controlResults = m.buildHelpResults(m.controlInput)
	case ControlModeAttachment:
		m.controlResults = m.buildAttachmentResults(m.controlInput)
	case ControlModePAT:
		m.controlResults = m.buildCommandResults(m.controlInput)
	default:
		m.controlResults = m.buildCommandResults(m.controlInput)
	}
	if len(m.controlResults) == 0 {
		m.controlSelected = 0
		return
	}
	if m.controlSelected >= len(m.controlResults) {
		m.controlSelected = len(m.controlResults) - 1
	}
	if m.controlSelected < 0 {
		m.controlSelected = 0
	}
}

func (m RootModel) renderControlCenter(width int) string {
	panelWidth := min(max(width-4, 44), 110)
	prefix := string(m.controlMode)
	if prefix == "" {
		prefix = ">"
	}

	if m.controlMode == ControlModePAT {
		header := fmt.Sprintf("%s %s", prefix, m.patInput)
		lines := []string{
			ControlCenterTitleStyle.Render(components.Truncate("Enter ClickUp Personal Access Token", panelWidth-2, "...")),
			"",
			header,
			"",
			HelpStyle.Render("Press Enter to save, Esc to cancel"),
		}
		body := strings.Join(lines, "\n")
		return ControlCenterPanelStyle.Width(panelWidth).Render(body)
	}

	header := fmt.Sprintf("%s %s", prefix, m.controlInput)
	lines := []string{ControlCenterTitleStyle.Render(components.Truncate(header, panelWidth-2, "..."))}

	if len(m.controlResults) == 0 {
		lines = append(lines, "  No results")
	} else {
		// Limit results to avoid overflow
		maxResults := 15
		if m.height > 0 {
			maxResults = max(m.height/2, 5)
		}
		start, end := components.VisibleWindow(len(m.controlResults), m.controlSelected, maxResults)

		subtitleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		shortcutStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)

		for i := start; i < end; i++ {
			r := m.controlResults[i]
			prefix := "  "
			style := lipgloss.NewStyle()
			if i == m.controlSelected {
				prefix = "> "
				style = ControlCenterSelectStyle
			}

			title := style.Render(r.Title)
			subtitle := ""
			if r.Subtitle != "" {
				subtitle = subtitleStyle.Render(" - " + r.Subtitle)
			}
			shortcut := ""
			if r.Shortcut != "" {
				shortcut = " " + shortcutStyle.Render("("+r.Shortcut+")")
			}

			line := prefix + title + shortcut + subtitle
			lines = append(lines, xansi.Cut(line, 0, panelWidth-2))
		}

		if len(m.controlResults) > maxResults {
			lines = append(lines, HelpStyle.Render(fmt.Sprintf("  ... %d more results", len(m.controlResults)-maxResults)))
		}
	}
	body := strings.Join(lines, "\n")
	return ControlCenterPanelStyle.Width(panelWidth).Render(body)
}

func (m *RootModel) buildCommandResults(query string) []controlResult {
	commands := m.controlCommands()
	query = strings.TrimSpace(strings.ToLower(query))
	results := make([]controlResult, 0, len(commands))
	type ranked struct {
		result controlResult
		score  int
	}
	rankedItems := make([]ranked, 0, len(commands))
	for _, cmd := range commands {
		score := m.commandMatchScore(cmd, query)
		if query != "" && score <= 0 {
			continue
		}
		rankedItems = append(rankedItems, ranked{
			result: controlResult{Kind: "command", Title: cmd.Title, Subtitle: cmd.Subtitle, Badge: cmd.Badge, Shortcut: cmd.Shortcut, CommandID: cmd.ID},
			score:  score,
		})
	}
	sort.SliceStable(rankedItems, func(i int, j int) bool {
		if rankedItems[i].score == rankedItems[j].score {
			return rankedItems[i].result.Title < rankedItems[j].result.Title
		}
		return rankedItems[i].score > rankedItems[j].score
	})
	for _, item := range rankedItems {
		results = append(results, item.result)
	}
	return results
}

func (m *RootModel) buildSpaceResults(query string) []controlResult {
	query = strings.ToLower(strings.TrimSpace(query))
	// We need spaces from cache, but they might not be in RootModel yet.
	// We added them to dataLoadedMsg so they are in m.repo.
	// For now let's assume we can fetch them from repo or add them to RootModel.
	// I added them to dataLoadedMsg, but I didn't add them to RootModel struct.
	// Let's add them to RootModel struct.
	spaces, _ := m.repo.GetSpacesByProvider(m.activeProviderID)

	results := make([]controlResult, 0, len(spaces))
	for _, s := range spaces {
		name := strings.ToLower(s.Name)
		wsName := strings.ToLower(s.WorkspaceName)
		if query != "" && !strings.Contains(name, query) && !strings.Contains(wsName, query) {
			continue
		}
		results = append(results, controlResult{
			Kind:     "space",
			Title:    s.Name,
			Subtitle: s.WorkspaceName,
			Badge:    "space",
			SpaceID:  s.ID,
		})
	}
	return results
}

func (m *RootModel) buildListResults(query string) []controlResult {
	query = strings.ToLower(strings.TrimSpace(query))
	results := make([]controlResult, 0, len(m.lists))
	for _, list := range m.lists {
		name := strings.ToLower(list.Name)
		if query != "" && !strings.Contains(name, query) {
			continue
		}
		badge := "list"
		if list.Favorite {
			badge = "favorite"
		}
		results = append(results, controlResult{Kind: "list", Title: list.Name, Subtitle: list.ID, Badge: badge, ListID: list.ID})
	}
	return results
}

func (m *RootModel) buildCommentResults(query string) []controlResult {
	if m.displayedTaskID == "" {
		return []controlResult{{Kind: "error", Title: "No task selected"}}
	}
	comments, err := m.repo.GetTaskComments(m.displayedTaskID, 100)
	if err != nil || len(comments) == 0 {
		return []controlResult{{Kind: "error", Title: "No comments found"}}
	}

	query = strings.ToLower(strings.TrimSpace(query))
	results := make([]controlResult, 0, len(comments))
	for _, c := range comments {
		if query != "" && !strings.Contains(strings.ToLower(c.BodyMD), query) && !strings.Contains(strings.ToLower(c.AuthorName), query) {
			continue
		}
		
		canDelete := c.AuthorID == m.currentUser.ID
		badge := "comment"
		if canDelete {
			badge = "yours"
		}

		results = append(results, controlResult{
			Kind:      "comment",
			Title:     c.BodyMD,
			Subtitle:  c.AuthorName + " - " + time.UnixMilli(c.CreatedAtUnix).Format("2006-01-02 15:04"),
			Badge:     badge,
			CommentID: c.ID,
		})
	}
	return results
}

func (m *RootModel) buildTaskResults(query string) []controlResult {
	query = strings.ToLower(strings.TrimSpace(query))
	results := make([]controlResult, 0, len(m.loadedTasks))
	for _, task := range m.loadedTasks {
		blob := strings.ToLower(task.Title + " " + task.Status + " " + task.DescriptionMD)
		if query != "" && !strings.Contains(blob, query) {
			continue
		}
		results = append(results, controlResult{
			Kind:      "task",
			Title:     task.Title,
			Subtitle:  task.Status,
			Badge:     "task",
			TaskID:    task.ID,
			TaskTitle: task.Title,
		})
	}
	return results
}

func (m *RootModel) buildHelpResults(query string) []controlResult {
	items := []controlResult{
		{Kind: "help", Title: "> Run commands", Subtitle: "Actions, toggles, settings", Badge: "mode", Shortcut: ">"},
		{Kind: "help", Title: "@ Search lists", Subtitle: "Jump directly to lists", Badge: "mode", Shortcut: "@"},
		{Kind: "help", Title: "# Search spaces", Subtitle: "Choose a space", Badge: "mode", Shortcut: "#"},
		{Kind: "help", Title: "/ Search tasks", Subtitle: "Open task detail quickly", Badge: "mode", Shortcut: "/"},
		{Kind: "help", Title: ", Search comments", Subtitle: "Manage your comments", Badge: "mode", Shortcut: ","},
		{Kind: "help", Title: "! Search attachments", Subtitle: "Open attachments for current task", Badge: "mode", Shortcut: "!"},
		{Kind: "help", Title: "? Help", Subtitle: "Searchable command center help", Badge: "mode", Shortcut: "?"},
		{Kind: "help", Title: "ctrl+p / ctrl+k / :", Subtitle: "Open control center", Badge: "keys", Shortcut: ":"},
		{Kind: "help", Title: "arrows", Subtitle: "Move cursor", Badge: "keys"},
		{Kind: "help", Title: "home / end", Subtitle: "Jump to top/bottom", Badge: "keys"},
		{Kind: "help", Title: "tab / shift+tab", Subtitle: "Cycle active pane", Badge: "keys"},
		{Kind: "help", Title: "q / ctrl+c", Subtitle: "Quit", Badge: "keys"},
		{Kind: "help", Title: m.keymap.CreateTask, Subtitle: "Create new task", Badge: "keys"},
		{Kind: "help", Title: m.keymap.DeleteTask, Subtitle: "Delete selected task", Badge: "keys"},
		{Kind: "help", Title: m.keymap.CreateList, Subtitle: "Create new list", Badge: "keys"},
		{Kind: "help", Title: m.keymap.DeleteList, Subtitle: "Delete selected list", Badge: "keys"},
		{Kind: "help", Title: m.keymap.Edit, Subtitle: "Edit selected task", Badge: "keys"},
		{Kind: "help", Title: m.keymap.AddComment, Subtitle: "Add comment to selected task", Badge: "keys"},
		{Kind: "help", Title: "f / F", Subtitle: "Cycle status filter", Badge: "keys"},
		{Kind: "help", Title: "r", Subtitle: "Refresh data from cache", Badge: "keys"},
		{Kind: "help", Title: "s", Subtitle: "Sync with provider", Badge: "keys"},
		{Kind: "help", Title: "R", Subtitle: "Refresh selected task from provider", Badge: "keys"},
		{Kind: "help", Title: "*", Subtitle: "Toggle favorite for selected list", Badge: "keys"},
		{Kind: "help", Title: "o", Subtitle: "Toggle list sort mode", Badge: "keys"},
		{Kind: "help", Title: "O", Subtitle: "Cycle task sort mode", Badge: "keys"},
		{Kind: "help", Title: "ctrl+o", Subtitle: "Toggle task sort direction", Badge: "keys"},
		{Kind: "help", Title: "g", Subtitle: "Cycle task group mode", Badge: "keys"},
		{Kind: "help", Title: "G", Subtitle: "Cycle subtask mode", Badge: "keys"},
		{Kind: "help", Title: "X", Subtitle: "Collapse all groups", Badge: "keys"},
		{Kind: "help", Title: "v", Subtitle: "Toggle favorites-only lists", Badge: "keys"},
		{Kind: "help", Title: "m", Subtitle: "Toggle Me Mode", Badge: "keys"},
		{Kind: "help", Title: "a", Subtitle: "Download all attachments", Badge: "keys"},
		{Kind: "help", Title: "A", Subtitle: "Download and open attachments", Badge: "keys"},
		{Kind: "help", Title: m.keymap.CopyTaskLink, Subtitle: "Copy task link to clipboard", Badge: "keys"},
		{Kind: "help", Title: m.keymap.RefreshTask, Subtitle: "Force refresh task from provider", Badge: "keys"},
		{Kind: "help", Title: m.keymap.CollapseAll, Subtitle: "Toggle collapse all groups", Badge: "keys"},
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return items
	}
	out := make([]controlResult, 0, len(items))
	for _, item := range items {
		blob := strings.ToLower(item.Title + " " + item.Subtitle)
		if strings.Contains(blob, query) {
			out = append(out, item)
		}
	}
	return out
}

func (m *RootModel) buildAttachmentResults(query string) []controlResult {
	if m.displayedTaskID == "" {
		return []controlResult{{Kind: "error", Title: "No task selected"}}
	}
	task, err := m.repo.GetTaskByID(m.displayedTaskID)
	if err != nil || task == nil || task.AttachmentsJSON == "" {
		return []controlResult{{Kind: "error", Title: "No attachments found"}}
	}

	var attachments []provider.Attachment
	if err := json.Unmarshal([]byte(task.AttachmentsJSON), &attachments); err != nil {
		return []controlResult{{Kind: "error", Title: "Failed to parse attachments"}}
	}

	query = strings.ToLower(strings.TrimSpace(query))
	results := make([]controlResult, 0, len(attachments))
	for i := range attachments {
		a := &attachments[i]
		if query != "" && !strings.Contains(strings.ToLower(a.Filename), query) {
			continue
		}
		results = append(results, controlResult{
			Kind:       "attachment",
			Title:      a.Filename,
			Subtitle:   formatSize(a.Size),
			Badge:      "file",
			Attachment: a,
		})
	}
	return results
}

func (m *RootModel) commandMatchScore(cmd controlCommand, query string) int {
	if query == "" {
		score := 100
		for i, recent := range m.recentCommands {
			if recent == cmd.ID {
				score += 60 - min(i, 10)*4
				break
			}
		}
		if usage, ok := m.commandUsage[cmd.ID]; ok {
			score += min(usage.Count, 25)
			if usage.LastUsedUnix > 0 {
				minutes := int((time.Now().UnixMilli() - usage.LastUsedUnix) / 60000)
				if minutes < 0 {
					minutes = 0
				}
				score += max(0, 25-min(minutes, 25))
			}
		}
		return score
	}
	query = strings.ToLower(query)
	name := strings.ToLower(cmd.Title)
	if name == query {
		return 1000
	}
	if strings.HasPrefix(name, query) {
		return 800
	}
	if strings.Contains(name, query) {
		return 600
	}
	for _, alias := range cmd.Aliases {
		a := strings.ToLower(alias)
		if a == query {
			return 900
		}
		if strings.HasPrefix(a, query) {
			return 700
		}
		if strings.Contains(a, query) {
			return 500
		}
	}
	return 0
}

func (m *RootModel) runControlResult(result controlResult) tea.Cmd {
	switch result.Kind {
	case "list":
		m.selectedListID = result.ListID
		m.activePane = 0
		m.statusLine = "List selected: " + result.Title
		m.persistLastOpenedList(result.ListID)
		m.persistSessionSnapshot()
		return m.loadDataCmd()
	case "space":
		// This is used for "Create List" flow
		_, cmd := m.openEditor(EditorTargetListCreate, "", "Create new List name in space '"+result.Title+"':")
		m.editorContext["spaceID"] = result.SpaceID
		return cmd
	case "comment":
		if result.CommentID != "" {
			// Actually, let's just trigger confirmation if it's "yours"
			if result.Badge == "yours" {
				_, cmd := m.openConfirm("Delete your comment: "+components.Truncate(result.Title, 40, "...")+"?", "DELETE", func() tea.Cmd {
					return func() tea.Msg {
						if err := m.sync.QueueDeleteComment(result.CommentID); err != nil {
							return editResultMsg{err: err}
						}
						return editResultMsg{}
					}
				})
				return cmd
			} else {
				m.statusLine = "You can only delete your own comments"
				return nil
			}
		}
		return nil
	case "task":
		m.displayedTaskID = result.TaskID
		m.selectedTaskID = result.TaskID
		m.taskTable.JumpToTask(result.TaskID)
		m.taskTable.SetDisplayedTaskID(result.TaskID)
		m.statusLine = "Task opened: " + result.TaskTitle
		m.persistSessionSnapshot()
		return m.loadDataCmd()
	case "attachment":
		if result.Attachment != nil {
			return m.openAttachmentCmd(*result.Attachment)
		}
		return nil
	case "command":
		return m.executeControlCommand(result.CommandID)
	default:
		return nil
	}
}

func (m *RootModel) executeControlCommand(id string) tea.Cmd {
	m.recordCommand(id)
	switch id {
	case "quit":
		if m.hasUnsavedChanges() {
			_, cmd := m.openConfirm("Changes not saved will be lost. Quit anyway?", "QUIT", func() tea.Cmd {
				m.persistSessionSnapshot()
				return tea.Quit
			})
			return cmd
		}
		m.persistSessionSnapshot()
		return tea.Quit
	case "create_task":
		if m.selectedListID != "" {
			_, cmd := m.openEditor(EditorTargetTaskCreate, "", "Create new Task title:")
			return cmd
		}
		m.statusLine = "No list selected"
		return nil
	case "delete_task":
		row, ok := m.taskTable.Selected()
		if ok && row.ID != "" {
			_, cmd := m.openConfirm("Delete task: "+row.Title+"?", "DELETE", func() tea.Cmd {
				return m.deleteTaskCmd(row.ID)
			})
			return cmd
		}
		m.statusLine = "No task selected"
		return nil
	case "edit_task":
		row, ok := m.taskTable.Selected()
		if ok && row.ID != "" {
			_, cmd := m.openEditor(EditorTargetTaskTitle, row.Title, "Edit Task title:")
			return cmd
		}
		m.statusLine = "No task selected"
		return nil
	case "create_list":
		m.openControlCenter(ControlModeSpace)
		return nil
	case "delete_list":
		if m.selectedListID != "" {
			list, _ := m.repo.GetListByID(m.selectedListID)
			if list != nil {
				_, cmd := m.openConfirm("Delete list: "+list.Name+"?", "DELETE", func() tea.Cmd {
					return m.deleteListCmd(list.ID)
				})
				return cmd
			}
		}
		m.statusLine = "No list selected"
		return nil
	case "add_comment":
		row, ok := m.taskTable.Selected()
		if ok && row.ID != "" {
			_, cmd := m.openEditor(EditorTargetCommentCreate, "", "Add Comment:")
			return cmd
		}
		m.statusLine = "No task selected"
		return nil
	case "delete_comment":
		if m.displayedTaskID != "" {
			m.openControlCenter(ControlModeComment)
			return nil
		}
		m.statusLine = "No task opened"
		return nil
	case "refresh":
		m.statusLine = "Refreshing data"
		m.persistSessionSnapshot()
		return m.loadDataCmd()
	case "sync_now":
		if !m.syncing {
			m.syncing = true
			m.syncFrame = 0
			m.statusLine = "Sync in progress..."
			m.persistSessionSnapshot()
			return tea.Batch(m.syncNowCmd(), m.syncTickCmd())
		}
		return nil
	case "toggle_favorites_only":
		m.favoritesOnly = !m.favoritesOnly
		m.statusLine = fmt.Sprintf("Favorites-only: %t", m.favoritesOnly)
		m.saveListPrefs()
		m.persistSessionSnapshot()
		return m.loadDataCmd()
	case "toggle_me_mode":
		m.meMode = !m.meMode
		m.statusLine = fmt.Sprintf("Me Mode: %t", m.meMode)
		m.saveTaskPrefs()
		m.persistSessionSnapshot()
		return m.loadDataCmd()
	case "toggle_list_sort":
		if m.listSortMode == cache.ListSortMostRecentlyOpen {
			m.listSortMode = cache.ListSortNameAsc
		} else {
			m.listSortMode = cache.ListSortMostRecentlyOpen
		}
		m.statusLine = "List sort: " + string(m.listSortMode)
		m.saveListPrefs()
		m.persistSessionSnapshot()
		return m.loadDataCmd()
	case "cycle_task_sort":
		m.cycleTaskSort(1)
		m.saveTaskPrefs()
		m.persistSessionSnapshot()
		return m.loadDataCmd()
	case "toggle_task_sort_direction":
		m.toggleTaskSortDirection()
		m.saveTaskPrefs()
		m.persistSessionSnapshot()
		return m.loadDataCmd()
	case "cycle_task_group":
		m.cycleTaskGroup(1)
		m.saveTaskPrefs()
		m.persistSessionSnapshot()
		return m.loadDataCmd()
	case "cycle_subtasks":
		m.cycleTaskSubtasks(1)
		m.saveTaskPrefs()
		m.persistSessionSnapshot()
		return m.loadDataCmd()
	case "open_attachments":
		m.openControlCenter(ControlModeAttachment)
		return nil
	case "clear_task_search":
		m.searchQuery = ""
		m.statusLine = "Task search cleared"
		m.saveTaskPrefs()
		m.persistSessionSnapshot()
		return m.loadDataCmd()
	case "toggle_selected_favorite":
		if err := m.repo.ToggleListFavorite(m.selectedListID); err != nil {
			m.statusLine = "Favorite toggle failed: " + err.Error()
			return nil
		}
		m.statusLine = "List favorite updated"
		m.persistSessionSnapshot()
		return m.loadDataCmd()
	case "restore_now":
		if m.hasRestoreState {
			m.applySessionSnapshot(m.restoreSnapshot)
			m.statusLine = "Session restored"
			m.persistSessionSnapshot()
			return m.loadDataCmd()
		}
		m.statusLine = "No session snapshot available"
		return nil
	case "start_fresh":
		m.clearSessionSnapshot()
		m.statusLine = "Session snapshot cleared"
		return nil
	case "toggle_kitty_graphics":
		m.kittyGraphicsEnabled = !m.kittyGraphicsEnabled
		m.statusLine = fmt.Sprintf("Kitty Graphics: %t", m.kittyGraphicsEnabled)
		m.saveTaskPrefs()
		m.persistSessionSnapshot()
		return m.refreshDetail(m.detailLoading, m.detailLoadingMsg)
	case "provider_next":
		return m.switchToNextProviderCmd()
	case "set_clickup_pat":
		m.openControlCenter(ControlModePAT)
		return nil
	default:
		if strings.HasPrefix(id, "provider_switch:") {
			providerID := strings.TrimPrefix(id, "provider_switch:")
			if m.switchProvider(providerID) {
				return tea.Batch(m.loadDataCmd(), m.fetchCurrentUserCmd())
			}
			return nil
		}
		return nil
	}
}

func (m *RootModel) controlCommands() []controlCommand {
	commands := []controlCommand{
		{ID: "quit", Title: "Quit app", Subtitle: "Exit lazy-click", Badge: "system", Aliases: []string{"quit", "exit", "q"}, Shortcut: "q"},
		{ID: "create_task", Title: "Create Task", Subtitle: "Create a new task in the current list", Badge: "task", Aliases: []string{"new task", "create task", "add task"}, Shortcut: m.keymap.CreateTask},
		{ID: "delete_task", Title: "Delete Task", Subtitle: "Delete the selected task", Badge: "task", Aliases: []string{"remove task", "delete task", "rm task"}, Shortcut: m.keymap.DeleteTask},
		{ID: "edit_task", Title: "Edit Task Title", Subtitle: "Rename the selected task", Badge: "task", Aliases: []string{"rename task", "edit task"}, Shortcut: m.keymap.Edit},
		{ID: "create_list", Title: "Create List", Subtitle: "Create a new list in the current space", Badge: "list", Aliases: []string{"new list", "create list", "add list"}, Shortcut: m.keymap.CreateList},
		{ID: "delete_list", Title: "Delete List", Subtitle: "Delete the selected list", Badge: "list", Aliases: []string{"remove list", "delete list", "rm list"}, Shortcut: m.keymap.DeleteList},
		{ID: "add_comment", Title: "Add Comment", Subtitle: "Post a comment to the selected task", Badge: "comment", Aliases: []string{"post comment", "add comment", "new comment"}, Shortcut: m.keymap.AddComment},
		{ID: "delete_comment", Title: "Delete Comment...", Subtitle: "Choose one of your comments to delete", Badge: "comment", Aliases: []string{"remove comment", "delete comment", "rm comment"}},
		{ID: "refresh", Title: "Refresh data", Subtitle: "Reload lists and tasks from cache", Badge: "core", Aliases: []string{"refresh", "reload"}, Shortcut: "r"},
		{ID: "sync_now", Title: "Sync now", Subtitle: "Run immediate provider sync", Badge: "sync", Aliases: []string{"sync", "s"}, Shortcut: "s"},
		{ID: "provider_next", Title: "Switch provider (next)", Subtitle: "Cycle active provider", Badge: "provider", Aliases: []string{"provider", "next provider"}},
		{ID: "set_clickup_pat", Title: "Set ClickUp Personal Access Token", Subtitle: "Set and save your ClickUp PAT", Badge: "provider", Aliases: []string{"clickup pat", "set clickup token"}},
		{ID: "toggle_me_mode", Title: "Toggle Me Mode", Subtitle: "Filter tasks by current user", Badge: "toggle", Aliases: []string{"me mode", "only me"}, Shortcut: m.keymap.MeMode},
		{ID: "toggle_favorites_only", Title: "Toggle favorites-only", Subtitle: "Filter sidebar lists by favorite", Badge: "toggle", Aliases: []string{"fav only", "favorites"}, Shortcut: m.keymap.FavOnly},
		{ID: "toggle_list_sort", Title: "Toggle list sort", Subtitle: "Switch name/recent sorting", Badge: "toggle", Aliases: []string{"list sort", "sort lists"}, Shortcut: m.keymap.SortLists},
		{ID: "cycle_task_sort", Title: "Cycle task sort", Subtitle: "Rotate current task sort mode", Badge: "toggle", Aliases: []string{"sort tasks", "task sort"}, Shortcut: m.keymap.SortTasks},
		{ID: "toggle_task_sort_direction", Title: "Toggle task sort direction", Subtitle: "Switch asc/desc task sorting", Badge: "toggle", Aliases: []string{"sort direction", "task sort direction"}, Shortcut: m.keymap.SortDirection},
		{ID: "cycle_task_group", Title: "Cycle task group", Subtitle: "Rotate current task grouping", Badge: "toggle", Aliases: []string{"group tasks", "grp"}, Shortcut: m.keymap.GroupTasks},
		{ID: "cycle_subtasks", Title: "Cycle subtask mode", Subtitle: "Flat/grouped subtasks", Badge: "toggle", Aliases: []string{"subtasks", "subtask"}, Shortcut: m.keymap.Subtasks},
		{ID: "open_attachments", Title: "Open attachments...", Subtitle: "Choose an attachment to open", Badge: "file", Aliases: []string{"attachments", "files"}, Shortcut: "A"},
		{ID: "clear_task_search", Title: "Clear task search", Subtitle: "Remove active task search query", Badge: "search", Aliases: []string{"clear search", "search off"}},
		{ID: "toggle_selected_favorite", Title: "Toggle selected list favorite", Subtitle: "Mark/unmark selected list", Badge: "list", Aliases: []string{"favorite", "fav"}, Shortcut: m.keymap.Favorite},
		{ID: "restore_now", Title: "Restore last session now", Subtitle: "Apply last saved snapshot", Badge: "restore", Aliases: []string{"restore now"}},
		{ID: "start_fresh", Title: "Start fresh now", Subtitle: "Clear saved session snapshot", Badge: "restore", Aliases: []string{"fresh", "clear session"}},
		{ID: "toggle_kitty_graphics", Title: "Toggle Kitty Graphics", Subtitle: "Enable/disable terminal image display (Kitty only)", Badge: "experimental", Aliases: []string{"kitty", "images", "graphics"}},
	}
	for _, p := range m.availableProviders {
		title := "Use provider: " + p.DisplayName
		subtitle := "Activate provider " + p.ID
		badge := "provider"
		if p.ID == m.activeProviderID {
			badge = "active"
		}
		commands = append(commands, controlCommand{
			ID:       "provider_switch:" + p.ID,
			Title:    title,
			Subtitle: subtitle,
			Badge:    badge,
			Aliases:  []string{p.ID, p.DisplayName, "switch provider " + p.ID},
		})
	}
	return commands
}

func (m *RootModel) recordCommand(commandID string) {
	if commandID == "" {
		return
	}
	now := time.Now().UnixMilli()
	stat := m.commandUsage[commandID]
	stat.Count++
	stat.LastUsedUnix = now
	m.commandUsage[commandID] = stat

	nextRecent := []string{commandID}
	for _, existing := range m.recentCommands {
		if existing == commandID {
			continue
		}
		nextRecent = append(nextRecent, existing)
		if len(nextRecent) >= maxRecentCommands {
			break
		}
	}
	m.recentCommands = nextRecent
	m.saveCommandStats()
}

func (m *RootModel) saveTaskPrefs() {
	if m.repo == nil {
		return
	}
	_ = m.repo.SaveAppState(appStateTaskSortMode, string(m.taskSortMode))
	_ = m.repo.SaveAppState(appStateTaskSortDirection, string(m.taskSortDirection))
	_ = m.repo.SaveAppState(appStateTaskGroupMode, string(m.taskGroupMode))
	_ = m.repo.SaveAppState(appStateTaskSubtasksMode, string(m.taskSubtasks))
	_ = m.repo.SaveAppState(appStateStatusFilter, m.statusFilter)
	_ = m.repo.SaveAppState(appStateTaskSearchQuery, m.searchQuery)
	_ = m.repo.SaveAppState(appStateMeMode, fmt.Sprintf("%t", m.meMode))
	_ = m.repo.SaveAppState(appStateKittyGraphicsEnabled, fmt.Sprintf("%t", m.kittyGraphicsEnabled))
}

func (m *RootModel) saveCommandStats() {
	if m.repo == nil {
		return
	}
	if data, err := json.Marshal(m.recentCommands); err == nil {
		_ = m.repo.SaveAppState(appStateRecentCommands, string(data))
	}
	if data, err := json.Marshal(m.commandUsage); err == nil {
		_ = m.repo.SaveAppState(appStateCommandUsage, string(data))
	}
}

func (m *RootModel) applySessionSnapshot(snapshot uiSessionSnapshot) {
	m.selectedListID = snapshot.SelectedListID
	m.displayedTaskID = snapshot.DisplayedTaskID
	m.selectedTaskID = snapshot.DisplayedTaskID
	m.activePane = snapshot.ActivePane
	m.listSearchQuery = snapshot.ListSearchQuery
	if snapshot.ListSortMode != "" {
		m.listSortMode = snapshot.ListSortMode
	}
	m.favoritesOnly = snapshot.FavoritesOnly
	if snapshot.TaskSortMode != "" {
		m.taskSortMode = snapshot.TaskSortMode
	}
	if snapshot.TaskSortDirection != "" {
		m.taskSortDirection = snapshot.TaskSortDirection
	}
	if snapshot.TaskGroupMode != "" {
		m.taskGroupMode = snapshot.TaskGroupMode
	}
	if snapshot.TaskSubtasks != "" {
		m.taskSubtasks = snapshot.TaskSubtasks
	}
	m.statusFilter = snapshot.StatusFilter
	m.searchQuery = snapshot.TaskSearchQuery
	m.meMode = snapshot.MeMode
	if m.activePane < 0 || m.activePane > 2 {
		m.activePane = 0
	}
}

func (m *RootModel) currentSnapshot() uiSessionSnapshot {
	return uiSessionSnapshot{
		SelectedListID:  m.selectedListID,
		DisplayedTaskID: m.displayedTaskID,
		ActivePane:      m.activePane,
		ListSearchQuery: m.listSearchQuery,
		ListSortMode:    m.listSortMode,
		FavoritesOnly:   m.favoritesOnly,
		TaskSortMode:    m.taskSortMode,
		TaskSortDirection: m.taskSortDirection,
		TaskGroupMode:   m.taskGroupMode,
		TaskSubtasks:    m.taskSubtasks,
		StatusFilter:    m.statusFilter,
		TaskSearchQuery: m.searchQuery,
		MeMode:          m.meMode,
	}
}

func (m *RootModel) persistSessionSnapshot() {
	if m.repo == nil {
		return
	}
	m.saveListPrefs()
	m.saveTaskPrefs()
	m.saveCommandStats()

	snapshot := m.currentSnapshot()
	if data, err := json.Marshal(snapshot); err == nil {
		_ = m.repo.SaveAppState(appStateRestoreSession, string(data))
		m.restoreSnapshot = snapshot
		m.hasRestoreState = true
	}
}

func (m *RootModel) clearSessionSnapshot() {
	if m.repo != nil {
		_ = m.repo.SaveAppState(appStateRestoreSession, "")
	}
	m.restoreSnapshot = uiSessionSnapshot{}
	m.hasRestoreState = false
}
