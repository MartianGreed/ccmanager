package tui

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/valentindosimont/ccmanager/internal/claude"
	"github.com/valentindosimont/ccmanager/internal/config"
	"github.com/valentindosimont/ccmanager/internal/daemon"
	"github.com/valentindosimont/ccmanager/internal/game"
	"github.com/valentindosimont/ccmanager/internal/store"
	"github.com/valentindosimont/ccmanager/internal/tmux"
	"github.com/valentindosimont/ccmanager/internal/tui/messages"
	"github.com/valentindosimont/ccmanager/internal/usage"
	"github.com/valentindosimont/ccmanager/internal/workspace"
)

// Model is the main Bubbletea model
type Model struct {
	// Dependencies
	monitor *daemon.Monitor
	engine  *game.Engine
	store   *store.Store
	tmux    *tmux.Client
	config  *config.Config

	// UI state
	width       int
	height      int
	sessions    []*daemon.SessionState
	selected    int
	focused     string
	showHelp    bool
	showStats   bool
	activityLog []ActivityEntry

	// Input mode
	inputMode  bool
	renameMode bool
	inputField textinput.Model

	// Path picker mode
	pathPickerMode   bool
	pathPickerList   list.Model
	selectedPath     string
	workspaceMode    bool
	workspaceManager *workspace.Manager

	// Prompt mode
	promptMode    bool
	promptField   textarea.Model
	promptHistory []string
	historyIndex  int

	// Activity overlay
	showActivity bool

	// Usage overlay
	showUsage   bool
	globalUsage *usage.GlobalUsage

	// Game state (cached for display)
	apm            int
	streakMult     float64
	streakCount    int
	score          int
	pomodoroState  game.PomodoroState
	pomodoroRemain time.Duration

	// Error state
	lastError error

	// Quit confirmation
	confirmQuit bool

	// Group assign mode
	groupAssignMode bool

	// Delete confirmation (dd)
	deletePressed bool
	deleteTime    time.Time

	// Interactive mode for urgent sessions
	interactiveMode bool

	// Validation state
	needsValidation bool

	// Session list toggle
	sessionListHidden bool

	// Notification overlay
	showNotification bool
	msgChan          chan tea.Msg

	// Preview cache (by session name, avoids stale pointer issues)
	previewCache           map[string]string
	previewHashes          map[string]uint64
	previewScrollPos       map[string]int
	autoScroll             map[string]bool
	selectedPreviewContent string // direct capture for selected session (bypasses cache)

	// Pending urgent session to switch to after prompt sent
	pendingUrgent string

	// Workspace repo cache (session name → source repo basename)
	workspaceRepos map[string]string
}

// ActivityEntry represents a log entry
type ActivityEntry struct {
	Time    time.Time
	Session string
	Message string
}

// New creates a new TUI model
func New(monitor *daemon.Monitor, engine *game.Engine, store *store.Store, cfg *config.Config, wsMgr *workspace.Manager) *Model {
	ti := textinput.New()
	ti.Placeholder = "session-name"
	ti.CharLimit = 64

	prompt := textarea.New()
	prompt.Placeholder = "Type command and press Enter..."
	prompt.CharLimit = 4096
	prompt.SetHeight(2)
	prompt.ShowLineNumbers = false
	prompt.SetWidth(60)

	msgChan := make(chan tea.Msg, 10)

	m := &Model{
		monitor:          monitor,
		engine:           engine,
		store:            store,
		tmux:             tmux.NewClient(),
		config:           cfg,
		workspaceManager: wsMgr,
		activityLog:      make([]ActivityEntry, 0, 100),
		inputField:       ti,
		promptField:      prompt,
		needsValidation:  true,
		msgChan:          msgChan,
		previewCache:     make(map[string]string),
		previewHashes:    make(map[string]uint64),
		previewScrollPos: make(map[string]int),
		autoScroll:       make(map[string]bool),
		workspaceRepos:   make(map[string]string),
	}

	engine.Pomodoro().OnComplete(func() {
		msgChan <- messages.PomodoroCompleteMsg{Points: engine.Config().PointsPomodoroComplete}
	})

	return m
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.tickCmd(),
		m.monitorCmd(),
		m.listenForMessages(),
	)
}

