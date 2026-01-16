package game

import (
	"sync"
	"time"
)

// ControlGroups manages SC2-style control group assignments (one session per group)
type ControlGroups struct {
	mu     sync.RWMutex
	groups map[int]string // group number -> session name

	// Double-tap detection
	lastTap      map[int]time.Time
	tapThreshold time.Duration
}

// NewControlGroups creates a new control groups manager
func NewControlGroups(doubleTapThresholdMs int) *ControlGroups {
	return &ControlGroups{
		groups:       make(map[int]string),
		lastTap:      make(map[int]time.Time),
		tapThreshold: time.Duration(doubleTapThresholdMs) * time.Millisecond,
	}
}

// Assign sets a session to a control group (replaces existing)
func (c *ControlGroups) Assign(groupNum int, session string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.groups[groupNum] = session
}

// Remove removes a session from a control group
func (c *ControlGroups) Remove(groupNum int, session string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.groups[groupNum] == session {
		delete(c.groups, groupNum)
	}
}

// RemoveSession removes a session from all groups
func (c *ControlGroups) RemoveSession(session string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for groupNum, s := range c.groups {
		if s == session {
			delete(c.groups, groupNum)
		}
	}
}

// Get returns the session in a group (empty string if none)
func (c *ControlGroups) Get(groupNum int) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.groups[groupNum]
}

// Cycle returns the session in a group and detects double-tap
func (c *ControlGroups) Cycle(groupNum int) (session string, doubleTap bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	// Check for double-tap
	if lastTap, ok := c.lastTap[groupNum]; ok {
		if now.Sub(lastTap) < c.tapThreshold {
			doubleTap = true
		}
	}
	c.lastTap[groupNum] = now

	return c.groups[groupNum], doubleTap
}

// Current returns the session in a group
func (c *ControlGroups) Current(groupNum int) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.groups[groupNum]
}

// Count returns 1 if group has a session, 0 otherwise
func (c *ControlGroups) Count(groupNum int) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.groups[groupNum] != "" {
		return 1
	}
	return 0
}

// FirstFreeGroup returns the first empty group (1-10), or 0 if all are occupied
func (c *ControlGroups) FirstFreeGroup() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for i := 1; i <= 10; i++ {
		if c.groups[i] == "" {
			return i
		}
	}
	return 0
}

// Contains checks if a session is in a group
func (c *ControlGroups) Contains(groupNum int, session string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.groups[groupNum] == session
}

// GroupsForSession returns all groups containing a session
func (c *ControlGroups) GroupsForSession(session string) []int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var groups []int
	for groupNum, s := range c.groups {
		if s == session {
			groups = append(groups, groupNum)
		}
	}
	return groups
}

// All returns all control group assignments
func (c *ControlGroups) All() map[int]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[int]string)
	for k, v := range c.groups {
		result[k] = v
	}
	return result
}

// Load loads control groups from a map
func (c *ControlGroups) Load(groups map[int]string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.groups = make(map[int]string)
	for k, v := range groups {
		c.groups[k] = v
	}
}
