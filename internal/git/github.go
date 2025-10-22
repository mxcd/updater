package git

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mxcd/updater/internal/configuration"
	"github.com/rs/zerolog/log"
)

// GitHubClient handles GitHub API operations
type GitHubClient struct {
	Token   string
	BaseURL string
	RepoURL string
	Owner   string
	Repo    string
}

// NewGitHubClient creates a new GitHub client
func NewGitHubClient(repoURL string, targetActor *configuration.TargetActor) (*GitHubClient, error) {
	owner, repo, err := parseGitHubURL(repoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse GitHub URL: %w", err)
	}

	token := targetActor.Token
	if token == "" {
		return nil, fmt.Errorf("GitHub token is required for PR creation")
	}

	// Extract base URL from repo URL
	baseURL := extractAPIBaseURL(repoURL)

	return &GitHubClient{
		Token:   token,
		BaseURL: baseURL,
		RepoURL: repoURL,
		Owner:   owner,
		Repo:    repo,
	}, nil
}

// extractAPIBaseURL extracts the API base URL from a repository URL
func extractAPIBaseURL(repoURL string) string {
	// Handle HTTPS URLs with credentials: https://user:token@host/owner/repo.git
	if strings.HasPrefix(repoURL, "https://") && strings.Contains(repoURL, "@") {
		atIndex := strings.Index(repoURL, "@")
		if atIndex != -1 {
			remainder := repoURL[atIndex+1:]
			slashIndex := strings.Index(remainder, "/")
			if slashIndex != -1 {
				host := remainder[:slashIndex]
				// Enterprise GitHub uses /api/v3, github.com uses api.github.com
				if host == "github.com" {
					return "https://api.github.com"
				}
				return fmt.Sprintf("https://%s/api/v3", host)
			}
		}
	}

	// Handle HTTPS URLs without credentials
	if strings.HasPrefix(repoURL, "https://github.com/") {
		return "https://api.github.com"
	}

	if strings.HasPrefix(repoURL, "https://") {
		// Extract host from URL
		url := strings.TrimPrefix(repoURL, "https://")
		slashIndex := strings.Index(url, "/")
		if slashIndex != -1 {
			host := url[:slashIndex]
			return fmt.Sprintf("https://%s/api/v3", host)
		}
	}

	// Handle SSH URLs: git@host:owner/repo.git
	if strings.HasPrefix(repoURL, "git@") {
		colonIndex := strings.Index(repoURL, ":")
		if colonIndex != -1 {
			host := strings.TrimPrefix(repoURL[:colonIndex], "git@")
			if host == "github.com" {
				return "https://api.github.com"
			}
			return fmt.Sprintf("https://%s/api/v3", host)
		}
	}

	// Default to github.com
	return "https://api.github.com"
}

// parseGitHubURL parses a GitHub URL to extract owner and repo
func parseGitHubURL(url string) (string, string, error) {
	// Handle HTTPS URLs with credentials: https://user:token@host/owner/repo.git
	// This supports both github.com and enterprise GitHub instances
	if strings.HasPrefix(url, "https://") && strings.Contains(url, "@") {
		// Extract path after @host/
		atIndex := strings.Index(url, "@")
		if atIndex != -1 {
			// Everything after @ is host/owner/repo.git
			remainder := url[atIndex+1:]
			
			// Find the first / after the host
			slashIndex := strings.Index(remainder, "/")
			if slashIndex != -1 {
				path := remainder[slashIndex+1:]
				path = strings.TrimSuffix(path, ".git")
				pathParts := strings.Split(path, "/")
				if len(pathParts) >= 2 {
					return pathParts[0], pathParts[1], nil
				}
			}
		}
	}

	// Handle HTTPS URLs: https://github.com/owner/repo.git or https://github.com/owner/repo
	if strings.HasPrefix(url, "https://github.com/") {
		path := strings.TrimPrefix(url, "https://github.com/")
		path = strings.TrimSuffix(path, ".git")
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}
	}

	// Handle HTTPS URLs for enterprise GitHub: https://host/owner/repo.git
	if strings.HasPrefix(url, "https://") {
		// Extract host and path
		urlWithoutProtocol := strings.TrimPrefix(url, "https://")
		slashIndex := strings.Index(urlWithoutProtocol, "/")
		if slashIndex != -1 {
			path := urlWithoutProtocol[slashIndex+1:]
			path = strings.TrimSuffix(path, ".git")
			pathParts := strings.Split(path, "/")
			if len(pathParts) >= 2 {
				return pathParts[0], pathParts[1], nil
			}
		}
	}

	// Handle SSH URLs: git@host:owner/repo.git
	if strings.HasPrefix(url, "git@") {
		// Extract everything after git@
		remainder := strings.TrimPrefix(url, "git@")
		colonIndex := strings.Index(remainder, ":")
		if colonIndex != -1 {
			// Everything after : is owner/repo.git
			path := remainder[colonIndex+1:]
			path = strings.TrimSuffix(path, ".git")
			pathParts := strings.Split(path, "/")
			if len(pathParts) >= 2 {
				return pathParts[0], pathParts[1], nil
			}
		}
	}

	return "", "", fmt.Errorf("unsupported GitHub URL format: %s", url)
}

