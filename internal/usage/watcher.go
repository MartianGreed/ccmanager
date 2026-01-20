package usage

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Watcher monitors session files for changes
type Watcher struct {
	mu            sync.RWMutex
	sessions      map[string]*sessionWatch
	stopCh        chan struct{}
	updateCh      chan string
	pollInterval  time.Duration
	claudeBaseDir string
}

type sessionWatch struct {
	sessionFile string
	lastSize    int64
	lastMod     time.Time
	usage       *SessionUsage
}

// NewWatcher creates a new usage watcher
func NewWatcher(pollInterval time.Duration) *Watcher {
	claudeDir, _ := GetClaudeProjectsDir()
	return &Watcher{
		sessions:      make(map[string]*sessionWatch),
		stopCh:        make(chan struct{}),
		updateCh:      make(chan string, 100),
		pollInterval:  pollInterval,
		claudeBaseDir: claudeDir,
	}
}

// Updates returns a channel that receives session names when usage updates
func (w *Watcher) Updates() <-chan string {
	return w.updateCh
}

// Start begins watching for file changes
func (w *Watcher) Start() {
	go w.pollLoop()
}

// Stop stops the watcher
func (w *Watcher) Stop() {
	close(w.stopCh)
}

// WatchSession adds a session to the watch list
func (w *Watcher) WatchSession(sessionName, workingDir string) {
	projectDir, err := FindProjectDir(workingDir)
	if err != nil {
		return
	}

	// Find the most recent JSONL file in the project dir
	files, err := FindSessionFiles(projectDir)
	if err != nil || len(files) == 0 {
		return
	}

	// Get the most recently modified file
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
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if _, exists := w.sessions[sessionName]; !exists {
		w.sessions[sessionName] = &sessionWatch{
			sessionFile: mostRecent,
		}
	}
}

// UnwatchSession removes a session from the watch list
func (w *Watcher) UnwatchSession(sessionName string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.sessions, sessionName)
}

// GetUsage returns the current usage for a session
func (w *Watcher) GetUsage(sessionName string) *SessionUsage {
	w.mu.RLock()
	defer w.mu.RUnlock()

	watch, ok := w.sessions[sessionName]
	if !ok || watch.usage == nil {
		return nil
	}
	return watch.usage
}

// RefreshSession forces a refresh of usage data for a session
func (w *Watcher) RefreshSession(sessionName string) {
	w.mu.RLock()
	watch, ok := w.sessions[sessionName]
	w.mu.RUnlock()

	if !ok {
		return
	}

	w.updateSession(sessionName, watch)
}

func (w *Watcher) pollLoop() {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.poll()
		}
	}
}

func (w *Watcher) poll() {
	w.mu.RLock()
	sessions := make(map[string]*sessionWatch)
	for name, watch := range w.sessions {
		sessions[name] = watch
	}
	w.mu.RUnlock()

	for name, watch := range sessions {
		w.updateSession(name, watch)
	}
}

func (w *Watcher) updateSession(name string, watch *sessionWatch) {
	info, err := os.Stat(watch.sessionFile)
	if err != nil {
		return
	}

	// Check if file changed
	if info.Size() == watch.lastSize && info.ModTime().Equal(watch.lastMod) {
		return
	}

	// Parse the file
	usage, err := ParseSessionFile(watch.sessionFile)
	if err != nil {
		return
	}

	// Calculate cost
	usage.EstimatedCost = CalculateCost(usage.TotalUsage, usage.Model)

	w.mu.Lock()
	if sw, ok := w.sessions[name]; ok {
		sw.lastSize = info.Size()
		sw.lastMod = info.ModTime()
		sw.usage = usage
	}
	w.mu.Unlock()

	// Notify update
	select {
	case w.updateCh <- name:
	default:
	}
}

// FindActiveSessionFile finds the active JSONL file for a given working directory
func FindActiveSessionFile(workingDir string) (string, error) {
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

	return mostRecent, nil
}

// GetSessionUsage parses and returns usage for a working directory
func GetSessionUsage(workingDir string) (*SessionUsage, error) {
	sessionFile, err := FindActiveSessionFile(workingDir)
	if err != nil {
		return nil, err
	}

	if sessionFile == "" {
		return nil, nil
	}

	usage, err := ParseSessionFile(sessionFile)
	if err != nil {
		return nil, err
	}

	usage.EstimatedCost = CalculateCost(usage.TotalUsage, usage.Model)
	return usage, nil
}

// GetAllSessionsUsage returns usage for all session files in a project directory
func GetAllSessionsUsage(workingDir string) ([]*SessionUsage, error) {
	projectDir, err := FindProjectDir(workingDir)
	if err != nil {
		return nil, err
	}

	files, err := FindSessionFiles(projectDir)
	if err != nil {
		return nil, err
	}

	var results []*SessionUsage
	for _, f := range files {
		usage, err := ParseSessionFile(f)
		if err != nil {
			continue
		}
		usage.EstimatedCost = CalculateCost(usage.TotalUsage, usage.Model)

		// Get file mod time as last updated
		if info, err := os.Stat(f); err == nil {
			usage.LastUpdated = info.ModTime()
		}

		results = append(results, usage)
	}

	// Sort by last updated, most recent first
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].LastUpdated.After(results[i].LastUpdated) {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results, nil
}

// GetMostRecentSession returns the most recently modified session in a project
func GetMostRecentSession(workingDir string) (*SessionUsage, error) {
	sessions, err := GetAllSessionsUsage(workingDir)
	if err != nil {
		return nil, err
	}

	if len(sessions) == 0 {
		return nil, nil
	}

	return sessions[0], nil
}

// TotalProjectUsage returns combined usage for all sessions in a project
func TotalProjectUsage(workingDir string) (*TokenUsage, float64, error) {
	sessions, err := GetAllSessionsUsage(workingDir)
	if err != nil {
		return nil, 0, err
	}

	total := &TokenUsage{}
	var totalCost float64

	for _, s := range sessions {
		total.Add(s.TotalUsage)
		totalCost += s.EstimatedCost
	}

	return total, totalCost, nil
}

// ListAllProjects returns all Claude Code project directories
func ListAllProjects() ([]string, error) {
	baseDir, err := GetClaudeProjectsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, err
	}

	var projects []string
	for _, entry := range entries {
		if entry.IsDir() {
			projects = append(projects, filepath.Join(baseDir, entry.Name()))
		}
	}

	return projects, nil
}
