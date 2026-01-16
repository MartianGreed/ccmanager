package game

import (
	"sync"
)

type StreakTracker struct {
	mu             sync.RWMutex
	activeSessions map[string]bool
	multiplierCap  float64
}

func NewStreakTracker(timeoutSeconds int, multiplierCap float64) *StreakTracker {
	return &StreakTracker{
		activeSessions: make(map[string]bool),
		multiplierCap:  multiplierCap,
	}
}

func (s *StreakTracker) SetSessionActive(name string, active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if active {
		s.activeSessions[name] = true
	} else {
		delete(s.activeSessions, name)
	}
}

func (s *StreakTracker) Multiplier() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := len(s.activeSessions)
	if count <= 1 {
		return 1.0
	}
	mult := float64(count)
	if mult > s.multiplierCap {
		mult = s.multiplierCap
	}
	return mult
}

func (s *StreakTracker) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.activeSessions)
}
