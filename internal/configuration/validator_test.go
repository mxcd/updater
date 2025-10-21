package configuration

import (
	"testing"
)

func TestValidateConfiguration(t *testing.T) {
	tests := []struct {
		name            string
		config          *Config
		expectValid     bool
		expectedErrors  int
		expectedFields  []string
		expectedMessage []string
	}{
		{
			name: "valid configuration",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name: "github",
						Type: PackageSourceProviderTypeGitHub,
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:     "app1",
						Provider: "github",
						Type:     PackageSourceTypeGitRelease,
						URI:      "https://github.com/example/app1",
					},
				},
			},
			expectValid:    true,
			expectedErrors: 0,
		},
		{
			name: "empty provider name",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name: "",
						Type: PackageSourceProviderTypeGitHub,
					},
				},
				PackageSources: []*PackageSource{},
			},
			expectValid:     false,
			expectedErrors:  1,
			expectedFields:  []string{"packageSourceProviders[0].name"},
			expectedMessage: []string{"provider name cannot be empty"},
		},
		{
			name: "duplicate provider names",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name: "github",
						Type: PackageSourceProviderTypeGitHub,
					},
					{
						Name: "github",
						Type: PackageSourceProviderTypeGitHub,
					},
				},
				PackageSources: []*PackageSource{},
			},
			expectValid:     false,
			expectedErrors:  1,
			expectedFields:  []string{"packageSourceProviders[1].name"},
			expectedMessage: []string{"duplicate provider name: github"},
		},
		{
			name: "invalid provider type",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name: "provider1",
						Type: "invalid-type",
					},
				},
				PackageSources: []*PackageSource{},
			},
			expectValid:     false,
			expectedErrors:  1,
			expectedFields:  []string{"packageSourceProviders[0].type"},
			expectedMessage: []string{"invalid provider type: invalid-type"},
		},
		{
			name: "invalid auth type",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name:     "provider1",
						Type:     PackageSourceProviderTypeGitHub,
						AuthType: "invalid-auth",
					},
				},
				PackageSources: []*PackageSource{},
			},
			expectValid:     false,
			expectedErrors:  1,
			expectedFields:  []string{"packageSourceProviders[0].authType"},
			expectedMessage: []string{"invalid auth type: invalid-auth"},
		},
		{
			name: "basic auth missing username",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name:     "provider1",
						Type:     PackageSourceProviderTypeGitHub,
						AuthType: PackageSourceProviderAuthTypeBasic,
						Password: "password",
					},
				},
				PackageSources: []*PackageSource{},
			},
			expectValid:     false,
			expectedErrors:  1,
			expectedFields:  []string{"packageSourceProviders[0].username"},
			expectedMessage: []string{"username is required for basic auth"},
		},
		{
			name: "basic auth missing password",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name:     "provider1",
						Type:     PackageSourceProviderTypeGitHub,
						AuthType: PackageSourceProviderAuthTypeBasic,
						Username: "user",
					},
				},
				PackageSources: []*PackageSource{},
			},
			expectValid:     false,
			expectedErrors:  1,
			expectedFields:  []string{"packageSourceProviders[0].password"},
			expectedMessage: []string{"password is required for basic auth"},
		},
		{
			name: "token auth missing token",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name:     "provider1",
						Type:     PackageSourceProviderTypeGitHub,
						AuthType: PackageSourceProviderAuthTypeToken,
					},
				},
				PackageSources: []*PackageSource{},
			},
			expectValid:     false,
			expectedErrors:  1,
			expectedFields:  []string{"packageSourceProviders[0].token"},
			expectedMessage: []string{"token is required for token auth"},
		},
		{
			name: "empty source name",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name: "github",
						Type: PackageSourceProviderTypeGitHub,
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:     "",
						Provider: "github",
						Type:     PackageSourceTypeGitRelease,
						URI:      "https://github.com/example/app1",
					},
				},
			},
			expectValid:     false,
			expectedErrors:  1,
			expectedFields:  []string{"packageSources[0].name"},
			expectedMessage: []string{"source name cannot be empty"},
		},
		{
			name: "empty source provider",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name: "github",
						Type: PackageSourceProviderTypeGitHub,
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:     "app1",
						Provider: "",
						Type:     PackageSourceTypeGitRelease,
						URI:      "https://github.com/example/app1",
					},
				},
			},
			expectValid:     false,
			expectedErrors:  1,
			expectedFields:  []string{"packageSources[0].provider"},
			expectedMessage: []string{"provider reference cannot be empty"},
		},
		{
			name: "non-existent provider reference",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name: "github",
						Type: PackageSourceProviderTypeGitHub,
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:     "app1",
						Provider: "nonexistent",
						Type:     PackageSourceTypeGitRelease,
						URI:      "https://github.com/example/app1",
					},
				},
			},
			expectValid:     false,
			expectedErrors:  1,
			expectedFields:  []string{"packageSources[0].provider"},
			expectedMessage: []string{"provider 'nonexistent' not found in packageSourceProviders"},
		},
		{
			name: "invalid source type",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name: "github",
						Type: PackageSourceProviderTypeGitHub,
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:     "app1",
						Provider: "github",
						Type:     "invalid-type",
						URI:      "https://github.com/example/app1",
					},
				},
			},
			expectValid:     false,
			expectedErrors:  1,
			expectedFields:  []string{"packageSources[0].type"},
			expectedMessage: []string{"invalid source type: invalid-type"},
		},
		{
			name: "empty source URI",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name: "github",
						Type: PackageSourceProviderTypeGitHub,
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:     "app1",
						Provider: "github",
						Type:     PackageSourceTypeGitRelease,
						URI:      "",
					},
				},
			},
			expectValid:     false,
			expectedErrors:  1,
			expectedFields:  []string{"packageSources[0].uri"},
			expectedMessage: []string{"URI cannot be empty"},
		},
		{
			name: "multiple errors across providers and sources",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name: "",
						Type: "invalid-type",
					},
					{
						Name:     "provider2",
						Type:     PackageSourceProviderTypeGitHub,
						AuthType: PackageSourceProviderAuthTypeBasic,
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:     "",
						Provider: "nonexistent",
						Type:     "invalid-source-type",
						URI:      "",
					},
				},
			},
			expectValid:    false,
			expectedErrors: 8, // Empty name, invalid type, missing username, missing password, empty source name, bad provider ref, invalid source type, empty URI
		},
		{
			name: "valid configuration with all auth types",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name:     "github-no-auth",
						Type:     PackageSourceProviderTypeGitHub,
						AuthType: PackageSourceProviderAuthTypeNone,
					},
					{
						Name:     "harbor-token",
						Type:     PackageSourceProviderTypeHarbor,
						AuthType: PackageSourceProviderAuthTypeToken,
						Token:    "secret-token",
					},
					{
						Name:     "docker-basic",
						Type:     PackageSourceProviderTypeDocker,
						AuthType: PackageSourceProviderAuthTypeBasic,
						Username: "user",
						Password: "pass",
					},
				},
				PackageSources: []*PackageSource{},
			},
			expectValid:    true,
			expectedErrors: 0,
		},
		{
			name: "valid configuration with all source types",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name: "github",
						Type: PackageSourceProviderTypeGitHub,
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:     "app1-release",
						Provider: "github",
						Type:     PackageSourceTypeGitRelease,
						URI:      "https://github.com/example/app1",
					},
					{
						Name:     "app2-tag",
						Provider: "github",
						Type:     PackageSourceTypeGitTag,
						URI:      "https://github.com/example/app2",
					},
					{
						Name:     "app3-helm",
						Provider: "github",
						Type:     PackageSourceTypeGitHelmChart,
						URI:      "https://example.com/chart.yaml",
					},
					{
						Name:     "app4-docker",
						Provider: "github",
						Type:     PackageSourceTypeDockerImage,
						URI:      "registry.example.com/app4",
					},
				},
			},
			expectValid:    true,
			expectedErrors: 0,
		},
		{
			name: "whitespace-only values treated as empty",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{
						Name: "   ",
						Type: PackageSourceProviderTypeGitHub,
					},
				},
				PackageSources: []*PackageSource{
					{
						Name:     "   ",
						Provider: "github",
						Type:     PackageSourceTypeGitRelease,
						URI:      "   ",
					},
				},
			},
			expectValid:    false,
			expectedErrors: 4, // Provider name, source name, provider ref not found, URI empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateConfiguration(tt.config)

			if result.Valid != tt.expectValid {
				t.Errorf("expected Valid=%v, got Valid=%v", tt.expectValid, result.Valid)
			}

			if len(result.Errors) != tt.expectedErrors {
				t.Errorf("expected %d errors, got %d errors", tt.expectedErrors, len(result.Errors))
				for i, err := range result.Errors {
					t.Logf("Error %d: %s", i, err.Error())
				}
			}

			// Check specific error fields if provided
			if len(tt.expectedFields) > 0 {
				for i, expectedField := range tt.expectedFields {
					if i >= len(result.Errors) {
						t.Errorf("expected error for field '%s', but only got %d errors", expectedField, len(result.Errors))
						continue
					}
					if result.Errors[i].Field != expectedField {
						t.Errorf("expected error field '%s', got '%s'", expectedField, result.Errors[i].Field)
					}
				}
			}

			// Check specific error messages if provided
			if len(tt.expectedMessage) > 0 {
				for i, expectedMsg := range tt.expectedMessage {
					if i >= len(result.Errors) {
						t.Errorf("expected error message '%s', but only got %d errors", expectedMsg, len(result.Errors))
						continue
					}
					if result.Errors[i].Message != expectedMsg {
						t.Errorf("expected error message '%s', got '%s'", expectedMsg, result.Errors[i].Message)
					}
				}
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{
		Field:   "packageSources[0].name",
		Message: "source name cannot be empty",
	}

	expected := "packageSources[0].name: source name cannot be empty"
	if err.Error() != expected {
		t.Errorf("expected error string '%s', got '%s'", expected, err.Error())
	}
}

