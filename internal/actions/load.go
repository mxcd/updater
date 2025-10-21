package actions

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/mxcd/updater/internal/configuration"
	"github.com/mxcd/updater/internal/scraper"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type LoadOptions struct {
	ConfigPath   string
	OutputFormat string
	Limit        int
}

func Load(options *LoadOptions) error {
	log.Debug().Str("config", options.ConfigPath).Msg("Loading configuration...")

	// Load configuration
	config, err := configuration.LoadConfiguration(options.ConfigPath)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load configuration")
		return fmt.Errorf("configuration load error: %w", err)
	}

	log.Debug().Msg("Configuration loaded successfully")

	// Validate configuration
	validationResult := configuration.ValidateConfiguration(config)
	if !validationResult.Valid {
		log.Error().Msg("Configuration validation failed")
		for _, validationErr := range validationResult.Errors {
			log.Error().Str("field", validationErr.Field).Msg(validationErr.Message)
		}
		return fmt.Errorf("configuration validation failed")
	}

	log.Info().Msg("Configuration is valid")

	// Create orchestrator
	orchestrator, err := scraper.NewOrchestrator(config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create scraper orchestrator")
		return fmt.Errorf("orchestrator creation error: %w", err)
	}

	log.Debug().Msg("Scraper orchestrator created successfully")

	// Scrape all sources
	scrapeOptions := &scraper.ScrapeOptions{
		Limit: options.Limit,
	}

	if err := orchestrator.ScrapeAllSources(scrapeOptions); err != nil {
		log.Error().Err(err).Msg("Failed to scrape package sources")
		return fmt.Errorf("scraping error: %w", err)
	}

	// Output results
	if err := outputLoadResults(orchestrator.GetConfig(), options.OutputFormat); err != nil {
		log.Error().Err(err).Msg("Failed to output results")
		return fmt.Errorf("output error: %w", err)
	}

	log.Info().Msg("Successfully loaded and scraped all package sources")
	return nil
}

func outputLoadResults(config *configuration.Config, format string) error {
	switch format {
	case "table":
		return outputLoadResultsTable(config)
	case "json":
		return outputLoadResultsJSON(config)
	case "yaml":
		return outputLoadResultsYAML(config)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func outputLoadResultsTable(config *configuration.Config) error {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetTitle("📦 Package Sources")
	t.AppendHeader(table.Row{"Name", "Provider", "Type", "Version", "Semantic Version", "Version Info"})

	for _, source := range config.PackageSources {
		if len(source.Versions) == 0 {
			t.AppendRow(table.Row{
				source.Name,
				source.Provider,
				source.Type,
				"-",
				"-",
				"No versions found",
			})
		} else {
			for i, version := range source.Versions {
				name := source.Name
				provider := source.Provider
				sourceType := source.Type

				// Only show name, provider, and type on the first row for each source
				if i > 0 {
					name = ""
					provider = ""
					sourceType = ""
				}

				semanticVersion := "-"
				if version.MajorVersion > 0 || version.MinorVersion > 0 || version.PatchVersion > 0 {
					semanticVersion = fmt.Sprintf("v%d.%d.%d", version.MajorVersion, version.MinorVersion, version.PatchVersion)
				}

				versionInfo := version.VersionInformation
				if versionInfo == "" {
					versionInfo = "-"
				}

				t.AppendRow(table.Row{
					name,
					provider,
					sourceType,
					version.Version,
					semanticVersion,
					versionInfo,
				})
			}
		}
		t.AppendSeparator()
	}

	t.SetStyle(table.StyleRounded)
	t.Render()
	fmt.Println()

	return nil
}

func outputLoadResultsJSON(config *configuration.Config) error {
	output := map[string]interface{}{
		"packageSources": config.PackageSources,
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func outputLoadResultsYAML(config *configuration.Config) error {
	output := map[string]interface{}{
		"packageSources": config.PackageSources,
	}
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	return encoder.Encode(output)
}