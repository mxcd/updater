package configuration

import (
	"testing"
)

func TestValidateConfiguration_HelmProvider(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		expectValid   bool
		errorContains string
	}{
		{
			name: "valid helm provider and helm-chart source",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name:     "helm-repo",
						Type:     PackageSourceProviderTypeHelm,
						BaseUrl:  "https://charts.example.com",
						AuthType: PackageSourceProviderAuthTypeNone,
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:      "nginx-chart",
						Provider:  "helm-repo",
						Type:      PackageSourceTypeHelmRepository,
						URI:       "https://charts.example.com",
						ChartName: "nginx",
					},
				},
			},
			expectValid: true,
		},
		{
			name: "helm-repository without chartName",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name:     "helm-repo",
						Type:     PackageSourceProviderTypeHelm,
						BaseUrl:  "https://charts.example.com",
						AuthType: PackageSourceProviderAuthTypeNone,
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:      "nginx-chart",
						Provider:  "helm-repo",
						Type:      PackageSourceTypeHelmRepository,
						ChartName: "",
					},
				},
			},
			expectValid:   false,
			errorContains: "chartName is required for helm-repository",
		},
		{
			name: "helm-repository without provider baseUrl",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name:     "helm-repo",
						Type:     PackageSourceProviderTypeHelm,
						AuthType: PackageSourceProviderAuthTypeNone,
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:      "nginx-chart",
						Provider:  "helm-repo",
						Type:      PackageSourceTypeHelmRepository,
						ChartName: "nginx",
					},
				},
			},
			expectValid:   false,
			errorContains: "must have baseUrl configured",
		},
		{
			name: "helm-repository with non-helm provider",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name: "github",
						Type: PackageSourceProviderTypeGitHub,
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:      "nginx-chart",
						Provider:  "github",
						Type:      PackageSourceTypeHelmRepository,
						ChartName: "nginx",
					},
				},
			},
			expectValid:   false,
			errorContains: "requires provider type 'helm'",
		},
		{
			name: "helm provider with git-release source",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name:     "helm-repo",
						Type:     PackageSourceProviderTypeHelm,
						BaseUrl:  "https://charts.example.com",
						AuthType: PackageSourceProviderAuthTypeNone,
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:     "app-release",
						Provider: "helm-repo",
						Type:     PackageSourceTypeGitRelease,
						URI:      "https://github.com/example/app",
					},
				},
			},
			expectValid:   false,
			errorContains: "requires provider type 'github'",
		},
		{
			name: "helm provider with authentication",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name:     "private-helm-repo",
						Type:     PackageSourceProviderTypeHelm,
						BaseUrl:  "https://private-charts.example.com",
						AuthType: PackageSourceProviderAuthTypeBasic,
						Username: "user",
						Password: "pass",
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:      "private-chart",
						Provider:  "private-helm-repo",
						Type:      PackageSourceTypeHelmRepository,
						ChartName: "app",
					},
				},
			},
			expectValid: true,
		},
		{
			name: "helm provider with token authentication",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name:     "token-helm-repo",
						Type:     PackageSourceProviderTypeHelm,
						BaseUrl:  "https://charts.example.com",
						AuthType: PackageSourceProviderAuthTypeToken,
						Token:    "secret-token",
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:      "token-chart",
						Provider:  "token-helm-repo",
						Type:      PackageSourceTypeHelmRepository,
						ChartName: "nginx",
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

func TestValidateSourceProviderCombination(t *testing.T) {
	tests := []struct {
		name          string
		sourceType    PackageSourceType
		providerType  PackageSourceProviderType
		expectError   bool
		errorContains string
	}{
		{
			name:         "helm-repository with helm provider",
			sourceType:   PackageSourceTypeHelmRepository,
			providerType: PackageSourceProviderTypeHelm,
			expectError:  false,
		},
		{
			name:          "helm-repository with github provider",
			sourceType:    PackageSourceTypeHelmRepository,
			providerType:  PackageSourceProviderTypeGitHub,
			expectError:   true,
			errorContains: "requires provider type 'helm'",
		},
		{
			name:         "git-release with github provider",
			sourceType:   PackageSourceTypeGitRelease,
			providerType: PackageSourceProviderTypeGitHub,
			expectError:  false,
		},
		{
			name:          "git-release with helm provider",
			sourceType:    PackageSourceTypeGitRelease,
			providerType:  PackageSourceProviderTypeHelm,
			expectError:   true,
			errorContains: "requires provider type 'github'",
		},
		{
			name:         "docker-image with docker provider",
			sourceType:   PackageSourceTypeDockerImage,
			providerType: PackageSourceProviderTypeDocker,
			expectError:  false,
		},
		{
			name:         "docker-image with harbor provider",
			sourceType:   PackageSourceTypeDockerImage,
			providerType: PackageSourceProviderTypeHarbor,
			expectError:  false,
		},
		{
			name:          "docker-image with github provider",
			sourceType:    PackageSourceTypeDockerImage,
			providerType:  PackageSourceProviderTypeGitHub,
			expectError:   true,
			errorContains: "requires provider type 'docker' or 'harbor'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSourceProviderCombination(tt.sourceType, tt.providerType)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestIsValidProviderType_Helm(t *testing.T) {
	tests := []struct {
		providerType PackageSourceProviderType
		expected     bool
	}{
		{PackageSourceProviderTypeHelm, true},
		{PackageSourceProviderTypeGitHub, true},
		{PackageSourceProviderTypeDocker, true},
		{PackageSourceProviderTypeHarbor, true},
		{PackageSourceProviderType("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.providerType), func(t *testing.T) {
			result := isValidProviderType(tt.providerType)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for provider type %s", tt.expected, result, tt.providerType)
			}
		})
	}
}

func TestIsValidSourceType_HelmRepository(t *testing.T) {
	tests := []struct {
		sourceType PackageSourceType
		expected   bool
	}{
		{PackageSourceTypeHelmRepository, true},
		{PackageSourceTypeGitRelease, true},
		{PackageSourceTypeGitTag, true},
		{PackageSourceTypeGitHelmChart, true},
		{PackageSourceTypeDockerImage, true},
		{PackageSourceType("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.sourceType), func(t *testing.T) {
			result := isValidSourceType(tt.sourceType)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for source type %s", tt.expected, result, tt.sourceType)
			}
		})
	}
}