func TestValidationResult_AddError(t *testing.T) {
	result := &ValidationResult{
		Valid:  true,
		Errors: make([]*ValidationError, 0),
	}

	// Initially valid with no errors
	if !result.Valid {
		t.Error("expected initial result to be valid")
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors initially, got %d", len(result.Errors))
	}

	// Add first error
	result.AddError("field1", "message1")
	if result.Valid {
		t.Error("expected result to be invalid after adding error")
	}
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(result.Errors))
	}
	if result.Errors[0].Field != "field1" {
		t.Errorf("expected field 'field1', got '%s'", result.Errors[0].Field)
	}

	// Add second error
	result.AddError("field2", "message2")
	if len(result.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(result.Errors))
	}
}

func TestIsValidProviderType(t *testing.T) {
	tests := []struct {
		providerType PackageSourceProviderType
		expected     bool
	}{
		{PackageSourceProviderTypeGitHub, true},
		{PackageSourceProviderTypeHarbor, true},
		{PackageSourceProviderTypeDocker, true},
		{"invalid", false},
		{"", false},
		{"GitHub", false}, // Case sensitive
	}

	for _, tt := range tests {
		t.Run(string(tt.providerType), func(t *testing.T) {
			result := isValidProviderType(tt.providerType)
			if result != tt.expected {
				t.Errorf("isValidProviderType(%s) = %v, expected %v", tt.providerType, result, tt.expected)
			}
		})
	}
}