// CreatePullRequest creates a pull request on GitHub
func (c *GitHubClient) CreatePullRequest(options *PullRequestOptions) (string, error) {
	log.Debug().
		Str("title", options.Title).
		Str("base", options.BaseBranch).
		Str("head", options.HeadBranch).
		Msg("Creating GitHub pull request")

	// Prepare request body
	requestBody := map[string]interface{}{
		"title": options.Title,
		"body":  options.Body,
		"base":  options.BaseBranch,
		"head":  options.HeadBranch,
	}

	bodyJSON, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/repos/%s/%s/pulls", c.BaseURL, c.Owner, c.Repo)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", c.Token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("failed to create PR, status: %d, body: %s", resp.StatusCode, string(responseBody))
	}

	// Parse response
	var prResponse struct {
		HTMLURL string `json:"html_url"`
		Number  int    `json:"number"`
	}

	if err := json.Unmarshal(responseBody, &prResponse); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	log.Debug().
		Str("url", prResponse.HTMLURL).
		Int("number", prResponse.Number).
		Msg("Created pull request")

	// Add labels if specified
	if len(options.Labels) > 0 {
		if err := c.addLabels(prResponse.Number, options.Labels); err != nil {
			log.Warn().Err(err).Msg("Failed to add labels to PR")
		}
	}

	return prResponse.HTMLURL, nil
}

// PullRequest represents a GitHub pull request
type PullRequest struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	State   string `json:"state"`
	Head    struct {
		Ref string `json:"ref"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
}

// FindOpenPullRequest finds an open PR for the given branch
func (c *GitHubClient) FindOpenPullRequest(headBranch string) (*PullRequest, error) {
	log.Debug().
		Str("headBranch", headBranch).
		Msg("Searching for open pull request")

	// Query GitHub API for open PRs with the head branch
	url := fmt.Sprintf("%s/repos/%s/%s/pulls?state=open&head=%s:%s",
		c.BaseURL, c.Owner, c.Repo, c.Owner, headBranch)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", c.Token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to search PRs, status: %d, body: %s", resp.StatusCode, string(responseBody))
	}

	// Parse response
	var prs []PullRequest
	if err := json.Unmarshal(responseBody, &prs); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Return the first matching PR if found
	if len(prs) > 0 {
		log.Debug().
			Int("number", prs[0].Number).
			Str("url", prs[0].HTMLURL).
			Msg("Found existing open pull request")
		return &prs[0], nil
	}

	log.Debug().Msg("No existing open pull request found")
	return nil, nil
}

// UpdatePullRequest updates an existing pull request
func (c *GitHubClient) UpdatePullRequest(prNumber int, options *PullRequestOptions) error {
	log.Debug().
		Int("pr", prNumber).
		Str("title", options.Title).
		Msg("Updating pull request")

	requestBody := map[string]interface{}{
		"title": options.Title,
		"body":  options.Body,
	}

	bodyJSON, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d", c.BaseURL, c.Owner, c.Repo, prNumber)
	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(bodyJSON))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", c.Token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update PR, status: %d, body: %s", resp.StatusCode, string(responseBody))
	}

	log.Debug().Int("number", prNumber).Msg("Updated pull request")

	// Update labels if specified
	if len(options.Labels) > 0 {
		if err := c.addLabels(prNumber, options.Labels); err != nil {
			log.Warn().Err(err).Msg("Failed to update labels on PR")
		}
	}

	return nil
}

// addLabels adds labels to a pull request
func (c *GitHubClient) addLabels(prNumber int, labels []string) error {
	log.Debug().
		Int("pr", prNumber).
		Strs("labels", labels).
		Msg("Adding labels to pull request")

	requestBody := map[string]interface{}{
		"labels": labels,
	}

	bodyJSON, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/labels", c.BaseURL, c.Owner, c.Repo, prNumber)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyJSON))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", c.Token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add labels, status: %d, body: %s", resp.StatusCode, string(responseBody))
	}

	log.Debug().Strs("labels", labels).Msg("Added labels to pull request")

	return nil
}
