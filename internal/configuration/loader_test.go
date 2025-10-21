package configuration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		configContent string
		wantErr       bool
		errContains   string
		validate      func(*testing.T, *Config)
	}{
		{
			name: "valid configuration with single provider and source",
			configContent: `packageSourceProviders:
  - name: github
    type: github

packageSources:
  - name: traefik-helm-chart
    provider: github
    type: git-helm-chart
    uri: https://raw.githubusercontent.com/traefik/traefik-helm-chart/refs/heads/master/traefik/Chart.yaml
`,
			wantErr: false,
			validate: func(t *testing.T, config *Config) {
				if len(config.PackageSourceProviders) != 1 {
					t.Errorf("expected 1 provider, got %d", len(config.PackageSourceProviders))
				}
				if config.PackageSourceProviders[0].Name != "github" {
					t.Errorf("expected provider name 'github', got '%s'", config.PackageSourceProviders[0].Name)
				}
				if config.PackageSourceProviders[0].Type != PackageSourceProviderTypeGitHub {
					t.Errorf("expected provider type 'github', got '%s'", config.PackageSourceProviders[0].Type)
				}
				if len(config.PackageSources) != 1 {
					t.Errorf("expected 1 source, got %d", len(config.PackageSources))
				}
				if config.PackageSources[0].Name != "traefik-helm-chart" {
					t.Errorf("expected source name 'traefik-helm-chart', got '%s'", config.PackageSources[0].Name)
				}
			},
		},
		{
			name: "valid configuration with multiple providers",
			configContent: `packageSourceProviders:
  - name: github
    type: github
  - name: harbor
    type: harbor
    baseUrl: https://harbor.example.com
    authType: token
    token: secret-token
  - name: docker
    type: docker
    authType: basic
    username: user
    password: pass

packageSources:
  - name: app1
    provider: github
    type: git-release
    uri: https://github.com/example/app1
`,
			wantErr: false,
			validate: func(t *testing.T, config *Config) {
				if len(config.PackageSourceProviders) != 3 {
					t.Errorf("expected 3 providers, got %d", len(config.PackageSourceProviders))
				}
				// Check harbor provider
				harbor := config.PackageSourceProviders[1]
				if harbor.Name != "harbor" {
					t.Errorf("expected provider name 'harbor', got '%s'", harbor.Name)
				}
				if harbor.BaseUrl != "https://harbor.example.com" {
					t.Errorf("expected baseUrl 'https://harbor.example.com', got '%s'", harbor.BaseUrl)
				}
				if harbor.AuthType != PackageSourceProviderAuthTypeToken {
					t.Errorf("expected authType 'token', got '%s'", harbor.AuthType)
				}
				if harbor.Token != "secret-token" {
					t.Errorf("expected token 'secret-token', got '%s'", harbor.Token)
				}
				// Check docker provider
				docker := config.PackageSourceProviders[2]
				if docker.AuthType != PackageSourceProviderAuthTypeBasic {
					t.Errorf("expected authType 'basic', got '%s'", docker.AuthType)
				}
				if docker.Username != "user" {
					t.Errorf("expected username 'user', got '%s'", docker.Username)
				}
			},
		},
		{
			name: "valid configuration with version constraints",
			configContent: `packageSourceProviders:
  - name: github
    type: github

packageSources:
  - name: app1
    provider: github
    type: git-release
    uri: https://github.com/example/app1
    versionConstraint: ">=1.0.0 <2.0.0"
`,
			wantErr: false,
			validate: func(t *testing.T, config *Config) {
				if config.PackageSources[0].VersionConstraint != ">=1.0.0 <2.0.0" {
					t.Errorf("expected version constraint '>=1.0.0 <2.0.0', got '%s'", config.PackageSources[0].VersionConstraint)
				}
			},
		},
		{
			name: "valid configuration with cached versions",
			configContent: `packageSourceProviders:
  - name: github
    type: github

packageSources:
  - name: app1
    provider: github
    type: git-release
    uri: https://github.com/example/app1
    versions:
      - version: "1.2.3"
        versionInformation: "Release v1.2.3"
        majorVersion: 1
        minorVersion: 2
        patchVersion: 3
`,
			wantErr: false,
			validate: func(t *testing.T, config *Config) {
				if len(config.PackageSources[0].Versions) != 1 {
					t.Errorf("expected 1 cached version, got %d", len(config.PackageSources[0].Versions))
				}
				version := config.PackageSources[0].Versions[0]
				if version.Version != "1.2.3" {
					t.Errorf("expected version '1.2.3', got '%s'", version.Version)
				}
				if version.MajorVersion != 1 || version.MinorVersion != 2 || version.PatchVersion != 3 {
					t.Errorf("expected version parts 1.2.3, got %d.%d.%d", version.MajorVersion, version.MinorVersion, version.PatchVersion)
				}
			},
		},
		{
			name: "empty configuration",
			configContent: `packageSourceProviders: []
packageSources: []
`,
			wantErr: false,
			validate: func(t *testing.T, config *Config) {
				if len(config.PackageSourceProviders) != 0 {
					t.Errorf("expected 0 providers, got %d", len(config.PackageSourceProviders))
				}
				if len(config.PackageSources) != 0 {
					t.Errorf("expected 0 sources, got %d", len(config.PackageSources))
				}
			},
		},
		{
			name:          "invalid YAML syntax",
			configContent: `packageSourceProviders: [invalid yaml`,
			wantErr:       true,
			errContains:   "failed to parse configuration YAML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for test
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yml")

			// Write test configuration
			if err := os.WriteFile(configPath, []byte(tt.configContent), 0644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			// Load configuration
			config, err := LoadConfiguration(configPath)

			// Check error expectation
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Validate configuration if validation function provided
			if tt.validate != nil {
				tt.validate(t, config)
			}
		})
	}
}

func TestLoadConfigurationFileErrors(t *testing.T) {
	tests := []struct {
		name        string
		configPath  string
		errContains string
	}{
		{
			name:        "non-existent file",
			configPath:  "/nonexistent/path/config.yml",
			errContains: "failed to read configuration file",
		},
		{
			name:        "empty path",
			configPath:  "",
			errContains: "failed to read configuration file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadConfiguration(tt.configPath)
			if err == nil {
				t.Error("expected error, got nil")
			} else if !contains(err.Error(), tt.errContains) {
				t.Errorf("expected error containing '%s', got '%s'", tt.errContains, err.Error())
			}
		})
	}
}

func TestLoadConfigurationWithDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := LoadConfiguration(tmpDir)
	if err == nil {
		t.Error("expected error when loading directory, got nil")
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}