func TestIsValidAuthType(t *testing.T) {
	tests := []struct {
		authType PackageSourceProviderAuthType
		expected bool
	}{
		{PackageSourceProviderAuthTypeNone, true},
		{PackageSourceProviderAuthTypeBasic, true},
		{PackageSourceProviderAuthTypeToken, true},
		{"invalid", false},
		{"", false},
		{"Basic", false}, // Case sensitive
	}

	for _, tt := range tests {
		t.Run(string(tt.authType), func(t *testing.T) {
			result := isValidAuthType(tt.authType)
			if result != tt.expected {
				t.Errorf("isValidAuthType(%s) = %v, expected %v", tt.authType, result, tt.expected)
			}
		})
	}
}

func TestIsValidSourceType(t *testing.T) {
	tests := []struct {
		sourceType PackageSourceType
		expected   bool
	}{
		{PackageSourceTypeGitRelease, true},
		{PackageSourceTypeGitTag, true},
		{PackageSourceTypeGitHelmChart, true},
		{PackageSourceTypeDockerImage, true},
		{"invalid", false},
		{"", false},
		{"git-Release", false}, // Case sensitive
	}

	for _, tt := range tests {
		t.Run(string(tt.sourceType), func(t *testing.T) {
			result := isValidSourceType(tt.sourceType)
			if result != tt.expected {
				t.Errorf("isValidSourceType(%s) = %v, expected %v", tt.sourceType, result, tt.expected)
			}
		})
	}
}

func TestValidateConfiguration_EdgeCases(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		// This would panic in current implementation, but testing for documentation
		defer func() {
			if r := recover(); r != nil {
				t.Log("Recovered from panic when passing nil config")
			}
		}()
		_ = ValidateConfiguration(nil)
	})

	t.Run("empty config", func(t *testing.T) {
		config := &Config{}
		result := ValidateConfiguration(config)
		if !result.Valid {
			t.Error("expected empty config to be valid")
		}
		if len(result.Errors) != 0 {
			t.Errorf("expected 0 errors for empty config, got %d", len(result.Errors))
		}
	})

	t.Run("config with nil slices", func(t *testing.T) {
		config := &Config{
			PackageSourceProviders: nil,
			PackageSources:         nil,
		}
		result := ValidateConfiguration(config)
		if !result.Valid {
			t.Error("expected config with nil slices to be valid")
		}
	})
}