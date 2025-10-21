package configuration

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadConfiguration reads and parses the configuration file from the given path
// It also performs environment variable and SOPS substitution
func LoadConfiguration(configPath string) (*Config, error) {
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

	// Perform variable substitution
	ctx := NewSubstitutionContext()
	if err := ctx.SubstituteInConfig(&config); err != nil {
		return nil, fmt.Errorf("failed to substitute variables: %w", err)
	}

	return &config, nil
}