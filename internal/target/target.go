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
func (f *TargetFactory) CreateTarget(target *configuration.Target) (TargetClient, error) {
	switch target.Type {
	case configuration.TargetTypeTerraformVariable:
		return NewTerraformVariableTarget(target)
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