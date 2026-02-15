package configuration

import (
	"os"
	"testing"
)

func TestTargetActorValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectValid bool
		expectError string
	}{
		{
			name: "valid targetActor with all fields",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{Name: "github", Type: PackageSourceProviderTypeGitHub},
				},
				PackageSources: []*PackageSource{
					{Name: "test-source", Provider: "github", Type: PackageSourceTypeGitRelease, URI: "https://github.com/test/repo"},
				},
				Targets: []*Target{
					{
						Name: "test-target",
						Type: TargetTypeTerraformVariable,
						File: "test.tf",
						Items: []TargetItem{
							{TerraformVariableName: "version", Source: "test-source"},
						},
					},
				},
				TargetActor: &TargetActor{
					Name:     "Test User",
					Email:    "test@example.com",
					Username: "testuser",
					Token:    "ghp_testtoken123",
				},
			},
			expectValid: true,
		},
		{
			name: "valid targetActor without optional token",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{Name: "github", Type: PackageSourceProviderTypeGitHub},
				},
				PackageSources: []*PackageSource{
					{Name: "test-source", Provider: "github", Type: PackageSourceTypeGitRelease, URI: "https://github.com/test/repo"},
				},
				Targets: []*Target{
					{
						Name: "test-target",
						Type: TargetTypeTerraformVariable,
						File: "test.tf",
						Items: []TargetItem{
							{TerraformVariableName: "version", Source: "test-source"},
						},
					},
				},
				TargetActor: &TargetActor{
					Name:     "Test User",
					Email:    "test@example.com",
					Username: "testuser",
				},
			},
			expectValid: true,
		},
		{
			name: "valid config without targetActor",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{Name: "github", Type: PackageSourceProviderTypeGitHub},
				},
				PackageSources: []*PackageSource{
					{Name: "test-source", Provider: "github", Type: PackageSourceTypeGitRelease, URI: "https://github.com/test/repo"},
				},
				Targets: []*Target{
					{
						Name: "test-target",
						Type: TargetTypeTerraformVariable,
						File: "test.tf",
						Items: []TargetItem{
							{TerraformVariableName: "version", Source: "test-source"},
						},
					},
				},
			},
			expectValid: true,
		},
		{
			name: "invalid targetActor missing name",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{Name: "github", Type: PackageSourceProviderTypeGitHub},
				},
				PackageSources: []*PackageSource{
					{Name: "test-source", Provider: "github", Type: PackageSourceTypeGitRelease, URI: "https://github.com/test/repo"},
				},
				Targets: []*Target{
					{
						Name: "test-target",
						Type: TargetTypeTerraformVariable,
						File: "test.tf",
						Items: []TargetItem{
							{TerraformVariableName: "version", Source: "test-source"},
						},
					},
				},
				TargetActor: &TargetActor{
					Name:     "",
					Email:    "test@example.com",
					Username: "testuser",
				},
			},
			expectValid: false,
			expectError: "targetActor.name",
		},
		{
			name: "invalid targetActor missing email",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{Name: "github", Type: PackageSourceProviderTypeGitHub},
				},
				PackageSources: []*PackageSource{
					{Name: "test-source", Provider: "github", Type: PackageSourceTypeGitRelease, URI: "https://github.com/test/repo"},
				},
				Targets: []*Target{
					{
						Name: "test-target",
						Type: TargetTypeTerraformVariable,
						File: "test.tf",
						Items: []TargetItem{
							{TerraformVariableName: "version", Source: "test-source"},
						},
					},
				},
				TargetActor: &TargetActor{
					Name:     "Test User",
					Email:    "",
					Username: "testuser",
				},
			},
			expectValid: false,
			expectError: "targetActor.email",
		},
		{
			name: "invalid targetActor missing username",
			config: &Config{
				PackageSourceProviders: []*PackageSourceProvider{
					{Name: "github", Type: PackageSourceProviderTypeGitHub},
				},
				PackageSources: []*PackageSource{
					{Name: "test-source", Provider: "github", Type: PackageSourceTypeGitRelease, URI: "https://github.com/test/repo"},
				},
				Targets: []*Target{
					{
						Name: "test-target",
						Type: TargetTypeTerraformVariable,
						File: "test.tf",
						Items: []TargetItem{
							{TerraformVariableName: "version", Source: "test-source"},
						},
					},
				},
				TargetActor: &TargetActor{
					Name:     "Test User",
					Email:    "test@example.com",
					Username: "",
				},
			},
			expectValid: false,
			expectError: "targetActor.username",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateConfiguration(tt.config)

			if result.Valid != tt.expectValid {
				t.Errorf("expected valid=%v, got valid=%v", tt.expectValid, result.Valid)
				for _, err := range result.Errors {
					t.Logf("  error: %s", err.Error())
				}
			}

			if !tt.expectValid && tt.expectError != "" {
				found := false
				for _, err := range result.Errors {
					if err.Field == tt.expectError {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error field %s not found in errors", tt.expectError)
					for _, err := range result.Errors {
						t.Logf("  actual error: %s", err.Error())
					}
				}
			}
		})
	}
}

