package daemon

import (
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/valentindosimont/ccmanager/internal/claude"
	"github.com/valentindosimont/ccmanager/internal/store"
	"github.com/valentindosimont/ccmanager/internal/tmux"
	"github.com/valentindosimont/ccmanager/internal/usage"
)

// SessionState represents the monitored state of a Claude session
type SessionState struct {
	Name            string
	State           claude.SessionState
	LastContent     string
	LastCapture     time.Time
	Tokens          int
	ThinkingTime    time.Duration
	LastLine        string
	Created         time.Time
	Attached        bool
	ClaudePane      *tmux.Pane
	WorkingDir      string
	Usage           *usage.SessionUsage
	ClaudeSessionID string // Locked Claude session UUID for usage tracking
}

// Event represents a session event
type Event struct {
	Type    EventType
	Session string
	State   claude.SessionState
	Time    time.Time
	Message string
}

// EventType represents the type of event
type EventType int

const (
	EventSessionDiscovered EventType = iota
	EventSessionClosed
	EventStateChanged
	EventTaskCompleted
	EventUrgent
	EventDebug
)

// Monitor polls tmux sessions and detects Claude state
type Monitor struct {
	tmux     *tmux.Client
	detector *claude.Detector
	store    *store.Store

	mu       sync.RWMutex
	sessions map[string]*SessionState

	pollInterval  time.Duration
	stopCh        chan struct{}
	eventCh       chan Event
	debug         bool
	usageWatcher  *usage.Watcher
	usagePollTick int
}

// NewMonitor creates a new session monitor
func NewMonitor(pollInterval time.Duration, st *store.Store) *Monitor {
	return &Monitor{
		tmux:         tmux.NewClient(),
		detector:     claude.NewDetector(),
		store:        st,
		sessions:     make(map[string]*SessionState),
		pollInterval: pollInterval,
		stopCh:       make(chan struct{}),
		eventCh:      make(chan Event, 100),
		debug:        os.Getenv("CCMANAGER_DEBUG") == "1",
		usageWatcher: usage.NewWatcher(5 * time.Second),
	}
}

func (m *Monitor) debugLog(format string, args ...interface{}) {
	if !m.debug {
		return
	}
	m.eventCh <- Event{
		Type:    EventDebug,
		Message: fmt.Sprintf(format, args...),
		Time:    time.Now(),
	}
}

// Events returns the event channel
func (m *Monitor) Events() <-chan Event {
	return m.eventCh
}

// Start starts the monitor polling loop
func (m *Monitor) Start() {
	m.usageWatcher.Start()
	go m.pollLoop()
}

// Stop stops the monitor
func (m *Monitor) Stop() {
	close(m.stopCh)
	m.usageWatcher.Stop()
}

// Sessions returns all currently known sessions
func (m *Monitor) Sessions() []*SessionState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SessionState, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Created.Before(result[j].Created)
	})
	return result
}

// GetSession returns a specific session
func (m *Monitor) GetSession(name string) *SessionState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[name]
}

func (m *Monitor) pollLoop() {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	// Initial poll
	m.poll()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.poll()
			// Update usage every 5 poll cycles (roughly every 2.5s with 500ms poll)
			m.usagePollTick++
			if m.usagePollTick >= 5 {
				m.usagePollTick = 0
				m.updateUsage()
			}
		}
	}
}

func (m *Monitor) updateUsage() {
	m.mu.RLock()
	type sessionInfo struct {
		workingDir      string
		claudeSessionID string
	}
	sessions := make(map[string]sessionInfo)
	for name, sess := range m.sessions {
		if sess.WorkingDir != "" {
			sessions[name] = sessionInfo{
				workingDir:      sess.WorkingDir,
				claudeSessionID: sess.ClaudeSessionID,
			}
		}
	}
	m.mu.RUnlock()

	for name, info := range sessions {
		// Use the locked session ID instead of finding most recent
		sessionUsage, err := usage.GetSessionByID(info.workingDir, info.claudeSessionID)
		if err != nil || sessionUsage == nil {
			continue
		}

		m.mu.Lock()
		if sess, ok := m.sessions[name]; ok {
			sess.Usage = sessionUsage
		}
		m.mu.Unlock()
	}
}

func (m *Monitor) findClaudePane(session string) (*tmux.Pane, string) {
	panes, err := m.tmux.ListPanes(session)
	if err != nil {
		m.debugLog("%s: ListPanes error: %v", session, err)
		return nil, ""
	}

	for _, p := range panes {
		content, err := m.tmux.CapturePane(session, p.WindowIndex, p.PaneIndex)
		if err != nil {
			continue
		}
		if m.detector.IsClaudeSession(content) {
			pane := p
			return &pane, content
		}
	}
	return nil, ""
}

