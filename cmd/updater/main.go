package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/joho/godotenv"
	"github.com/mxcd/updater/internal/compare"
	"github.com/mxcd/updater/internal/configuration"
	"github.com/mxcd/updater/internal/scraper"
	"github.com/mxcd/updater/internal/util"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"
)

var version = "development"

func main() {

	cli.VersionFlag = &cli.BoolFlag{
		Name:    "version",
		Aliases: []string{},
		Usage:   "print only the version",
	}

	cmd := &cli.Command{
		Name:    "updater",
		Version: version,
		Usage:   "Updater for GitOps resources",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "debug output",
				Sources: cli.EnvVars("UPDATER_VERBOSE"),
			},
			&cli.BoolFlag{
				Name:    "very-verbose",
				Aliases: []string{"vv"},
				Usage:   "trace output",
				Sources: cli.EnvVars("UPDATER_VERY_VERBOSE"),
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			return initCli(ctx, cmd)
		},
		Commands: []*cli.Command{
			{
				Name:  "validate",
				Usage: "Validate configuration",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "config",
						Aliases: []string{"c"},
						Usage:   "Path to configuration file",
						Value:   ".updaterconfig.yml",
						Sources: cli.EnvVars("UPDATER_CONFIG"),
					},
					&cli.StringFlag{
						Name:  "output",
						Usage: "Output format: table, json, yaml, sarif",
						Value: "table",
					},
					&cli.BoolFlag{
						Name:  "probe-providers",
						Usage: "Verify provider connectivity and credentials",
						Value: false,
					},
				},
				Action: validateCommand,
			},
			{
				Name:  "load",
				Usage: "Load configuration and scrape all package sources",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "config",
						Aliases: []string{"c"},
						Usage:   "Path to configuration file",
						Value:   ".updaterconfig.yml",
						Sources: cli.EnvVars("UPDATER_CONFIG"),
					},
					&cli.StringFlag{
						Name:  "output",
						Usage: "Output format: table, json, yaml",
						Value: "table",
					},
					&cli.IntFlag{
						Name:  "limit",
						Usage: "Maximum number of versions to retrieve per source",
						Value: 10,
					},
				},
				Action: loadCommand,
			},
			{
				Name:  "compare",
				Usage: "Compare current versions in targets with latest available versions",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "config",
						Aliases: []string{"c"},
						Usage:   "Path to configuration file",
						Value:   ".updaterconfig.yml",
						Sources: cli.EnvVars("UPDATER_CONFIG"),
					},
					&cli.StringFlag{
						Name:  "output",
						Usage: "Output format: table, json, yaml",
						Value: "table",
					},
					&cli.IntFlag{
						Name:  "limit",
						Usage: "Maximum number of versions to retrieve per source",
						Value: 10,
					},
					&cli.StringFlag{
						Name:  "only",
						Usage: "Only show specific update types: major, minor, patch, all",
						Value: "all",
					},
				},
				Action: compareCommand,
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal().Err(err).Msg("command terminated with error")
	}
}

func initCli(ctx context.Context, cmd *cli.Command) (context.Context, error) {
	godotenv.Load()
	util.SetCliLoggerDefaults()
	util.SetCliLogLevel(cmd)
	log.Trace().Msg("Trace logging enabled")
	log.Debug().Msg("Debug logging enabled")
	log.Info().Msg("Info logging enabled")

	return ctx, nil
}

func validateCommand(ctx context.Context, cmd *cli.Command) error {
	configPath := cmd.String("config")
	outputFormat := cmd.String("output")
	probeProviders := cmd.Bool("probe-providers")

	log.Info().Str("config", configPath).Msg("Loading configuration...")

	// Load configuration
	config, err := configuration.LoadConfiguration(configPath)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load configuration")
		return cli.Exit(fmt.Sprintf("Configuration load error: %v", err), 3)
	}

	log.Debug().Msg("Configuration loaded successfully")

	// Validate configuration
	validationResult := configuration.ValidateConfiguration(config)

	// Output results based on format
	if err := outputValidationResult(validationResult, outputFormat, probeProviders); err != nil {
		log.Error().Err(err).Msg("Failed to output validation results")
		return cli.Exit(fmt.Sprintf("Output error: %v", err), 1)
	}

	if !validationResult.Valid {
		return cli.Exit("Configuration validation failed", 3)
	}

	log.Info().Msg("Configuration is valid")
	return nil
}

