package actions

import (
	"fmt"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/mxcd/updater/internal/compare"
	"github.com/mxcd/updater/internal/configuration"
	"github.com/mxcd/updater/internal/scraper"
	"github.com/rs/zerolog/log"
)

type ApplyOptions struct {
	ConfigPath string
	DryRun     bool
	Limit      int
	Only       string
}

// PatchGroup represents a group of updates that should be applied together
type PatchGroup struct {
	Name    string
	Updates []*UpdateItem
	Labels  []string
}

// UpdateItem represents a single update to be applied
type UpdateItem struct {
	TargetName     string
	TargetFile     string
	ItemName       string
	SourceName     string
	CurrentVersion string
	LatestVersion  string
	UpdateType     compare.UpdateType
	PatchGroup     string
	Labels         []string
}

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

	log.Info().Msg("Configuration is valid")

	// Get comparison results without outputting them
	compareResult, err := compareInternal(config, options.Limit, options.Only)
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
	} else {
		outputApplyPlan(patchGroups)
		// TODO: Implement actual apply logic (git operations, PR creation)
		log.Info().Msg("Apply functionality will be implemented in future iterations")
		fmt.Println("\n⚠️  Note: Actual apply functionality (git operations, PR creation) is not yet implemented")
	}

	return nil
}

// buildUpdateItems creates UpdateItem objects from comparison results with patch groups and labels
func buildUpdateItems(config *configuration.Config, results []*compare.ComparisonResult) []*UpdateItem {
	items := make([]*UpdateItem, 0)

	for _, result := range results {
		if !result.NeedsUpdate || result.Error != nil {
			continue
		}

		// Find the target and item configuration
		targetConfig, updateItemConfig := findTargetAndItem(config, result)
		if targetConfig == nil || updateItemConfig == nil {
			log.Warn().
				Str("target", result.TargetName).
				Str("source", result.SourceName).
				Msg("Could not find target or item configuration")
			continue
		}

		// Determine patch group (item overrides target)
		patchGroup := updateItemConfig.PatchGroup
		if patchGroup == "" {
			patchGroup = targetConfig.PatchGroup
		}
		if patchGroup == "" {
			patchGroup = "default"
		}

		// Merge labels (target labels + item labels)
		labels := mergeLabels(targetConfig.Labels, updateItemConfig.Labels)

		item := &UpdateItem{
			TargetName:     result.TargetName,
			TargetFile:     result.TargetFile,
			ItemName:       updateItemConfig.Name,
			SourceName:     result.SourceName,
			CurrentVersion: result.CurrentVersion,
			LatestVersion:  result.LatestVersion,
			UpdateType:     result.UpdateType,
			PatchGroup:     patchGroup,
			Labels:         labels,
		}

		items = append(items, item)
	}

	return items
}

// findTargetAndItem finds the target and item configuration for a comparison result
func findTargetAndItem(config *configuration.Config, result *compare.ComparisonResult) (*configuration.Target, *configuration.TargetItem) {
	for _, target := range config.Targets {
		if target.File != result.TargetFile {
			continue
		}

		for _, item := range target.Items {
			itemName := item.Name
			if itemName == "" {
				itemName = target.Name
			}

			if itemName == result.TargetName && item.Source == result.SourceName {
				return target, &item
			}
		}
	}
	return nil, nil
}

// mergeLabels merges two label slices, removing duplicates
func mergeLabels(targetLabels, itemLabels []string) []string {
	labelMap := make(map[string]bool)
	result := make([]string, 0)

	// Add target labels
	for _, label := range targetLabels {
		if label != "" && !labelMap[label] {
			labelMap[label] = true
			result = append(result, label)
		}
	}

	// Add item labels
	for _, label := range itemLabels {
		if label != "" && !labelMap[label] {
			labelMap[label] = true
			result = append(result, label)
		}
	}

	return result
}

