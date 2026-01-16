package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/valentindosimont/ccmanager/internal/config"
)

type Manager struct {
	cfg         *config.WorkspaceConfig
	basePath    string
	gitProvider *GitProvider
	jjProvider  *JJProvider
}

func NewManager(cfg *config.WorkspaceConfig) (*Manager, error) {
	basePath := expandHome(cfg.BasePath)
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("create workspace base path: %w", err)
	}

	return &Manager{
		cfg:         cfg,
		basePath:    basePath,
		gitProvider: NewGitProvider(basePath),
		jjProvider:  NewJJProvider(basePath),
	}, nil
}

func (m *Manager) detectVCS(repoPath string) string {
	if _, err := os.Stat(filepath.Join(repoPath, ".jj")); err == nil {
		return "jj"
	}
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err == nil {
		return "git"
	}
	return ""
}

func (m *Manager) getProvider(repoPath string) (Provider, string) {
	strategy := m.cfg.Strategy
	if strategy == "auto" || strategy == "" {
		strategy = m.detectVCS(repoPath)
	}
	switch strategy {
	case "jj":
		return m.jjProvider, "jj"
	default:
		return m.gitProvider, "git"
	}
}

func (m *Manager) CreateWorkspace(sourceRepo, name string) (string, error) {
	return m.CreateWorkspaceWithOptions(CreateOptions{
		SourceRepo: sourceRepo,
		Name:       name,
	})
}

func (m *Manager) CreateWorkspaceWithOptions(opts CreateOptions) (string, error) {
	provider, strategy := m.getProvider(opts.SourceRepo)
	if !provider.IsSupported(opts.SourceRepo) {
		return "", fmt.Errorf("repository not supported by %s provider", strategy)
	}
	return provider.Create(opts)
}

func (m *Manager) DeleteWorkspace(sourceRepo, path string) error {
	provider, _ := m.getProvider(sourceRepo)
	return provider.Delete(sourceRepo, path)
}

func (m *Manager) IsSupported(repoPath string) bool {
	provider, _ := m.getProvider(repoPath)
	return provider.IsSupported(repoPath)
}

func (m *Manager) Strategy() string {
	return m.cfg.Strategy
}

func (m *Manager) DetectedStrategy(repoPath string) string {
	_, strategy := m.getProvider(repoPath)
	return strategy
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
