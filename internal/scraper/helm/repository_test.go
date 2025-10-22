package helm

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mxcd/updater/internal/configuration"
)

func TestScrapeHelmRepository(t *testing.T) {
	// Mock Helm index.yaml
	mockIndexYAML := `apiVersion: v1
entries:
  nginx:
    - name: nginx
      version: 1.5.0
      appVersion: 1.21.6
      description: NGINX Open Source
      created: "2024-01-15T10:00:00Z"
    - name: nginx
      version: 1.4.2
      appVersion: 1.21.5
      description: NGINX Open Source
      created: "2024-01-10T10:00:00Z"
    - name: nginx
      version: 1.4.1
      appVersion: 1.21.4
      description: NGINX Open Source
      created: "2024-01-05T10:00:00Z"
  redis:
    - name: redis
      version: 2.1.0
      appVersion: 7.0.0
      description: Redis
      created: "2024-01-20T10:00:00Z"
generated: "2024-01-20T12:00:00Z"
`

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/index.yaml" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/x-yaml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockIndexYAML))
	}))
	defer server.Close()

	tests := []struct {
		name           string
		provider       *configuration.PackageSourceProvider
		source         *configuration.PackageSource
		opts           *ScrapeOptions
		expectError    bool
		expectedCount  int
		firstVersion   string
		errorContains  string
	}{
		{
			name: "successful scrape with valid chart",
			provider: &configuration.PackageSourceProvider{
				Name:     "helm-repo",
				Type:     configuration.PackageSourceProviderTypeHelm,
				BaseUrl:  server.URL,
				AuthType: configuration.PackageSourceProviderAuthTypeNone,
			},
			source: &configuration.PackageSource{
				Name:      "nginx-chart",
				Provider:  "helm-repo",
				Type:      configuration.PackageSourceTypeHelmRepository,
				ChartName: "nginx",
			},
			opts: &ScrapeOptions{
				Limit: 0,
			},
			expectError:   false,
			expectedCount: 3,
			firstVersion:  "1.5.0", // Should be sorted with newest first
		},
		{
			name: "scrape with limit",
			provider: &configuration.PackageSourceProvider{
				Name:     "helm-repo",
				Type:     configuration.PackageSourceProviderTypeHelm,
				BaseUrl:  server.URL,
				AuthType: configuration.PackageSourceProviderAuthTypeNone,
			},
			source: &configuration.PackageSource{
				Name:      "nginx-chart",
				Provider:  "helm-repo",
				Type:      configuration.PackageSourceTypeHelmRepository,
				ChartName: "nginx",
			},
			opts: &ScrapeOptions{
				Limit: 2,
			},
			expectError:   false,
			expectedCount: 2,
			firstVersion:  "1.5.0",
		},
		{
			name: "scrape different chart",
			provider: &configuration.PackageSourceProvider{
				Name:     "helm-repo",
				Type:     configuration.PackageSourceProviderTypeHelm,
				BaseUrl:  server.URL,
				AuthType: configuration.PackageSourceProviderAuthTypeNone,
			},
			source: &configuration.PackageSource{
				Name:      "redis-chart",
				Provider:  "helm-repo",
				Type:      configuration.PackageSourceTypeHelmRepository,
				ChartName: "redis",
			},
			opts: &ScrapeOptions{
				Limit: 0,
			},
			expectError:   false,
			expectedCount: 1,
			firstVersion:  "2.1.0",
		},
		{
			name: "missing chart name",
			provider: &configuration.PackageSourceProvider{
				Name:     "helm-repo",
				Type:     configuration.PackageSourceProviderTypeHelm,
				BaseUrl:  server.URL,
				AuthType: configuration.PackageSourceProviderAuthTypeNone,
			},
			source: &configuration.PackageSource{
				Name:      "invalid-chart",
				Provider:  "helm-repo",
				Type:      configuration.PackageSourceTypeHelmRepository,
				ChartName: "",
			},
			opts: &ScrapeOptions{
				Limit: 0,
			},
			expectError:   true,
			errorContains: "chartName is required",
		},
		{
			name: "chart not found in repository",
			provider: &configuration.PackageSourceProvider{
				Name:     "helm-repo",
				Type:     configuration.PackageSourceProviderTypeHelm,
				BaseUrl:  server.URL,
				AuthType: configuration.PackageSourceProviderAuthTypeNone,
			},
			source: &configuration.PackageSource{
				Name:      "nonexistent-chart",
				Provider:  "helm-repo",
				Type:      configuration.PackageSourceTypeHelmRepository,
				ChartName: "nonexistent",
			},
			opts: &ScrapeOptions{
				Limit: 0,
			},
			expectError:   true,
			errorContains: "not found in Helm repository",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			versions, err := scrapeHelmRepository(tt.provider, tt.source, tt.opts)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(versions) != tt.expectedCount {
				t.Errorf("Expected %d versions, got %d", tt.expectedCount, len(versions))
			}

			if len(versions) > 0 && versions[0].Version != tt.firstVersion {
				t.Errorf("Expected first version to be %s, got %s", tt.firstVersion, versions[0].Version)
			}
		})
	}
}