// groupUpdatesByPatchGroup groups updates by their patch group
func groupUpdatesByPatchGroup(items []*UpdateItem) []*PatchGroup {
	groupMap := make(map[string]*PatchGroup)

	for _, item := range items {
		group, exists := groupMap[item.PatchGroup]
		if !exists {
			group = &PatchGroup{
				Name:    item.PatchGroup,
				Updates: make([]*UpdateItem, 0),
				Labels:  make([]string, 0),
			}
			groupMap[item.PatchGroup] = group
		}

		group.Updates = append(group.Updates, item)

		// Merge labels from all items in the group
		group.Labels = mergeLabels(group.Labels, item.Labels)
	}

	// Convert map to slice
	groups := make([]*PatchGroup, 0, len(groupMap))
	for _, group := range groupMap {
		groups = append(groups, group)
	}

	return groups
}

// outputDryRunPlan outputs the plan in dry-run mode
func outputDryRunPlan(groups []*PatchGroup) {
	fmt.Println("\n🔍 DRY RUN - Apply Plan")
	fmt.Println("========================\n")

	totalCommits := 0
	totalPRs := len(groups)

	for i, group := range groups {
		fmt.Printf("📦 Patch Group %d/%d: %s\n", i+1, len(groups), group.Name)
		if len(group.Labels) > 0 {
			fmt.Printf("   Labels: %s\n", strings.Join(group.Labels, ", "))
		}
		fmt.Printf("   Updates: %d\n\n", len(group.Updates))

		// Group by target file for commits
		fileGroups := groupUpdatesByFile(group.Updates)
		totalCommits += len(fileGroups)

		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.AppendHeader(table.Row{"Target", "File", "Source", "Current", "→", "Latest", "Type"})

		for _, update := range group.Updates {
			displayName := update.TargetName
			if update.ItemName != "" {
				displayName = update.ItemName
			}

			t.AppendRow(table.Row{
				displayName,
				update.TargetFile,
				update.SourceName,
				update.CurrentVersion,
				"→",
				update.LatestVersion,
				update.UpdateType,
			})
		}

		t.SetStyle(table.StyleRounded)
		t.Render()
		fmt.Println()

		fmt.Printf("   📝 Would create: %d commit(s) in %d file(s)\n", len(fileGroups), len(fileGroups))
		fmt.Printf("   🔀 Would create: 1 pull request\n")
		if len(group.Labels) > 0 {
			fmt.Printf("   🏷️  PR labels: %s\n", strings.Join(group.Labels, ", "))
		}
		fmt.Println()
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("📊 Summary:\n")
	fmt.Printf("   • Total patch groups: %d\n", totalPRs)
	fmt.Printf("   • Total commits: %d\n", totalCommits)
	fmt.Printf("   • Total pull requests: %d\n", totalPRs)
	fmt.Println()
	fmt.Println("💡 This is a dry run. Use 'apply' without --dry-run to execute.")
}

// outputApplyPlan outputs the plan for actual execution
func outputApplyPlan(groups []*PatchGroup) {
	fmt.Println("\n🚀 Apply Plan")
	fmt.Println("=============\n")

	for i, group := range groups {
		fmt.Printf("📦 Patch Group %d/%d: %s\n", i+1, len(groups), group.Name)
		if len(group.Labels) > 0 {
			fmt.Printf("   Labels: %s\n", strings.Join(group.Labels, ", "))
		}

		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.AppendHeader(table.Row{"Target", "File", "Current", "→", "Latest", "Type"})

		for _, update := range group.Updates {
			displayName := update.TargetName
			if update.ItemName != "" {
				displayName = update.ItemName
			}

			t.AppendRow(table.Row{
				displayName,
				update.TargetFile,
				update.CurrentVersion,
				"→",
				update.LatestVersion,
				update.UpdateType,
			})
		}

		t.SetStyle(table.StyleRounded)
		t.Render()
		fmt.Println()
	}
}

// groupUpdatesByFile groups updates by target file
func groupUpdatesByFile(updates []*UpdateItem) map[string][]*UpdateItem {
	fileMap := make(map[string][]*UpdateItem)

	for _, update := range updates {
		fileMap[update.TargetFile] = append(fileMap[update.TargetFile], update)
	}

	return fileMap
}

// compareInternal performs comparison without outputting results
func compareInternal(config *configuration.Config, limit int, only string) (*CompareResult, error) {
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
