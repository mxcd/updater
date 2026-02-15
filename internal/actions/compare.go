package actions

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/mxcd/updater/internal/compare"
	"github.com/mxcd/updater/internal/configuration"
	"github.com/mxcd/updater/internal/scraper"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type CompareOptions struct {
	ConfigPath   string
	OutputFormat string
	Limit        int
	Only         string
}

type CompareResult struct {
	Results    []*compare.ComparisonResult
	HasUpdates bool
}

func Compare(options *CompareOptions) (*CompareResult, error) {
	log.Debug().Str("config", options.ConfigPath).Msg("Loading configuration...")

	// Load configuration
	config, err := configuration.LoadConfiguration(options.ConfigPath)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load configuration")
		return nil, fmt.Errorf("configuration load error: %w", err)
	}

	log.Debug().Msg("Configuration loaded successfully")

	// Validate configuration
	validationResult := configuration.ValidateConfiguration(config)
	if !validationResult.Valid {
		log.Error().Msg("Configuration validation failed")
		for _, validationErr := range validationResult.Errors {
			log.Error().Str("field", validationErr.Field).Msg(validationErr.Message)
		}
		return nil, fmt.Errorf("configuration validation failed")
	}

	log.Debug().Msg("Configuration is valid")

	// Create orchestrator and scrape sources
	orchestrator, err := scraper.NewOrchestrator(config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create scraper orchestrator")
		return nil, fmt.Errorf("orchestrator creation error: %w", err)
	}

	log.Debug().Msg("Scraper orchestrator created successfully")

	// Scrape all sources
	scrapeOptions := &scraper.ScrapeOptions{
		Limit: options.Limit,
	}

	scrapeResult := orchestrator.ScrapeAllSources(scrapeOptions)

	log.Debug().
		Int("succeeded", scrapeResult.Succeeded).
		Int("failed", scrapeResult.Failed).
		Msg("Scraping complete")

	// Create comparison engine (works with partial results from successful sources)
	compareEngine := compare.NewCompareEngine(orchestrator.GetConfig())

	// Perform comparison
	results, err := compareEngine.CompareAll()
	if err != nil {
		log.Error().Err(err).Msg("Failed to compare targets")
		return nil, fmt.Errorf("comparison error: %w", err)
	}

	// Filter results based on 'only' flag
	filteredResults := filterComparisonResults(results, options.Only)

	// Output results
	if err := outputComparisonResults(filteredResults, options.OutputFormat); err != nil {
		log.Error().Err(err).Msg("Failed to output comparison results")
		return nil, fmt.Errorf("output error: %w", err)
	}

	// Show scraping errors at the end
	if scrapeResult.HasErrors() {
		fmt.Printf("\n⚠️  %d of %d source(s) failed to scrape:\n", scrapeResult.Failed, scrapeResult.Succeeded+scrapeResult.Failed)
		for _, scrapeErr := range scrapeResult.Errors {
			fmt.Printf("  ❌ %s (provider: %s): %v\n", scrapeErr.SourceName, scrapeErr.Provider, scrapeErr.Err)
		}
		fmt.Println()
	}

	// Check if there are pending updates
	hasUpdates := false
	for _, result := range filteredResults {
		if result.NeedsUpdate {
			hasUpdates = true
			break
		}
	}

	if hasUpdates {
		log.Info().Msg("Updates are available")
	} else {
		log.Info().Msg("All targets are up to date")
	}

	return &CompareResult{
		Results:    filteredResults,
		HasUpdates: hasUpdates,
	}, nil
}

func filterComparisonResults(results []*compare.ComparisonResult, only string) []*compare.ComparisonResult {
	if only == "all" {
		return results
	}

	filtered := make([]*compare.ComparisonResult, 0)
	for _, result := range results {
		switch only {
		case "major":
			if result.UpdateType == compare.UpdateTypeMajor {
				filtered = append(filtered, result)
			}
		case "minor":
			if result.UpdateType == compare.UpdateTypeMinor {
				filtered = append(filtered, result)
			}
		case "patch":
			if result.UpdateType == compare.UpdateTypePatch {
				filtered = append(filtered, result)
			}
		}
	}
	return filtered
}

