package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mxcd/updater/internal/configuration"
	"github.com/rs/zerolog/log"
)

// NewRepository creates a new repository instance
func NewRepository(workingDirectory string, targetActor *configuration.TargetActor) *Repository {
	return &Repository{
		WorkingDirectory: workingDirectory,
		TargetActor:      targetActor,
	}
}

// DetectRepository detects git repository information from a file path
func (r *Repository) DetectRepository(filePath string) error {
	log.Debug().Str("file", filePath).Msg("Detecting git repository for file")

	// Get absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Find git root
	gitRoot, err := r.findGitRoot(absPath)
	if err != nil {
		return fmt.Errorf("failed to find git root: %w", err)
	}

	r.WorkingDirectory = gitRoot
	log.Debug().Str("gitRoot", gitRoot).Msg("Found git repository root")

	// Get remote URL
	remoteURL, err := r.getRemoteURL()
	if err != nil {
		return fmt.Errorf("failed to get remote URL: %w", err)
	}

	r.RepoURL = remoteURL
	log.Debug().Str("remoteURL", remoteURL).Msg("Found remote URL")

	// Get current branch
	currentBranch, err := r.getCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	r.BaseBranch = currentBranch
	log.Debug().Str("branch", currentBranch).Msg("Found current branch")

	return nil
}

// findGitRoot finds the root directory of a git repository
func (r *Repository) findGitRoot(startPath string) (string, error) {
	dir := startPath
	if !isDirectory(startPath) {
		dir = filepath.Dir(startPath)
	}

	for {
		gitDir := filepath.Join(dir, ".git")
		if exists(gitDir) {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not a git repository (or any parent up to mount point)")
		}
		dir = parent
	}
}

// getRemoteURL gets the remote URL for origin
func (r *Repository) getRemoteURL() (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = r.WorkingDirectory

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get remote URL: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// getCurrentBranch gets the current branch name
func (r *Repository) getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = r.WorkingDirectory

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// CreateBranch creates a new branch
func (r *Repository) CreateBranch(branchName string) error {
	log.Debug().
		Str("branch", branchName).
		Str("baseBranch", r.BaseBranch).
		Msg("Creating new branch")

	// Ensure we're on the base branch
	if err := r.CheckoutBranch(r.BaseBranch); err != nil {
		return fmt.Errorf("failed to checkout base branch: %w", err)
	}

	// Pull latest changes
	if err := r.pull(); err != nil {
		log.Warn().Err(err).Msg("Failed to pull latest changes, continuing anyway")
	}

	// Create and checkout new branch
	cmd := exec.Command("git", "checkout", "-b", branchName)
	cmd.Dir = r.WorkingDirectory

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create branch: %w, output: %s", err, string(output))
	}

	r.BranchName = branchName
	log.Debug().Str("branch", branchName).Msg("Created and checked out new branch")

	return nil
}

// CheckoutOrCreateBranch checks out an existing branch or creates it if it doesn't exist
func (r *Repository) CheckoutOrCreateBranch(branchName string) (bool, error) {
	log.Debug().
		Str("branch", branchName).
		Str("baseBranch", r.BaseBranch).
		Msg("Checking out or creating branch")

	// Ensure we're on the base branch first
	if err := r.CheckoutBranch(r.BaseBranch); err != nil {
		return false, fmt.Errorf("failed to checkout base branch: %w", err)
	}

	// Pull latest changes from base branch
	if err := r.pull(); err != nil {
		log.Warn().Err(err).Msg("Failed to pull latest changes from base branch, continuing anyway")
	}

	// Try to fetch the branch from remote
	remoteBranchExists := r.fetchBranch(branchName) == nil

	// Check if branch exists locally
	branchExistsLocally := r.CheckoutBranch(branchName) == nil

	if branchExistsLocally {
		r.BranchName = branchName
		log.Debug().Str("branch", branchName).Msg("Checked out existing local branch")

		if remoteBranchExists {
			// Pull latest changes from the remote branch
			if err := r.pullFromRemote(branchName); err != nil {
				log.Warn().Err(err).Msg("Failed to pull latest changes from remote branch, continuing anyway")
			}
			log.Debug().Str("branch", branchName).Msg("Pulled latest changes from remote branch")
		} else {
			// Remote branch doesn't exist, reset to base branch to get latest changes
			if err := r.resetToBaseBranch(); err != nil {
				log.Warn().Err(err).Msg("Failed to reset branch to base, continuing anyway")
			}
		}

		return true, nil
	}

	// Branch doesn't exist locally
	if remoteBranchExists {
		// Create local branch tracking the remote branch
		cmd := exec.Command("git", "checkout", "-b", branchName, fmt.Sprintf("origin/%s", branchName))
		cmd.Dir = r.WorkingDirectory

		output, err := cmd.CombinedOutput()
		if err != nil {
			return false, fmt.Errorf("failed to checkout remote branch: %w, output: %s", err, string(output))
		}

		r.BranchName = branchName
		log.Debug().Str("branch", branchName).Msg("Checked out branch from remote")

		return true, nil
	}

	// Branch doesn't exist locally or remotely, create it from base branch
	cmd := exec.Command("git", "checkout", "-b", branchName)
	cmd.Dir = r.WorkingDirectory

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to create branch: %w, output: %s", err, string(output))
	}

	r.BranchName = branchName
	log.Debug().Str("branch", branchName).Msg("Created new branch")

	return false, nil
}

// fetchBranch attempts to fetch a branch from remote
func (r *Repository) fetchBranch(branchName string) error {
	cmd := exec.Command("git", "fetch", "origin", branchName)
	cmd.Dir = r.WorkingDirectory

	output, err := cmd.CombinedOutput()
	if err != nil {
		// It's okay if fetch fails (branch might not exist on remote)
		log.Debug().Err(err).Str("output", string(output)).Msg("Failed to fetch branch from remote")
		return err
	}

	return nil
}

