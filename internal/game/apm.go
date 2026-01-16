package game

import (
	"sync"
	"time"
)

// APMTracker tracks actions per minute
type APMTracker struct {
	mu            sync.RWMutex
	actions       []time.Time
	current       int
	windowSeconds int
}

// NewAPMTracker creates a new APM tracker
func NewAPMTracker(windowSeconds int) *APMTracker {
	return &APMTracker{
		actions:       make([]time.Time, 0, 100),
		windowSeconds: windowSeconds,
	}
}

// RecordAction records an action at the given time
func (a *APMTracker) RecordAction(t time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.actions = append(a.actions, t)
	a.recalculate(t)
}

// Tick updates APM calculation (prunes old actions)
func (a *APMTracker) Tick(now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.recalculate(now)
}

// Current returns the current APM
func (a *APMTracker) Current() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.current
}

func (a *APMTracker) recalculate(now time.Time) {
	cutoff := now.Add(-time.Duration(a.windowSeconds) * time.Second)

	// Prune old actions
	newActions := make([]time.Time, 0, len(a.actions))
	for _, t := range a.actions {
		if t.After(cutoff) {
			newActions = append(newActions, t)
		}
	}
	a.actions = newActions

	count := len(a.actions)
	if count == 0 {
		a.current = 0
		return
	}

	// Calculate elapsed time from first action in window
	elapsed := now.Sub(a.actions[0])
	if elapsed < time.Second {
		elapsed = time.Second
	}

	// Extrapolate to 1 minute
	if elapsed < time.Minute {
		a.current = int(float64(count) * float64(time.Minute) / float64(elapsed))
	} else {
		a.current = count
	}
}
