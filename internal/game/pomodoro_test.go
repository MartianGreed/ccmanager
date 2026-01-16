package game

import (
	"testing"
	"time"
)

const (
	testWorkMinutes       = 25
	testShortBreakMinutes = 5
	testLongBreakMinutes  = 15
	testSessionsBeforeLongBreak = 4
)

func newTestPomodoroTimer() *PomodoroTimer {
	return NewPomodoroTimer(testWorkMinutes, testShortBreakMinutes, testLongBreakMinutes, testSessionsBeforeLongBreak)
}

func TestPomodoroStart(t *testing.T) {
	p := newTestPomodoroTimer()

	if p.IsActive() {
		t.Error("new timer should not be active")
	}

	p.Start()

	if !p.IsActive() {
		t.Error("timer should be active after Start()")
	}

	if p.State() != PomodoroWork {
		t.Errorf("State() = %v, want PomodoroWork", p.State())
	}

	expected := time.Duration(testWorkMinutes) * time.Minute
	if p.Remaining() != expected {
		t.Errorf("Remaining() = %v, want %v", p.Remaining(), expected)
	}
}

func TestPomodoroToggle(t *testing.T) {
	p := newTestPomodoroTimer()

	p.Toggle()
	if !p.IsActive() {
		t.Error("timer should be active after first Toggle()")
	}
	if p.State() != PomodoroWork {
		t.Errorf("State() = %v, want PomodoroWork", p.State())
	}

	initialRemaining := p.Remaining()
	time.Sleep(10 * time.Millisecond)
	p.Tick(time.Now())
	beforePause := p.Remaining()

	p.Toggle()
	if p.State() != PomodoroPaused {
		t.Errorf("State() = %v, want PomodoroPaused", p.State())
	}
	if p.Remaining() != beforePause {
		t.Errorf("Remaining() = %v should be preserved at %v after pause", p.Remaining(), beforePause)
	}
	if p.Remaining() >= initialRemaining {
		t.Errorf("Remaining() = %v should be less than initial %v", p.Remaining(), initialRemaining)
	}

	p.Toggle()
	if p.State() != PomodoroWork {
		t.Errorf("State() = %v, want PomodoroWork after resume", p.State())
	}
	if p.Remaining() != beforePause {
		t.Errorf("Remaining() = %v should still be %v after resume", p.Remaining(), beforePause)
	}
}

func TestPomodoroTick(t *testing.T) {
	p := newTestPomodoroTimer()
	p.Start()

	initial := p.Remaining()
	time.Sleep(10 * time.Millisecond)
	p.Tick(time.Now())

	// Remaining should be less than initial
	if p.Remaining() >= initial {
		t.Errorf("Remaining() = %v should be less than initial %v", p.Remaining(), initial)
	}
}

func TestPomodoroStateString(t *testing.T) {
	tests := []struct {
		state PomodoroState
		want  string
	}{
		{PomodoroStopped, "stopped"},
		{PomodoroPaused, "paused"},
		{PomodoroWork, "work"},
		{PomodoroShortBreak, "short_break"},
		{PomodoroLongBreak, "long_break"},
	}

	for _, tt := range tests {
		got := tt.state.String()
		if got != tt.want {
			t.Errorf("state %v: String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}
