package game

import (
	"sync"
	"time"
)

// PomodoroState represents the pomodoro timer state
type PomodoroState int

const (
	PomodoroStopped PomodoroState = iota
	PomodoroPaused
	PomodoroWork
	PomodoroShortBreak
	PomodoroLongBreak
)

func (s PomodoroState) String() string {
	switch s {
	case PomodoroPaused:
		return "paused"
	case PomodoroWork:
		return "work"
	case PomodoroShortBreak:
		return "short_break"
	case PomodoroLongBreak:
		return "long_break"
	default:
		return "stopped"
	}
}

// PomodoroTimer manages the pomodoro timer
type PomodoroTimer struct {
	mu          sync.RWMutex
	state       PomodoroState
	pausedState PomodoroState
	remaining   time.Duration
	lastTick    time.Time
	completed   int // Pomodoros completed in current cycle

	// Config
	workMinutes       int
	shortBreakMinutes int
	longBreakMinutes  int
	sessionsBeforeLongBreak int

	// Callbacks
	onComplete func()
	onTick     func(state PomodoroState, remaining time.Duration)
}

// NewPomodoroTimer creates a new pomodoro timer
func NewPomodoroTimer(workMinutes, shortBreakMinutes, longBreakMinutes, sessionsBeforeLongBreak int) *PomodoroTimer {
	return &PomodoroTimer{
		state:                   PomodoroStopped,
		lastTick:                time.Now(),
		workMinutes:             workMinutes,
		shortBreakMinutes:       shortBreakMinutes,
		longBreakMinutes:        longBreakMinutes,
		sessionsBeforeLongBreak: sessionsBeforeLongBreak,
	}
}

// Start starts a work session
func (p *PomodoroTimer) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == PomodoroStopped {
		p.state = PomodoroWork
		p.remaining = time.Duration(p.workMinutes) * time.Minute
		p.lastTick = time.Now()
	}
}

// Stop stops the timer
func (p *PomodoroTimer) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.state = PomodoroStopped
	p.remaining = 0
}

// Pause pauses the timer while preserving remaining time
func (p *PomodoroTimer) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state != PomodoroStopped && p.state != PomodoroPaused {
		p.pausedState = p.state
		p.state = PomodoroPaused
	}
}

// Resume resumes a paused timer
func (p *PomodoroTimer) Resume() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state == PomodoroPaused {
		p.state = p.pausedState
		p.lastTick = time.Now()
	}
}

// Toggle toggles between running and paused
func (p *PomodoroTimer) Toggle() {
	p.mu.Lock()
	state := p.state
	p.mu.Unlock()

	switch state {
	case PomodoroStopped:
		p.Start()
	case PomodoroPaused:
		p.Resume()
	default:
		p.Pause()
	}
}

// Tick updates the timer
func (p *PomodoroTimer) Tick(now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == PomodoroStopped || p.state == PomodoroPaused {
		return
	}

	elapsed := now.Sub(p.lastTick)
	p.lastTick = now

	if p.remaining > elapsed {
		p.remaining -= elapsed
	} else {
		p.remaining = 0
		p.transition()
	}

	if p.onTick != nil {
		p.onTick(p.state, p.remaining)
	}
}

func (p *PomodoroTimer) transition() {
	switch p.state {
	case PomodoroWork:
		p.completed++
		if p.completed >= p.sessionsBeforeLongBreak {
			p.state = PomodoroLongBreak
			p.remaining = time.Duration(p.longBreakMinutes) * time.Minute
			p.completed = 0
		} else {
			p.state = PomodoroShortBreak
			p.remaining = time.Duration(p.shortBreakMinutes) * time.Minute
		}
		if p.onComplete != nil {
			p.onComplete()
		}
	case PomodoroShortBreak, PomodoroLongBreak:
		p.state = PomodoroWork
		p.remaining = time.Duration(p.workMinutes) * time.Minute
	}
}

// State returns the current state
func (p *PomodoroTimer) State() PomodoroState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

// StateString returns the state as a string
func (p *PomodoroTimer) StateString() string {
	return p.State().String()
}

// Remaining returns the remaining time
func (p *PomodoroTimer) Remaining() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.remaining
}

// IsActive returns true if timer is running
func (p *PomodoroTimer) IsActive() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state != PomodoroStopped
}

// IsWorking returns true if in work mode
func (p *PomodoroTimer) IsWorking() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state == PomodoroWork
}

// CompletedCount returns the number of completed pomodoros
func (p *PomodoroTimer) CompletedCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.completed
}

// SetState sets the timer state (for loading)
func (p *PomodoroTimer) SetState(stateStr string, remaining time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch stateStr {
	case "paused":
		p.state = PomodoroPaused
	case "work":
		p.state = PomodoroWork
	case "short_break":
		p.state = PomodoroShortBreak
	case "long_break":
		p.state = PomodoroLongBreak
	default:
		p.state = PomodoroStopped
	}
	p.remaining = remaining
	p.lastTick = time.Now()
}

// OnComplete sets the completion callback
func (p *PomodoroTimer) OnComplete(fn func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onComplete = fn
}

// OnTick sets the tick callback
func (p *PomodoroTimer) OnTick(fn func(state PomodoroState, remaining time.Duration)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onTick = fn
}
