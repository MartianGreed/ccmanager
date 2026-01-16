package game

import (
	"sync"
	"time"
)

type EngineConfig struct {
	APMWindowSeconds           int
	StreakTimeoutSeconds       int
	StreakMultiplierCap        float64
	PomodoroWorkMinutes        int
	PomodoroShortBreakMinutes  int
	PomodoroLongBreakMinutes   int
	PomodorosBeforeLongBreak   int
	PomodoroMultiplier         float64
	FocusBonusMinutes          int
	FocusBonusMultiplier       float64
	PointsAction               int
	PointsTaskComplete         int
	PointsUrgentHandled        int
	PointsPomodoroComplete     int
	DoubleTapThresholdMs       int
}

func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		APMWindowSeconds:          60,
		StreakTimeoutSeconds:      30,
		StreakMultiplierCap:       10.0,
		PomodoroWorkMinutes:       25,
		PomodoroShortBreakMinutes: 5,
		PomodoroLongBreakMinutes:  15,
		PomodorosBeforeLongBreak:  4,
		PomodoroMultiplier:        1.5,
		FocusBonusMinutes:         5,
		FocusBonusMultiplier:      1.2,
		PointsAction:              10,
		PointsTaskComplete:        100,
		PointsUrgentHandled:       500,
		PointsPomodoroComplete:    1000,
		DoubleTapThresholdMs:      300,
	}
}

// ActionType represents the type of user action
type ActionType int

const (
	ActionKeypress ActionType = iota
	ActionSwitch
	ActionGroupAssign
	ActionPomodoroToggle
	ActionTaskComplete
	ActionUrgentHandled
)

// Engine manages all game state
type Engine struct {
	mu     sync.RWMutex
	config EngineConfig

	// Core state
	dailyScore    int
	totalScore    int
	lastScoreDate string
	apm           *APMTracker
	streak        *StreakTracker
	pomodoro      *PomodoroTimer
	controlGrps   *ControlGroups

	// Focus tracking
	focusSession string
	focusStart   time.Time

	// Callbacks
	onScoreChange    func(score int)
	onStreakChange   func(multiplier float64)
	onPomodoroChange func(state PomodoroState, remaining time.Duration)
}

// NewEngine creates a new game engine
func NewEngine(cfg EngineConfig) *Engine {
	return &Engine{
		config:      cfg,
		apm:         NewAPMTracker(cfg.APMWindowSeconds),
		streak:      NewStreakTracker(cfg.StreakTimeoutSeconds, cfg.StreakMultiplierCap),
		pomodoro:    NewPomodoroTimer(cfg.PomodoroWorkMinutes, cfg.PomodoroShortBreakMinutes, cfg.PomodoroLongBreakMinutes, cfg.PomodorosBeforeLongBreak),
		controlGrps: NewControlGroups(cfg.DoubleTapThresholdMs),
	}
}

func (e *Engine) Config() EngineConfig {
	return e.config
}

func (e *Engine) checkDailyReset() {
	today := time.Now().Format("2006-01-02")
	if e.lastScoreDate != today {
		e.dailyScore = 0
		e.lastScoreDate = today
	}
}

func (e *Engine) calculateMultiplier() float64 {
	streakMult := e.streak.Multiplier()
	pomodoroMult := 1.0
	if e.pomodoro.IsActive() {
		pomodoroMult = e.config.PomodoroMultiplier
	}
	focusMult := 1.0
	if e.focusSession != "" && time.Since(e.focusStart) >= time.Duration(e.config.FocusBonusMinutes)*time.Minute {
		focusMult = e.config.FocusBonusMultiplier
	}
	return streakMult * pomodoroMult * focusMult
}

func (e *Engine) RecordTaskComplete() int {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.checkDailyReset()

	basePoints := e.config.PointsTaskComplete
	mult := e.calculateMultiplier()
	points := int(float64(basePoints) * mult)

	e.dailyScore += points
	e.totalScore += points
	return points
}

// RecordAction records a user action and updates game state
func (e *Engine) RecordAction(actionType ActionType) int {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()

	// Update APM
	e.apm.RecordAction(now)

	return 0
}

func (e *Engine) getBasePoints(actionType ActionType) int {
	switch actionType {
	case ActionTaskComplete:
		return e.config.PointsTaskComplete
	case ActionUrgentHandled:
		return e.config.PointsUrgentHandled
	default:
		return e.config.PointsAction
	}
}

// SetFocusSession updates the focused session
func (e *Engine) SetFocusSession(session string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.focusSession != session {
		e.focusSession = session
		e.focusStart = time.Now()
	}
}

// Score returns the current daily score
func (e *Engine) Score() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.checkDailyReset()
	return e.dailyScore
}

// TotalScore returns the total score
func (e *Engine) TotalScore() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.totalScore
}

// APM returns the current APM
func (e *Engine) APM() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.apm.Current()
}

// StreakMultiplier returns the current streak multiplier
func (e *Engine) StreakMultiplier() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.streak.Multiplier()
}

// StreakCount returns the current streak count
func (e *Engine) StreakCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.streak.Count()
}

// SetSessionActivity updates a session's activity state
func (e *Engine) SetSessionActivity(name string, isActive bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.streak.SetSessionActive(name, isActive)
}

// RemoveSession removes a session from activity tracking
func (e *Engine) RemoveSession(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.streak.SetSessionActive(name, false)
}

// Pomodoro returns the pomodoro timer
func (e *Engine) Pomodoro() *PomodoroTimer {
	return e.pomodoro
}

// ControlGroups returns the control groups manager
func (e *Engine) ControlGroups() *ControlGroups {
	return e.controlGrps
}

// Tick updates time-based state (call every 100ms)
func (e *Engine) Tick() {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	e.apm.Tick(now)
	e.pomodoro.Tick(now)
}

// LoadState loads game state from persistence
func (e *Engine) LoadState(score int, lastScoreDate string, pomodoroState string, pomodoroRemaining int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.dailyScore = score
	e.totalScore = score
	e.lastScoreDate = lastScoreDate
	e.pomodoro.SetState(pomodoroState, time.Duration(pomodoroRemaining)*time.Second)
}

// State returns current state for persistence
func (e *Engine) State() (score int, lastScoreDate string, pomodoroState string, pomodoroRemaining int) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.dailyScore, e.lastScoreDate, e.pomodoro.StateString(), int(e.pomodoro.Remaining().Seconds())
}
