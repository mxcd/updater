package actions

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/mxcd/updater/internal/configuration"
	"github.com/mxcd/updater/internal/git"
	"github.com/mxcd/updater/internal/target"
	"github.com/rs/zerolog/log"
)

// applyPatchGroups applies all patch groups
func applyPatchGroups(config *configuration.Config, patchGroups []*PatchGroup) error {
	log.Debug().Int("groups", len(patchGroups)).Msg("Applying patch groups")

	for i, group := range patchGroups {
		fmt.Printf("\n📦 Processing Patch Group %d/%d: %s\n", i+1, len(patchGroups), group.Name)

		if err := applyPatchGroup(config, group); err != nil {
			return fmt.Errorf("failed to apply patch group %s: %w", group.Name, err)
		}

		fmt.Printf("✅ Completed patch group: %s\n", group.Name)
	}

	return nil
}

// applyPatchGroup applies a single patch group
func applyPatchGroup(config *configuration.Config, group *PatchGroup) error {
	// Group updates by file
	fileGroups := groupUpdatesByFile(group.Updates)

	// Track repository and branch info (should be same for all files in group)
	var repo *git.Repository
	var branchExists bool
	var branchPushed bool
	var prURL string

	// Sort file paths for deterministic processing order
	filePaths := make([]string, 0, len(fileGroups))
	for filePath := range fileGroups {
		filePaths = append(filePaths, filePath)
	}
	sort.Strings(filePaths)

	// Process each file separately
	fileIndex := 0
	totalFiles := len(fileGroups)
	for _, filePath := range filePaths {
		updates := fileGroups[filePath]
		fileIndex++
		isLastFile := fileIndex == totalFiles

		// Pass whether this is the last file so PR is only created once
		fileRepo, fileBranchExists, fileBranchPushed, err := applyFileUpdates(config, filePath, updates, group, isLastFile)
		if err != nil {
			return fmt.Errorf("failed to apply updates to file %s: %w", filePath, err)
		}

		// Store repo and branch info from first file
		if repo == nil {
			repo = fileRepo
			branchExists = fileBranchExists
		}
		// Track if branch was pushed in any file processing
		if fileBranchPushed {
			branchPushed = true
		}
	}

	// Create or update pull request after all files are processed
	// Only create PR if the branch was actually pushed to remote
	if repo != nil && branchPushed {
		var err error
		prURL, err = createOrUpdatePullRequest(repo, config.TargetActor, group, group.Updates, branchExists)
		if err != nil {
			return fmt.Errorf("failed to create or update pull request: %w", err)
		}

		if branchExists {
			fmt.Printf("  🔄 Updated pull request: %s\n", prURL)
		} else {
			fmt.Printf("  🔀 Created pull request: %s\n", prURL)
		}
	} else if repo != nil && !branchPushed {
		fmt.Printf("  ℹ️  No changes to push, skipping PR creation\n")
	}

	return nil
}

