package target

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/mxcd/updater/internal/configuration"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// SubchartTarget implements the TargetClient interface for Helm Chart.yaml files
type SubchartTarget struct {
	config       *configuration.Target
	updateItem   *configuration.TargetItem
	fileContents string
	chartData    *ChartYAML
}

// ChartYAML represents the structure of a Helm Chart.yaml file
type ChartYAML struct {
	APIVersion   string                 `yaml:"apiVersion"`
	Name         string                 `yaml:"name"`
	Description  string                 `yaml:"description,omitempty"`
	Type         string                 `yaml:"type,omitempty"`
	Version      string                 `yaml:"version"`
	AppVersion   string                 `yaml:"appVersion,omitempty"`
	Dependencies []ChartDependency      `yaml:"dependencies,omitempty"`
	Raw          map[string]interface{} `yaml:",inline"`
}

// ChartDependency represents a dependency in Chart.yaml
type ChartDependency struct {
	Name         string        `yaml:"name"`
	Version      string        `yaml:"version"`
	Repository   string        `yaml:"repository"`
	Condition    string        `yaml:"condition,omitempty"`
	Tags         []string      `yaml:"tags,omitempty"`
	Enabled      *bool         `yaml:"enabled,omitempty"`
	ImportValues []interface{} `yaml:"import-values,omitempty"`
	Alias        string        `yaml:"alias,omitempty"`
}

// NewSubchartTargetForUpdateItem creates a new subchart target for a specific update item
func NewSubchartTargetForUpdateItem(config *configuration.Target, updateItem *configuration.TargetItem) (*SubchartTarget, error) {
	if updateItem.SubchartName == "" {
		return nil, fmt.Errorf("subchartName is required for subchart target")
	}

	target := &SubchartTarget{
		config:     config,
		updateItem: updateItem,
	}

	// Read and parse the file contents during initialization
	if err := target.readFile(); err != nil {
		return nil, err
	}

	return target, nil
}

// readFile reads and parses the Chart.yaml file
func (t *SubchartTarget) readFile() error {
	content, err := os.ReadFile(t.config.File)
	if err != nil {
		if os.IsNotExist(err) {
			return &FileNotFoundError{Path: t.config.File}
		}
		return fmt.Errorf("failed to read file %s: %w", t.config.File, err)
	}
	t.fileContents = string(content)

	// Parse the YAML
	t.chartData = &ChartYAML{}
	if err := yaml.Unmarshal(content, t.chartData); err != nil {
		return fmt.Errorf("failed to parse Chart.yaml: %w", err)
	}

	return nil
}

// ReadCurrentVersion reads the current version of the specified subchart dependency
func (t *SubchartTarget) ReadCurrentVersion() (string, error) {
	log.Debug().
		Str("file", t.config.File).
		Str("subchart", t.updateItem.SubchartName).
		Msg("Reading current version from Chart.yaml")

	// Find the dependency with matching name
	for _, dep := range t.chartData.Dependencies {
		if dep.Name == t.updateItem.SubchartName {
			log.Debug().
				Str("file", t.config.File).
				Str("subchart", t.updateItem.SubchartName).
				Str("version", dep.Version).
				Msg("Found current version")
			return dep.Version, nil
		}
	}

	return "", &DependencyNotFoundError{
		Dependency: t.updateItem.SubchartName,
		File:       t.config.File,
	}
}

