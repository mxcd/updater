package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/mxcd/updater/internal/configuration"
	"github.com/rs/zerolog/log"
)

func scrapeRelease(provider *configuration.PackageSourceProvider, source *configuration.PackageSource, opts *ScrapeOptions) ([]*configuration.PackageSourceVersion, error) {
	log.Debug().Str("uri", source.URI).Msg("scraping GitHub release")

	// Parse repository information from URI
	repoInfo, err := ParseRepositoryURL(source.URI)
	if err != nil {
		return nil, err
	}

	// Build API base URL
	apiBaseURL := BuildAPIURL(provider.BaseUrl)

	// Construct GitHub API URL for latest release
	apiURL := fmt.Sprintf("%s/repos/%s/%s/releases/latest", apiBaseURL, repoInfo.Owner, repoInfo.Repo)

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
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	// Execute request
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch release: HTTP %d", response.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read release response: %w", err)
	}

	// Parse JSON response
	var releaseData struct {
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		Body        string `json:"body"`
		Draft       bool   `json:"draft"`
		PreRelease  bool   `json:"prerelease"`
		CreatedAt   string `json:"created_at"`
		PublishedAt string `json:"published_at"`
	}

	if err := json.Unmarshal(body, &releaseData); err != nil {
		return nil, fmt.Errorf("failed to parse release response: %w", err)
	}

	if releaseData.TagName == "" {
		return nil, fmt.Errorf("no tag found in release")
	}

	// Parse version from tag
	version := &configuration.PackageSourceVersion{
		Version: releaseData.TagName,
	}

	// Try to parse semantic version (e.g., "v1.2.3" or "1.2.3")
	versionString := strings.TrimPrefix(releaseData.TagName, "v")
	versionParts := strings.Split(versionString, ".")
	
	if len(versionParts) >= 1 {
		if major, err := strconv.Atoi(versionParts[0]); err == nil {
			version.MajorVersion = major
		}
	}
	if len(versionParts) >= 2 {
		if minor, err := strconv.Atoi(versionParts[1]); err == nil {
			version.MinorVersion = minor
		}
	}
	if len(versionParts) >= 3 {
		// Handle patch versions that might have additional suffixes (e.g., "3-beta1")
		patchPart := strings.Split(versionParts[2], "-")[0]
		if patch, err := strconv.Atoi(patchPart); err == nil {
			version.PatchVersion = patch
		}
	}

	// Add version information if available
	var infoItems []string
	if releaseData.Name != "" {
		infoItems = append(infoItems, fmt.Sprintf("name: %s", releaseData.Name))
	}
	if releaseData.PreRelease {
		infoItems = append(infoItems, "prerelease: true")
	}
	if releaseData.PublishedAt != "" {
		infoItems = append(infoItems, fmt.Sprintf("published: %s", releaseData.PublishedAt))
	}
	if len(infoItems) > 0 {
		version.VersionInformation = strings.Join(infoItems, ", ")
	}

	log.Info().
		Str("version", version.Version).
		Int("major", version.MajorVersion).
		Int("minor", version.MinorVersion).
		Int("patch", version.PatchVersion).
		Bool("prerelease", releaseData.PreRelease).
		Msg("scraped GitHub release version")

	return []*configuration.PackageSourceVersion{version}, nil
}