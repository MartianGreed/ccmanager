package workspace

import (
	"os/exec"
	"path/filepath"
)

type GitProvider struct {
	basePath string
}

func NewGitProvider(basePath string) *GitProvider {
	return &GitProvider{basePath: basePath}
}

func (g *GitProvider) Create(opts CreateOptions) (string, error) {
	workspacePath := filepath.Join(g.basePath, opts.Name)
	args := []string{"worktree", "add"}
	if opts.StartPoint != "" {
		args = append(args, "-b", opts.StartPoint)
	}
	args = append(args, workspacePath)
	if opts.StartPoint != "" {
		args = append(args, "HEAD")
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = opts.SourceRepo
	return workspacePath, cmd.Run()
}

func (g *GitProvider) Delete(sourceRepo, workspacePath string) error {
	cmd := exec.Command("git", "worktree", "remove", workspacePath)
	cmd.Dir = sourceRepo
	return cmd.Run()
}

func (g *GitProvider) IsSupported(repoPath string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = repoPath
	return cmd.Run() == nil
}
