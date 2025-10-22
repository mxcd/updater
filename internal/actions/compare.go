package actions

import (
	"encoding/json"
	"fmt"
	"os"

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

	if err := orchestrator.ScrapeAllSources(scrapeOptions); err != nil {
		log.Error().Err(err).Msg("Failed to scrape package sources")
		return nil, fmt.Errorf("scraping error: %w", err)
	}

	log.Debug().Msg("Successfully scraped all package sources")

	// Create comparison engine
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
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetTitle("🔍 Version Comparison")
	t.AppendHeader(table.Row{"Target", "Source", "Current", "Latest", "Update Type", "Status"})

	for _, result := range results {
		if result.Error != nil {
			t.AppendRow(table.Row{
				result.TargetName,
				result.SourceName,
				"-",
				"-",
				"-",
				fmt.Sprintf("❌ Error: %v", result.Error),
			})
		} else {
			status := "✅ Up to date"
			if result.NeedsUpdate {
				status = fmt.Sprintf("🔄 Update available (%s)", result.UpdateType)
			}

			t.AppendRow(table.Row{
				result.TargetName,
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
	fmt.Println()

	// Summary
	updatesCount := 0
	errorsCount := 0
	for _, r := range results {
		if r.Error != nil {
			errorsCount++
		} else if r.NeedsUpdate {
			updatesCount++
		}
	}

	if errorsCount > 0 {
		fmt.Printf("⚠️  %d target(s) with errors\n", errorsCount)
	}
	if updatesCount > 0 {
		fmt.Printf("🔄 %d target(s) need updating\n", updatesCount)
	} else {
		fmt.Println("✅ All targets are up to date")
	}

	return nil
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
