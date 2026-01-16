CREATE TABLE IF NOT EXISTS session_workspaces (
    session_name TEXT PRIMARY KEY,
    workspace_path TEXT NOT NULL,
    source_repo TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