// applyFileUpdates applies updates to a single file and returns the repository, branch status, and whether branch was pushed
func applyFileUpdates(config *configuration.Config, filePath string, updates []*UpdateItem, group *PatchGroup, isLastFile bool) (repo *git.Repository, branchExists bool, branchPushed bool, err error) {
	log.Debug().
		Str("file", filePath).
		Int("updates", len(updates)).
		Msg("Applying updates to file")

	// Create repository instance
	repo = git.NewRepository("", config.TargetActor)

	// Detect git repository from file path
	if err = repo.DetectRepository(filePath); err != nil {
		return nil, false, false, fmt.Errorf("failed to detect git repository: %w", err)
	}

	// Ensure we always checkout back to the base branch on error
	defer func() {
		if err != nil {
			if checkoutErr := repo.CheckoutBranch(repo.BaseBranch); checkoutErr != nil {
				log.Error().Err(checkoutErr).Str("branch", repo.BaseBranch).Msg("Failed to checkout base branch after error")
				fmt.Printf("  ❌ Error: Could not checkout back to %s: %v\n", repo.BaseBranch, checkoutErr)
			} else {
				fmt.Printf("  ↩️  Reverted to %s due to error\n", repo.BaseBranch)
			}
		} else if isLastFile {
			// Only checkout back to base branch after the last file (and after PR creation)
			if checkoutErr := repo.CheckoutBranch(repo.BaseBranch); checkoutErr != nil {
				log.Warn().Err(checkoutErr).Str("branch", repo.BaseBranch).Msg("Failed to checkout base branch")
				fmt.Printf("  ⚠️  Warning: Could not checkout back to %s: %v\n", repo.BaseBranch, checkoutErr)
			} else {
				fmt.Printf("  ✓ Checked out back to %s\n", repo.BaseBranch)
			}
		}
	}()

	// Create branch name using format: chore/update/<patchGroup>
	branchName := fmt.Sprintf("chore/update/%s", group.Name)

	// Check if branch already exists (reuse existing PR)
	branchExists, err = repo.CheckoutOrCreateBranch(branchName)
	if err != nil {
		return nil, false, false, fmt.Errorf("failed to checkout or create branch: %w", err)
	}

	if branchExists {
		fmt.Printf("  🔄 Reusing existing branch: %s\n", branchName)
	} else {
		fmt.Printf("  📝 Created new branch: %s\n", branchName)
	}

	// Check for uncommitted changes from a previous run
	hasUncommitted, err := repo.HasUncommittedChanges()
	if err != nil {
		return nil, false, false, fmt.Errorf("failed to check for uncommitted changes: %w", err)
	}

	if hasUncommitted && branchExists {
		fmt.Printf("  ⚠️  Found uncommitted changes from previous run, will include them\n")
	}

	// Apply each update to the file
	for _, update := range updates {
		if err = applyUpdate(config, update); err != nil {
			return nil, false, false, fmt.Errorf("failed to apply update for %s: %w", update.ItemName, err)
		}

		fmt.Printf("  ✓ Updated %s: %s → %s\n",
			update.ItemName,
			update.CurrentVersion,
			update.LatestVersion)
	}

	// Get relative path for commit
	relPath, err := filepath.Rel(repo.WorkingDirectory, filePath)
	if err != nil {
		relPath = filePath
	}

	// Create commit message
	commitMessage := buildCommitMessage(updates, group)

	// Check if there are changes to commit
	hasChanges, err := repo.HasUncommittedChanges()
	if err != nil {
		return nil, false, false, fmt.Errorf("failed to check for changes: %w", err)
	}

	var needsPush bool
	if hasChanges {
		// Commit changes
		commitOptions := &git.CommitOptions{
			Message: commitMessage,
			Files:   []string{relPath},
		}

		if err = repo.Commit(commitOptions); err != nil {
			return nil, false, false, fmt.Errorf("failed to commit changes: %w", err)
		}

		fmt.Printf("  📝 Created commit: %s\n", commitMessage)
		needsPush = true
	} else {
		fmt.Printf("  ℹ️  No new changes to commit\n")

		// Check if there are unpushed commits from a previous run
		hasUnpushed, err := repo.HasUnpushedCommits()
		if err != nil {
			return nil, false, false, fmt.Errorf("failed to check for unpushed commits: %w", err)
		}

		if hasUnpushed {
			fmt.Printf("  📦 Found unpushed commits from previous run\n")
			lastCommit, _ := repo.GetLastCommitMessage()
			if lastCommit != "" {
				fmt.Printf("  📝 Last commit: %s\n", lastCommit)
			}
			needsPush = true
		}
	}

	// Track whether branch was pushed
	branchPushed = false

	// Push branch only if this is the last file (after all commits are made)
	if isLastFile && needsPush {
		if err = repo.Push(); err != nil {
			return nil, false, false, fmt.Errorf("failed to push branch: %w", err)
		}
		fmt.Printf("  📤 Pushed branch to remote\n")
		branchPushed = true
	} else if isLastFile && !needsPush {
		fmt.Printf("  ℹ️  No changes to push\n")
	}

	return repo, branchExists, branchPushed, nil
}

// applyUpdate applies a single update to a target
func applyUpdate(config *configuration.Config, update *UpdateItem) error {
	// Find the target and item configuration
	targetConfig, updateItemConfig := findTargetAndItemByFile(config, update.TargetFile, update.SourceName)
	if targetConfig == nil || updateItemConfig == nil {
		return fmt.Errorf("could not find target configuration for %s", update.TargetFile)
	}

	// Create target factory
	targetFactory := target.NewTargetFactory(config)

	// Create target client for the specific update item
	targetClient, err := targetFactory.CreateTargetForUpdateItem(targetConfig, updateItemConfig)
	if err != nil {
		return fmt.Errorf("failed to create target client: %w", err)
	}

	// Write new version
	if err := targetClient.WriteVersion(update.LatestVersion); err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}

	return nil
}

// findTargetAndItemByFile finds target and item configuration by file path and source
func findTargetAndItemByFile(config *configuration.Config, filePath string, sourceName string) (*configuration.Target, *configuration.TargetItem) {
	for _, target := range config.Targets {
		if target.File != filePath {
			continue
		}

		for _, item := range target.Items {
			if item.Source == sourceName {
				return target, &item
			}
		}
	}
	return nil, nil
}
