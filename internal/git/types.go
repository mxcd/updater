package git

import "github.com/mxcd/updater/internal/configuration"

// Repository represents a git repository
type Repository struct {
	WorkingDirectory string
	TargetActor      *configuration.TargetActor
	RepoURL          string
	BaseBranch       string
	BranchName       string
}

// CommitOptions represents options for creating a commit
type CommitOptions struct {
	Message string
	Files   []string
}

// PullRequestOptions represents options for creating a pull request
type PullRequestOptions struct {
	Title       string
	Body        string
	BaseBranch  string
	HeadBranch  string
	Labels      []string
	PatchGroup  string
}

// FileChange represents a change to be committed
type FileChange struct {
	FilePath string
	Content  string
}