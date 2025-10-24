package configuration

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// LoadConfiguration reads and parses the configuration from the given path
// If the path is a directory, it loads all .yml files within it and merges them
// It also performs environment variable and SOPS substitution
func LoadConfiguration(configPath string) (*Config, error) {
	// Check if path is a directory
	fileInfo, err := os.Stat(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to access configuration path: %w", err)
	}

	var config *Config
	if fileInfo.IsDir() {
		// Load all .yml files from directory
		config, err = loadConfigurationFromDirectory(configPath)
		if err != nil {
			return nil, err
		}
	} else {
		// Load single configuration file
		config, err = loadSingleConfigurationFile(configPath)
		if err != nil {
			return nil, err
		}
	}

	// Perform variable substitution
	ctx := NewSubstitutionContext()
	if err := ctx.SubstituteInConfig(config); err != nil {
		return nil, fmt.Errorf("failed to substitute variables: %w", err)
	}

	// Expand wildcard patterns in target files
	if err := ExpandWildcardTargets(config); err != nil {
		return nil, fmt.Errorf("failed to expand wildcard targets: %w", err)
	}

	return config, nil
}

// loadSingleConfigurationFile reads and parses a single configuration file
func loadSingleConfigurationFile(configPath string) (*Config, error) {
	// Read the configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file: %w", err)
	}

	// Parse the YAML configuration
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse configuration YAML: %w", err)
	}

	return &config, nil
}

// loadConfigurationFromDirectory loads all .yml files from a directory and merges them
func loadConfigurationFromDirectory(dirPath string) (*Config, error) {
	// Read directory entries
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration directory: %w", err)
	}

	// Collect all .yml files
	var configFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") {
			configFiles = append(configFiles, filepath.Join(dirPath, name))
		}
	}

	if len(configFiles) == 0 {
		return nil, fmt.Errorf("no .yml or .yaml files found in directory: %s", dirPath)
	}

	log.Debug().
		Str("directory", dirPath).
		Int("fileCount", len(configFiles)).
		Msg("Loading configuration from directory")

	// Load all configuration files
	var configs []*Config
	for _, filePath := range configFiles {
		config, err := loadSingleConfigurationFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load %s: %w", filePath, err)
		}
		configs = append(configs, config)
	}

	// Merge all configurations
	mergedConfig, err := mergeConfigurations(configs)
	if err != nil {
		return nil, fmt.Errorf("failed to merge configurations: %w", err)
	}

	return mergedConfig, nil
}

// mergeConfigurations merges multiple Config objects into a single Config
// It concatenates all slices and checks for duplicate names
func mergeConfigurations(configs []*Config) (*Config, error) {
	if len(configs) == 0 {
		return &Config{}, nil
	}

	if len(configs) == 1 {
		return configs[0], nil
	}

	merged := &Config{
		PackageSourceProviders: make([]*PackageSourceProvider, 0),
		PackageSources:         make([]*PackageSource, 0),
		Targets:                make([]*Target, 0),
	}

	// Track names for duplicate detection
	providerNames := make(map[string]bool)
	sourceNames := make(map[string]bool)
	targetNames := make(map[string]bool)

	for _, config := range configs {
		// Merge package source providers
		for _, provider := range config.PackageSourceProviders {
			if providerNames[provider.Name] {
				return nil, fmt.Errorf("duplicate package source provider name: %s", provider.Name)
			}
			providerNames[provider.Name] = true
			merged.PackageSourceProviders = append(merged.PackageSourceProviders, provider)
		}

		// Merge package sources
		for _, source := range config.PackageSources {
			if sourceNames[source.Name] {
				return nil, fmt.Errorf("duplicate package source name: %s", source.Name)
			}
			sourceNames[source.Name] = true
			merged.PackageSources = append(merged.PackageSources, source)
		}

		// Merge targets
		for _, target := range config.Targets {
			if targetNames[target.Name] {
				return nil, fmt.Errorf("duplicate target name: %s", target.Name)
			}
			targetNames[target.Name] = true
			merged.Targets = append(merged.Targets, target)
		}

		// Use the last non-nil targetActor
		if config.TargetActor != nil {
			merged.TargetActor = config.TargetActor
		}
	}

	return merged, nil
}

