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
			errContains: "failed to access configuration path",
		},
		{
			name:        "empty path",
			configPath:  "",
			errContains: "failed to access configuration path",
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
	t.Run("directory with multiple yml files", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create first config file - providers.yml
		providersContent := `packageSourceProviders:
  - name: github
    type: github
  - name: docker
    type: docker
`
		if err := os.WriteFile(filepath.Join(tmpDir, "providers.yml"), []byte(providersContent), 0644); err != nil {
			t.Fatalf("failed to write providers.yml: %v", err)
		}

		// Create second config file - sources.yml
		sourcesContent := `packageSources:
  - name: app1
    provider: github
    type: git-release
    uri: https://github.com/example/app1
  - name: app2
    provider: docker
    type: docker-image
    uri: registry.example.com/app2
`
		if err := os.WriteFile(filepath.Join(tmpDir, "sources.yml"), []byte(sourcesContent), 0644); err != nil {
			t.Fatalf("failed to write sources.yml: %v", err)
		}

		// Create third config file - targets.yml
		targetsContent := `targets:
  - name: target1
    type: terraform-variable
    file: vars.tf
    items:
      - terraformVariableName: app1_version
        source: app1
`
		if err := os.WriteFile(filepath.Join(tmpDir, "targets.yml"), []byte(targetsContent), 0644); err != nil {
			t.Fatalf("failed to write targets.yml: %v", err)
		}

		// Load configuration from directory
		config, err := LoadConfiguration(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error loading from directory: %v", err)
		}

		// Verify merged configuration
		if len(config.PackageSourceProviders) != 2 {
			t.Errorf("expected 2 providers, got %d", len(config.PackageSourceProviders))
		}
		if len(config.PackageSources) != 2 {
			t.Errorf("expected 2 sources, got %d", len(config.PackageSources))
		}
		if len(config.Targets) != 1 {
			t.Errorf("expected 1 target, got %d", len(config.Targets))
		}

		// Verify specific items
		if config.PackageSourceProviders[0].Name != "github" {
			t.Errorf("expected first provider to be 'github', got '%s'", config.PackageSourceProviders[0].Name)
		}
		if config.PackageSources[0].Name != "app1" {
			t.Errorf("expected first source to be 'app1', got '%s'", config.PackageSources[0].Name)
		}
	})

	t.Run("directory with yaml extension", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create config file with .yaml extension
		content := `packageSourceProviders:
  - name: github
    type: github
packageSources:
  - name: test
    provider: github
    type: git-release
    uri: https://github.com/example/test
`
		if err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write config.yaml: %v", err)
		}

		config, err := LoadConfiguration(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(config.PackageSourceProviders) != 1 {
			t.Errorf("expected 1 provider, got %d", len(config.PackageSourceProviders))
		}
	})

	t.Run("directory with duplicate provider names", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create first file with github provider
		content1 := `packageSourceProviders:
  - name: github
    type: github
`
		if err := os.WriteFile(filepath.Join(tmpDir, "file1.yml"), []byte(content1), 0644); err != nil {
			t.Fatalf("failed to write file1.yml: %v", err)
		}

		// Create second file with duplicate github provider
		content2 := `packageSourceProviders:
  - name: github
    type: github
    authType: token
    token: secret
`
		if err := os.WriteFile(filepath.Join(tmpDir, "file2.yml"), []byte(content2), 0644); err != nil {
			t.Fatalf("failed to write file2.yml: %v", err)
		}

		_, err := LoadConfiguration(tmpDir)
		if err == nil {
			t.Error("expected error for duplicate provider name, got nil")
		} else if !contains(err.Error(), "duplicate package source provider name") {
			t.Errorf("expected error about duplicate provider name, got: %v", err)
		}
	})

	t.Run("directory with duplicate source names", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create first file
		content1 := `packageSourceProviders:
  - name: github
    type: github
packageSources:
  - name: app1
    provider: github
    type: git-release
    uri: https://github.com/example/app1
`
		if err := os.WriteFile(filepath.Join(tmpDir, "file1.yml"), []byte(content1), 0644); err != nil {
			t.Fatalf("failed to write file1.yml: %v", err)
		}

		// Create second file with duplicate source name
		content2 := `packageSources:
  - name: app1
    provider: github
    type: git-tag
    uri: https://github.com/example/app1
`
		if err := os.WriteFile(filepath.Join(tmpDir, "file2.yml"), []byte(content2), 0644); err != nil {
			t.Fatalf("failed to write file2.yml: %v", err)
		}

		_, err := LoadConfiguration(tmpDir)
		if err == nil {
			t.Error("expected error for duplicate source name, got nil")
		} else if !contains(err.Error(), "duplicate package source name") {
			t.Errorf("expected error about duplicate source name, got: %v", err)
		}
	})

	t.Run("directory with duplicate target names", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create provider and source first
		baseContent := `packageSourceProviders:
  - name: github
    type: github
packageSources:
  - name: app1
    provider: github
    type: git-release
    uri: https://github.com/example/app1
`
		if err := os.WriteFile(filepath.Join(tmpDir, "base.yml"), []byte(baseContent), 0644); err != nil {
			t.Fatalf("failed to write base.yml: %v", err)
		}

		// Create first file with target
		content1 := `targets:
  - name: target1
    type: terraform-variable
    file: vars.tf
    items:
      - terraformVariableName: version
        source: app1
`
		if err := os.WriteFile(filepath.Join(tmpDir, "targets1.yml"), []byte(content1), 0644); err != nil {
			t.Fatalf("failed to write targets1.yml: %v", err)
		}

		// Create second file with duplicate target name
		content2 := `targets:
  - name: target1
    type: subchart
    file: Chart.yaml
    items:
      - subchartName: subchart1
        source: app1
`
		if err := os.WriteFile(filepath.Join(tmpDir, "targets2.yml"), []byte(content2), 0644); err != nil {
			t.Fatalf("failed to write targets2.yml: %v", err)
		}

		_, err := LoadConfiguration(tmpDir)
		if err == nil {
			t.Error("expected error for duplicate target name, got nil")
		} else if !contains(err.Error(), "duplicate target name") {
			t.Errorf("expected error about duplicate target name, got: %v", err)
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		_, err := LoadConfiguration(tmpDir)
		if err == nil {
			t.Error("expected error for empty directory, got nil")
		} else if !contains(err.Error(), "no .yml or .yaml files found") {
			t.Errorf("expected error about no yml files, got: %v", err)
		}
	})

	t.Run("directory with non-yml files only", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a non-yml file
		if err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("test"), 0644); err != nil {
			t.Fatalf("failed to write readme.txt: %v", err)
		}

		_, err := LoadConfiguration(tmpDir)
		if err == nil {
			t.Error("expected error for directory with no yml files, got nil")
		} else if !contains(err.Error(), "no .yml or .yaml files found") {
			t.Errorf("expected error about no yml files, got: %v", err)
		}
	})

	t.Run("directory with targetActor in multiple files", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create first file with targetActor
		content1 := `packageSourceProviders:
  - name: github
    type: github
targetActor:
  name: First Actor
  email: first@example.com
  username: first
`
		if err := os.WriteFile(filepath.Join(tmpDir, "file1.yml"), []byte(content1), 0644); err != nil {
			t.Fatalf("failed to write file1.yml: %v", err)
		}

		// Create second file with different targetActor (should override)
		content2 := `targetActor:
  name: Second Actor
  email: second@example.com
  username: second
`
		if err := os.WriteFile(filepath.Join(tmpDir, "file2.yml"), []byte(content2), 0644); err != nil {
			t.Fatalf("failed to write file2.yml: %v", err)
		}

		config, err := LoadConfiguration(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should use the last non-nil targetActor
		if config.TargetActor == nil {
			t.Error("expected targetActor to be set")
		} else if config.TargetActor.Name != "Second Actor" && config.TargetActor.Name != "First Actor" {
			t.Errorf("expected targetActor name to be from one of the files, got '%s'", config.TargetActor.Name)
		}
	})
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