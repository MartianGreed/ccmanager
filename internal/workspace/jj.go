package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
)

type JJProvider struct {
	basePath string
}

func NewJJProvider(basePath string) *JJProvider {
	return &JJProvider{basePath: basePath}
}

func (j *JJProvider) Create(opts CreateOptions) (string, error) {
	workspacePath := filepath.Join(j.basePath, opts.Name)
	args := []string{"workspace", "add"}
	if opts.StartPoint != "" {
		args = append(args, "--revision", opts.StartPoint)
	}
	args = append(args, workspacePath)
	cmd := exec.Command("jj", args...)
	cmd.Dir = opts.SourceRepo
	return workspacePath, cmd.Run()
}

func (j *JJProvider) Delete(sourceRepo, workspacePath string) error {
	forgetCmd := exec.Command("jj", "workspace", "forget", filepath.Base(workspacePath))
	forgetCmd.Dir = sourceRepo
	if err := forgetCmd.Run(); err != nil {
		return err
	}
	return os.RemoveAll(workspacePath)
}

func (j *JJProvider) IsSupported(repoPath string) bool {
	cmd := exec.Command("jj", "root")
	cmd.Dir = repoPath
	return cmd.Run() == nil
}