// resetToBaseBranch resets the current branch to match the base branch
func (r *Repository) resetToBaseBranch() error {
	cmd := exec.Command("git", "reset", "--hard", fmt.Sprintf("origin/%s", r.BaseBranch))
	cmd.Dir = r.WorkingDirectory

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to reset branch: %w, output: %s", err, string(output))
	}

	return nil
}

// CheckoutBranch checks out an existing branch
func (r *Repository) CheckoutBranch(branchName string) error {
	cmd := exec.Command("git", "checkout", branchName)
	cmd.Dir = r.WorkingDirectory

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to checkout branch %s: %w, output: %s", branchName, err, string(output))
	}

	return nil
}

// pull pulls latest changes from remote for the current branch
func (r *Repository) pull() error {
	// Get current branch name
	currentBranch, err := r.getCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	return r.pullFromRemote(currentBranch)
}

// pullFromRemote pulls latest changes from a specific remote branch
func (r *Repository) pullFromRemote(branchName string) error {
	// Pull with explicit remote and branch to avoid tracking issues
	cmd := exec.Command("git", "pull", "origin", branchName)
	cmd.Dir = r.WorkingDirectory

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to pull: %w, output: %s", err, string(output))
	}

	return nil
}

// Commit creates a commit with the specified changes
func (r *Repository) Commit(options *CommitOptions) error {
	log.Debug().
		Str("message", options.Message).
		Int("files", len(options.Files)).
		Msg("Creating commit")

	if r.TargetActor == nil {
		return fmt.Errorf("target actor not configured")
	}

	// Stage files
	for _, file := range options.Files {
		if err := r.stageFile(file); err != nil {
			return fmt.Errorf("failed to stage file %s: %w", file, err)
		}
	}

	// Commit with environment variables to avoid persisting git config changes
	cmd := exec.Command("git", "commit", "-m", options.Message)
	cmd.Dir = r.WorkingDirectory
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("GIT_AUTHOR_NAME=%s", r.TargetActor.Name),
		fmt.Sprintf("GIT_AUTHOR_EMAIL=%s", r.TargetActor.Email),
		fmt.Sprintf("GIT_COMMITTER_NAME=%s", r.TargetActor.Name),
		fmt.Sprintf("GIT_COMMITTER_EMAIL=%s", r.TargetActor.Email),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to commit: %w, output: %s", err, string(output))
	}

	log.Debug().Str("message", options.Message).Msg("Created commit")

	return nil
}

// stageFile stages a file for commit
func (r *Repository) stageFile(filePath string) error {
	cmd := exec.Command("git", "add", filePath)
	cmd.Dir = r.WorkingDirectory

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stage file: %w, output: %s", err, string(output))
	}

	return nil
}

// Push pushes the current branch to remote
func (r *Repository) Push() error {
	log.Debug().Str("branch", r.BranchName).Msg("Pushing branch to remote")

	cmd := exec.Command("git", "push", "-u", "origin", r.BranchName)
	cmd.Dir = r.WorkingDirectory

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to push: %w, output: %s", err, string(output))
	}

	log.Debug().Str("branch", r.BranchName).Msg("Pushed branch to remote")

	return nil
}

// ForcePush force pushes the current branch to remote
func (r *Repository) ForcePush() error {
	log.Debug().Str("branch", r.BranchName).Msg("Force pushing branch to remote")

	cmd := exec.Command("git", "push", "-f", "origin", r.BranchName)
	cmd.Dir = r.WorkingDirectory

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to force push: %w, output: %s", err, string(output))
	}

	log.Debug().Str("branch", r.BranchName).Msg("Force pushed branch to remote")

	return nil
}

// isDirectory checks if a path is a directory
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// exists checks if a path exists
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// HasUncommittedChanges checks if there are uncommitted changes in the working directory
func (r *Repository) HasUncommittedChanges() (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = r.WorkingDirectory

	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}

	return len(strings.TrimSpace(string(output))) > 0, nil
}

// HasUnpushedCommits checks if there are commits that haven't been pushed to remote
func (r *Repository) HasUnpushedCommits() (bool, error) {
	// First check if the remote branch exists
	cmd := exec.Command("git", "rev-parse", "--verify", fmt.Sprintf("origin/%s", r.BranchName))
	cmd.Dir = r.WorkingDirectory

	if err := cmd.Run(); err != nil {
		// Remote branch doesn't exist, so we have unpushed commits if we have any commits
		return r.hasLocalCommits()
	}

	// Remote branch exists, check if we're ahead
	cmd = exec.Command("git", "rev-list", "--count", fmt.Sprintf("origin/%s..%s", r.BranchName, r.BranchName))
	cmd.Dir = r.WorkingDirectory

	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check unpushed commits: %w", err)
	}

	count := strings.TrimSpace(string(output))
	return count != "0", nil
}

// hasLocalCommits checks if the current branch has any commits
func (r *Repository) hasLocalCommits() (bool, error) {
	cmd := exec.Command("git", "rev-list", "--count", r.BranchName)
	cmd.Dir = r.WorkingDirectory

	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check local commits: %w", err)
	}

	count := strings.TrimSpace(string(output))
	return count != "0", nil
}

// GetLastCommitMessage gets the last commit message on the current branch
func (r *Repository) GetLastCommitMessage() (string, error) {
	cmd := exec.Command("git", "log", "-1", "--pretty=%B")
	cmd.Dir = r.WorkingDirectory

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get last commit message: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}
