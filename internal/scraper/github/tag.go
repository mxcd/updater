package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"time"

	"github.com/mxcd/updater/internal/configuration"
	"github.com/rs/zerolog/log"
)

func scrapeTag(provider *configuration.PackageSourceProvider, source *configuration.PackageSource, opts *ScrapeOptions) ([]*configuration.PackageSourceVersion, error) {
	log.Debug().Str("uri", source.URI).Msg("scraping GitHub tags")

	// Parse repository information from URI
	repoInfo, err := ParseRepositoryURL(source.URI)
	if err != nil {
		return nil, err
	}

	// Build API base URL
	apiBaseURL := BuildAPIURL(provider.BaseUrl)

	// Fetch all tags from GitHub
	tags, err := fetchAllGitHubTags(apiBaseURL, repoInfo, provider, source)
	if err != nil {
		return nil, err
	}

	log.Debug().
		Int("total_tags_fetched", len(tags)).
		Str("repo", fmt.Sprintf("%s/%s", repoInfo.Owner, repoInfo.Repo)).
		Msg("fetched tags from GitHub")

	// Convert ALL tags to PackageSourceVersion objects FIRST
	allVersions := make([]*configuration.PackageSourceVersion, 0, len(tags))
	for _, tag := range tags {
		version := parseGitTag(tag.Name, tag.Commit.SHA)
		allVersions = append(allVersions, version)
	}

	// Sort ALL versions based on configuration BEFORE filtering
	sortVersions(allVersions, source)

	log.Debug().
		Int("total_versions", len(allVersions)).
		Msg("sorted all versions")

	// NOW filter the sorted versions based on patterns
	filteredVersions, err := filterGitVersions(allVersions, source)
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
		Str("repo", fmt.Sprintf("%s/%s", repoInfo.Owner, repoInfo.Repo)).
		Msg("scraped GitHub tags")

	return versions, nil
}

type GitHubTag struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
		URL string `json:"url"`
	} `json:"commit"`
}

func fetchAllGitHubTags(apiBaseURL string, repoInfo *RepositoryInfo, provider *configuration.PackageSourceProvider, source *configuration.PackageSource) ([]GitHubTag, error) {
	allTags := make([]GitHubTag, 0)
	perPage := 100
	page := 1

	// Determine tag limit (default to 0 = unlimited)
	tagLimit := source.TagLimit
	if tagLimit < 0 {
		tagLimit = 0 // Normalize negative values to unlimited
	}

	client := &http.Client{Timeout: 30 * time.Second}

	for {
		// Check if we've reached the tag limit
		if tagLimit > 0 && len(allTags) >= tagLimit {
			log.Debug().
				Int("tags_fetched", len(allTags)).
				Int("tag_limit", tagLimit).
				Msg("reached tag limit, stopping pagination")
			break
		}

		apiURL := fmt.Sprintf("%s/repos/%s/%s/tags?per_page=%d&page=%d", apiBaseURL, repoInfo.Owner, repoInfo.Repo, perPage, page)

		log.Trace().
			Str("url", apiURL).
			Int("page", page).
			Msg("fetching GitHub tags page")

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

		var pageTags []GitHubTag
		if err := json.Unmarshal(body, &pageTags); err != nil {
			return nil, fmt.Errorf("failed to parse tags response: %w", err)
		}

		// If no tags returned, we've reached the end
		if len(pageTags) == 0 {
			break
		}

		for _, tag := range pageTags {
			// Check tag limit before adding more tags
			if tagLimit > 0 && len(allTags) >= tagLimit {
				break
			}
			allTags = append(allTags, tag)
		}

		log.Trace().
			Int("page", page).
			Int("page_tags", len(pageTags)).
			Int("total_tags", len(allTags)).
			Msg("fetched GitHub tags page")

		// If we got fewer tags than requested, we're done
		if len(pageTags) < perPage {
			break
		}

		page++
	}

	log.Debug().
		Int("total_tags", len(allTags)).
		Int("pages", page).
		Int("tag_limit", tagLimit).
		Bool("limit_reached", tagLimit > 0 && len(allTags) >= tagLimit).
		Msg("finished fetching GitHub tags")

	return allTags, nil
}

func parseGitTag(tagName string, commitSHA string) *configuration.PackageSourceVersion {
	version := &configuration.PackageSourceVersion{
		Version: tagName,
	}

	version.MajorVersion, version.MinorVersion, version.PatchVersion = configuration.ParseSemver(tagName)

	// Add commit SHA as version information
	if commitSHA != "" {
		version.VersionInformation = fmt.Sprintf("commit: %.7s", commitSHA)
	}

	return version
}

func filterGitVersions(versions []*configuration.PackageSourceVersion, source *configuration.PackageSource) ([]*configuration.PackageSourceVersion, error) {
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
		// Sort alphabetically (reverse)
		sort.Slice(versions, func(i, j int) bool {
			return versions[i].Version > versions[j].Version
		})
	default:
		log.Warn().Str("sortBy", sortBy).Msg("unknown sort method, using semantic")
		sortVersions(versions, &configuration.PackageSource{SortBy: "semantic"})
	}
}
