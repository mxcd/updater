package docker

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mxcd/updater/internal/configuration"
	"github.com/rs/zerolog/log"
)

// scrapeDockerImage scrapes version information for a Docker image from a registry
// Supports Docker Hub and custom registries
func scrapeDockerImage(provider *configuration.PackageSourceProvider, source *configuration.PackageSource, opts *ScrapeOptions) ([]*configuration.PackageSourceVersion, error) {
	log.Debug().Str("uri", source.URI).Msg("scraping Docker image")

	// Parse image information from URI
	imageInfo, err := ParseImageURL(source.URI)
	if err != nil {
		return nil, err
	}

	// Build registry URL
	registryURL := BuildRegistryURL(provider.BaseUrl, imageInfo.Registry)

	// Fetch tags from registry
	tags, err := fetchDockerTags(registryURL, imageInfo, provider, source, opts)
	if err != nil {
		return nil, err
	}

	log.Debug().
		Int("total_tags_fetched", len(tags)).
		Str("image", imageInfo.Repository).
		Msg("fetched tags from registry")

	// Convert ALL tags to PackageSourceVersion objects FIRST
	allVersions := make([]*configuration.PackageSourceVersion, 0, len(tags))
	for _, tag := range tags {
		version := parseDockerTag(tag)
		allVersions = append(allVersions, version)
	}

	// Sort ALL versions based on configuration BEFORE filtering
	sortVersions(allVersions, source)

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

	// Apply limit if specified and we have more versions than requested
	versions := filteredVersions
	if opts.Limit > 0 && len(versions) > opts.Limit {
		versions = versions[:opts.Limit]
	}

	log.Debug().
		Int("count", len(versions)).
		Int("total_fetched", len(tags)).
		Int("after_filtering", len(filteredVersions)).
		Int("after_limit", len(versions)).
		Str("image", imageInfo.Repository).
		Msg("scraped Docker image tags")

	return versions, nil
}

func fetchDockerTags(registryURL string, imageInfo *ImageInfo, provider *configuration.PackageSourceProvider, source *configuration.PackageSource, opts *ScrapeOptions) ([]string, error) {
	// Determine if this is Docker Hub or a custom registry
	isDockerHub := imageInfo.Registry == "" || imageInfo.Registry == "docker.io"

	if isDockerHub {
		return fetchDockerHubTagsPaginated(imageInfo, provider, source, opts)
	}

	// Docker Registry API v2 for custom registries (ghcr.io, gcr.io, etc.)
	// Uses token exchange auth flow and pagination
	return fetchV2TagsPaginated(registryURL, imageInfo, provider, source, opts)
}