func TestTargetActorSubstitution(t *testing.T) {
	// Set up test environment variables
	os.Setenv("TEST_NAME", "Environment User")
	os.Setenv("TEST_EMAIL", "env@example.com")
	os.Setenv("TEST_USERNAME", "envuser")
	os.Setenv("TEST_TOKEN", "env_token_123")
	defer func() {
		os.Unsetenv("TEST_NAME")
		os.Unsetenv("TEST_EMAIL")
		os.Unsetenv("TEST_USERNAME")
		os.Unsetenv("TEST_TOKEN")
	}()

	tests := []struct {
		name          string
		targetActor   *TargetActor
		expectedName  string
		expectedEmail string
		expectedUser  string
		expectedToken string
		expectError   bool
	}{
		{
			name: "substitute all fields with env vars",
			targetActor: &TargetActor{
				Name:     "${TEST_NAME}",
				Email:    "${TEST_EMAIL}",
				Username: "${TEST_USERNAME}",
				Token:    "${TEST_TOKEN}",
			},
			expectedName:  "Environment User",
			expectedEmail: "env@example.com",
			expectedUser:  "envuser",
			expectedToken: "env_token_123",
			expectError:   false,
		},
		{
			name: "substitute some fields with env vars",
			targetActor: &TargetActor{
				Name:     "${TEST_NAME}",
				Email:    "literal@example.com",
				Username: "${TEST_USERNAME}",
				Token:    "literal_token",
			},
			expectedName:  "Environment User",
			expectedEmail: "literal@example.com",
			expectedUser:  "envuser",
			expectedToken: "literal_token",
			expectError:   false,
		},
		{
			name: "no substitution needed",
			targetActor: &TargetActor{
				Name:     "Literal User",
				Email:    "literal@example.com",
				Username: "literaluser",
				Token:    "literal_token",
			},
			expectedName:  "Literal User",
			expectedEmail: "literal@example.com",
			expectedUser:  "literaluser",
			expectedToken: "literal_token",
			expectError:   false,
		},
		{
			name: "missing environment variable",
			targetActor: &TargetActor{
				Name:     "${NONEXISTENT_VAR}",
				Email:    "test@example.com",
				Username: "testuser",
				Token:    "test_token",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewSubstitutionContext()
			err := ctx.substituteInTargetActor(tt.targetActor)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.targetActor.Name != tt.expectedName {
				t.Errorf("expected name=%q, got %q", tt.expectedName, tt.targetActor.Name)
			}
			if tt.targetActor.Email != tt.expectedEmail {
				t.Errorf("expected email=%q, got %q", tt.expectedEmail, tt.targetActor.Email)
			}
			if tt.targetActor.Username != tt.expectedUser {
				t.Errorf("expected username=%q, got %q", tt.expectedUser, tt.targetActor.Username)
			}
			if tt.targetActor.Token != tt.expectedToken {
				t.Errorf("expected token=%q, got %q", tt.expectedToken, tt.targetActor.Token)
			}
		})
	}
}