func (m *Monitor) poll() {
	if !m.tmux.IsRunning() {
		m.debugLog("tmux not running")
		return
	}

	tmuxSessions, err := m.tmux.ListSessions()
	if err != nil {
		m.debugLog("ListSessions error: %v", err)
		return
	}

	m.debugLog("Poll: %d tmux sessions", len(tmuxSessions))

	now := time.Now()
	seen := make(map[string]bool)

	for _, ts := range tmuxSessions {
		seen[ts.Name] = true

		m.mu.RLock()
		existing, exists := m.sessions[ts.Name]
		var knownPane *tmux.Pane
		if exists {
			knownPane = existing.ClaudePane
		}
		m.mu.RUnlock()

		var content string
		var claudePane *tmux.Pane

		if knownPane != nil {
			content, err = m.tmux.CapturePane(ts.Name, knownPane.WindowIndex, knownPane.PaneIndex)
			if err == nil && m.detector.IsClaudeSession(content) {
				claudePane = knownPane
			} else {
				claudePane, content = m.findClaudePane(ts.Name)
			}
		} else {
			claudePane, content = m.findClaudePane(ts.Name)
		}

		if claudePane == nil {
			content, err = m.tmux.CapturePaneDefault(ts.Name)
			if err != nil {
				m.debugLog("%s: capture error: %v", ts.Name, err)
				continue
			}
			if !m.detector.IsClaudeSession(content) {
				continue
			}
		}

		m.debugLog("%s: Claude=true pane=%v", ts.Name, claudePane)

		m.mu.Lock()
		existing, exists = m.sessions[ts.Name]

		if !exists {
			state := m.detector.DetectState(content, "", time.Time{})
			info := m.detector.ParseInfo(content)

			// Get working directory for usage tracking
			workingDir, _ := m.tmux.GetSessionPath(ts.Name)

			// Find and lock the Claude session ID for this tmux session
			// Try to load persisted ID first, so usage survives restarts
			var claudeSessionID string
			var initialUsage *usage.SessionUsage
			if m.store != nil {
				claudeSessionID, _ = m.store.GetClaudeSessionID(ts.Name)
			}
			if claudeSessionID == "" && workingDir != "" {
				claudeSessionID, _ = usage.FindActiveSessionID(workingDir)
				// Persist it for next restart
				if m.store != nil && claudeSessionID != "" {
					_ = m.store.CreateSession(ts.Name)
					_ = m.store.SetClaudeSessionID(ts.Name, claudeSessionID)
				}
			}
			if claudeSessionID != "" && workingDir != "" {
				initialUsage, _ = usage.GetSessionByID(workingDir, claudeSessionID)
			}

			m.sessions[ts.Name] = &SessionState{
				Name:            ts.Name,
				State:           state,
				LastContent:     content,
				LastCapture:     now,
				Tokens:          info.Tokens,
				ThinkingTime:    info.ThinkingTime,
				LastLine:        info.LastLine,
				Created:         ts.Created,
				Attached:        ts.Attached,
				ClaudePane:      claudePane,
				WorkingDir:      workingDir,
				Usage:           initialUsage,
				ClaudeSessionID: claudeSessionID,
			}

			// Start watching for usage updates with the locked session ID
			if workingDir != "" {
				m.usageWatcher.WatchSession(ts.Name, workingDir, claudeSessionID)
			}
			m.mu.Unlock()

			m.eventCh <- Event{
				Type:    EventSessionDiscovered,
				Session: ts.Name,
				State:   state,
				Time:    now,
			}
		} else {
			oldState := existing.State
			newState := m.detector.DetectState(content, existing.LastContent, existing.LastCapture)
			info := m.detector.ParseInfo(content)

			existing.State = newState
			existing.LastContent = content
			existing.LastCapture = now
			existing.Tokens = info.Tokens
			existing.ThinkingTime = info.ThinkingTime
			existing.LastLine = info.LastLine
			existing.Attached = ts.Attached
			existing.ClaudePane = claudePane

			m.mu.Unlock()

			if oldState != newState {
				m.eventCh <- Event{
					Type:    EventStateChanged,
					Session: ts.Name,
					State:   newState,
					Time:    now,
				}

				if oldState == claude.StateThinking && (newState == claude.StateIdle || newState == claude.StateActive) {
					m.eventCh <- Event{
						Type:    EventTaskCompleted,
						Session: ts.Name,
						State:   newState,
						Time:    now,
					}
				}

				if newState == claude.StateUrgent {
					m.eventCh <- Event{
						Type:    EventUrgent,
						Session: ts.Name,
						State:   newState,
						Time:    now,
						Message: info.LastLine,
					}
				}
			}
		}
	}

	m.mu.Lock()
	for name := range m.sessions {
		if !seen[name] {
			m.usageWatcher.UnwatchSession(name)
			delete(m.sessions, name)
			m.eventCh <- Event{
				Type:    EventSessionClosed,
				Session: name,
				Time:    now,
			}
		}
	}
	m.mu.Unlock()
}
