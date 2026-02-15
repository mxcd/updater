package helm

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mxcd/updater/internal/configuration"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// HelmIndexEntry represents a single chart version in the Helm index.yaml
type HelmIndexEntry struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	AppVersion  string `yaml:"appVersion,omitempty"`
	Description string `yaml:"description,omitempty"`
	Created     string `yaml:"created,omitempty"`
}

// HelmIndex represents the structure of a Helm repository index.yaml
type HelmIndex struct {
	APIVersion string                       `yaml:"apiVersion"`
	Entries    map[string][]*HelmIndexEntry `yaml:"entries"`
	Generated  string                       `yaml:"generated,omitempty"`
}

func scrapeHelmRepository(provider *configuration.PackageSourceProvider, source *configuration.PackageSource, opts *ScrapeOptions) ([]*configuration.PackageSourceVersion, error) {
	log.Debug().
		Str("baseUrl", provider.BaseUrl).
		Str("chartName", source.ChartName).
		Msg("scraping Helm repository")

	// Validate that ChartName is specified
	if source.ChartName == "" {
		return nil, fmt.Errorf("chartName is required for helm-repository source type")
	}

	// Validate that provider has baseUrl
	if provider.BaseUrl == "" {
		return nil, fmt.Errorf("baseUrl is required in provider configuration for helm-repository source type")
	}

	// Construct index.yaml URL from provider's baseUrl
	indexURL := buildIndexURL(provider.BaseUrl)
	log.Debug().Str("indexURL", indexURL).Msg("fetching Helm index.yaml")

	// Fetch index.yaml
	indexData, err := fetchHelmIndex(indexURL, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Helm index: %w", err)
	}

	// Parse index.yaml
	var index HelmIndex
	if err := yaml.Unmarshal(indexData, &index); err != nil {
		return nil, fmt.Errorf("failed to parse Helm index.yaml: %w", err)
	}

	// Find the chart in the index
	if index.Entries == nil {
		return nil, fmt.Errorf("Helm index.yaml contains no entries")
	}
	chartEntries, exists := index.Entries[source.ChartName]
	if !exists {
		return nil, fmt.Errorf("chart '%s' not found in Helm repository", source.ChartName)
	}

	if len(chartEntries) == 0 {
		return nil, fmt.Errorf("no versions found for chart '%s'", source.ChartName)
	}

	// Convert ALL entries to PackageSourceVersion FIRST
	allVersions := make([]*configuration.PackageSourceVersion, 0, len(chartEntries))
	for _, entry := range chartEntries {
		version := convertToPackageSourceVersion(entry)
		allVersions = append(allVersions, version)
	}

	// Sort ALL versions by semantic version (descending) BEFORE filtering
	sortVersions(allVersions)

	log.Debug().
		Int("total_versions", len(allVersions)).
		Msg("sorted all versions")

	// NOW filter the sorted versions based on patterns
	filteredVersions, err := filterVersions(allVersions, source)
	if err != nil {
		return nil, err
	}

	log.Debug().
		Int("filtered_versions", len(filteredVersions)).
		Int("removed", len(allVersions)-len(filteredVersions)).
		Msg("filtered versions")

	// Apply limit if specified
	versions := filteredVersions
	if opts.Limit > 0 && len(versions) > opts.Limit {
		versions = versions[:opts.Limit]
	}

	log.Debug().
		Int("count", len(versions)).
		Int("total_fetched", len(chartEntries)).
		Int("after_filtering", len(filteredVersions)).
		Int("after_limit", len(versions)).
		Str("chartName", source.ChartName).
		Msg("successfully scraped Helm repository")

	return versions, nil
}

// filterVersions filters versions based on tagPattern and excludePattern
func filterVersions(versions []*configuration.PackageSourceVersion, source *configuration.PackageSource) ([]*configuration.PackageSourceVersion, error) {
	// Compile regex patterns once before the loop
	var tagPatternRe *regexp.Regexp
	if source.TagPattern != "" {
		var err error
		tagPatternRe, err = regexp.Compile(source.TagPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid tag pattern %q: %w", source.TagPattern, err)
		}
	}

	var excludePatternRe *regexp.Regexp
	if source.ExcludePattern != "" {
		var err error
		excludePatternRe, err = regexp.Compile(source.ExcludePattern)
		if err != nil {
			return nil, fmt.Errorf("invalid exclude pattern %q: %w", source.ExcludePattern, err)
		}
	}

	filtered := make([]*configuration.PackageSourceVersion, 0, len(versions))

	for _, version := range versions {
		versionString := version.Version

		// Apply tag pattern if specified
		if tagPatternRe != nil {
			if !tagPatternRe.MatchString(versionString) {
				continue
			}
		}

		// Apply exclude pattern if specified
		if excludePatternRe != nil {
			if excludePatternRe.MatchString(versionString) {
				continue
			}
		}

		filtered = append(filtered, version)
	}

	return filtered, nil
}

// buildIndexURL constructs the full URL to the index.yaml file
func buildIndexURL(baseURL string) string {
	// Ensure baseURL doesn't end with a slash
	baseURL = strings.TrimSuffix(baseURL, "/")
	return fmt.Sprintf("%s/index.yaml", baseURL)
}

// fetchHelmIndex fetches the index.yaml from the Helm repository
func fetchHelmIndex(indexURL string, provider *configuration.PackageSourceProvider) ([]byte, error) {
	// Create HTTP request
	request, err := http.NewRequest("GET", indexURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication if configured
	if provider.AuthType == configuration.PackageSourceProviderAuthTypeToken && provider.Token != "" {
		request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", provider.Token))
	} else if provider.AuthType == configuration.PackageSourceProviderAuthTypeBasic && provider.Username != "" {
		request.SetBasicAuth(provider.Username, provider.Password)
	}

	// Execute request
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch index.yaml: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch index.yaml: HTTP %d", response.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read index.yaml: %w", err)
	}

	return body, nil
}

// convertToPackageSourceVersion converts a HelmIndexEntry to PackageSourceVersion
func convertToPackageSourceVersion(entry *HelmIndexEntry) *configuration.PackageSourceVersion {
	version := &configuration.PackageSourceVersion{
		Version: entry.Version,
	}

	version.MajorVersion, version.MinorVersion, version.PatchVersion = configuration.ParseSemver(entry.Version)

	if entry.AppVersion != "" {
		version.VersionInformation = fmt.Sprintf("appVersion: %s", entry.AppVersion)
	}

	return version
}

// sortVersions sorts versions by semantic version in descending order (newest first)
func sortVersions(versions []*configuration.PackageSourceVersion) {
	sort.Slice(versions, func(i, j int) bool {
		v1 := versions[i]
		v2 := versions[j]

		// Compare major version
		if v1.MajorVersion != v2.MajorVersion {
			return v1.MajorVersion > v2.MajorVersion
		}

		// Compare minor version
		if v1.MinorVersion != v2.MinorVersion {
			return v1.MinorVersion > v2.MinorVersion
		}

		// Compare patch version
		if v1.PatchVersion != v2.PatchVersion {
			return v1.PatchVersion > v2.PatchVersion
		}

		// If all numeric parts are equal, compare version strings lexicographically
		// This handles pre-release versions like "1.0.0-beta" vs "1.0.0-alpha"
		return v1.Version > v2.Version
	})
}
