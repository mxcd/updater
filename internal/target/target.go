package target

import (
	"github.com/mxcd/updater/internal/configuration"
)

// TargetClient defines the interface for all target implementations
type TargetClient interface {
	// ReadCurrentVersion reads the current version from the target
	ReadCurrentVersion() (string, error)

	// WriteVersion writes a new version to the target
	WriteVersion(version string) error

	// GetTargetInfo returns metadata about this target
	GetTargetInfo() *TargetInfo

	// Validate checks if the target is valid and accessible
	Validate() error
}

// TargetInfo contains metadata about a target
type TargetInfo struct {
	Name         string
	Type         configuration.TargetType
	File         string
	Source       string
	CurrentValue string
}

// TargetFactory creates target clients based on configuration
type TargetFactory struct {
	config *configuration.Config
}

// NewTargetFactory creates a new target factory
func NewTargetFactory(config *configuration.Config) *TargetFactory {
	return &TargetFactory{
		config: config,
	}
}

// CreateTarget creates a target client based on the target configuration
// This method is deprecated - use CreateTargetForUpdateItem instead
func (f *TargetFactory) CreateTarget(target *configuration.Target) (TargetClient, error) {
	// For backward compatibility, use the first update item if available
	if len(target.Items) > 0 {
		return f.CreateTargetForUpdateItem(target, &target.Items[0])
	}
	return nil, &UnsupportedTargetTypeError{Type: target.Type}
}

// CreateTargetForUpdateItem creates a target client for a specific update item
func (f *TargetFactory) CreateTargetForUpdateItem(target *configuration.Target, updateItem *configuration.TargetItem) (TargetClient, error) {
	switch target.Type {
	case configuration.TargetTypeTerraformVariable:
		return NewTerraformVariableTargetForUpdateItem(target, updateItem)
	case configuration.TargetTypeSubchart:
		return NewSubchartTargetForUpdateItem(target, updateItem)
	default:
		return nil, &UnsupportedTargetTypeError{Type: target.Type}
	}
}

// CreateAllTargets creates target clients for all configured targets
func (f *TargetFactory) CreateAllTargets() ([]TargetClient, error) {
	targets := make([]TargetClient, 0, len(f.config.Targets))
	for _, targetConfig := range f.config.Targets {
		target, err := f.CreateTarget(targetConfig)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, nil
}
