package store

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store handles SQLite persistence
type Store struct {
	db *sql.DB
}

// GameState represents the current game state
type GameState struct {
	CurrentScore      int
	LastScoreDate     string
	LastActionAt      *time.Time
	PomodoroState     string
	PomodoroRemaining int
	PomodorosToday    int
}

// DailyStats represents aggregated daily statistics
type DailyStats struct {
	Date               string
	TotalScore         int
	TotalActions       int
	MaxStreak          float64
	PomodorosCompleted int
	FlowTimeSeconds    int
}

// ActivityEntry represents a log entry
type ActivityEntry struct {
	ID          int
	Timestamp   time.Time
	SessionName string
	EventType   string
	Message     string
}

// Session represents a tracked session
type Session struct {
	ID         int
	Name       string
	CreatedAt  time.Time
	LastSeenAt time.Time
}

// New creates a new Store with the database at the given path
func New(dbPath string) (*Store, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	store := &Store{db: db}

	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return store, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema, err := migrationsFS.ReadFile("migrations/001_initial.sql")
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}

	_, err = s.db.Exec(string(schema))
	if err != nil {
		return fmt.Errorf("exec migration: %w", err)
	}

	schema2, err := migrationsFS.ReadFile("migrations/002_workspaces.sql")
	if err != nil {
		return fmt.Errorf("read migration 002: %w", err)
	}

	_, err = s.db.Exec(string(schema2))
	if err != nil {
		return fmt.Errorf("exec migration 002: %w", err)
	}

	schema3, err := migrationsFS.ReadFile("migrations/003_last_score_date.sql")
	if err != nil {
		return fmt.Errorf("read migration 003: %w", err)
	}

	_, _ = s.db.Exec(string(schema3))

	schema4, err := migrationsFS.ReadFile("migrations/004_claude_session_id.sql")
	if err != nil {
		return fmt.Errorf("read migration 004: %w", err)
	}
	_, _ = s.db.Exec(string(schema4))

	return nil
}

// GetGameState retrieves the current game state
func (s *Store) GetGameState() (*GameState, error) {
	var state GameState
	var lastActionAt sql.NullTime
	var lastScoreDate sql.NullString

	err := s.db.QueryRow(`
		SELECT current_score, COALESCE(last_score_date, ''), last_action_at,
		       pomodoro_state, pomodoro_remaining_seconds, pomodoros_today
		FROM game_state WHERE id = 1
	`).Scan(
		&state.CurrentScore,
		&lastScoreDate,
		&lastActionAt,
		&state.PomodoroState,
		&state.PomodoroRemaining,
		&state.PomodorosToday,
	)

	if err != nil {
		return nil, fmt.Errorf("get game state: %w", err)
	}

	if lastActionAt.Valid {
		state.LastActionAt = &lastActionAt.Time
	}
	if lastScoreDate.Valid {
		state.LastScoreDate = lastScoreDate.String
	}

	return &state, nil
}

// UpdateGameState updates the game state
func (s *Store) UpdateGameState(state *GameState) error {
	_, err := s.db.Exec(`
		UPDATE game_state SET
			current_score = ?,
			last_score_date = ?,
			last_action_at = ?,
			pomodoro_state = ?,
			pomodoro_remaining_seconds = ?,
			pomodoros_today = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`,
		state.CurrentScore,
		state.LastScoreDate,
		state.LastActionAt,
		state.PomodoroState,
		state.PomodoroRemaining,
		state.PomodorosToday,
	)

	if err != nil {
		return fmt.Errorf("update game state: %w", err)
	}

	return nil
}

// GetControlGroups retrieves all control group assignments
func (s *Store) GetControlGroups() (map[int]string, error) {
	rows, err := s.db.Query(`
		SELECT group_num, session_name FROM control_groups ORDER BY group_num
	`)
	if err != nil {
		return nil, fmt.Errorf("get control groups: %w", err)
	}
	defer func() { _ = rows.Close() }()

	groups := make(map[int]string)
	for rows.Next() {
		var groupNum int
		var sessionName string
		if err := rows.Scan(&groupNum, &sessionName); err != nil {
			return nil, fmt.Errorf("scan control group: %w", err)
		}
		groups[groupNum] = sessionName
	}

	return groups, rows.Err()
}

