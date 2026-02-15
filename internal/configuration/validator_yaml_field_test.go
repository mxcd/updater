package configuration

import (
	"testing"
)

func TestValidateConfiguration_YamlFieldTarget(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		expectedValid bool
		expectedError string
	}{
		{
			name: "valid yaml-field target",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name: "docker-hub",
						Type: PackageSourceProviderTypeDocker,
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:     "my-image",
						Provider: "docker-hub",
						Type:     PackageSourceTypeDockerImage,
						URI:      "nginx",
					},
				},
				Targets: []*Target{
					{
						Name: "update-image",
						Type: TargetTypeYamlField,
						File: "values.yaml",
						Items: []TargetItem{
							{
								Source:   "my-image",
								YamlPath: "image.tag",
							},
						},
					},
				},
			},
			expectedValid: true,
		},
		{
			name: "missing yamlPath",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name: "docker-hub",
						Type: PackageSourceProviderTypeDocker,
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:     "my-image",
						Provider: "docker-hub",
						Type:     PackageSourceTypeDockerImage,
						URI:      "nginx",
					},
				},
				Targets: []*Target{
					{
						Name: "update-image",
						Type: TargetTypeYamlField,
						File: "values.yaml",
						Items: []TargetItem{
							{
								Source: "my-image",
								// YamlPath is missing
							},
						},
					},
				},
			},
			expectedValid: false,
			expectedError: "yamlPath is required for yaml-field target",
		},
		{
			name: "empty yamlPath (whitespace only)",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name: "docker-hub",
						Type: PackageSourceProviderTypeDocker,
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:     "my-image",
						Provider: "docker-hub",
						Type:     PackageSourceTypeDockerImage,
						URI:      "nginx",
					},
				},
				Targets: []*Target{
					{
						Name: "update-image",
						Type: TargetTypeYamlField,
						File: "values.yaml",
						Items: []TargetItem{
							{
								Source:   "my-image",
								YamlPath: "   ",
							},
						},
					},
				},
			},
			expectedValid: false,
			expectedError: "yamlPath is required for yaml-field target",
		},
		{
			name: "multiple yaml-field items with valid config",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name: "docker-hub",
						Type: PackageSourceProviderTypeDocker,
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:     "app-image",
						Provider: "docker-hub",
						Type:     PackageSourceTypeDockerImage,
						URI:      "myapp",
					},
					{
						Name:     "sidecar-image",
						Provider: "docker-hub",
						Type:     PackageSourceTypeDockerImage,
						URI:      "envoy",
					},
				},
				Targets: []*Target{
					{
						Name: "update-images",
						Type: TargetTypeYamlField,
						File: "values.yaml",
						Items: []TargetItem{
							{
								Source:   "app-image",
								YamlPath: "image.tag",
							},
							{
								Source:   "sidecar-image",
								YamlPath: "sidecar.image.tag",
							},
						},
					},
				},
			},
			expectedValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateConfiguration(tt.config)

			if result.Valid != tt.expectedValid {
				t.Errorf("expected valid=%v, got valid=%v", tt.expectedValid, result.Valid)
				if len(result.Errors) > 0 {
					t.Logf("validation errors:")
					for _, err := range result.Errors {
						t.Logf("  - %s: %s", err.Field, err.Message)
					}
				}
			}

			if !tt.expectedValid && tt.expectedError != "" {
				found := false
				for _, err := range result.Errors {
					if err.Message == tt.expectedError {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error message '%s' not found in validation errors", tt.expectedError)
					t.Logf("actual errors:")
					for _, err := range result.Errors {
						t.Logf("  - %s: %s", err.Field, err.Message)
					}
				}
			}
		})
	}
}
