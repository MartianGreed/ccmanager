package messages

import (
	"time"

	"github.com/valentindosimont/ccmanager/internal/claude"
	"github.com/valentindosimont/ccmanager/internal/daemon"
	"github.com/valentindosimont/ccmanager/internal/game"
)

// TickMsg is sent on every game tick (100ms)
type TickMsg struct {
	Time time.Time
}

// SessionUpdateMsg contains updated session information
type SessionUpdateMsg struct {
	Sessions []*daemon.SessionState
}

// SessionEventMsg wraps daemon events for the TUI
type SessionEventMsg struct {
	Event daemon.Event
}

// GameStateMsg contains updated game state
type GameStateMsg struct {
	APM            int
	StreakMult     float64
	StreakCount    int
	Score          int
	PomodoroState  game.PomodoroState
	PomodoroRemain time.Duration
}

// FocusSessionMsg requests focusing a session
type FocusSessionMsg struct {
	Session string
}

// SwitchSessionMsg requests switching to a session in tmux
type SwitchSessionMsg struct {
	Session string
}

// ControlGroupCycleMsg cycles through a control group
type ControlGroupCycleMsg struct {
	Group     int
	DoubleTap bool
}

// AssignControlGroupMsg assigns current session to a group
type AssignControlGroupMsg struct {
	Group int
}

// PomodoroToggleMsg toggles the pomodoro timer
type PomodoroToggleMsg struct{}

// PomodoroStopMsg stops the pomodoro timer
type PomodoroStopMsg struct{}

// PomodoroCompleteMsg indicates a pomodoro was completed
type PomodoroCompleteMsg struct {
	Points int
}

// StateChangeMsg indicates a session state changed
type StateChangeMsg struct {
	Session  string
	OldState claude.SessionState
	NewState claude.SessionState
}

// UrgentMsg indicates a session needs urgent attention
type UrgentMsg struct {
	Session string
	Message string
}

// ActionMsg records a user action for scoring
type ActionMsg struct {
	Type game.ActionType
}

// ErrorMsg contains an error message
type ErrorMsg struct {
	Err error
}

// QuitMsg requests application quit
type QuitMsg struct{}

// PreviewCaptureMsg contains captured tmux pane content for preview
type PreviewCaptureMsg struct {
	SessionName string
	Content     string
	Hash        uint64
	Err         error
}
