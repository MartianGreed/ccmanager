package config

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type PomodoroConfig struct {
	WorkMinutes             int     `yaml:"work_minutes"`
	ShortBreakMinutes       int     `yaml:"short_break_minutes"`
	LongBreakMinutes        int     `yaml:"long_break_minutes"`
	SessionsBeforeLongBreak int     `yaml:"sessions_before_long_break"`
	Multiplier              float64 `yaml:"multiplier"`
}

type StreakConfig struct {
	TimeoutSeconds int     `yaml:"timeout_seconds"`
	MultiplierCap  float64 `yaml:"multiplier_cap"`
}

type ScoringConfig struct {
	PointsPerAction        int `yaml:"points_per_action"`
	PointsTaskComplete     int `yaml:"points_task_complete"`
	PointsUrgentHandled    int `yaml:"points_urgent_handled"`
	PointsPomodoroComplete int `yaml:"points_pomodoro_complete"`
}

type FocusConfig struct {
	BonusMinutes    int     `yaml:"bonus_minutes"`
	BonusMultiplier float64 `yaml:"bonus_multiplier"`
}

type APMConfig struct {
	WindowSeconds int `yaml:"window_seconds"`
}

type MonitorConfig struct {
	PollIntervalMs int `yaml:"poll_interval_ms"`
}

type UIConfig struct {
	DoubleTapThresholdMs int    `yaml:"double_tap_threshold_ms"`
	NewlineSequence      string `yaml:"newline_sequence"`
	SessionListWidthPct  int    `yaml:"session_list_width_pct"`
	Editor               string `yaml:"editor"`
	DefaultMode          string `yaml:"default_mode"`
}

type WorkspaceConfig struct {
	Strategy string `yaml:"strategy"`
	BasePath string `yaml:"base_path"`
}

type Config struct {
	Pomodoro     PomodoroConfig  `yaml:"pomodoro"`
	Streak       StreakConfig    `yaml:"streak"`
	Scoring      ScoringConfig   `yaml:"scoring"`
	Focus        FocusConfig     `yaml:"focus"`
	APM          APMConfig       `yaml:"apm"`
	Monitor      MonitorConfig   `yaml:"monitor"`
	UI           UIConfig        `yaml:"ui"`
	Workspace    WorkspaceConfig `yaml:"workspace"`
	SessionPaths []string        `yaml:"session_paths"`
}

func Default() *Config {
	return &Config{
		Pomodoro: PomodoroConfig{
			WorkMinutes:             25,
			ShortBreakMinutes:       5,
			LongBreakMinutes:        15,
			SessionsBeforeLongBreak: 4,
			Multiplier:              1.5,
		},
		Streak: StreakConfig{
			TimeoutSeconds: 30,
			MultiplierCap:  10.0,
		},
		Scoring: ScoringConfig{
			PointsPerAction:        10,
			PointsTaskComplete:     100,
			PointsUrgentHandled:    500,
			PointsPomodoroComplete: 1000,
		},
		Focus: FocusConfig{
			BonusMinutes:    5,
			BonusMultiplier: 1.2,
		},
		APM: APMConfig{
			WindowSeconds: 60,
		},
		Monitor: MonitorConfig{
			PollIntervalMs: 500,
		},
		UI: UIConfig{
			DoubleTapThresholdMs: 300,
			NewlineSequence:      "\\",
			SessionListWidthPct:  35,
			Editor:               "nvim",
		},
		Workspace: WorkspaceConfig{
			Strategy: "git",
			BasePath: "~/worktrees",
		},
	}
}

func DefaultPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".config", "ccmanager", "config.yaml")
}

func Load(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) PollInterval() time.Duration {
	return time.Duration(c.Monitor.PollIntervalMs) * time.Millisecond
}

func (c *Config) DoubleTapThreshold() time.Duration {
	return time.Duration(c.UI.DoubleTapThresholdMs) * time.Millisecond
}
