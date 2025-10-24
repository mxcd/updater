package compare

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mxcd/updater/internal/configuration"
	"github.com/mxcd/updater/internal/target"
	"github.com/rs/zerolog/log"
)

// ComparisonResult represents the result of comparing a target with its source
type ComparisonResult struct {
	TargetName         string
	TargetFile         string
	TargetType         configuration.TargetType
	TargetItemName     string // Variable name for terraform, subchart name for helm
	SourceName         string
	CurrentVersion     string
	LatestVersion      string
	UpdateType         UpdateType
	NeedsUpdate        bool
	Error              error
	IsWildcardMatch    bool   // True if this target was expanded from a wildcard pattern
	WildcardPattern    string // The original wildcard pattern if IsWildcardMatch is true
	PatchGroup         string // Patch group for grouping updates together
}

// UpdateType represents the type of update (major, minor, patch, none)
type UpdateType string

const (
	UpdateTypeMajor UpdateType = "major"
	UpdateTypeMinor UpdateType = "minor"
	UpdateTypePatch UpdateType = "patch"
	UpdateTypeNone  UpdateType = "none"
)

// CompareEngine performs comparison between targets and sources
type CompareEngine struct {
	config        *configuration.Config
	targetFactory *target.TargetFactory
}

// NewCompareEngine creates a new comparison engine
func NewCompareEngine(config *configuration.Config) *CompareEngine {
	return &CompareEngine{
		config:        config,
		targetFactory: target.NewTargetFactory(config),
	}
}

// CompareAll compares all configured targets with their sources
func (e *CompareEngine) CompareAll() ([]*ComparisonResult, error) {
	log.Debug().Msg("Starting comparison of all targets")

	results := make([]*ComparisonResult, 0)

	for _, targetConfig := range e.config.Targets {
		// Each target can have multiple update items
		for _, updateItem := range targetConfig.Items {
			result := e.compareTargetUpdateItem(targetConfig, &updateItem)
			results = append(results, result)
		}
	}

	log.Debug().
		Int("total", len(results)).
		Int("needsUpdate", countNeedingUpdate(results)).
		Msg("Comparison complete")

	return results, nil
}

// compareTargetUpdateItem compares a single target update item with its source
func (e *CompareEngine) compareTargetUpdateItem(targetConfig *configuration.Target, updateItem *configuration.TargetItem) *ComparisonResult {
	// Use updateItem name if specified, otherwise use target name
	targetName := updateItem.Name
	if targetName == "" {
		targetName = targetConfig.Name
	}

	// Get target-specific item name (variable name or subchart name)
	var itemName string
	switch targetConfig.Type {
	case configuration.TargetTypeTerraformVariable:
		itemName = updateItem.TerraformVariableName
	case configuration.TargetTypeSubchart:
		itemName = updateItem.SubchartName
	}

	// Determine patch group - use item's patch group if set, otherwise use target's patch group
	patchGroup := updateItem.PatchGroup
	if patchGroup == "" {
		patchGroup = targetConfig.PatchGroup
	}

	result := &ComparisonResult{
		TargetName:      targetName,
		TargetFile:      targetConfig.File,
		TargetType:      targetConfig.Type,
		TargetItemName:  itemName,
		SourceName:      updateItem.Source,
		IsWildcardMatch: targetConfig.IsWildcardMatch,
		WildcardPattern: targetConfig.WildcardPattern,
		PatchGroup:      patchGroup,
	}

	log.Debug().
		Str("target", targetName).
		Str("source", updateItem.Source).
		Msg("Comparing target with source")

	// Find the source
	source := e.findSource(updateItem.Source)
	if source == nil {
		result.Error = fmt.Errorf("source '%s' not found", updateItem.Source)
		log.Error().
			Str("target", targetName).
			Str("source", updateItem.Source).
			Msg("Source not found")
		return result
	}

	// Check if source has versions
	if len(source.Versions) == 0 {
		result.Error = fmt.Errorf("no versions available for source '%s'", updateItem.Source)
		log.Warn().
			Str("target", targetName).
			Str("source", updateItem.Source).
			Msg("No versions available for source")
		return result
	}

	// Get latest version from source (first version is the latest)
	latestVersion := source.Versions[0]
	result.LatestVersion = latestVersion.Version

	// Create target client
	targetClient, err := e.targetFactory.CreateTargetForUpdateItem(targetConfig, updateItem)
	if err != nil {
		result.Error = fmt.Errorf("failed to create target client: %w", err)
		log.Error().
			Err(err).
			Str("target", targetName).
			Msg("Failed to create target client")
		return result
	}

	// Read current version from target
	currentVersion, err := targetClient.ReadCurrentVersion()
	if err != nil {
		result.Error = fmt.Errorf("failed to read current version: %w", err)
		
		// For wildcard matches with dependency not found errors, use debug level logging
		// These are expected when not all files contain the specified dependency
		errStr := err.Error()
		if targetConfig.IsWildcardMatch && strings.Contains(errStr, "dependency") && strings.Contains(errStr, "not found") {
			log.Debug().
				Err(err).
				Str("target", targetName).
				Str("file", targetConfig.File).
				Msg("Dependency not found in wildcard-matched file (expected)")
		} else {
			log.Error().
				Err(err).
				Str("target", targetName).
				Msg("Failed to read current version")
		}
		return result
	}
	result.CurrentVersion = currentVersion

	// Normalize versions for comparison (remove v prefix)
	normalizedCurrent := normalizeVersion(currentVersion)
	normalizedLatest := normalizeVersion(latestVersion.Version)

	// Determine if update is needed and what type
	if normalizedCurrent == normalizedLatest {
		result.NeedsUpdate = false
		result.UpdateType = UpdateTypeNone
		log.Debug().
			Str("target", targetConfig.Name).
			Str("version", currentVersion).
			Msg("Target is up to date")
	} else {
		result.NeedsUpdate = true

		// Try to find current version in source versions to get semantic version info
		var currentSemVer *configuration.PackageSourceVersion
		for _, v := range source.Versions {
			if normalizeVersion(v.Version) == normalizedCurrent {
				currentSemVer = v
				break
			}
		}

		// If current version not found in source, try to parse it
		if currentSemVer == nil {
			currentSemVer = parseVersionString(currentVersion)
		}

		result.UpdateType = determineUpdateType(currentSemVer, latestVersion)
		log.Debug().
			Str("target", targetConfig.Name).
			Str("current", currentVersion).
			Str("latest", latestVersion.Version).
			Str("updateType", string(result.UpdateType)).
			Msg("Update available")
	}

	return result
}