func outputComparisonResults(results []*compare.ComparisonResult, format string) error {
	switch format {
	case "table":
		return outputComparisonTable(results)
	case "json":
		return outputComparisonJSON(results)
	case "yaml":
		return outputComparisonYAML(results)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func outputComparisonTable(results []*compare.ComparisonResult) error {
	// Filter out dependency not found errors from wildcard matches
	// These are expected when some files don't have the dependency
	filteredResults := filterWildcardDependencyErrors(results)

	// Group results by patch group
	groupedResults := groupResultsByPatchGroup(filteredResults)

	// Get sorted group names
	groupNames := make([]string, 0, len(groupedResults))
	for groupName := range groupedResults {
		groupNames = append(groupNames, groupName)
	}
	// Sort groups: empty group first, then alphabetically
	sortPatchGroups(groupNames)

	totalUpdates := 0
	totalErrors := 0

	// Render each group
	for i, groupName := range groupNames {
		groupResults := groupedResults[groupName]

		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)

		// Set title based on whether this is a named group or not
		if groupName == "" {
			t.SetTitle("🔍 Version Comparison")
		} else {
			t.SetTitle(fmt.Sprintf("🔍 Version Comparison - Patch Group: %s", groupName))
		}

		t.AppendHeader(table.Row{"File / Variable", "Source", "Current", "Latest", "Update Type", "Status"})

		groupUpdates := 0
		groupErrors := 0

		for _, result := range groupResults {
			// Build the first column based on target type
			var firstColumn string
			if result.TargetItemName != "" {
				// Show file path and item name (variable/subchart)
				firstColumn = fmt.Sprintf("%s\n  → %s", result.TargetFile, result.TargetItemName)
			} else {
				// Fallback to target name if no item name
				firstColumn = result.TargetName
			}

			if result.Error != nil {
				groupErrors++
				t.AppendRow(table.Row{
					firstColumn,
					result.SourceName,
					"-",
					"-",
					"-",
					fmt.Sprintf("❌ Error: %v", result.Error),
				})
			} else {
				status := "✅ Up to date"
				if result.NeedsUpdate {
					groupUpdates++
					status = fmt.Sprintf("🔄 Update available (%s)", result.UpdateType)
				}

				t.AppendRow(table.Row{
					firstColumn,
					result.SourceName,
					result.CurrentVersion,
					result.LatestVersion,
					result.UpdateType,
					status,
				})
			}
		}

		t.SetStyle(table.StyleRounded)
		t.Render()

		// Group summary
		if groupErrors > 0 || groupUpdates > 0 {
			fmt.Print("  ")
			if groupErrors > 0 {
				fmt.Printf("⚠️  %d error(s)  ", groupErrors)
			}
			if groupUpdates > 0 {
				fmt.Printf("🔄 %d update(s)", groupUpdates)
			}
			fmt.Println()
		}

		// Add spacing between groups
		if i < len(groupNames)-1 {
			fmt.Println()
		}

		totalUpdates += groupUpdates
		totalErrors += groupErrors
	}

	fmt.Println()

	// Overall summary
	if totalErrors > 0 {
		fmt.Printf("⚠️  Total: %d target(s) with errors\n", totalErrors)
	}
	if totalUpdates > 0 {
		fmt.Printf("🔄 Total: %d target(s) need updating\n", totalUpdates)
	} else {
		fmt.Println("✅ All targets are up to date")
	}

	return nil
}

// groupResultsByPatchGroup groups comparison results by their patch group
func groupResultsByPatchGroup(results []*compare.ComparisonResult) map[string][]*compare.ComparisonResult {
	grouped := make(map[string][]*compare.ComparisonResult)
	for _, result := range results {
		groupName := result.PatchGroup
		grouped[groupName] = append(grouped[groupName], result)
	}
	return grouped
}

// sortPatchGroups sorts patch group names with empty string first, then alphabetically
func sortPatchGroups(groups []string) {
	sort.Slice(groups, func(i, j int) bool {
		if groups[i] == "" {
			return true
		}
		if groups[j] == "" {
			return false
		}
		return groups[i] < groups[j]
	})
}

// filterWildcardDependencyErrors filters out "dependency not found" errors from wildcard matches
// These errors are expected when using wildcards where not all files contain the specified dependency
func filterWildcardDependencyErrors(results []*compare.ComparisonResult) []*compare.ComparisonResult {
	// Group results by wildcard pattern and item name
	wildcardGroups := make(map[string][]*compare.ComparisonResult)
	nonWildcardResults := make([]*compare.ComparisonResult, 0)

	for _, result := range results {
		if result.IsWildcardMatch {
			key := result.WildcardPattern + "|" + result.TargetItemName
			wildcardGroups[key] = append(wildcardGroups[key], result)
		} else {
			nonWildcardResults = append(nonWildcardResults, result)
		}
	}

	// Process wildcard groups
	filteredResults := make([]*compare.ComparisonResult, 0, len(results))
	for _, group := range wildcardGroups {
		// Check if at least one result in the group is successful or has a different error
		hasValidResult := false
		for _, result := range group {
			if result.Error == nil || !isDependencyNotFoundError(result.Error) {
				hasValidResult = true
				break
			}
		}

		// If we have at least one valid result, only include non-dependency-not-found results
		for _, result := range group {
			if hasValidResult {
				// Skip dependency not found errors since we have valid results
				if result.Error != nil && isDependencyNotFoundError(result.Error) {
					continue
				}
			}
			filteredResults = append(filteredResults, result)
		}
	}

	// Add all non-wildcard results
	filteredResults = append(filteredResults, nonWildcardResults...)

	return filteredResults
}

// isDependencyNotFoundError checks if an error is a "dependency not found" error
func isDependencyNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "dependency") && strings.Contains(errStr, "not found")
}

func outputComparisonJSON(results []*compare.ComparisonResult) error {
	output := map[string]interface{}{
		"results": results,
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func outputComparisonYAML(results []*compare.ComparisonResult) error {
	output := map[string]interface{}{
		"results": results,
	}
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	return encoder.Encode(output)
}
