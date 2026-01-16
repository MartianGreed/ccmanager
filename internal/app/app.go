package app

import (
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/valentindosimont/ccmanager/internal/config"
	"github.com/valentindosimont/ccmanager/internal/daemon"
	"github.com/valentindosimont/ccmanager/internal/game"
	"github.com/valentindosimont/ccmanager/internal/store"
	"github.com/valentindosimont/ccmanager/internal/tui"
	"github.com/valentindosimont/ccmanager/internal/workspace"
)

// Config holds application configuration
type Config struct {
	DBPath       string
	PollInterval time.Duration
	GameConfig   game.EngineConfig
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	homeDir, _ := os.UserHomeDir()
	return Config{
		DBPath:       filepath.Join(homeDir, ".config", "ccmanager", "ccmanager.db"),
		PollInterval: 500 * time.Millisecond,
		GameConfig:   game.DefaultEngineConfig(),
	}
}

// LoadConfig loads configuration from file and merges with defaults
func LoadConfig() (Config, *config.Config) {
	cfg := DefaultConfig()
	fileCfg, err := config.Load(config.DefaultPath())
	if err != nil {
		return cfg, nil
	}
	cfg.PollInterval = fileCfg.PollInterval()
	cfg.GameConfig = game.EngineConfig{
		APMWindowSeconds:          fileCfg.APM.WindowSeconds,
		StreakTimeoutSeconds:      fileCfg.Streak.TimeoutSeconds,
		StreakMultiplierCap:       fileCfg.Streak.MultiplierCap,
		PomodoroWorkMinutes:       fileCfg.Pomodoro.WorkMinutes,
		PomodoroShortBreakMinutes: fileCfg.Pomodoro.ShortBreakMinutes,
		PomodoroLongBreakMinutes:  fileCfg.Pomodoro.LongBreakMinutes,
		PomodorosBeforeLongBreak:  fileCfg.Pomodoro.SessionsBeforeLongBreak,
		PomodoroMultiplier:        fileCfg.Pomodoro.Multiplier,
		FocusBonusMinutes:         fileCfg.Focus.BonusMinutes,
		FocusBonusMultiplier:      fileCfg.Focus.BonusMultiplier,
		PointsAction:              fileCfg.Scoring.PointsPerAction,
		PointsTaskComplete:        fileCfg.Scoring.PointsTaskComplete,
		PointsUrgentHandled:       fileCfg.Scoring.PointsUrgentHandled,
		PointsPomodoroComplete:    fileCfg.Scoring.PointsPomodoroComplete,
		DoubleTapThresholdMs:      fileCfg.UI.DoubleTapThresholdMs,
	}
	return cfg, fileCfg
}

// App is the main application
type App struct {
	config     Config
	fileConfig *config.Config
	store      *store.Store
	monitor    *daemon.Monitor
	engine     *game.Engine
	wsMgr      *workspace.Manager
}

// New creates a new App
func New(cfg Config, fileCfg *config.Config) (*App, error) {
	// Initialize store
	st, err := store.New(cfg.DBPath)
	if err != nil {
		return nil, err
	}

	// Initialize monitor
	monitor := daemon.NewMonitor(cfg.PollInterval)

	// Initialize game engine
	engine := game.NewEngine(cfg.GameConfig)

	// Load persisted state
	if gameState, err := st.GetGameState(); err == nil {
		engine.LoadState(
			gameState.CurrentScore,
			gameState.LastScoreDate,
			gameState.PomodoroState,
			gameState.PomodoroRemaining,
		)
	}

	// Load control groups
	if groups, err := st.GetControlGroups(); err == nil {
		engine.ControlGroups().Load(groups)
	}

	// Initialize workspace manager
	var wsMgr *workspace.Manager
	if fileCfg != nil {
		wsMgr, _ = workspace.NewManager(&fileCfg.Workspace)
	}

	return &App{
		config:     cfg,
		fileConfig: fileCfg,
		store:      st,
		monitor:    monitor,
		engine:     engine,
		wsMgr:      wsMgr,
	}, nil
}

// Run starts the application
func (a *App) Run() error {
	// Start monitor
	a.monitor.Start()
	defer a.monitor.Stop()

	// Create TUI model
	model := tui.New(a.monitor, a.engine, a.store, a.fileConfig, a.wsMgr)

	// Run Bubbletea
	p := tea.NewProgram(model, tea.WithAltScreen())

	_, err := p.Run()
	if err != nil {
		return err
	}

	// Save state on exit
	a.saveState()

	return nil
}

// Close cleans up resources
func (a *App) Close() error {
	a.saveState()
	return a.store.Close()
}

func (a *App) saveState() {
	score, lastScoreDate, pomodoroState, pomodoroRemaining := a.engine.State()

	now := time.Now()
	a.store.UpdateGameState(&store.GameState{
		CurrentScore:      score,
		LastScoreDate:     lastScoreDate,
		LastActionAt:      &now,
		PomodoroState:     pomodoroState,
		PomodoroRemaining: pomodoroRemaining,
	})

	// Save control groups
	for groupNum, session := range a.engine.ControlGroups().All() {
		a.store.SetControlGroup(groupNum, session)
	}
}
