package github

import (
	"fmt"
	"strings"
)

// RepositoryInfo contains parsed repository information
type RepositoryInfo struct {
	Owner string
	Repo  string
}

// ParseRepositoryURL extracts the owner and repository name from a GitHub URL.
// Supports multiple URI formats:
// - https://github.com/owner/repo
// - https://github.com/owner/repo.git
// - https://api.github.com/repos/owner/repo
// - https://api.github.com/repos/owner/repo/releases/latest
// - https://enterprise.example.com/owner/repo
// - https://enterprise.example.com/api/v3/repos/owner/repo
func ParseRepositoryURL(uri string) (*RepositoryInfo, error) {
	if uri == "" {
		return nil, fmt.Errorf("empty URI provided")
	}

	repoPath := uri
	
	// Remove protocol prefixes
	repoPath = strings.TrimPrefix(repoPath, "https://")
	repoPath = strings.TrimPrefix(repoPath, "http://")
	
	// Remove the domain part to get the path
	if idx := strings.Index(repoPath, "/"); idx != -1 {
		repoPath = repoPath[idx+1:]
	} else {
		return nil, fmt.Errorf("invalid GitHub repository URI: %s (no path found)", uri)
	}
	
	// Remove API-specific paths
	repoPath = strings.TrimPrefix(repoPath, "api/v3/")
	repoPath = strings.TrimPrefix(repoPath, "repos/")
	
	// Remove common suffixes
	repoPath = strings.TrimSuffix(repoPath, "/releases/latest")
	repoPath = strings.TrimSuffix(repoPath, "/releases")
	repoPath = strings.TrimSuffix(repoPath, ".git")
	repoPath = strings.TrimSuffix(repoPath, "/")
	
	// Split to get owner and repo
	parts := strings.Split(repoPath, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid GitHub repository URI: %s (expected format: owner/repo)", uri)
	}
	
	owner := parts[0]
	repo := parts[1]
	
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("invalid GitHub repository URI: %s (owner or repo is empty)", uri)
	}
	
	return &RepositoryInfo{
		Owner: owner,
		Repo:  repo,
	}, nil
}

// BuildAPIURL constructs the appropriate GitHub API URL based on the base URL configuration.
// For GitHub Enterprise, it automatically adds /api/v3 if not present.
func BuildAPIURL(baseURL string) string {
	if baseURL == "" {
		return "https://api.github.com"
	}
	
	baseURL = strings.TrimSuffix(baseURL, "/")
	
	// If the base URL already contains /api/v3, use it as-is
	if strings.Contains(baseURL, "/api/v3") {
		return baseURL
	}
	
	// For GitHub Enterprise, add /api/v3
	return baseURL + "/api/v3"
}