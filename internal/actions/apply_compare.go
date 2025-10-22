package actions

import (
	"fmt"

	"github.com/mxcd/updater/internal/compare"
	"github.com/mxcd/updater/internal/configuration"
	"github.com/mxcd/updater/internal/scraper"
	"github.com/rs/zerolog/log"
)

// compareInternal performs comparison without outputting results
func compareInternal(config *configuration.Config, limit int, only string, outputFormat string) (*CompareResult, error) {
	// Create orchestrator and scrape sources
	orchestrator, err := scraper.NewOrchestrator(config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create scraper orchestrator")
		return nil, fmt.Errorf("orchestrator creation error: %w", err)
	}

	log.Debug().Msg("Scraper orchestrator created successfully")

	// Scrape all sources
	scrapeOptions := &scraper.ScrapeOptions{
		Limit: limit,
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
	filteredResults := filterComparisonResults(results, only)

	err = outputComparisonResults(filteredResults, outputFormat)

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