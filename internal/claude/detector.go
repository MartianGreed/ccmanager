package claude

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SessionState represents the current state of a Claude session
type SessionState int

const (
	StateUnknown  SessionState = iota
	StateIdle                  // Prompt visible, waiting for input
	StateActive                // User recently typed
	StateThinking              // Claude is thinking
	StateUrgent                // Needs immediate input
)

func (s SessionState) String() string {
	switch s {
	case StateIdle:
		return "IDLE"
	case StateActive:
		return "ACTIVE"
	case StateThinking:
		return "THINKING"
	case StateUrgent:
		return "URGENT"
	default:
		return "UNKNOWN"
	}
}

// SessionInfo contains parsed information from Claude output
type SessionInfo struct {
	State        SessionState
	Tokens       int
	ThinkingTime time.Duration
	LastLine     string
}

// Detector detects Claude session states from terminal output
type Detector struct {
	urgentPatterns       []*regexp.Regexp
	thinkingPatterns     []*regexp.Regexp
	idlePatterns         []*regexp.Regexp
	promptActivePatterns []*regexp.Regexp
	claudePatterns       []*regexp.Regexp
	tokenPattern         *regexp.Regexp
	thinkingPattern      *regexp.Regexp
	ansiPattern          *regexp.Regexp
	modePattern          *regexp.Regexp
}

// NewDetector creates a new Claude state detector
func NewDetector() *Detector {
	return &Detector{
		urgentPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)\[Y/n\]`),
			regexp.MustCompile(`(?i)\[y/N\]`),
			regexp.MustCompile(`(?i)Permission requested`),
			regexp.MustCompile(`(?i)Allow\?`),
			regexp.MustCompile(`(?i)Proceed\?`),
			regexp.MustCompile(`(?i)Are you sure`),
			regexp.MustCompile(`(?i)Continue\?`),
			regexp.MustCompile(`(?i)\(yes/no\)`),
			regexp.MustCompile(`(?i)Press any key`),
			regexp.MustCompile(`(?i)Hit enter`),
			regexp.MustCompile(`(?i)chat about this`),
			regexp.MustCompile(`(?i)skip interview and plan`),
			regexp.MustCompile(`(?i)Ready to submit your answers`),
		},
		thinkingPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)thinking\.{3}`),
			regexp.MustCompile(`(?i)thinking…`),
			regexp.MustCompile(`(?i)reasoning`),
			regexp.MustCompile(`⠋|⠙|⠹|⠸|⠼|⠴|⠦|⠧|⠇|⠏`),
			regexp.MustCompile(`Working\.\.\.`),
			regexp.MustCompile(`Processing\.\.\.`),
			regexp.MustCompile(`\(esc to cancel\)`),
			regexp.MustCompile(`\(ctrl.* to interrupt\)`),
			regexp.MustCompile(`thought for \d+`),
			regexp.MustCompile(`thinking`),
		},
		idlePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?m)❯\s*$`),
			regexp.MustCompile(`(?m)>\s*$`),
			regexp.MustCompile(`(?m)claude>\s*$`),
			regexp.MustCompile(`↵ send`),
			regexp.MustCompile(`⏵⏵`),
			regexp.MustCompile(`▐▛███▜▌`),
		},
		promptActivePatterns: []*regexp.Regexp{},
		claudePatterns: []*regexp.Regexp{
			regexp.MustCompile(`Claude Code`),
			regexp.MustCompile(`claude>`),
			regexp.MustCompile(`❯`),
			regexp.MustCompile(`ing\.\.\. \(ctrl`),
		},
		tokenPattern:    regexp.MustCompile(`↓\s*([\d,.]+)k?\s*tokens?`),
		thinkingPattern: regexp.MustCompile(`Thinking[^(]*\((\d+)m?\s*(\d+)?s?\)`),
		ansiPattern:     regexp.MustCompile(`\x1b\[[0-9;]*m`),
		modePattern:     regexp.MustCompile(`(?i)(plan|code|auto|accept[\s-]?edits?)\s+(?:mode\s+)?on\s+\(shift\+tab`),
	}
}

// IsClaudeSession checks if the content appears to be from a Claude session
func (d *Detector) IsClaudeSession(content string) bool {
	content = d.stripANSI(content)
	for _, pattern := range d.claudePatterns {
		if pattern.MatchString(content) {
			return true
		}
	}
	return false
}

// DetectState determines the current state of a Claude session
func (d *Detector) DetectState(content string, lastContent string, lastCapture time.Time) SessionState {
	content = d.stripANSI(content)
	lines := d.getLastLines(content, 20)

	// Check URGENT first (highest priority)
	for _, pattern := range d.urgentPatterns {
		if pattern.MatchString(lines) {
			return StateUrgent
		}
	}

	// Check THINKING
	for _, pattern := range d.thinkingPatterns {
		if pattern.MatchString(lines) {
			return StateThinking
		}
	}

	// Check IDLE (prompt with "↵ send" visible)
	for _, pattern := range d.idlePatterns {
		if pattern.MatchString(lines) {
			return StateIdle
		}
	}

	// Default: ACTIVE
	return StateActive
}

// ParseInfo extracts additional information from Claude output
func (d *Detector) ParseInfo(content string) SessionInfo {
	content = d.stripANSI(content)
	lines := d.getLastLines(content, 20)

	info := SessionInfo{
		State:    d.DetectState(content, "", time.Time{}),
		LastLine: d.getLastNonEmptyLine(content),
	}

	// Parse tokens
	if matches := d.tokenPattern.FindStringSubmatch(content); len(matches) >= 2 {
		numStr := strings.ReplaceAll(matches[1], ",", "")
		multiplier := 1
		if strings.Contains(matches[0], "k token") {
			multiplier = 1000
		}
		if num, err := strconv.ParseFloat(numStr, 64); err == nil {
			info.Tokens = int(num * float64(multiplier))
		}
	}

	// Parse thinking time
	if matches := d.thinkingPattern.FindStringSubmatch(lines); len(matches) >= 2 {
		var duration time.Duration
		if strings.Contains(matches[0], "m") {
			if mins, err := strconv.Atoi(matches[1]); err == nil {
				duration += time.Duration(mins) * time.Minute
			}
			if len(matches) > 2 && matches[2] != "" {
				if secs, err := strconv.Atoi(matches[2]); err == nil {
					duration += time.Duration(secs) * time.Second
				}
			}
		} else {
			if secs, err := strconv.Atoi(matches[1]); err == nil {
				duration += time.Duration(secs) * time.Second
			}
		}
		info.ThinkingTime = duration
	}

	return info
}

// DetectMode detects the current Claude mode (plan, code, auto) from terminal output
func (d *Detector) DetectMode(content string) string {
	content = d.stripANSI(content)
	if matches := d.modePattern.FindStringSubmatch(content); len(matches) >= 2 {
		mode := strings.ToLower(matches[1])
		if strings.Contains(mode, "accept") || strings.Contains(mode, "edit") {
			return "edit"
		}
		return mode
	}
	return ""
}

func (d *Detector) stripANSI(s string) string {
	return d.ansiPattern.ReplaceAllString(s, "")
}

func (d *Detector) getLastLines(content string, n int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= n {
		return content
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

func (d *Detector) getLastNonEmptyLine(content string) string {
	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if trimmed := strings.TrimSpace(lines[i]); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
