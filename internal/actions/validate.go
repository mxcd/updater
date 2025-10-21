package actions

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mxcd/updater/internal/configuration"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type ValidateOptions struct {
	ConfigPath     string
	OutputFormat   string
	ProbeProviders bool
}

func Validate(options *ValidateOptions) error {
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

	// Output results based on format
	if err := outputValidationResult(validationResult, options.OutputFormat, options.ProbeProviders); err != nil {
		log.Error().Err(err).Msg("Failed to output validation results")
		return fmt.Errorf("output error: %w", err)
	}

	if !validationResult.Valid {
		return fmt.Errorf("configuration validation failed")
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
						"version":        "development",
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
