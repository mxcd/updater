package configuration

import (
	"testing"
)

func TestValidateConfiguration_Targets(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		expectValid   bool
		errorContains string
	}{
		{
			name: "valid target configuration",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name: "github",
						Type: PackageSourceProviderTypeGitHub,
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:     "test-source",
						Provider: "github",
						Type:     PackageSourceTypeGitRelease,
						URI:      "https://github.com/test/repo",
					},
				},
				Targets: []*Target{
					{
						Name:                  "test-target",
						Type:                  TargetTypeTerraformVariable,
						File:                  "test.tf",
						TerraformVariableName: "version",
						Source:                "test-source",
					},
				},
			},
			expectValid: true,
		},
		{
			name: "target with empty name",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{Name: "github", Type: PackageSourceProviderTypeGitHub},
				},
				PackageSources: []*PackageSource{
					{Name: "test-source", Provider: "github", Type: PackageSourceTypeGitRelease, URI: "https://github.com/test/repo"},
				},
				Targets: []*Target{
					{
						Name:   "",
						Type:   TargetTypeTerraformVariable,
						File:   "test.tf",
						Source: "test-source",
					},
				},
			},
			expectValid:   false,
			errorContains: "name cannot be empty",
		},
		{
			name: "target with invalid type",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{Name: "github", Type: PackageSourceProviderTypeGitHub},
				},
				PackageSources: []*PackageSource{
					{Name: "test-source", Provider: "github", Type: PackageSourceTypeGitRelease, URI: "https://github.com/test/repo"},
				},
				Targets: []*Target{
					{
						Name:   "test-target",
						Type:   TargetType("invalid"),
						File:   "test.tf",
						Source: "test-source",
					},
				},
			},
			expectValid:   false,
			errorContains: "invalid target type",
		},
		{
			name: "target with empty file",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{Name: "github", Type: PackageSourceProviderTypeGitHub},
				},
				PackageSources: []*PackageSource{
					{Name: "test-source", Provider: "github", Type: PackageSourceTypeGitRelease, URI: "https://github.com/test/repo"},
				},
				Targets: []*Target{
					{
						Name:   "test-target",
						Type:   TargetTypeTerraformVariable,
						File:   "",
						Source: "test-source",
					},
				},
			},
			expectValid:   false,
			errorContains: "file path cannot be empty",
		},
		{
			name: "target with missing source reference",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{Name: "github", Type: PackageSourceProviderTypeGitHub},
				},
				PackageSources: []*PackageSource{
					{Name: "test-source", Provider: "github", Type: PackageSourceTypeGitRelease, URI: "https://github.com/test/repo"},
				},
				Targets: []*Target{
					{
						Name:   "test-target",
						Type:   TargetTypeTerraformVariable,
						File:   "test.tf",
						Source: "non-existent-source",
					},
				},
			},
			expectValid:   false,
			errorContains: "not found in packageSources",
		},
		{
			name: "terraform target without variable name",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{Name: "github", Type: PackageSourceProviderTypeGitHub},
				},
				PackageSources: []*PackageSource{
					{Name: "test-source", Provider: "github", Type: PackageSourceTypeGitRelease, URI: "https://github.com/test/repo"},
				},
				Targets: []*Target{
					{
						Name:                  "test-target",
						Type:                  TargetTypeTerraformVariable,
						File:                  "test.tf",
						TerraformVariableName: "",
						Source:                "test-source",
					},
				},
			},
			expectValid:   false,
			errorContains: "terraformVariableName is required",
		},
		{
			name: "multiple valid targets",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{Name: "github", Type: PackageSourceProviderTypeGitHub},
				},
				PackageSources: []*PackageSource{
					{Name: "source1", Provider: "github", Type: PackageSourceTypeGitRelease, URI: "https://github.com/test/repo1"},
					{Name: "source2", Provider: "github", Type: PackageSourceTypeGitRelease, URI: "https://github.com/test/repo2"},
				},
				Targets: []*Target{
					{
						Name:                  "target1",
						Type:                  TargetTypeTerraformVariable,
						File:                  "test1.tf",
						TerraformVariableName: "version1",
						Source:                "source1",
					},
					{
						Name:                  "target2",
						Type:                  TargetTypeTerraformVariable,
						File:                  "test2.tf",
						TerraformVariableName: "version2",
						Source:                "source2",
					},
				},
			},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateConfiguration(tt.config)

			if tt.expectValid && !result.Valid {
				t.Errorf("Expected valid configuration, but got errors: %v", result.Errors)
			}

			if !tt.expectValid && result.Valid {
				t.Errorf("Expected invalid configuration, but validation passed")
			}

			if !tt.expectValid && tt.errorContains != "" {
				found := false
				for _, err := range result.Errors {
					if contains(err.Message, tt.errorContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error containing '%s', but got errors: %v", tt.errorContains, result.Errors)
				}
			}
		})
	}
}

func TestIsValidTargetType(t *testing.T) {
	tests := []struct {
		targetType TargetType
		expected   bool
	}{
		{TargetTypeTerraformVariable, true},
		{TargetType("invalid"), false},
		{TargetType(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.targetType), func(t *testing.T) {
			result := isValidTargetType(tt.targetType)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}