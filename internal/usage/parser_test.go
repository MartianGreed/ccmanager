package usage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSessionFile(t *testing.T) {
	tmpDir := t.TempDir()
	sessionFile := filepath.Join(tmpDir, "test-session.jsonl")

	content := `{"type":"user","message":"hello"}
{"type":"assistant","message":{"model":"claude-sonnet-4-20250514","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":200,"cache_read_input_tokens":300}}}
{"type":"user","message":"world"}
{"type":"assistant","message":{"model":"claude-sonnet-4-20250514","usage":{"input_tokens":150,"output_tokens":75,"cache_creation_input_tokens":0,"cache_read_input_tokens":500}}}
`
	if err := os.WriteFile(sessionFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	usage, err := ParseSessionFile(sessionFile)
	if err != nil {
		t.Fatalf("ParseSessionFile failed: %v", err)
	}

	if usage.TotalUsage.InputTokens != 250 {
		t.Errorf("InputTokens = %d, want 250", usage.TotalUsage.InputTokens)
	}
	if usage.TotalUsage.OutputTokens != 125 {
		t.Errorf("OutputTokens = %d, want 125", usage.TotalUsage.OutputTokens)
	}
	if usage.TotalUsage.CacheCreationInputTokens != 200 {
		t.Errorf("CacheCreationInputTokens = %d, want 200", usage.TotalUsage.CacheCreationInputTokens)
	}
	if usage.TotalUsage.CacheReadInputTokens != 800 {
		t.Errorf("CacheReadInputTokens = %d, want 800", usage.TotalUsage.CacheReadInputTokens)
	}
	if usage.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %s, want claude-sonnet-4-20250514", usage.Model)
	}
}

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		name     string
		usage    TokenUsage
		model    string
		wantCost float64
	}{
		{
			name: "sonnet basic",
			usage: TokenUsage{
				InputTokens:  1_000_000,
				OutputTokens: 1_000_000,
			},
			model:    "claude-sonnet-4-20250514",
			wantCost: 18.0, // $3 input + $15 output
		},
		{
			name: "sonnet with cache",
			usage: TokenUsage{
				InputTokens:              0,
				OutputTokens:             100_000,
				CacheCreationInputTokens: 1_000_000,
				CacheReadInputTokens:     1_000_000,
			},
			model:    "sonnet",
			wantCost: 5.55, // $3.75 cache write + $0.30 cache read + $1.50 output
		},
		{
			name: "opus",
			usage: TokenUsage{
				InputTokens:  1_000_000,
				OutputTokens: 100_000,
			},
			model:    "claude-opus-4-5-20251101",
			wantCost: 22.5, // $15 input + $7.5 output
		},
		{
			name: "haiku",
			usage: TokenUsage{
				InputTokens:  1_000_000,
				OutputTokens: 1_000_000,
			},
			model:    "claude-3-5-haiku-20241022",
			wantCost: 4.8, // $0.80 input + $4 output
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := CalculateCost(tt.usage, tt.model)
			if cost < tt.wantCost-0.01 || cost > tt.wantCost+0.01 {
				t.Errorf("CalculateCost() = %v, want %v", cost, tt.wantCost)
			}
		})
	}
}

func TestNormalizeModelName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"claude-opus-4-5-20251101", "opus"},
		{"claude-sonnet-4-20250514", "sonnet"},
		{"claude-3-5-haiku-20241022", "haiku"},
		{"CLAUDE-OPUS-4", "opus"},
		{"unknown-model", "sonnet"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeModelName(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeModelName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTokenUsageAdd(t *testing.T) {
	a := TokenUsage{
		InputTokens:              100,
		OutputTokens:             50,
		CacheCreationInputTokens: 200,
		CacheReadInputTokens:     300,
	}
	b := TokenUsage{
		InputTokens:              50,
		OutputTokens:             25,
		CacheCreationInputTokens: 100,
		CacheReadInputTokens:     150,
	}

	a.Add(b)

	if a.InputTokens != 150 {
		t.Errorf("InputTokens = %d, want 150", a.InputTokens)
	}
	if a.OutputTokens != 75 {
		t.Errorf("OutputTokens = %d, want 75", a.OutputTokens)
	}
	if a.CacheCreationInputTokens != 300 {
		t.Errorf("CacheCreationInputTokens = %d, want 300", a.CacheCreationInputTokens)
	}
	if a.CacheReadInputTokens != 450 {
		t.Errorf("CacheReadInputTokens = %d, want 450", a.CacheReadInputTokens)
	}
}

func TestTokenUsageTotalInput(t *testing.T) {
	usage := TokenUsage{
		InputTokens:              100,
		CacheCreationInputTokens: 200,
		CacheReadInputTokens:     300,
	}

	total := usage.TotalInput()
	if total != 600 {
		t.Errorf("TotalInput() = %d, want 600", total)
	}
}
