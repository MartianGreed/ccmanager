package tmux

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Session represents a tmux session
type Session struct {
	Name     string
	Created  time.Time
	Attached bool
}

// Pane represents a tmux pane
type Pane struct {
	WindowIndex int
	PaneIndex   int
	Active      bool
}

// Client wraps tmux commands
type Client struct{}

// NewClient creates a new tmux client
func NewClient() *Client {
	return &Client{}
}

// IsRunning checks if tmux server is running
func (c *Client) IsRunning() bool {
	cmd := exec.Command("tmux", "list-sessions")
	err := cmd.Run()
	return err == nil
}

// ListSessions returns all tmux sessions
func (c *Client) ListSessions() ([]Session, error) {
	cmd := exec.Command("tmux", "list-sessions", "-F",
		"#{session_name}:#{session_created}:#{session_attached}")

	output, err := cmd.Output()
	if err != nil {
		if strings.Contains(err.Error(), "no server running") {
			return nil, ErrNoServer
		}
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	var sessions []Session
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 3 {
			continue
		}

		created, _ := strconv.ParseInt(parts[1], 10, 64)
		attached := parts[2] == "1"

		sessions = append(sessions, Session{
			Name:     parts[0],
			Created:  time.Unix(created, 0),
			Attached: attached,
		})
	}

	return sessions, nil
}

// ListPanes returns all panes in a session
func (c *Client) ListPanes(session string) ([]Pane, error) {
	cmd := exec.Command("tmux", "list-panes", "-t", session, "-a",
		"-F", "#{window_index}:#{pane_index}:#{pane_active}")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list panes: %w", err)
	}

	var panes []Pane
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 3 {
			continue
		}

		windowIdx, _ := strconv.Atoi(parts[0])
		paneIdx, _ := strconv.Atoi(parts[1])
		active := parts[2] == "1"

		panes = append(panes, Pane{
			WindowIndex: windowIdx,
			PaneIndex:   paneIdx,
			Active:      active,
		})
	}

	return panes, nil
}

// CapturePane captures the content of a tmux pane
func (c *Client) CapturePane(session string, window, pane int) (string, error) {
	target := fmt.Sprintf("%s:%d.%d", session, window, pane)
	cmd := exec.Command("tmux", "capture-pane", "-e", "-t", target, "-p", "-S", "-50")

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("capture pane %s: %w", target, err)
	}

	return string(output), nil
}

// CapturePaneDefault captures the active pane of a session
func (c *Client) CapturePaneDefault(session string) (string, error) {
	target := session + ":"
	cmd := exec.Command("tmux", "capture-pane", "-e", "-t", target, "-p", "-S", "-50")

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("capture pane %s: %w", target, err)
	}

	return string(output), nil
}

// SwitchClient switches the tmux client to a session
func (c *Client) SwitchClient(session string) error {
	cmd := exec.Command("tmux", "switch-client", "-t", session)
	return cmd.Run()
}

// NewSession creates a new tmux session
func (c *Client) NewSession(name, path string) error {
	args := []string{"new-session", "-d", "-s", name}
	if path != "" {
		args = append(args, "-c", path)
	}
	cmd := exec.Command("tmux", args...)
	return cmd.Run()
}

// KillSession kills a tmux session
func (c *Client) KillSession(name string) error {
	cmd := exec.Command("tmux", "kill-session", "-t", name)
	return cmd.Run()
}

// RenameSession renames a tmux session
func (c *Client) RenameSession(oldName, newName string) error {
	cmd := exec.Command("tmux", "rename-session", "-t", oldName, newName)
	return cmd.Run()
}

// SendKeys sends keys to a tmux session
func (c *Client) SendKeys(session, keys string) error {
	cmd := exec.Command("tmux", "send-keys", "-t", session, keys, "Enter")
	return cmd.Run()
}

// SendKeysToPane sends keys to a specific pane in a tmux session
func (c *Client) SendKeysToPane(session string, pane *Pane, keys string) error {
	var target string
	if pane != nil {
		target = fmt.Sprintf("%s:%d.%d", session, pane.WindowIndex, pane.PaneIndex)
	} else {
		target = session
	}
	cmd := exec.Command("tmux", "send-keys", "-t", target, "-l", keys)
	if err := cmd.Run(); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	// Send first Enter
	cmd = exec.Command("tmux", "send-keys", "-t", target, "Enter")
	if err := cmd.Run(); err != nil {
		return err
	}
	// Send second Enter for multi-line content to signal paste completion
	if strings.Count(keys, "\n") >= 1 {
		time.Sleep(30 * time.Millisecond)
		cmd = exec.Command("tmux", "send-keys", "-t", target, "Enter")
		return cmd.Run()
	}
	return nil
}

// SendKeysRaw sends keys to a tmux session without auto-Enter
func (c *Client) SendKeysRaw(session, keys string) error {
	cmd := exec.Command("tmux", "send-keys", "-t", session, keys)
	return cmd.Run()
}

// SendKeysToPaneRaw sends keys to a specific pane without auto-Enter
func (c *Client) SendKeysToPaneRaw(session string, pane *Pane, keys string) error {
	var target string
	if pane != nil {
		target = fmt.Sprintf("%s:%d.%d", session, pane.WindowIndex, pane.PaneIndex)
	} else {
		target = session
	}
	cmd := exec.Command("tmux", "send-keys", "-t", target, keys)
	return cmd.Run()
}

// GetSessionPath returns the working directory of a session
func (c *Client) GetSessionPath(session string) (string, error) {
	cmd := exec.Command("tmux", "display-message", "-p", "-t", session, "#{pane_current_path}")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// NewWindow creates a new window in an existing session
func (c *Client) NewWindow(session, name, path, command string) error {
	args := []string{"new-window", "-t", session, "-n", name}
	if path != "" {
		args = append(args, "-c", path)
	}
	if command != "" {
		args = append(args, command)
	}
	cmd := exec.Command("tmux", args...)
	return cmd.Run()
}

// Errors
var (
	ErrNoServer = fmt.Errorf("tmux server not running")
)
