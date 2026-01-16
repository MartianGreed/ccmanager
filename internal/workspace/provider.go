package workspace

type CreateOptions struct {
	SourceRepo string
	Name       string
	StartPoint string // branch for git, revision for jj
}

type Provider interface {
	Create(opts CreateOptions) (workspacePath string, err error)
	Delete(sourceRepo, workspacePath string) error
	IsSupported(repoPath string) bool
}
