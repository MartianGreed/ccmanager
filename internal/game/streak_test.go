package game

import (
	"testing"
)

const (
	testStreakTimeoutSeconds = 30
	testStreakMultiplierCap  = 10.0
)

func newTestStreakTracker() *StreakTracker {
	return NewStreakTracker(testStreakTimeoutSeconds, testStreakMultiplierCap)
}

func TestStreakMultiplier(t *testing.T) {
	s := newTestStreakTracker()

	if s.Multiplier() != 1.0 {
		t.Errorf("empty: Multiplier() = %v, want 1.0", s.Multiplier())
	}

	s.SetSessionActive("session1", true)
	if s.Multiplier() != 1.0 {
		t.Errorf("1 session: Multiplier() = %v, want 1.0", s.Multiplier())
	}

	s.SetSessionActive("session2", true)
	if s.Multiplier() != 2.0 {
		t.Errorf("2 sessions: Multiplier() = %v, want 2.0", s.Multiplier())
	}

	s.SetSessionActive("session3", true)
	s.SetSessionActive("session4", true)
	s.SetSessionActive("session5", true)
	if s.Multiplier() != 5.0 {
		t.Errorf("5 sessions: Multiplier() = %v, want 5.0", s.Multiplier())
	}
}

func TestStreakMultiplierCap(t *testing.T) {
	s := newTestStreakTracker()

	for i := 0; i < 15; i++ {
		s.SetSessionActive("session"+string(rune('a'+i)), true)
	}

	if s.Multiplier() != testStreakMultiplierCap {
		t.Errorf("15 sessions: Multiplier() = %v, want %v (capped)", s.Multiplier(), testStreakMultiplierCap)
	}
}

func TestStreakCount(t *testing.T) {
	s := newTestStreakTracker()

	if s.Count() != 0 {
		t.Errorf("empty: Count() = %d, want 0", s.Count())
	}

	s.SetSessionActive("session1", true)
	if s.Count() != 1 {
		t.Errorf("after adding session1: Count() = %d, want 1", s.Count())
	}

	s.SetSessionActive("session2", true)
	if s.Count() != 2 {
		t.Errorf("after adding session2: Count() = %d, want 2", s.Count())
	}

	s.SetSessionActive("session1", false)
	if s.Count() != 1 {
		t.Errorf("after removing session1: Count() = %d, want 1", s.Count())
	}

	s.SetSessionActive("session2", false)
	if s.Count() != 0 {
		t.Errorf("after removing session2: Count() = %d, want 0", s.Count())
	}
}

func TestStreakDeduplication(t *testing.T) {
	s := newTestStreakTracker()

	s.SetSessionActive("session1", true)
	s.SetSessionActive("session1", true)
	s.SetSessionActive("session1", true)

	if s.Count() != 1 {
		t.Errorf("after setting same session active 3 times: Count() = %d, want 1", s.Count())
	}
}