// SetControlGroup assigns a session to a control group
func (s *Store) SetControlGroup(groupNum int, sessionName string) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO control_groups (group_num, session_name)
		VALUES (?, ?)
	`, groupNum, sessionName)

	if err != nil {
		return fmt.Errorf("set control group: %w", err)
	}

	return nil
}

// RemoveFromControlGroup removes a session from a control group
func (s *Store) RemoveFromControlGroup(groupNum int, sessionName string) error {
	_, err := s.db.Exec(`
		DELETE FROM control_groups WHERE group_num = ? AND session_name = ?
	`, groupNum, sessionName)

	if err != nil {
		return fmt.Errorf("remove from control group: %w", err)
	}

	return nil
}

// GetTodayStats retrieves today's statistics
func (s *Store) GetTodayStats() (*DailyStats, error) {
	today := time.Now().Format("2006-01-02")

	var stats DailyStats
	err := s.db.QueryRow(`
		SELECT date, total_score, total_actions, max_streak,
		       pomodoros_completed, flow_time_seconds
		FROM daily_stats WHERE date = ?
	`, today).Scan(
		&stats.Date,
		&stats.TotalScore,
		&stats.TotalActions,
		&stats.MaxStreak,
		&stats.PomodorosCompleted,
		&stats.FlowTimeSeconds,
	)

	if err == sql.ErrNoRows {
		// Create today's entry
		_, err = s.db.Exec(`
			INSERT INTO daily_stats (date) VALUES (?)
		`, today)
		if err != nil {
			return nil, fmt.Errorf("create today stats: %w", err)
		}
		return &DailyStats{Date: today}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("get today stats: %w", err)
	}

	return &stats, nil
}

// UpdateTodayStats updates today's statistics
func (s *Store) UpdateTodayStats(stats *DailyStats) error {
	_, err := s.db.Exec(`
		INSERT INTO daily_stats (date, total_score, total_actions, max_streak,
		                         pomodoros_completed, flow_time_seconds)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(date) DO UPDATE SET
			total_score = excluded.total_score,
			total_actions = excluded.total_actions,
			max_streak = excluded.max_streak,
			pomodoros_completed = excluded.pomodoros_completed,
			flow_time_seconds = excluded.flow_time_seconds,
			updated_at = CURRENT_TIMESTAMP
	`,
		stats.Date,
		stats.TotalScore,
		stats.TotalActions,
		stats.MaxStreak,
		stats.PomodorosCompleted,
		stats.FlowTimeSeconds,
	)

	if err != nil {
		return fmt.Errorf("update today stats: %w", err)
	}

	return nil
}

// AddActivityLog adds an entry to the activity log
func (s *Store) AddActivityLog(sessionName, eventType, message string) error {
	_, err := s.db.Exec(`
		INSERT INTO activity_log (session_name, event_type, message)
		VALUES (?, ?, ?)
	`, sessionName, eventType, message)

	if err != nil {
		return fmt.Errorf("add activity log: %w", err)
	}

	return nil
}

// GetRecentActivity retrieves recent activity log entries
func (s *Store) GetRecentActivity(limit int) ([]ActivityEntry, error) {
	rows, err := s.db.Query(`
		SELECT id, timestamp, session_name, event_type, message
		FROM activity_log
		ORDER BY timestamp DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("get recent activity: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []ActivityEntry
	for rows.Next() {
		var entry ActivityEntry
		var sessionName sql.NullString
		if err := rows.Scan(&entry.ID, &entry.Timestamp, &sessionName,
			&entry.EventType, &entry.Message); err != nil {
			return nil, fmt.Errorf("scan activity: %w", err)
		}
		if sessionName.Valid {
			entry.SessionName = sessionName.String
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// CreateSession creates or updates a session (upsert)
func (s *Store) CreateSession(name string) error {
	_, err := s.db.Exec(`
		INSERT INTO sessions (name)
		VALUES (?)
		ON CONFLICT(name) DO UPDATE SET
			last_seen_at = CURRENT_TIMESTAMP
	`, name)

	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	return nil
}

// GetAllSessions retrieves all sessions
func (s *Store) GetAllSessions() ([]Session, error) {
	rows, err := s.db.Query(`
		SELECT id, name, created_at, last_seen_at
		FROM sessions
		ORDER BY last_seen_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("get all sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var sessions []Session
	for rows.Next() {
		var session Session
		if err := rows.Scan(&session.ID, &session.Name, &session.CreatedAt, &session.LastSeenAt); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sessions = append(sessions, session)
	}

	return sessions, rows.Err()
}

// GetSession retrieves a single session by name
func (s *Store) GetSession(name string) (*Session, error) {
	var session Session
	err := s.db.QueryRow(`
		SELECT id, name, created_at, last_seen_at
		FROM sessions WHERE name = ?
	`, name).Scan(&session.ID, &session.Name, &session.CreatedAt, &session.LastSeenAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	return &session, nil
}

// UpdateSessionLastSeen updates the last_seen_at timestamp
func (s *Store) UpdateSessionLastSeen(name string) error {
	_, err := s.db.Exec(`
		UPDATE sessions SET last_seen_at = CURRENT_TIMESTAMP WHERE name = ?
	`, name)

	if err != nil {
		return fmt.Errorf("update session last seen: %w", err)
	}

	return nil
}

// DeleteSession deletes a session by name
func (s *Store) DeleteSession(name string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE name = ?`, name)

	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

// AddRecentPath adds or updates a recently used path
func (s *Store) AddRecentPath(path string) error {
	_, err := s.db.Exec(`
		INSERT INTO recent_paths (path, last_used)
		VALUES (?, CURRENT_TIMESTAMP)
		ON CONFLICT(path) DO UPDATE SET
			last_used = CURRENT_TIMESTAMP
	`, path)

	if err != nil {
		return fmt.Errorf("add recent path: %w", err)
	}

	return nil
}

// GetRecentPaths retrieves recently used paths
func (s *Store) GetRecentPaths(limit int) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT path FROM recent_paths
		ORDER BY last_used DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("get recent paths: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, fmt.Errorf("scan recent path: %w", err)
		}
		paths = append(paths, path)
	}

	return paths, rows.Err()
}

func (s *Store) SaveSessionWorkspace(sessionName, workspacePath, sourceRepo string) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO session_workspaces (session_name, workspace_path, source_repo)
		VALUES (?, ?, ?)
	`, sessionName, workspacePath, sourceRepo)

	if err != nil {
		return fmt.Errorf("save session workspace: %w", err)
	}

	return nil
}

func (s *Store) GetSessionWorkspace(sessionName string) (path, sourceRepo string, err error) {
	err = s.db.QueryRow(`
		SELECT workspace_path, source_repo FROM session_workspaces WHERE session_name = ?
	`, sessionName).Scan(&path, &sourceRepo)

	if err == sql.ErrNoRows {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("get session workspace: %w", err)
	}

	return path, sourceRepo, nil
}

func (s *Store) DeleteSessionWorkspace(sessionName string) error {
	_, err := s.db.Exec(`DELETE FROM session_workspaces WHERE session_name = ?`, sessionName)

	if err != nil {
		return fmt.Errorf("delete session workspace: %w", err)
	}

	return nil
}

func (s *Store) SetClaudeSessionID(sessionName, claudeSessionID string) error {
	_, err := s.db.Exec(`
		UPDATE sessions SET claude_session_id = ? WHERE name = ?
	`, claudeSessionID, sessionName)
	if err != nil {
		return fmt.Errorf("set claude session id: %w", err)
	}
	return nil
}

func (s *Store) GetClaudeSessionID(sessionName string) (string, error) {
	var id sql.NullString
	err := s.db.QueryRow(`
		SELECT claude_session_id FROM sessions WHERE name = ?
	`, sessionName).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get claude session id: %w", err)
	}
	return id.String, nil
}
