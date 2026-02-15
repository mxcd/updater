package actions

import (
	"fmt"

	"github.com/mxcd/updater/internal/configuration"
	"github.com/rs/zerolog/log"
)

func Apply(options *ApplyOptions) error {
	log.Debug().Str("config", options.ConfigPath).Msg("Starting apply process...")

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

	log.Debug().Msg("Configuration is valid")

	// Get comparison results without outputting them
	compareResult, err := compareInternal(config, options.Limit, options.Only, options.OutputFormat)
	if err != nil {
		log.Error().Err(err).Msg("Failed to compare versions")
		return fmt.Errorf("comparison error: %w", err)
	}

	if !compareResult.HasUpdates {
		log.Info().Msg("No updates available")
		fmt.Println("✅ All targets are up to date")
		return nil
	}

	// Build update items with patch groups and labels
	updateItems := buildUpdateItems(config, compareResult.Results)

	// Group updates by patch group
	patchGroups := groupUpdatesByPatchGroup(updateItems)

	// Output the apply plan
	if options.DryRun {
		outputDryRunPlan(patchGroups)
	} else if options.Local {
		outputLocalPlan(updateItems)

		// Apply all updates directly to local files — no git operations
		for _, update := range updateItems {
			if err := applyUpdate(config, update); err != nil {
				return fmt.Errorf("failed to apply update for %s in %s: %w", update.ItemName, update.TargetFile, err)
			}
			fmt.Printf("  ✓ Updated %s in %s: %s → %s\n",
				update.ItemName,
				update.TargetFile,
				update.CurrentVersion,
				update.LatestVersion)
		}

		fmt.Println("\n✅ Successfully applied all updates locally")
	} else {
		outputApplyPlan(patchGroups)

		// Check if target actor is configured
		if config.TargetActor == nil {
			return fmt.Errorf("targetActor is required for applying changes")
		}

		// Apply changes for each patch group
		if err := applyPatchGroups(config, patchGroups); err != nil {
			log.Error().Err(err).Msg("Failed to apply patch groups")
			return fmt.Errorf("apply error: %w", err)
		}

		fmt.Println("\n✅ Successfully applied all updates")
	}

	return nil
}
