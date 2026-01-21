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
		{"thinking lowercase", "thinking...", StateThinking},
		{"thinking unicode ellipsis", "Thinking…", StateThinking},
		{"reasoning variant", "Reasoning about the problem", StateThinking},
		{"esc to cancel", "Working (esc to cancel)", StateThinking},
		{"ctrl to interrupt", "Processing (ctrl+c to interrupt)", StateThinking},
		{"spinner char", "⠋ Loading", StateThinking},
		{"thought for time", "(ctrl+c to interrupt · thought for 5s)", StateThinking},
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

func TestDetectMode(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"plan mode", "plan mode on (shift+tab to cycle)", "plan"},
		{"code mode", "code mode on (shift+tab to cycle)", "code"},
		{"auto mode", "auto mode on (shift+tab to cycle)", "auto"},
		{"accept edits", "accept edits on (shift+tab to cycle)", "edit"},
		{"uppercase", "PLAN mode on (shift+tab to cycle)", "plan"},
		{"no mode", "some content", ""},
		{"partial match", "mode on (shift+tab", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := d.DetectMode(tt.content); got != tt.want {
				t.Errorf("DetectMode() = %q, want %q", got, tt.want)
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
