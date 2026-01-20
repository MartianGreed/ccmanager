package usage

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// jsonlMessage represents a message in the JSONL file
type jsonlMessage struct {
	Type    string `json:"type"`
	Message struct {
		Model string `json:"model"`
		Usage struct {
			InputTokens              int64 `json:"input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// ParseSessionFile parses a Claude JSONL session file and returns usage data
func ParseSessionFile(path string) (*SessionUsage, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	usage := &SessionUsage{
		SessionID:   filepath.Base(strings.TrimSuffix(path, ".jsonl")),
		ProjectPath: filepath.Dir(path),
		LastUpdated: time.Now(),
	}

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg jsonlMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}

		if msg.Type == "assistant" {
			usage.TotalUsage.Add(TokenUsage{
				InputTokens:              msg.Message.Usage.InputTokens,
				OutputTokens:             msg.Message.Usage.OutputTokens,
				CacheCreationInputTokens: msg.Message.Usage.CacheCreationInputTokens,
				CacheReadInputTokens:     msg.Message.Usage.CacheReadInputTokens,
			})
			if msg.Message.Model != "" {
				usage.Model = msg.Message.Model
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return usage, err
	}

	return usage, nil
}

// ParseSessionFileTail parses only the last N bytes of a session file for efficiency
func ParseSessionFileTail(path string, tailBytes int64) (*SessionUsage, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	// If file is small enough, parse the whole thing
	if info.Size() <= tailBytes {
		return ParseSessionFile(path)
	}

	// Seek to near the end
	_, err = file.Seek(-tailBytes, 2)
	if err != nil {
		return nil, err
	}

	usage := &SessionUsage{
		SessionID:   filepath.Base(strings.TrimSuffix(path, ".jsonl")),
		ProjectPath: filepath.Dir(path),
		LastUpdated: time.Now(),
	}

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	first := true
	for scanner.Scan() {
		line := scanner.Bytes()
		// Skip partial first line when reading from middle
		if first {
			first = false
			continue
		}
		if len(line) == 0 {
			continue
		}

		var msg jsonlMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}

		if msg.Type == "assistant" {
			usage.TotalUsage.Add(TokenUsage{
				InputTokens:              msg.Message.Usage.InputTokens,
				OutputTokens:             msg.Message.Usage.OutputTokens,
				CacheCreationInputTokens: msg.Message.Usage.CacheCreationInputTokens,
				CacheReadInputTokens:     msg.Message.Usage.CacheReadInputTokens,
			})
			if msg.Message.Model != "" {
				usage.Model = msg.Message.Model
			}
		}
	}

	return usage, nil
}

// FindSessionFiles finds all JSONL session files for a given project directory
func FindSessionFiles(projectDir string) ([]string, error) {
	pattern := filepath.Join(projectDir, "*.jsonl")
	return filepath.Glob(pattern)
}

// GetClaudeProjectsDir returns the Claude projects directory path
func GetClaudeProjectsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// FindProjectDir finds the Claude project directory for a given working directory
func FindProjectDir(workingDir string) (string, error) {
	claudeProjectsDir, err := GetClaudeProjectsDir()
	if err != nil {
		return "", err
	}

	// Claude Code uses path encoding: /Users/foo/bar -> -Users-foo-bar
	encodedPath := strings.ReplaceAll(workingDir, "/", "-")

	projectDir := filepath.Join(claudeProjectsDir, encodedPath)
	if _, err := os.Stat(projectDir); err != nil {
		return "", err
	}

	return projectDir, nil
}

// SumUsageFromDir sums usage from all session files in a project directory
func SumUsageFromDir(projectDir string) (*TokenUsage, error) {
	files, err := FindSessionFiles(projectDir)
	if err != nil {
		return nil, err
	}

	total := &TokenUsage{}
	for _, file := range files {
		usage, err := ParseSessionFile(file)
		if err != nil {
			continue
		}
		total.Add(usage.TotalUsage)
	}

	return total, nil
}

// FindActiveSessionID returns the session ID of the most recently modified JSONL file
func FindActiveSessionID(workingDir string) (string, error) {
	projectDir, err := FindProjectDir(workingDir)
	if err != nil {
		return "", err
	}

	files, err := FindSessionFiles(projectDir)
	if err != nil {
		return "", err
	}

	var mostRecent string
	var mostRecentTime time.Time
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		if info.ModTime().After(mostRecentTime) {
			mostRecentTime = info.ModTime()
			mostRecent = f
		}
	}

	if mostRecent == "" {
		return "", nil
	}

	// Extract session ID from filename (e.g., "abc123.jsonl" -> "abc123")
	return strings.TrimSuffix(filepath.Base(mostRecent), ".jsonl"), nil
}

// GetSessionByID returns usage for a specific Claude session ID
func GetSessionByID(workingDir, sessionID string) (*SessionUsage, error) {
	if sessionID == "" {
		return nil, nil
	}

	projectDir, err := FindProjectDir(workingDir)
	if err != nil {
		return nil, err
	}

	sessionFile := filepath.Join(projectDir, sessionID+".jsonl")
	if _, err := os.Stat(sessionFile); err != nil {
		return nil, nil // File doesn't exist, return nil without error
	}

	usage, err := ParseSessionFile(sessionFile)
	if err != nil {
		return nil, err
	}

	usage.EstimatedCost = CalculateCost(usage.TotalUsage, usage.Model)
	return usage, nil
}
