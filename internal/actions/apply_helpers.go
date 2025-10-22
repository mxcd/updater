package actions

import (
	"github.com/mxcd/updater/internal/compare"
	"github.com/mxcd/updater/internal/configuration"
	"github.com/rs/zerolog/log"
)

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

		// Determine item name to display (priority: TerraformVariableName > Name > SourceName)
		itemName := updateItemConfig.TerraformVariableName
		if itemName == "" {
			itemName = updateItemConfig.Name
		}
		if itemName == "" {
			// Find the source to get its name as fallback
			for _, source := range config.PackageSources {
				if source.Name == result.SourceName {
					itemName = source.Name
					break
				}
			}
		}

		item := &UpdateItem{
			TargetName:     result.TargetName,
			TargetFile:     result.TargetFile,
			ItemName:       itemName,
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

// groupUpdatesByFile groups updates by target file
func groupUpdatesByFile(updates []*UpdateItem) map[string][]*UpdateItem {
	fileMap := make(map[string][]*UpdateItem)

	for _, update := range updates {
		fileMap[update.TargetFile] = append(fileMap[update.TargetFile], update)
	}

	return fileMap
}