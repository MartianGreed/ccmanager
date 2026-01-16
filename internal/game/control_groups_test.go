package game

import (
	"testing"
)

func TestControlGroupsAssign(t *testing.T) {
	cg := NewControlGroups(300)

	cg.Assign(1, "session-a")

	session := cg.Get(1)
	if session != "session-a" {
		t.Errorf("Get(1) = %q, want %q", session, "session-a")
	}

	// Assigning again should replace
	cg.Assign(1, "session-b")
	session = cg.Get(1)
	if session != "session-b" {
		t.Errorf("after reassign: Get(1) = %q, want %q", session, "session-b")
	}
}

func TestControlGroupsRemove(t *testing.T) {
	cg := NewControlGroups(300)

	cg.Assign(1, "session-a")
	cg.Remove(1, "session-a")

	session := cg.Get(1)
	if session != "" {
		t.Errorf("Get(1) = %q, want empty", session)
	}

	// Removing non-matching session should be no-op
	cg.Assign(1, "session-a")
	cg.Remove(1, "session-b")
	session = cg.Get(1)
	if session != "session-a" {
		t.Errorf("after non-matching remove: Get(1) = %q, want %q", session, "session-a")
	}
}

func TestControlGroupsCycle(t *testing.T) {
	cg := NewControlGroups(300)

	cg.Assign(1, "session-a")

	// First cycle
	s, _ := cg.Cycle(1)
	if s != "session-a" {
		t.Errorf("Cycle() = %q, want %q", s, "session-a")
	}

	// Empty group
	s, _ = cg.Cycle(2)
	if s != "" {
		t.Errorf("Cycle(2) = %q, want empty", s)
	}
}

func TestControlGroupsContains(t *testing.T) {
	cg := NewControlGroups(300)

	cg.Assign(1, "session-a")

	if !cg.Contains(1, "session-a") {
		t.Error("Contains(1, session-a) = false, want true")
	}

	if cg.Contains(1, "session-b") {
		t.Error("Contains(1, session-b) = true, want false")
	}

	if cg.Contains(2, "session-a") {
		t.Error("Contains(2, session-a) = true, want false")
	}
}

func TestControlGroupsForSession(t *testing.T) {
	cg := NewControlGroups(300)

	cg.Assign(1, "session-a")
	cg.Assign(2, "session-a")
	cg.Assign(3, "session-b")

	groups := cg.GroupsForSession("session-a")
	if len(groups) != 2 {
		t.Errorf("GroupsForSession() len = %d, want 2", len(groups))
	}
}

func TestControlGroupsRemoveSession(t *testing.T) {
	cg := NewControlGroups(300)

	cg.Assign(1, "session-a")
	cg.Assign(2, "session-a")
	cg.RemoveSession("session-a")

	if cg.Count(1) != 0 {
		t.Errorf("Count(1) = %d, want 0", cg.Count(1))
	}

	if cg.Count(2) != 0 {
		t.Errorf("Count(2) = %d, want 0", cg.Count(2))
	}
}

func TestControlGroupsCount(t *testing.T) {
	cg := NewControlGroups(300)

	if cg.Count(1) != 0 {
		t.Errorf("Count(1) = %d, want 0", cg.Count(1))
	}

	cg.Assign(1, "session-a")
	if cg.Count(1) != 1 {
		t.Errorf("Count(1) = %d, want 1", cg.Count(1))
	}
}