// WriteVersion writes a new version to the specified subchart dependency
func (t *SubchartTarget) WriteVersion(version string) error {
	log.Debug().
		Str("file", t.config.File).
		Str("subchart", t.updateItem.SubchartName).
		Str("version", version).
		Msg("Writing new version to Chart.yaml")

	// Update the version in the parsed data
	found := false
	for i := range t.chartData.Dependencies {
		if t.chartData.Dependencies[i].Name == t.updateItem.SubchartName {
			t.chartData.Dependencies[i].Version = version
			found = true
			break
		}
	}

	if !found {
		return &DependencyNotFoundError{
			Dependency: t.updateItem.SubchartName,
			File:       t.config.File,
		}
	}

	// Use regex to replace the version while preserving formatting
	// This approach maintains comments and formatting better than full YAML rewrite

	// Try multiple patterns to handle different formatting styles
	patterns := []string{
		// Pattern 1: Multi-line format with potential extra fields
		// Matches: - name: xxx\n    version: yyy
		// Uses [^\n-]* between name and version lines to avoid crossing dependency boundaries
		fmt.Sprintf(
			`(?m)(^\s*-\s+name:\s+%s\s*\n(?:\s+[^\n]*\n)*?\s+version:\s+)([^\s\n]+)`,
			regexp.QuoteMeta(t.updateItem.SubchartName),
		),
		// Pattern 2: Inline format with commas and braces
		// Matches: - { name: xxx, version: yyy, repository: zzz }
		// Constrained to single brace block
		fmt.Sprintf(
			`(\{[^}]*name:\s+%s[^}]*version:\s+)([^,}\s]+)`,
			regexp.QuoteMeta(t.updateItem.SubchartName),
		),
		// Pattern 3: Single line with spaces between fields (no braces)
		// Matches: - name: xxx version: yyy repository: zzz
		fmt.Sprintf(
			`(?m)(^\s*-[^-\n]*name:\s+%s[^-\n]*version:\s+)([^\s,}\n]+)`,
			regexp.QuoteMeta(t.updateItem.SubchartName),
		),
	}

	var newContents string
	matched := false

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(t.fileContents) {
			newContents = re.ReplaceAllString(t.fileContents, fmt.Sprintf("${1}%s", version))
			matched = true
			break
		}
	}

	if !matched {
		return &DependencyNotFoundError{
			Dependency: t.updateItem.SubchartName,
			File:       t.config.File,
		}
	}

	// Write the file
	if err := os.WriteFile(t.config.File, []byte(newContents), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", t.config.File, err)
	}

	// Update internal state
	t.fileContents = newContents

	log.Debug().
		Str("file", t.config.File).
		Str("subchart", t.updateItem.SubchartName).
		Str("version", version).
		Msg("Successfully wrote new version")

	return nil
}

// GetTargetInfo returns metadata about this target
func (t *SubchartTarget) GetTargetInfo() *TargetInfo {
	currentVersion, err := t.ReadCurrentVersion()
	if err != nil {
		log.Warn().Err(err).Str("file", t.config.File).Str("subchart", t.updateItem.SubchartName).Msg("Failed to read current version for target info")
	}
	targetName := t.updateItem.Name
	if targetName == "" {
		targetName = t.config.Name
	}
	return &TargetInfo{
		Name:         targetName,
		Type:         t.config.Type,
		File:         t.config.File,
		Source:       t.updateItem.Source,
		CurrentValue: currentVersion,
	}
}

// Validate checks if the target is valid and accessible
func (t *SubchartTarget) Validate() error {
	// Check if file exists and is readable
	if err := t.readFile(); err != nil {
		return err
	}

	// Check if file is named Chart.yaml or Chart.yml
	fileName := strings.ToLower(t.config.File)
	if !strings.HasSuffix(fileName, "chart.yaml") && !strings.HasSuffix(fileName, "chart.yml") {
		return &InvalidFileFormatError{
			File:   t.config.File,
			Reason: "file must be named Chart.yaml or Chart.yml",
		}
	}

	// Note: We don't check if the dependency exists here because:
	// - When using wildcards, not all matched files may contain the dependency
	// - This is permissive behavior - only error if NO files match
	// - ReadCurrentVersion() and WriteVersion() will handle missing dependencies gracefully

	log.Debug().
		Str("file", t.config.File).
		Str("subchart", t.updateItem.SubchartName).
		Msg("Subchart target validation successful")

	return nil
}