// ExpandWildcardTargets expands wildcard patterns in target file paths
// Supports both single-level wildcards (*) and recursive wildcards (**)
func ExpandWildcardTargets(config *Config) error {
	expandedTargets := make([]*Target, 0, len(config.Targets))

	for _, target := range config.Targets {
		// Check if file path contains wildcard characters
		if strings.Contains(target.File, "*") || strings.Contains(target.File, "?") || strings.Contains(target.File, "[") {
			var matches []string
			var err error

			// Check if pattern contains ** for recursive matching
			if strings.Contains(target.File, "**") {
				matches, err = recursiveGlob(target.File)
			} else {
				// Use standard filepath.Glob for single-level wildcards
				matches, err = filepath.Glob(target.File)
			}

			if err != nil {
				log.Warn().
					Err(err).
					Str("pattern", target.File).
					Msg("Failed to expand wildcard pattern")
				// Keep the original target if glob fails
				expandedTargets = append(expandedTargets, target)
				continue
			}

			if len(matches) == 0 {
				log.Warn().
					Str("pattern", target.File).
					Msg("Wildcard pattern matched no files")
				// Keep the original target if no matches
				expandedTargets = append(expandedTargets, target)
				continue
			}

			log.Debug().
				Str("pattern", target.File).
				Int("matches", len(matches)).
				Msg("Expanded wildcard pattern")

			// Create a new target for each matched file
			for _, match := range matches {
				expandedTarget := &Target{
					Name:            target.Name,
					Type:            target.Type,
					File:            match,
					Items:           target.Items,
					PatchGroup:      target.PatchGroup,
					Labels:          target.Labels,
					WildcardPattern: target.File, // Store the original pattern
					IsWildcardMatch: true,
				}
				expandedTargets = append(expandedTargets, expandedTarget)
			}
		} else {
			// No wildcard, keep as-is
			expandedTargets = append(expandedTargets, target)
		}
	}

	config.Targets = expandedTargets
	return nil
}

// recursiveGlob performs recursive glob matching for patterns containing **
// The ** pattern matches zero or more directories
func recursiveGlob(pattern string) ([]string, error) {
	// Split pattern into parts
	parts := strings.Split(filepath.ToSlash(pattern), "/")
	
	// Find the index of the first ** in the pattern
	recursiveIndex := -1
	for i, part := range parts {
		if part == "**" {
			recursiveIndex = i
			break
		}
	}

	if recursiveIndex == -1 {
		// No ** found, use standard glob
		return filepath.Glob(pattern)
	}

	// Get the base directory (everything before **)
	var baseDir string
	if recursiveIndex == 0 {
		baseDir = "."
	} else {
		// Reconstruct the base directory path
		// If the original pattern started with /, preserve it
		if strings.HasPrefix(pattern, string(filepath.Separator)) {
			baseDir = string(filepath.Separator) + filepath.Join(parts[:recursiveIndex]...)
		} else {
			baseDir = filepath.Join(parts[:recursiveIndex]...)
		}
	}

	// Get the pattern after ** (everything after **)
	var afterPattern string
	if recursiveIndex < len(parts)-1 {
		afterPattern = filepath.Join(parts[recursiveIndex+1:]...)
	}

	// Collect all matches
	var matches []string

	// Walk the directory tree starting from baseDir
	err := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip directories we can't read
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip directories in matching
		if d.IsDir() {
			return nil
		}

		// If we have a pattern after **, match it
		if afterPattern != "" {
			// Get the relative path from baseDir
			relPath, err := filepath.Rel(baseDir, path)
			if err != nil {
				return nil
			}

			// Check if the path ends with the after pattern
			// This handles patterns like "Chart.yaml" matching "dev/app1/Chart.yaml"
			if matchesAfterPattern(relPath, afterPattern) {
				matches = append(matches, path)
			}
		} else {
			// No pattern after **, match all files
			matches = append(matches, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return matches, nil
}

// matchesAfterPattern checks if a path matches the pattern after **
// It supports both exact matches and suffix matches
func matchesAfterPattern(path, pattern string) bool {
	// Normalize path separators
	path = filepath.ToSlash(path)
	pattern = filepath.ToSlash(pattern)
	
	// If pattern has no wildcards, check if path ends with pattern
	if !strings.ContainsAny(pattern, "*?[") {
		return strings.HasSuffix(path, pattern) || path == pattern
	}
	
	// For patterns with wildcards, try matching against each segment
	// This handles cases like "*/*.yaml" matching "dev/Chart.yaml"
	matched, _ := filepath.Match(pattern, path)
	if matched {
		return true
	}
	
	// Also check if the basename matches the pattern
	// This handles "*.yaml" matching files at any depth
	basename := filepath.Base(path)
	matched, _ = filepath.Match(pattern, basename)
	return matched
}