func (m *Model) tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return messages.TickMsg{Time: t}
	})
}

func (m *Model) monitorCmd() tea.Cmd {
	return func() tea.Msg {
		event, ok := <-m.monitor.Events()
		if !ok {
			return nil
		}
		return messages.SessionEventMsg{Event: event}
	}
}

func (m *Model) listenForMessages() tea.Cmd {
	return func() tea.Msg {
		return <-m.msgChan
	}
}

func (m *Model) capturePreviewCmd(sessionName string, pane *tmux.Pane) tea.Cmd {
	return func() tea.Msg {
		if pane == nil {
			return messages.PreviewCaptureMsg{
				SessionName: sessionName,
				Content:     "",
				Hash:        0,
				Err:         nil,
			}
		}
		content, err := m.tmux.CapturePane(sessionName, pane.WindowIndex, pane.PaneIndex)
		hash := fnv.New64a()
		hash.Write([]byte(content))
		return messages.PreviewCaptureMsg{
			SessionName: sessionName,
			Content:     content,
			Hash:        hash.Sum64(),
			Err:         err,
		}
	}
}

// Update handles messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Always process system messages first (tick, window size, captures)
	switch msg := msg.(type) {
	case messages.TickMsg:
		if m.needsValidation {
			m.sessions = m.monitor.Sessions()
			m.validateControlGroups()
			m.needsValidation = false
		}
		m.engine.Tick()
		m.updateGameState()
		cmds = append(cmds, m.tickCmd())
		for _, sess := range m.sessions {
			cmds = append(cmds, m.capturePreviewCmd(sess.Name, sess.ClaudePane))
		}

	case messages.PreviewCaptureMsg:
		if msg.Err == nil && msg.Content != "" {
			isSelected := m.selected < len(m.sessions) && m.sessions[m.selected].Name == msg.SessionName

			m.previewHashes[msg.SessionName] = msg.Hash

			if isSelected {
				m.selectedPreviewContent = msg.Content
			}
			m.previewCache[msg.SessionName] = msg.Content

			if _, exists := m.autoScroll[msg.SessionName]; !exists {
				m.autoScroll[msg.SessionName] = true
			}
			if m.autoScroll[msg.SessionName] {
				m.previewScrollPos[msg.SessionName] = 0
			}
		}

	case messages.SessionEventMsg:
		m.handleSessionEvent(msg.Event)
		cmds = append(cmds, m.monitorCmd())

	case messages.SessionUpdateMsg:
		m.sessions = msg.Sessions

	case messages.PomodoroCompleteMsg:
		m.showNotification = true
		m.addActivity("", "Pomodoro complete! +%d points", msg.Points)
		cmds = append(cmds, m.listenForMessages())

	case messages.ErrorMsg:
		m.lastError = msg.Err

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		promptWidth := int(float64(msg.Width) * 0.6)
		if promptWidth < 40 {
			promptWidth = 40
		}
		m.promptField.SetWidth(promptWidth)
		if m.pathPickerMode {
			m.pathPickerList.SetSize(msg.Width-18, msg.Height-18)
		}
	}

	// Handle quit confirmation
	if m.confirmQuit {
		if msg, ok := msg.(tea.KeyMsg); ok {
			switch msg.String() {
			case "y":
				for _, s := range m.sessions {
					_ = m.tmux.KillSession(s.Name)
				}
				return m, tea.Quit
			case "n":
				return m, tea.Quit
			case "esc", "c":
				m.confirmQuit = false
			}
		}
		return m, tea.Batch(cmds...)
	}

	// Handle notification overlay
	if m.showNotification {
		if _, ok := msg.(tea.KeyMsg); ok {
			m.showNotification = false
		}
		return m, tea.Batch(cmds...)
	}

	// Handle path picker mode
	if m.pathPickerMode {
		if msg, ok := msg.(tea.KeyMsg); ok {
			switch msg.String() {
			case "w":
				m.workspaceMode = !m.workspaceMode
				m.pathPickerList.Title = m.pathPickerTitle()
				return m, tea.Batch(cmds...)
			case "enter":
				if item, ok := m.pathPickerList.SelectedItem().(pathItem); ok {
					m.selectedPath = item.path
				} else {
					filterValue := m.pathPickerList.FilterValue()
					if filterValue != "" {
						m.selectedPath = resolveCustomPath(filterValue)
					}
				}

				if m.selectedPath != "" && dirExists(m.selectedPath) {
					m.pathPickerMode = false
					m.inputMode = true
					m.inputField.SetValue(m.generateSessionNameFromPath(m.selectedPath))
					m.inputField.Focus()
				}
				return m, tea.Batch(cmds...)
			case "esc":
				m.pathPickerMode = false
				m.workspaceMode = false
				m.selectedPath = ""
				return m, tea.Batch(cmds...)
			}
		}
		var cmd tea.Cmd
		m.pathPickerList, cmd = m.pathPickerList.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}

	// Handle input mode
	if m.inputMode {
		if msg, ok := msg.(tea.KeyMsg); ok {
			switch msg.String() {
			case "enter":
				name := m.inputField.Value()
				if name != "" {
					path := m.selectedPath
					if path == "" {
						path, _ = os.Getwd()
					}

					if m.workspaceMode && m.workspaceManager != nil {
						wsPath, err := m.workspaceManager.CreateWorkspace(path, name)
						if err != nil {
							m.lastError = fmt.Errorf("workspace creation failed: %w", err)
							m.addActivity("", "Workspace creation failed: %v", err)
						} else {
							m.workspaceRepos[name] = filepath.Base(path)
							if m.store != nil {
								_ = m.store.SaveSessionWorkspace(name, wsPath, path)
							}
							path = wsPath
						}
					}
					m.workspaceMode = false

					if err := m.tmux.NewSession(name, path); err != nil {
						m.lastError = fmt.Errorf("failed to create session: %w", err)
						m.addActivity("", "Session creation failed: %v", err)
						m.inputMode = false
						m.inputField.Blur()
						m.selectedPath = ""
						return m, tea.Batch(cmds...)
					}
					time.Sleep(100 * time.Millisecond)
					if err := m.tmux.SendKeys(name, "claude"); err != nil {
						m.addActivity("", "Failed to send claude command: %v", err)
					}
					m.addActivity("", "Created session: %s", name)

					m.focused = name
					m.engine.SetFocusSession(name)
					_ = m.tmux.SwitchClient(name)

					if groupNum := m.engine.ControlGroups().FirstFreeGroup(); groupNum > 0 {
						m.engine.ControlGroups().Assign(groupNum, name)
						if m.store != nil {
							_ = m.store.SetControlGroup(groupNum, name)
						}
					}

					if m.store != nil {
						_ = m.store.AddRecentPath(path)
					}

					m.sessions = m.monitor.Sessions()
				}
				m.inputMode = false
				m.inputField.Blur()
				m.selectedPath = ""
				return m, tea.Batch(cmds...)
			case "esc":
				m.inputMode = false
				m.inputField.Blur()
				m.selectedPath = ""
				return m, tea.Batch(cmds...)
			}
			var cmd tea.Cmd
			m.inputField, cmd = m.inputField.Update(msg)
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}

	// Handle rename mode
	if m.renameMode {
		if msg, ok := msg.(tea.KeyMsg); ok {
			switch msg.String() {
			case "enter":
				newName := m.inputField.Value()
				if newName != "" && m.selected < len(m.sessions) {
					oldName := m.sessions[m.selected].Name
					if newName != oldName {
						if err := m.tmux.RenameSession(oldName, newName); err != nil {
							m.lastError = fmt.Errorf("failed to rename session: %w", err)
							m.addActivity(oldName, "Rename failed: %v", err)
						} else {
							m.addActivity(newName, "Renamed from %s", oldName)
							groups := m.engine.ControlGroups().GroupsForSession(oldName)
							for _, groupNum := range groups {
								m.engine.ControlGroups().Assign(groupNum, newName)
								if m.store != nil {
									_ = m.store.SetControlGroup(groupNum, newName)
								}
							}
							if m.focused == oldName {
								m.focused = newName
							}
							m.sessions = m.monitor.Sessions()
						}
					}
				}
				m.renameMode = false
				m.inputField.Blur()
				return m, tea.Batch(cmds...)
			case "esc":
				m.renameMode = false
				m.inputField.Blur()
				return m, tea.Batch(cmds...)
			}
			var cmd tea.Cmd
			m.inputField, cmd = m.inputField.Update(msg)
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}

	// Handle prompt mode
	if m.promptMode {
		if msg, ok := msg.(tea.KeyMsg); ok {
			m.engine.RecordAction(game.ActionKeypress)
			switch msg.String() {
			case "enter":
				text := m.promptField.Value()
				newlineSeq := m.config.UI.NewlineSequence
				if newlineSeq == "" {
					newlineSeq = "\\"
				}
				if strings.HasSuffix(text, newlineSeq) {
					m.promptField.SetValue(strings.TrimSuffix(text, newlineSeq))
					m.promptField.InsertString("\n")
					m.autoGrowPrompt()
					return m, tea.Batch(cmds...)
				}
				if text != "" && m.selected < len(m.sessions) {
					session := m.sessions[m.selected]
					if targetMode := m.config.UI.DefaultMode; targetMode != "" {
						if m.switchToMode(session, targetMode) {
							m.addActivity(session.Name, "Switched to %s mode", targetMode)
						}
					}
					_ = m.tmux.SendKeysToPane(session.Name, session.ClaudePane, text)
					m.addActivity(session.Name, "Sent: %s", text)
					m.addToPromptHistory(text)
				}
				m.promptMode = false
				m.promptField.Blur()
				m.promptField.SetValue("")
				m.promptField.SetHeight(2)
				m.historyIndex = -1
				if m.pendingUrgent != "" {
					m.selectByName(m.pendingUrgent)
					m.pendingUrgent = ""
				}
				return m, tea.Batch(cmds...)
			case "esc", "ctrl+c":
				text := m.promptField.Value()
				if text != "" {
					m.addToPromptHistory(text)
				}
				m.promptMode = false
				m.promptField.Blur()
				m.promptField.SetValue("")
				m.promptField.SetHeight(2)
				m.historyIndex = -1
				return m, tea.Batch(cmds...)
			case "up":
				text := m.promptField.Value()
				row := m.promptField.Line()
				if text == "" || row == 0 {
					if len(m.promptHistory) > 0 {
						if m.historyIndex < len(m.promptHistory)-1 {
							m.historyIndex++
						}
						m.promptField.SetValue(m.promptHistory[m.historyIndex])
						m.autoGrowPrompt()
						m.promptField.CursorEnd()
					}
					return m, tea.Batch(cmds...)
				}
			case "down":
				text := m.promptField.Value()
				row := m.promptField.Line()
				lineCount := strings.Count(text, "\n")
				if text == "" || row == lineCount {
					if m.historyIndex > 0 {
						m.historyIndex--
						m.promptField.SetValue(m.promptHistory[m.historyIndex])
						m.autoGrowPrompt()
						m.promptField.CursorEnd()
					} else if m.historyIndex == 0 {
						m.historyIndex = -1
						m.promptField.SetValue("")
						m.autoGrowPrompt()
					}
					return m, tea.Batch(cmds...)
				}
			case "shift+tab":
				if m.selected < len(m.sessions) {
					session := m.sessions[m.selected]
					_ = m.tmux.SendKeysToPaneRaw(session.Name, session.ClaudePane, "BTab")
					m.addActivity(session.Name, "Sent Shift+Tab (cycle mode)")
				}
				return m, tea.Batch(cmds...)
			}
			var cmd tea.Cmd
			m.promptField, cmd = m.promptField.Update(msg)
			cmds = append(cmds, cmd)
			m.autoGrowPrompt()
		}
		return m, tea.Batch(cmds...)
	}

	if msg, ok := msg.(tea.KeyMsg); ok {
		cmd := m.handleKey(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	// Reset delete pending on any other key
	if msg.String() != "d" {
		m.deletePressed = false
	}

	// Always record action for scoring
	m.engine.RecordAction(game.ActionKeypress)

	// Handle interactive mode for urgent sessions
	if m.interactiveMode && m.isInteractiveEligible() {
		return m.handleInteractiveKey(msg)
	}

	// Handle overlays first
	if m.showHelp || m.showStats || m.showActivity || m.showUsage {
		m.showHelp = false
		m.showStats = false
		m.showActivity = false
		m.showUsage = false
		return nil
	}

	// Handle group assign mode
	if m.groupAssignMode {
		m.groupAssignMode = false
		switch msg.String() {
		case "1", "2", "3", "4", "5", "6", "7", "8", "9", "0":
			if m.selected < len(m.sessions) {
				session := m.sessions[m.selected].Name
				groupNum := int(msg.String()[0] - '0')
				if groupNum == 0 {
					groupNum = 10
				}
				m.engine.ControlGroups().Assign(groupNum, session)
				m.engine.RecordAction(game.ActionGroupAssign)
				if m.store != nil {
					_ = m.store.SetControlGroup(groupNum, session)
				}
			}
		}
		return nil
	}

	switch msg.String() {
	case "q", "ctrl+c":
		m.confirmQuit = true
		return nil

	case "?":
		m.showHelp = true

	case "s":
		m.showStats = true

	case "u":
		m.showUsage = true
		go func() {
			global, _ := usage.GetGlobalUsage()
			m.globalUsage = global
		}()

	case "up", "k":
		if len(m.sessions) > 0 {
			session := m.sessions[m.selected]
			if session.State == claude.StateUrgent {
				_ = m.tmux.SendKeysToPaneRaw(session.Name, session.ClaudePane, "Up")
				return nil
			}
			m.selected = (m.selected - 1 + len(m.sessions)) % len(m.sessions)
			m.selectedPreviewContent = ""
			return m.capturePreviewCmd(m.sessions[m.selected].Name, m.sessions[m.selected].ClaudePane)
		}

	case "down", "j":
		if len(m.sessions) > 0 {
			session := m.sessions[m.selected]
			if session.State == claude.StateUrgent {
				_ = m.tmux.SendKeysToPaneRaw(session.Name, session.ClaudePane, "Down")
				return nil
			}
			m.selected = (m.selected + 1) % len(m.sessions)
			m.selectedPreviewContent = ""
			return m.capturePreviewCmd(m.sessions[m.selected].Name, m.sessions[m.selected].ClaudePane)
		}

	case "enter":
		if m.selected < len(m.sessions) {
			session := m.sessions[m.selected]
			if session.State == claude.StateUrgent {
				_ = m.tmux.SendKeysToPaneRaw(session.Name, session.ClaudePane, "Enter")
				return nil
			}
			m.focused = session.Name
			m.engine.SetFocusSession(session.Name)
			_ = m.tmux.SwitchClient(session.Name)
			m.engine.RecordAction(game.ActionSwitch)
		}

	case "tab":
		if len(m.sessions) > 0 {
			m.selected = (m.selected + 1) % len(m.sessions)
			m.selectedPreviewContent = ""
			return m.capturePreviewCmd(m.sessions[m.selected].Name, m.sessions[m.selected].ClaudePane)
		}

	case "shift+tab":
		if m.selected < len(m.sessions) {
			session := m.sessions[m.selected]
			_ = m.tmux.SendKeysToPaneRaw(session.Name, session.ClaudePane, "BTab")
			m.addActivity(session.Name, "Sent Shift+Tab (cycle mode)")
		}

	case "p":
		m.engine.Pomodoro().Toggle()
		m.engine.RecordAction(game.ActionPomodoroToggle)

	case "P":
		m.engine.Pomodoro().Stop()

	case "1", "2", "3", "4", "5", "6", "7", "8", "9", "0":
		groupNum := int(msg.String()[0] - '0')
		if groupNum == 0 {
			groupNum = 10
		}
		session, doubleTap := m.engine.ControlGroups().Cycle(groupNum)
		if session != "" {
			m.selectByName(session)
			if doubleTap {
				m.focused = session
				m.engine.SetFocusSession(session)
				_ = m.tmux.SwitchClient(session)
				m.engine.RecordAction(game.ActionSwitch)
			}
		}

	case "g":
		if m.selected < len(m.sessions) {
			m.groupAssignMode = true
		}

	case "n":
		m.pathPickerMode = true
		m.pathPickerList = m.buildPathList()
		return nil

	case "D":
		m.showActivity = true

	case "i", "/":
		if m.selected < len(m.sessions) {
			m.promptMode = true
			m.promptField.Focus()
			m.historyIndex = -1
		}

	case "x":
		if m.selected < len(m.sessions) {
			session := m.sessions[m.selected]
			_ = m.tmux.SendKeysToPaneRaw(session.Name, session.ClaudePane, "Escape")
			m.addActivity(session.Name, "Sent Escape (cancel)")
		}

	case "c":
		if m.selected < len(m.sessions) {
			session := m.sessions[m.selected]
			_ = m.tmux.SendKeysToPaneRaw(session.Name, session.ClaudePane, "C-c")
			m.addActivity(session.Name, "Sent Ctrl+C (interrupt)")
		}

	case "d":
		if m.selected < len(m.sessions) {
			now := time.Now()
			if m.deletePressed && now.Sub(m.deleteTime) < 500*time.Millisecond {
				session := m.sessions[m.selected]
				_ = m.tmux.KillSession(session.Name)
				m.addActivity(session.Name, "Session deleted")
				m.sessions = m.monitor.Sessions()
				m.deletePressed = false
			} else {
				m.deletePressed = true
				m.deleteTime = now
			}
		}

	case "ctrl+u":
		if m.selected < len(m.sessions) {
			sess := m.sessions[m.selected]
			m.previewScrollPos[sess.Name] += 10
			m.autoScroll[sess.Name] = false
		}

	case "ctrl+d":
		if m.selected < len(m.sessions) {
			sess := m.sessions[m.selected]
			pos := m.previewScrollPos[sess.Name] - 10
			if pos <= 0 {
				pos = 0
				m.autoScroll[sess.Name] = true
			}
			m.previewScrollPos[sess.Name] = pos
		}

	case "G":
		if m.selected < len(m.sessions) {
			sess := m.sessions[m.selected]
			m.previewScrollPos[sess.Name] = 0
			m.autoScroll[sess.Name] = true
		}

	case "[":
		m.sessionListHidden = !m.sessionListHidden

	case "e":
		if m.selected < len(m.sessions) {
			session := m.sessions[m.selected]
			path, err := m.tmux.GetSessionPath(session.Name)
			if err == nil && path != "" {
				editor := m.config.UI.Editor
				if editor == "" {
					editor = "nvim"
				}
				_ = m.tmux.NewWindow(session.Name, "editor", path, editor+" .")
				m.addActivity(session.Name, "Opened %s", editor)
			}
		}

	case "r":
		if m.selected < len(m.sessions) {
			m.renameMode = true
			m.inputField.SetValue(m.sessions[m.selected].Name)
			m.inputField.Focus()
		}
	}

	return nil
}

func (m *Model) generateSessionNameFromPath(dir string) string {
	base := filepath.Base(dir)
	for i := 1; ; i++ {
		name := fmt.Sprintf("%s-%d", base, i)
		exists := false
		for _, s := range m.sessions {
			if s.Name == name {
				exists = true
				break
			}
		}
		if !exists {
			return name
		}
	}
}

func (m *Model) buildPathList() list.Model {
	var items []list.Item
	seen := make(map[string]bool)

	if m.config != nil {
		for _, p := range m.config.SessionPaths {
			expanded := expandHome(p)
			children, err := listChildDirs(expanded)
			if err == nil {
				for _, child := range children {
					if !seen[child] {
						items = append(items, pathItem{path: child, source: "config"})
						seen[child] = true
					}
				}
			}
		}
	}

	if m.store != nil {
		recent, _ := m.store.GetRecentPaths(10)
		for _, p := range recent {
			if dirExists(p) && !seen[p] {
				items = append(items, pathItem{path: p, source: "recent"})
				seen[p] = true
			}
		}
	}

	cwd, _ := os.Getwd()
	if !seen[cwd] {
		items = append(items, pathItem{path: cwd, source: "cwd"})
	}

	l := list.New(items, newPathDelegate(), 0, 0)
	l.Title = m.pathPickerTitle()
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = l.Styles.Title.Foreground(colorPrimary).Bold(true)
	l.SetSize(m.width-18, m.height-18)
	return l
}

func (m *Model) pathPickerTitle() string {
	if m.workspaceMode && m.workspaceManager != nil {
		return fmt.Sprintf("Select directory [%s workspace] (w=toggle)", m.workspaceManager.Strategy())
	}
	return "Select directory (w=workspace mode)"
}

func listChildDirs(parent string) ([]string, error) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			dirs = append(dirs, filepath.Join(parent, entry.Name()))
		}
	}
	return dirs, nil
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func resolveCustomPath(input string) string {
	if strings.HasPrefix(input, "/") {
		return input
	}
	if strings.HasPrefix(input, "~") {
		return expandHome(input)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return input
	}
	return filepath.Join(cwd, input)
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (m *Model) handleSessionEvent(event daemon.Event) {
	switch event.Type {
	case daemon.EventSessionDiscovered:
		m.addActivity(event.Session, "Session discovered")
		m.sessions = m.monitor.Sessions()
		if m.store != nil {
			_ = m.store.CreateSession(event.Session)
			if _, sourceRepo, err := m.store.GetSessionWorkspace(event.Session); err == nil && sourceRepo != "" {
				m.workspaceRepos[event.Session] = filepath.Base(sourceRepo)
			}
		}

	case daemon.EventSessionClosed:
		m.addActivity(event.Session, "Session closed")
		m.sessions = m.monitor.Sessions()
		m.engine.ControlGroups().RemoveSession(event.Session)
		m.engine.RemoveSession(event.Session)
		delete(m.workspaceRepos, event.Session)
		if m.selected >= len(m.sessions) {
			m.selected = max(0, len(m.sessions)-1)
		}
		if m.store != nil {
			wsPath, sourceRepo, err := m.store.GetSessionWorkspace(event.Session)
			if err == nil && wsPath != "" && m.workspaceManager != nil {
				if delErr := m.workspaceManager.DeleteWorkspace(sourceRepo, wsPath); delErr != nil {
					m.addActivity(event.Session, "Workspace cleanup failed: %v", delErr)
				}
				_ = m.store.DeleteSessionWorkspace(event.Session)
			}
			_ = m.store.DeleteSession(event.Session)
		}

	case daemon.EventStateChanged:
		m.addActivity(event.Session, "State → %s", event.State)
		m.sessions = m.monitor.Sessions()
		isActive := event.State == claude.StateThinking || event.State == claude.StateActive
		m.engine.SetSessionActivity(event.Session, isActive)
		if m.interactiveMode && m.focused == event.Session && event.State != claude.StateUrgent {
			m.interactiveMode = false
		}
		if m.store != nil {
			_ = m.store.UpdateSessionLastSeen(event.Session)
		}

	case daemon.EventTaskCompleted:
		points := m.engine.RecordTaskComplete()
		m.addActivity(event.Session, "Task completed (+%d)", points)

	case daemon.EventUrgent:
		m.addActivity(event.Session, "⚠ URGENT: %s", event.Message)
		if m.promptMode {
			m.pendingUrgent = event.Session
		} else {
			m.selectByName(event.Session)
		}
		if m.store != nil {
			_ = m.store.UpdateSessionLastSeen(event.Session)
		}

	case daemon.EventDebug:
		m.addActivity("DEBUG", event.Message)
	}
}

func (m *Model) updateGameState() {
	m.apm = m.engine.APM()
	m.streakMult = m.engine.StreakMultiplier()
	m.streakCount = m.engine.StreakCount()
	m.score = m.engine.Score()
	m.pomodoroState = m.engine.Pomodoro().State()
	m.pomodoroRemain = m.engine.Pomodoro().Remaining()
}

func (m *Model) validateControlGroups() {
	sessionNames := make(map[string]bool)
	for _, s := range m.sessions {
		sessionNames[s.Name] = true
	}
	for groupNum, sessionName := range m.engine.ControlGroups().All() {
		if !sessionNames[sessionName] {
			m.engine.ControlGroups().Remove(groupNum, sessionName)
			if m.store != nil {
				_ = m.store.RemoveFromControlGroup(groupNum, sessionName)
			}
		}
	}
}

func (m *Model) selectByName(name string) {
	for i, s := range m.sessions {
		if s.Name == name {
			m.selected = i
			return
		}
	}
}

func (m *Model) addToPromptHistory(text string) {
	if len(m.promptHistory) > 0 && m.promptHistory[0] == text {
		return
	}
	m.promptHistory = append([]string{text}, m.promptHistory...)
	if len(m.promptHistory) > 50 {
		m.promptHistory = m.promptHistory[:50]
	}
}

func (m *Model) autoGrowPrompt() {
	text := m.promptField.Value()
	if text == "" {
		m.promptField.SetHeight(2)
		return
	}

	width := m.promptField.Width()
	if width <= 0 {
		width = 60
	}

	lines := 0
	for _, line := range strings.Split(text, "\n") {
		lineLen := len(line)
		if lineLen == 0 {
			lines++
		} else {
			lines += (lineLen + width - 1) / width
		}
	}

	maxHeight := 10
	if lines > maxHeight {
		lines = maxHeight
	}
	if lines < 2 {
		lines = 2
	}
	m.promptField.SetHeight(lines)
}

func (m *Model) addActivity(session string, format string, args ...interface{}) {
	entry := ActivityEntry{
		Time:    time.Now(),
		Session: session,
	}
	if len(args) > 0 {
		entry.Message = formatString(format, args...)
	} else {
		entry.Message = format
	}

	m.activityLog = append([]ActivityEntry{entry}, m.activityLog...)
	if len(m.activityLog) > 100 {
		m.activityLog = m.activityLog[:100]
	}
}

func formatString(format string, args ...interface{}) string {
	// Simple formatting without importing fmt in this file
	result := format
	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			result = replaceFirst(result, "%s", v)
		case int:
			result = replaceFirst(result, "%d", intToString(v))
		default:
			if stringer, ok := arg.(interface{ String() string }); ok {
				result = replaceFirst(result, "%s", stringer.String())
			}
		}
	}
	return result
}