func outputValidationResult(result *configuration.ValidationResult, format string, probeProviders bool) error {
	switch format {
	case "table":
		return outputValidationTable(result, probeProviders)
	case "json":
		return outputValidationJSON(result, probeProviders)
	case "yaml":
		return outputValidationYAML(result, probeProviders)
	case "sarif":
		return outputValidationSARIF(result, probeProviders)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func outputValidationTable(result *configuration.ValidationResult, probeProviders bool) error {
	if result.Valid {
		fmt.Println("✓ Configuration is valid")
		if probeProviders {
			fmt.Println("  Note: Provider probing not yet implemented")
		}
		return nil
	}

	fmt.Println("✗ Configuration validation failed:")
	fmt.Println()
	for _, err := range result.Errors {
		fmt.Printf("  • %s\n", err.Error())
	}
	fmt.Printf("\nTotal errors: %d\n", len(result.Errors))
	return nil
}

func outputValidationJSON(result *configuration.ValidationResult, probeProviders bool) error {
	output := map[string]interface{}{
		"valid":          result.Valid,
		"errorCount":     len(result.Errors),
		"errors":         result.Errors,
		"probeProviders": probeProviders,
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func outputValidationYAML(result *configuration.ValidationResult, probeProviders bool) error {
	output := map[string]interface{}{
		"valid":          result.Valid,
		"errorCount":     len(result.Errors),
		"errors":         result.Errors,
		"probeProviders": probeProviders,
	}
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	return encoder.Encode(output)
}

func outputValidationSARIF(result *configuration.ValidationResult, probeProviders bool) error {
	// Basic SARIF 2.1.0 format
	sarif := map[string]interface{}{
		"version": "2.1.0",
		"$schema": "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
		"runs": []interface{}{
			map[string]interface{}{
				"tool": map[string]interface{}{
					"driver": map[string]interface{}{
						"name":           "updater-validate",
						"informationUri": "https://github.com/mxcd/updater",
						"version":        version,
					},
				},
				"results": convertErrorsToSARIF(result.Errors),
			},
		},
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(sarif)
}

func convertErrorsToSARIF(errors []*configuration.ValidationError) []interface{} {
	results := make([]interface{}, len(errors))
	for i, err := range errors {
		results[i] = map[string]interface{}{
			"ruleId": "configuration-error",
			"level":  "error",
			"message": map[string]interface{}{
				"text": err.Message,
			},
			"locations": []interface{}{
				map[string]interface{}{
					"logicalLocations": []interface{}{
						map[string]interface{}{
							"fullyQualifiedName": err.Field,
						},
					},
				},
			},
		}
	}
	return results
}

func loadCommand(ctx context.Context, cmd *cli.Command) error {
	configPath := cmd.String("config")
	outputFormat := cmd.String("output")
	limit := cmd.Int("limit")

	log.Info().Str("config", configPath).Msg("Loading configuration...")

	// Load configuration
	config, err := configuration.LoadConfiguration(configPath)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load configuration")
		return cli.Exit(fmt.Sprintf("Configuration load error: %v", err), 3)
	}

	log.Debug().Msg("Configuration loaded successfully")

	// Validate configuration
	validationResult := configuration.ValidateConfiguration(config)
	if !validationResult.Valid {
		log.Error().Msg("Configuration validation failed")
		for _, validationErr := range validationResult.Errors {
			log.Error().Str("field", validationErr.Field).Msg(validationErr.Message)
		}
		return cli.Exit("Configuration validation failed", 3)
	}

	log.Info().Msg("Configuration is valid")

	// Create orchestrator
	orchestrator, err := scraper.NewOrchestrator(config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create scraper orchestrator")
		return cli.Exit(fmt.Sprintf("Orchestrator creation error: %v", err), 1)
	}

	log.Debug().Msg("Scraper orchestrator created successfully")

	// Scrape all sources
	scrapeOptions := &scraper.ScrapeOptions{
		Limit: limit,
	}

	if err := orchestrator.ScrapeAllSources(scrapeOptions); err != nil {
		log.Error().Err(err).Msg("Failed to scrape package sources")
		return cli.Exit(fmt.Sprintf("Scraping error: %v", err), 1)
	}

	// Output results
	if err := outputLoadResults(orchestrator.GetConfig(), outputFormat); err != nil {
		log.Error().Err(err).Msg("Failed to output results")
		return cli.Exit(fmt.Sprintf("Output error: %v", err), 1)
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

func compareCommand(ctx context.Context, cmd *cli.Command) error {
	configPath := cmd.String("config")
	outputFormat := cmd.String("output")
	limit := cmd.Int("limit")
	only := cmd.String("only")

	log.Info().Str("config", configPath).Msg("Loading configuration...")

	// Load configuration
	config, err := configuration.LoadConfiguration(configPath)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load configuration")
		return cli.Exit(fmt.Sprintf("Configuration load error: %v", err), 3)
	}

	log.Debug().Msg("Configuration loaded successfully")

	// Validate configuration
	validationResult := configuration.ValidateConfiguration(config)
	if !validationResult.Valid {
		log.Error().Msg("Configuration validation failed")
		for _, validationErr := range validationResult.Errors {
			log.Error().Str("field", validationErr.Field).Msg(validationErr.Message)
		}
		return cli.Exit("Configuration validation failed", 3)
	}

	log.Info().Msg("Configuration is valid")

	// Create orchestrator and scrape sources
	orchestrator, err := scraper.NewOrchestrator(config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create scraper orchestrator")
		return cli.Exit(fmt.Sprintf("Orchestrator creation error: %v", err), 1)
	}

	log.Debug().Msg("Scraper orchestrator created successfully")

	// Scrape all sources
	scrapeOptions := &scraper.ScrapeOptions{
		Limit: limit,
	}

	if err := orchestrator.ScrapeAllSources(scrapeOptions); err != nil {
		log.Error().Err(err).Msg("Failed to scrape package sources")
		return cli.Exit(fmt.Sprintf("Scraping error: %v", err), 1)
	}

	log.Info().Msg("Successfully scraped all package sources")

	// Create comparison engine
	compareEngine := compare.NewCompareEngine(orchestrator.GetConfig())

	// Perform comparison
	results, err := compareEngine.CompareAll()
	if err != nil {
		log.Error().Err(err).Msg("Failed to compare targets")
		return cli.Exit(fmt.Sprintf("Comparison error: %v", err), 1)
	}

	// Filter results based on 'only' flag
	filteredResults := filterComparisonResults(results, only)

	// Output results
	if err := outputComparisonResults(filteredResults, outputFormat); err != nil {
		log.Error().Err(err).Msg("Failed to output comparison results")
		return cli.Exit(fmt.Sprintf("Output error: %v", err), 1)
	}

	// Exit with code 1 if there are pending updates (for CI gating)
	hasUpdates := false
	for _, result := range filteredResults {
		if result.NeedsUpdate {
			hasUpdates = true
			break
		}
	}

	if hasUpdates {
		log.Info().Msg("Updates are available")
		return cli.Exit("", 1)
	}

	log.Info().Msg("All targets are up to date")
	return nil
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