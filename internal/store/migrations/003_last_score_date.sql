-- Add last_score_date column for daily score reset tracking
ALTER TABLE game_state ADD COLUMN last_score_date TEXT DEFAULT '';
