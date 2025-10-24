package configuration

import (
	"testing"
)

func TestValidateConfiguration_SubchartTarget(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		expectedValid bool
		expectedError string
	}{
		{
			name: "valid subchart target",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name:    "helm-registry",
						Type:    PackageSourceProviderTypeHelm,
						BaseUrl: "oci://registry.example.com/charts",
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:      "my-chart",
						Provider:  "helm-registry",
						Type:      PackageSourceTypeHelmRepository,
						ChartName: "my-chart",
					},
				},
				Targets: []*Target{
					{
						Name: "update-chart",
						Type: TargetTypeSubchart,
						File: "Chart.yaml",
						Items: []TargetItem{
							{
								Source:       "my-chart",
								SubchartName: "my-dependency",
							},
						},
					},
				},
			},
			expectedValid: true,
		},
		{
			name: "missing subchartName",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name:    "helm-registry",
						Type:    PackageSourceProviderTypeHelm,
						BaseUrl: "oci://registry.example.com/charts",
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:      "my-chart",
						Provider:  "helm-registry",
						Type:      PackageSourceTypeHelmRepository,
						ChartName: "my-chart",
					},
				},
				Targets: []*Target{
					{
						Name: "update-chart",
						Type: TargetTypeSubchart,
						File: "Chart.yaml",
						Items: []TargetItem{
							{
								Source: "my-chart",
								// SubchartName is missing
							},
						},
					},
				},
			},
			expectedValid: false,
			expectedError: "subchartName is required for subchart target",
		},
		{
			name: "empty subchartName",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name:    "helm-registry",
						Type:    PackageSourceProviderTypeHelm,
						BaseUrl: "oci://registry.example.com/charts",
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:      "my-chart",
						Provider:  "helm-registry",
						Type:      PackageSourceTypeHelmRepository,
						ChartName: "my-chart",
					},
				},
				Targets: []*Target{
					{
						Name: "update-chart",
						Type: TargetTypeSubchart,
						File: "Chart.yaml",
						Items: []TargetItem{
							{
								Source:       "my-chart",
								SubchartName: "   ", // Only whitespace
							},
						},
					},
				},
			},
			expectedValid: false,
			expectedError: "subchartName is required for subchart target",
		},
		{
			name: "multiple subchart items with valid config",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name:    "helm-registry",
						Type:    PackageSourceProviderTypeHelm,
						BaseUrl: "oci://registry.example.com/charts",
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:      "chart-a",
						Provider:  "helm-registry",
						Type:      PackageSourceTypeHelmRepository,
						ChartName: "chart-a",
					},
					{
						Name:      "chart-b",
						Provider:  "helm-registry",
						Type:      PackageSourceTypeHelmRepository,
						ChartName: "chart-b",
					},
				},
				Targets: []*Target{
					{
						Name: "update-charts",
						Type: TargetTypeSubchart,
						File: "Chart.yaml",
						Items: []TargetItem{
							{
								Source:       "chart-a",
								SubchartName: "dependency-a",
							},
							{
								Source:       "chart-b",
								SubchartName: "dependency-b",
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