func fetchDockerHubTagsPaginated(imageInfo *ImageInfo, provider *configuration.PackageSourceProvider, source *configuration.PackageSource, opts *ScrapeOptions) ([]string, error) {
	allTags := make([]string, 0)
	pageSize := 100
	nextURL := fmt.Sprintf("https://registry.hub.docker.com/v2/repositories/%s/tags?page_size=%d", imageInfo.Repository, pageSize)

	client := &http.Client{Timeout: 30 * time.Second}

	// Determine tag limit (default to 0 = unlimited)
	tagLimit := source.TagLimit
	if tagLimit < 0 {
		tagLimit = 0 // Normalize negative values to unlimited
	}

	pageCount := 0

	for nextURL != "" {
		// Check if we've reached the tag limit
		if tagLimit > 0 && len(allTags) >= tagLimit {
			log.Debug().
				Int("tags_fetched", len(allTags)).
				Int("tag_limit", tagLimit).
				Msg("reached tag limit, stopping pagination")
			break
		}

		pageCount++
		log.Trace().
			Str("url", nextURL).
			Int("page", pageCount).
			Msg("fetching Docker Hub tags page")

		request, err := http.NewRequest("GET", nextURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Add authentication if configured
		if provider.AuthType == configuration.PackageSourceProviderAuthTypeToken && provider.Token != "" {
			request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", provider.Token))
		} else if provider.AuthType == configuration.PackageSourceProviderAuthTypeBasic && provider.Username != "" {
			request.SetBasicAuth(provider.Username, provider.Password)
		}

		response, err := client.Do(request)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch tags: %w", err)
		}

		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return nil, fmt.Errorf("failed to fetch tags: HTTP %d", response.StatusCode)
		}

		body, err := io.ReadAll(response.Body)
		response.Body.Close()

		if err != nil {
			return nil, fmt.Errorf("failed to read tags response: %w", err)
		}

		var pageResponse struct {
			Count   int    `json:"count"`
			Next    string `json:"next"`
			Results []struct {
				Name string `json:"name"`
			} `json:"results"`
		}

		if err := json.Unmarshal(body, &pageResponse); err != nil {
			return nil, fmt.Errorf("failed to parse Docker Hub response: %w", err)
		}

		for _, result := range pageResponse.Results {
			// Check tag limit before adding more tags
			if tagLimit > 0 && len(allTags) >= tagLimit {
				break
			}
			allTags = append(allTags, result.Name)
		}

		// Use the Next URL from the response, or stop if there isn't one
		nextURL = pageResponse.Next

		log.Trace().
			Int("page", pageCount).
			Int("page_tags", len(pageResponse.Results)).
			Int("total_tags", len(allTags)).
			Bool("has_next", nextURL != "").
			Msg("fetched Docker Hub tags page")
	}

	log.Debug().
		Int("total_tags", len(allTags)).
		Int("pages", pageCount).
		Int("tag_limit", tagLimit).
		Bool("limit_reached", tagLimit > 0 && len(allTags) >= tagLimit).
		Msg("finished fetching Docker Hub tags")

	return allTags, nil
}

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
		tag := version.Version

		// Apply tag pattern if specified
		if tagPatternRe != nil {
			if !tagPatternRe.MatchString(tag) {
				continue
			}
		}

		// Apply exclude pattern if specified
		if excludePatternRe != nil {
			if excludePatternRe.MatchString(tag) {
				continue
			}
		}

		filtered = append(filtered, version)
	}

	return filtered, nil
}

func sortVersions(versions []*configuration.PackageSourceVersion, source *configuration.PackageSource) {
	sortBy := source.SortBy
	if sortBy == "" {
		sortBy = "semantic" // Default to semantic sorting
	}

	switch sortBy {
	case "semantic":
		// Sort by semantic version (highest first)
		sort.Slice(versions, func(i, j int) bool {
			// Compare major version
			if versions[i].MajorVersion != versions[j].MajorVersion {
				return versions[i].MajorVersion > versions[j].MajorVersion
			}
			// Compare minor version
			if versions[i].MinorVersion != versions[j].MinorVersion {
				return versions[i].MinorVersion > versions[j].MinorVersion
			}
			// Compare patch version
			return versions[i].PatchVersion > versions[j].PatchVersion
		})
	case "alphabetical":
		// Sort alphabetically
		sort.Slice(versions, func(i, j int) bool {
			return versions[i].Version > versions[j].Version
		})
	case "date":
		// For Docker tags, we don't have date information from the tags list
		// This would require additional API calls to get image metadata
		log.Warn().Msg("date sorting not yet implemented for Docker images, using semantic sort")
		sortVersions(versions, &configuration.PackageSource{SortBy: "semantic"})
	default:
		log.Warn().Str("sortBy", sortBy).Msg("unknown sort method, using semantic")
		sortVersions(versions, &configuration.PackageSource{SortBy: "semantic"})
	}
}

func parseDockerTag(tag string) *configuration.PackageSourceVersion {
	version := &configuration.PackageSourceVersion{
		Version: tag,
	}

	version.MajorVersion, version.MinorVersion, version.PatchVersion = configuration.ParseSemver(tag)

	// Add additional tag information if it's not a plain version
	if strings.Contains(tag, "-") || strings.Contains(tag, "_") {
		version.VersionInformation = fmt.Sprintf("tag: %s", tag)
	}

	return version
}