// findSource finds a source by name
func (e *CompareEngine) findSource(name string) *configuration.PackageSource {
	for _, source := range e.config.PackageSources {
		if source.Name == name {
			return source
		}
	}
	return nil
}

// normalizeVersion removes the "v" or "V" prefix from a version string for comparison
func normalizeVersion(version string) string {
	normalized := strings.TrimPrefix(version, "v")
	normalized = strings.TrimPrefix(normalized, "V")
	return normalized
}

// parseVersionString attempts to parse a version string into semantic version components
func parseVersionString(version string) *configuration.PackageSourceVersion {
	v := &configuration.PackageSourceVersion{
		Version: version,
	}

	// Remove common prefixes
	versionStr := strings.TrimPrefix(version, "v")
	versionStr = strings.TrimPrefix(versionStr, "V")

	// Split by dots
	parts := strings.Split(versionStr, ".")

	if len(parts) >= 1 {
		if major, err := strconv.Atoi(parts[0]); err == nil {
			v.MajorVersion = major
		}
	}
	if len(parts) >= 2 {
		if minor, err := strconv.Atoi(parts[1]); err == nil {
			v.MinorVersion = minor
		}
	}
	if len(parts) >= 3 {
		// Handle patch versions that might have additional suffixes
		patchPart := strings.Split(parts[2], "-")[0]
		if patch, err := strconv.Atoi(patchPart); err == nil {
			v.PatchVersion = patch
		}
	}

	return v
}

// determineUpdateType determines the type of update (major, minor, patch)
func determineUpdateType(current, latest *configuration.PackageSourceVersion) UpdateType {
	if current == nil || latest == nil {
		// If we can't parse versions, we can't determine type
		return UpdateTypeNone
	}

	if latest.MajorVersion > current.MajorVersion {
		return UpdateTypeMajor
	}
	if latest.MajorVersion < current.MajorVersion {
		return UpdateTypeNone // Downgrade
	}

	if latest.MinorVersion > current.MinorVersion {
		return UpdateTypeMinor
	}
	if latest.MinorVersion < current.MinorVersion {
		return UpdateTypeNone // Downgrade
	}

	if latest.PatchVersion > current.PatchVersion {
		return UpdateTypePatch
	}

	return UpdateTypeNone
}

// countNeedingUpdate counts how many results need an update
func countNeedingUpdate(results []*ComparisonResult) int {
	count := 0
	for _, r := range results {
		if r.NeedsUpdate {
			count++
		}
	}
	return count
}