func TestSortVersions(t *testing.T) {
	tests := []struct {
		name     string
		versions []*configuration.PackageSourceVersion
		expected []string
	}{
		{
			name: "sort semantic versions",
			versions: []*configuration.PackageSourceVersion{
				{Version: "1.0.0", MajorVersion: 1, MinorVersion: 0, PatchVersion: 0},
				{Version: "2.1.0", MajorVersion: 2, MinorVersion: 1, PatchVersion: 0},
				{Version: "1.5.2", MajorVersion: 1, MinorVersion: 5, PatchVersion: 2},
				{Version: "2.0.1", MajorVersion: 2, MinorVersion: 0, PatchVersion: 1},
			},
			expected: []string{"2.1.0", "2.0.1", "1.5.2", "1.0.0"},
		},
		{
			name: "sort with pre-release versions",
			versions: []*configuration.PackageSourceVersion{
				{Version: "1.0.0", MajorVersion: 1, MinorVersion: 0, PatchVersion: 0},
				{Version: "1.0.0-beta", MajorVersion: 1, MinorVersion: 0, PatchVersion: 0},
				{Version: "1.0.0-alpha", MajorVersion: 1, MinorVersion: 0, PatchVersion: 0},
			},
			expected: []string{"1.0.0-beta", "1.0.0-alpha", "1.0.0"},
		},
		{
			name: "sort mixed versions",
			versions: []*configuration.PackageSourceVersion{
				{Version: "0.9.0", MajorVersion: 0, MinorVersion: 9, PatchVersion: 0},
				{Version: "1.0.0", MajorVersion: 1, MinorVersion: 0, PatchVersion: 0},
				{Version: "0.10.0", MajorVersion: 0, MinorVersion: 10, PatchVersion: 0},
			},
			expected: []string{"1.0.0", "0.10.0", "0.9.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortVersions(tt.versions)

			for i, expected := range tt.expected {
				if tt.versions[i].Version != expected {
					t.Errorf("Expected version at index %d to be %s, got %s", i, expected, tt.versions[i].Version)
				}
			}
		})
	}
}

func TestConvertToPackageSourceVersion(t *testing.T) {
	tests := []struct {
		name              string
		entry             *HelmIndexEntry
		expectedVersion   string
		expectedMajor     int
		expectedMinor     int
		expectedPatch     int
		expectedInfo      string
	}{
		{
			name: "standard version with appVersion",
			entry: &HelmIndexEntry{
				Name:       "nginx",
				Version:    "1.2.3",
				AppVersion: "1.21.0",
			},
			expectedVersion: "1.2.3",
			expectedMajor:   1,
			expectedMinor:   2,
			expectedPatch:   3,
			expectedInfo:    "appVersion: 1.21.0",
		},
		{
			name: "version with v prefix",
			entry: &HelmIndexEntry{
				Name:    "nginx",
				Version: "v2.0.1",
			},
			expectedVersion: "v2.0.1",
			expectedMajor:   2,
			expectedMinor:   0,
			expectedPatch:   1,
			expectedInfo:    "",
		},
		{
			name: "version with pre-release suffix",
			entry: &HelmIndexEntry{
				Name:    "nginx",
				Version: "1.0.0-beta1",
			},
			expectedVersion: "1.0.0-beta1",
			expectedMajor:   1,
			expectedMinor:   0,
			expectedPatch:   0,
			expectedInfo:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version := convertToPackageSourceVersion(tt.entry)

			if version.Version != tt.expectedVersion {
				t.Errorf("Expected version %s, got %s", tt.expectedVersion, version.Version)
			}

			if version.MajorVersion != tt.expectedMajor {
				t.Errorf("Expected major version %d, got %d", tt.expectedMajor, version.MajorVersion)
			}

			if version.MinorVersion != tt.expectedMinor {
				t.Errorf("Expected minor version %d, got %d", tt.expectedMinor, version.MinorVersion)
			}

			if version.PatchVersion != tt.expectedPatch {
				t.Errorf("Expected patch version %d, got %d", tt.expectedPatch, version.PatchVersion)
			}

			if version.VersionInformation != tt.expectedInfo {
				t.Errorf("Expected version info '%s', got '%s'", tt.expectedInfo, version.VersionInformation)
			}
		})
	}
}

func TestBuildIndexURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected string
	}{
		{
			name:     "URL without trailing slash",
			baseURL:  "https://charts.example.com",
			expected: "https://charts.example.com/index.yaml",
		},
		{
			name:     "URL with trailing slash",
			baseURL:  "https://charts.example.com/",
			expected: "https://charts.example.com/index.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildIndexURL(tt.baseURL)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}