func replaceFirst(s, old, new string) string {
	for i := 0; i <= len(s)-len(old); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + new + s[i+len(old):]
		}
	}
	return s
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + intToString(-n)
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

func (m *Model) isInteractiveEligible() bool {
	if m.focused == "" {
		return false
	}
	for _, sess := range m.sessions {
		if sess.Name == m.focused && sess.State == claude.StateUrgent {
			return true
		}
	}
	return false
}

func (m *Model) handleInteractiveKey(msg tea.KeyMsg) tea.Cmd {
	var focusedSession *daemon.SessionState
	for _, sess := range m.sessions {
		if sess.Name == m.focused {
			focusedSession = sess
			break
		}
	}
	if focusedSession == nil {
		m.interactiveMode = false
		return nil
	}

	switch msg.String() {
	case "esc", "escape":
		m.interactiveMode = false
		m.focused = ""
		return nil
	case "up", "k":
		_ = m.tmux.SendKeysToPaneRaw(focusedSession.Name, focusedSession.ClaudePane, "Up")
		return nil
	case "down", "j":
		_ = m.tmux.SendKeysToPaneRaw(focusedSession.Name, focusedSession.ClaudePane, "Down")
		return nil
	case "enter":
		_ = m.tmux.SendKeysToPaneRaw(focusedSession.Name, focusedSession.ClaudePane, "Enter")
		return nil
	case "y":
		_ = m.tmux.SendKeysToPaneRaw(focusedSession.Name, focusedSession.ClaudePane, "y")
		return nil
	case "n":
		_ = m.tmux.SendKeysToPaneRaw(focusedSession.Name, focusedSession.ClaudePane, "n")
		return nil
	case "i":
		m.promptMode = true
		m.promptField.Focus()
		m.historyIndex = -1
		return nil
	default:
		m.interactiveMode = false
	}
	return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m *Model) switchToMode(session *daemon.SessionState, targetMode string) bool {
	if targetMode == "" || session.State == claude.StateUrgent {
		return false
	}

	content := m.previewCache[session.Name]
	detector := claude.NewDetector()
	currentMode := detector.DetectMode(content)

	if strings.EqualFold(currentMode, targetMode) {
		return false
	}

	const maxCycles = 3
	for i := 0; i < maxCycles; i++ {
		_ = m.tmux.SendKeysToPaneRaw(session.Name, session.ClaudePane, "BTab")
		time.Sleep(50 * time.Millisecond)

		newContent, _ := m.tmux.CapturePaneDefault(session.Name)
		if strings.EqualFold(detector.DetectMode(newContent), targetMode) {
			time.Sleep(50 * time.Millisecond)
			return true
		}
	}
	return false
}
