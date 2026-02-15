package target

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/mxcd/updater/internal/configuration"
	"github.com/rs/zerolog/log"
)

// TerraformVariableTarget implements the TargetClient interface for Terraform variable files
type TerraformVariableTarget struct {
	config       *configuration.Target
	updateItem   *configuration.TargetItem
	fileContents string
}

// NewTerraformVariableTarget creates a new terraform variable target (deprecated)
// Use NewTerraformVariableTargetForUpdateItem instead
func NewTerraformVariableTarget(config *configuration.Target) (*TerraformVariableTarget, error) {
	// For backward compatibility, use the first update item
	if len(config.Items) == 0 {
		return nil, fmt.Errorf("no updateItems configured for target")
	}
	return NewTerraformVariableTargetForUpdateItem(config, &config.Items[0])
}

// NewTerraformVariableTargetForUpdateItem creates a new terraform variable target for a specific update item
func NewTerraformVariableTargetForUpdateItem(config *configuration.Target, updateItem *configuration.TargetItem) (*TerraformVariableTarget, error) {
	if updateItem.TerraformVariableName == "" {
		return nil, fmt.Errorf("terraformVariableName is required for terraform-variable target")
	}

	target := &TerraformVariableTarget{
		config:     config,
		updateItem: updateItem,
	}

	// Read the file contents during initialization
	if err := target.readFile(); err != nil {
		return nil, err
	}

	return target, nil
}

// readFile reads the target file into memory
func (t *TerraformVariableTarget) readFile() error {
	content, err := os.ReadFile(t.config.File)
	if err != nil {
		if os.IsNotExist(err) {
			return &FileNotFoundError{Path: t.config.File}
		}
		return fmt.Errorf("failed to read file %s: %w", t.config.File, err)
	}
	t.fileContents = string(content)
	return nil
}

// ReadCurrentVersion reads the current version from the terraform variable file
func (t *TerraformVariableTarget) ReadCurrentVersion() (string, error) {
	log.Debug().
		Str("file", t.config.File).
		Str("variable", t.updateItem.TerraformVariableName).
		Msg("Reading current version from Terraform variable file")

	// Pattern to match Terraform variable default value
	// Supports both single and multi-line variable declarations
	// Examples:
	//   variable "version" { default = "1.0.0" }
	//   variable "version" {
	//     default = "1.0.0"
	//   }
	pattern := fmt.Sprintf(
		`(?s)variable\s+"%s"\s*\{.*?default\s*=\s*"([^"]+)"`,
		regexp.QuoteMeta(t.updateItem.TerraformVariableName),
	)

	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(t.fileContents)

	if len(matches) < 2 {
		return "", &VariableNotFoundError{
			Variable: t.updateItem.TerraformVariableName,
			File:     t.config.File,
		}
	}

	version := matches[1]
	log.Debug().
		Str("file", t.config.File).
		Str("variable", t.updateItem.TerraformVariableName).
		Str("version", version).
		Msg("Found current version")

	return version, nil
}

// WriteVersion writes a new version to the terraform variable file
func (t *TerraformVariableTarget) WriteVersion(version string) error {
	log.Debug().
		Str("file", t.config.File).
		Str("variable", t.updateItem.TerraformVariableName).
		Str("version", version).
		Msg("Writing new version to Terraform variable file")

	// Pattern to match and replace the default value
	pattern := fmt.Sprintf(
		`(?s)(variable\s+"%s"\s*\{.*?default\s*=\s*")([^"]+)(")`,
		regexp.QuoteMeta(t.updateItem.TerraformVariableName),
	)

	re := regexp.MustCompile(pattern)

	// Check if the pattern exists
	if !re.MatchString(t.fileContents) {
		return &VariableNotFoundError{
			Variable: t.updateItem.TerraformVariableName,
			File:     t.config.File,
		}
	}

	// Replace the version
	newContents := re.ReplaceAllString(t.fileContents, fmt.Sprintf("${1}%s${3}", version))

	// Write the file
	if err := os.WriteFile(t.config.File, []byte(newContents), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", t.config.File, err)
	}

	// Update internal state
	t.fileContents = newContents

	log.Debug().
		Str("file", t.config.File).
		Str("variable", t.updateItem.TerraformVariableName).
		Str("version", version).
		Msg("Successfully wrote new version")

	return nil
}

// GetTargetInfo returns metadata about this target
func (t *TerraformVariableTarget) GetTargetInfo() *TargetInfo {
	currentVersion, err := t.ReadCurrentVersion()
	if err != nil {
		log.Warn().Err(err).Str("file", t.config.File).Str("variable", t.updateItem.TerraformVariableName).Msg("Failed to read current version for target info")
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
func (t *TerraformVariableTarget) Validate() error {
	// Check if file exists and is readable
	if err := t.readFile(); err != nil {
		return err
	}

	// Check if file has .tf or .tfvars extension
	if !strings.HasSuffix(t.config.File, ".tf") && !strings.HasSuffix(t.config.File, ".tfvars") {
		return &InvalidFileFormatError{
			File:   t.config.File,
			Reason: "file must have .tf or .tfvars extension",
		}
	}

	// Check if variable exists in file
	_, err := t.ReadCurrentVersion()
	if err != nil {
		return err
	}

	log.Debug().
		Str("file", t.config.File).
		Str("variable", t.updateItem.TerraformVariableName).
		Msg("Terraform variable target validation successful")

	return nil
}
