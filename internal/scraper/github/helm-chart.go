package github

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mxcd/updater/internal/configuration"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

func scrapeHelmChart(provider *configuration.PackageSourceProvider, source *configuration.PackageSource, opts *ScrapeOptions) ([]*configuration.PackageSourceVersion, error) {
	log.Debug().Str("uri", source.URI).Msg("scraping GitHub Helm chart")

	var body []byte
	var err error

	// Check if URI is a raw.githubusercontent.com URL
	if isRawGitHubURL(source.URI) {
		log.Debug().Str("uri", source.URI).Msg("detected raw.githubusercontent.com URL, fetching directly")
		body, err = fetchFromRawURL(source.URI, provider)
		if err != nil {
			return nil, err
		}
	} else {
		// Use GitHub API for regular repository URLs
		body, err = fetchViaGitHubAPI(provider, source)
		if err != nil {
			return nil, err
		}
	}

	// Parse the YAML content
	var chartData struct {
		Version     string `yaml:"version"`
		AppVersion  string `yaml:"appVersion"`
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}

	if err := yaml.Unmarshal(body, &chartData); err != nil {
		return nil, fmt.Errorf("failed to parse Chart.yaml: %w", err)
	}

	if chartData.Version == "" {
		return nil, fmt.Errorf("no version found in Chart.yaml")
	}

	// Parse version into major, minor, patch components
	version := &configuration.PackageSourceVersion{
		Version: chartData.Version,
	}

	version.MajorVersion, version.MinorVersion, version.PatchVersion = configuration.ParseSemver(chartData.Version)

	// Add version information if appVersion is available
	if chartData.AppVersion != "" {
		version.VersionInformation = fmt.Sprintf("appVersion: %s", chartData.AppVersion)
	}

	log.Debug().
		Str("version", version.Version).
		Int("major", version.MajorVersion).
		Int("minor", version.MinorVersion).
		Int("patch", version.PatchVersion).
		Msg("scraped Helm chart version")

	return []*configuration.PackageSourceVersion{version}, nil
}

// isRawGitHubURL checks if the URI is a raw.githubusercontent.com URL
func isRawGitHubURL(uri string) bool {
	return strings.Contains(uri, "raw.githubusercontent.com")
}

// fetchFromRawURL fetches Chart.yaml content directly from raw.githubusercontent.com URL
func fetchFromRawURL(uri string, provider *configuration.PackageSourceProvider) ([]byte, error) {
	log.Debug().Str("uri", uri).Msg("fetching from raw URL (bypassing GitHub API)")

	// Create HTTP request
	request, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication if configured (raw URLs also support authentication)
	if provider.AuthType == configuration.PackageSourceProviderAuthTypeToken && provider.Token != "" {
		request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", provider.Token))
	} else if provider.AuthType == configuration.PackageSourceProviderAuthTypeBasic && provider.Username != "" {
		request.SetBasicAuth(provider.Username, provider.Password)
	}

	// Execute request
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Chart.yaml from raw URL: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch Chart.yaml from raw URL: HTTP %d", response.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Chart.yaml from raw URL: %w", err)
	}

	return body, nil
}

// fetchViaGitHubAPI fetches Chart.yaml content via GitHub API
func fetchViaGitHubAPI(provider *configuration.PackageSourceProvider, source *configuration.PackageSource) ([]byte, error) {
	// Parse repository information from URI
	repoInfo, err := ParseRepositoryURL(source.URI)
	if err != nil {
		return nil, err
	}

	// Determine branch (default to "main" if not specified)
	branch := source.Branch
	if branch == "" {
		branch = "main"
	}

	// Determine path (try to extract from old-style raw URLs or use explicit path field)
	chartPath := source.Path
	if chartPath == "" {
		// Try to extract path from old-style raw content URLs
		chartPath = extractPathFromRawURL(source.URI)
		if chartPath == "" {
			// Default path for Helm charts
			chartPath = "Chart.yaml"
		}
	}

	// Build API base URL
	apiBaseURL := BuildAPIURL(provider.BaseUrl)

	// Construct GitHub API URL for file contents
	// Format: /repos/{owner}/{repo}/contents/{path}?ref={branch}
	apiURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s",
		apiBaseURL, repoInfo.Owner, repoInfo.Repo, chartPath, branch)

	log.Debug().
		Str("api_url", apiURL).
		Str("owner", repoInfo.Owner).
		Str("repo", repoInfo.Repo).
		Str("path", chartPath).
		Str("branch", branch).
		Msg("fetching Helm chart via GitHub API")

	// Create HTTP request
	request, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication if configured
	if provider.AuthType == configuration.PackageSourceProviderAuthTypeToken && provider.Token != "" {
		request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", provider.Token))
	} else if provider.AuthType == configuration.PackageSourceProviderAuthTypeBasic && provider.Username != "" {
		request.SetBasicAuth(provider.Username, provider.Password)
	}

	// Add GitHub API headers
	request.Header.Set("Accept", "application/vnd.github.raw") // Get raw file content
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	// Execute request
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Chart.yaml: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch Chart.yaml: HTTP %d", response.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Chart.yaml: %w", err)
	}

	return body, nil
}

// extractPathFromRawURL attempts to extract the file path from old-style GitHub raw content URLs
// Example: https://raw.githubusercontent.com/owner/repo/refs/heads/main/path/to/Chart.yaml
// or: https://raw.git.example.com/owner/repo/refs/heads/main/path/to/Chart.yaml
func extractPathFromRawURL(uri string) string {
	// Check if this is a raw content URL
	if !strings.Contains(uri, "/raw/") && !strings.Contains(uri, "raw.githubusercontent.com") && !strings.Contains(uri, "raw.git") {
		return ""
	}

	// Try to find the path after refs/heads/{branch}/
	if idx := strings.Index(uri, "/refs/heads/"); idx != -1 {
		// Find the next slash after the branch name
		branchStart := idx + len("/refs/heads/")
		if nextSlash := strings.Index(uri[branchStart:], "/"); nextSlash != -1 {
			return uri[branchStart+nextSlash+1:]
		}
	}

	// Try alternative patterns
	parts := strings.Split(uri, "/")
	for i, part := range parts {
		if part == "raw" && i+3 < len(parts) {
			// Pattern: .../raw/{owner}/{repo}/{branch}/path/to/file
			// Return everything after the branch
			if i+4 < len(parts) {
				return strings.Join(parts[i+4:], "/")
			}
		}
	}

	return ""
}
