package docker

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

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
	filteredVersions := filterVersions(allVersions, source)

	log.Debug().
		Int("filtered_versions", len(filteredVersions)).
		Int("removed", len(allVersions)-len(filteredVersions)).
		Msg("filtered versions")

	// Apply limit if specified and we have more versions than requested
	versions := filteredVersions
	if opts.Limit > 0 && len(versions) > opts.Limit {
		versions = versions[:opts.Limit]
	}

	log.Info().
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

	// Docker Registry API v2 for custom registries
	tagsURL := fmt.Sprintf("%s/v2/%s/tags/list", registryURL, imageInfo.Repository)

	// Create HTTP request
	request, err := http.NewRequest("GET", tagsURL, nil)
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
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tags: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch tags: HTTP %d", response.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read tags response: %w", err)
	}

	return parseDockerRegistryResponse(body, opts)
}

func fetchDockerHubTagsPaginated(imageInfo *ImageInfo, provider *configuration.PackageSourceProvider, source *configuration.PackageSource, opts *ScrapeOptions) ([]string, error) {
	allTags := make([]string, 0)
	pageSize := 100
	nextURL := fmt.Sprintf("https://registry.hub.docker.com/v2/repositories/%s/tags?page_size=%d", imageInfo.Repository, pageSize)

	client := &http.Client{}
	
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

func parseDockerRegistryResponse(body []byte, opts *ScrapeOptions) ([]string, error) {
	var response struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse registry response: %w", err)
	}

	// Apply limit if specified
	if opts.Limit > 0 && len(response.Tags) > opts.Limit {
		return response.Tags[:opts.Limit], nil
	}

	return response.Tags, nil
}

func filterVersions(versions []*configuration.PackageSourceVersion, source *configuration.PackageSource) []*configuration.PackageSourceVersion {
	filtered := make([]*configuration.PackageSourceVersion, 0, len(versions))

	for _, version := range versions {
		tag := version.Version
		
		// Apply tag pattern if specified
		if source.TagPattern != "" {
			matched, err := regexp.MatchString(source.TagPattern, tag)
			if err != nil {
				log.Warn().Err(err).Str("pattern", source.TagPattern).Msg("invalid tag pattern")
				continue
			}
			if !matched {
				continue
			}
		}

		// Apply exclude pattern if specified
		if source.ExcludePattern != "" {
			matched, err := regexp.MatchString(source.ExcludePattern, tag)
			if err != nil {
				log.Warn().Err(err).Str("pattern", source.ExcludePattern).Msg("invalid exclude pattern")
				continue
			}
			if matched {
				continue
			}
		}

		filtered = append(filtered, version)
	}

	return filtered
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

	// Try to parse semantic version from tag
	// Common patterns: v1.2.3, 1.2.3, 1.2.3-alpine, etc.
	versionString := strings.TrimPrefix(tag, "v")

	// Split on common separators to get the base version
	baseParts := strings.FieldsFunc(versionString, func(r rune) bool {
		return r == '-' || r == '_' || r == '+'
	})

	if len(baseParts) > 0 {
		parts := strings.Split(baseParts[0], ".")

		if len(parts) >= 1 {
			if major, err := strconv.Atoi(parts[0]); err == nil {
				version.MajorVersion = major
			}
		}
		if len(parts) >= 2 {
			if minor, err := strconv.Atoi(parts[1]); err == nil {
				version.MinorVersion = minor
			}
		}
		if len(parts) >= 3 {
			if patch, err := strconv.Atoi(parts[2]); err == nil {
				version.PatchVersion = patch
			}
		}
	}

	// Add additional tag information if it's not a plain version
	if strings.Contains(tag, "-") || strings.Contains(tag, "_") {
		version.VersionInformation = fmt.Sprintf("tag: %s", tag)
	}

	return version
}
