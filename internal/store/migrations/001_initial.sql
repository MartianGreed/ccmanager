-- CCManager Initial Schema

-- Sessions table: track known sessions
CREATE TABLE IF NOT EXISTS sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Control groups: map hotkeys to sessions (one session per group)
CREATE TABLE IF NOT EXISTS control_groups (
    group_num INTEGER PRIMARY KEY,
    session_name TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Daily stats: aggregated per day
CREATE TABLE IF NOT EXISTS daily_stats (
    date TEXT NOT NULL PRIMARY KEY,
    total_score INTEGER DEFAULT 0,
    total_actions INTEGER DEFAULT 0,
    max_streak REAL DEFAULT 1.0,
    pomodoros_completed INTEGER DEFAULT 0,
    flow_time_seconds INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Achievements: unlocked achievements
CREATE TABLE IF NOT EXISTS achievements (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    unlocked_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Game state: current game state (single row)
CREATE TABLE IF NOT EXISTS game_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    current_score INTEGER DEFAULT 0,
    current_streak_count INTEGER DEFAULT 0,
    last_action_at DATETIME,
    pomodoro_state TEXT DEFAULT 'stopped',
    pomodoro_remaining_seconds INTEGER DEFAULT 0,
    pomodoros_today INTEGER DEFAULT 0,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Initialize game state with default row
INSERT OR IGNORE INTO game_state (id) VALUES (1);

-- Activity log: recent events
CREATE TABLE IF NOT EXISTS activity_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    session_name TEXT,
    event_type TEXT NOT NULL,
    message TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Index for activity log queries
CREATE INDEX IF NOT EXISTS idx_activity_log_timestamp ON activity_log(timestamp DESC);

-- Index for daily stats
CREATE INDEX IF NOT EXISTS idx_daily_stats_date ON daily_stats(date DESC);

-- Recent paths: track recently used directories for session creation
CREATE TABLE IF NOT EXISTS recent_paths (
    path TEXT PRIMARY KEY,
    last_used DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Index for recent paths by usage
CREATE INDEX IF NOT EXISTS idx_recent_paths_last_used ON recent_paths(last_used DESC);
