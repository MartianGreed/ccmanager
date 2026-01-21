package claude

import (
	"testing"
	"time"
)

func TestIsClaudeSession(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"empty", "", false},
		{"random text", "hello world", false},
		{"claude prompt", "some output\n❯", true},
		{"thinking", "✽ Thinking... (ctrl+c to cancel)", true},
		{"claude code", "Claude Code version 1.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.IsClaudeSession(tt.content)
			if got != tt.want {
				t.Errorf("IsClaudeSession() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectState(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name    string
		content string
		want    SessionState
	}{
		{"urgent yn", "Allow execution? [Y/n]", StateUrgent},
		{"urgent permission", "Permission requested for bash", StateUrgent},
		{"waiting thinking", "✽ Thinking... (ctrl+c to cancel)", StateThinking},
		{"idle prompt", "some output\n❯ ", StateIdle},
		{"idle claude code logo", " ▐▛███▜▌   Claude Code v2.1.12\n▝▜█████▛▘  Opus 4.5", StateIdle},
		{"active shortcuts hint", "? for shortcuts", StateActive},
		{"active accept edits", "accept edits on (shift+tab to cycle)", StateActive},
		{"active plan mode", "plan mode on (shift+tab to cycle)", StateActive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.DetectState(tt.content, "", time.Time{})
			if got != tt.want {
				t.Errorf("DetectState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseInfo(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name       string
		content    string
		wantTokens int
	}{
		{"no tokens", "hello world", 0},
		{"tokens", "↓ 500 tokens", 500},
		{"k tokens", "↓ 3.3k tokens", 3300},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := d.ParseInfo(tt.content)
			if info.Tokens != tt.wantTokens {
				t.Errorf("ParseInfo().Tokens = %v, want %v", info.Tokens, tt.wantTokens)
			}
		})
	}
}
