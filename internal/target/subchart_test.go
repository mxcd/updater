package target

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mxcd/updater/internal/configuration"
)

func TestSubchartTarget_ReadCurrentVersion(t *testing.T) {
	tests := []struct {
		name          string
		fileContent   string
		subchartName  string
		expectedVer   string
		expectError   bool
		errorContains string
	}{
		{
			name: "simple single dependency",
			fileContent: `apiVersion: v2
name: my-app
description: My application
type: application
version: 1.0.0
appVersion: "1.0.0"
dependencies:
  - name: backend-service
    version: 1.2.0
    repository: oci://registry.example.com/charts
`,
			subchartName: "backend-service",
			expectedVer:  "1.2.0",
			expectError:  false,
		},
		{
			name: "multiple dependencies",
			fileContent: `apiVersion: v2
name: my-app
version: 1.0.0
dependencies:
  - name: postgres
    version: 12.1.5
    repository: https://charts.bitnami.com/bitnami
  - name: redis
    version: 17.3.7
    repository: https://charts.bitnami.com/bitnami
  - name: nginx
    version: 13.2.23
    repository: https://charts.bitnami.com/bitnami
`,
			subchartName: "redis",
			expectedVer:  "17.3.7",
			expectError:  false,
		},
		{
			name: "dependency not found",
			fileContent: `apiVersion: v2
name: my-app
version: 1.0.0
dependencies:
  - name: postgres
    version: 12.1.5
    repository: https://charts.bitnami.com/bitnami
`,
			subchartName:  "redis",
			expectError:   true,
			errorContains: "not found",
		},
		{
			name: "no dependencies section",
			fileContent: `apiVersion: v2
name: my-app
version: 1.0.0
appVersion: "1.0.0"
`,
			subchartName:  "redis",
			expectError:   true,
			errorContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "Chart.yaml")
			if err := os.WriteFile(tmpFile, []byte(tt.fileContent), 0644); err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}

			// Create target
			config := &configuration.Target{
				Name: "test-target",
				Type: configuration.TargetTypeSubchart,
				File: tmpFile,
				Items: []configuration.TargetItem{
					{
						SubchartName: tt.subchartName,
						Source:       "test-source",
					},
				},
			}

			target, err := NewSubchartTargetForUpdateItem(config, &config.Items[0])
			if err != nil {
				t.Fatalf("Failed to create target: %v", err)
			}

			// Test ReadCurrentVersion
			version, err := target.ReadCurrentVersion()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if version != tt.expectedVer {
					t.Errorf("Expected version '%s', got '%s'", tt.expectedVer, version)
				}
			}
		})
	}
}

func TestSubchartTarget_WriteVersion(t *testing.T) {
	tests := []struct {
		name         string
		fileContent  string
		subchartName string
		newVersion   string
		expectError  bool
	}{
		{
			name: "update single dependency",
			fileContent: `apiVersion: v2
name: my-app
description: My application
type: application
version: 1.0.0
appVersion: "1.0.0"
dependencies:
  - name: backend-service
    version: 1.2.0
    repository: oci://registry.example.com/charts
`,
			subchartName: "backend-service",
			newVersion:   "1.3.0",
			expectError:  false,
		},
		{
			name: "update one of multiple dependencies",
			fileContent: `apiVersion: v2
name: my-app
version: 1.0.0
dependencies:
  - name: postgres
    version: 12.1.5
    repository: https://charts.bitnami.com/bitnami
  - name: redis
    version: 17.3.7
    repository: https://charts.bitnami.com/bitnami
  - name: nginx
    version: 13.2.23
    repository: https://charts.bitnami.com/bitnami
`,
			subchartName: "redis",
			newVersion:   "18.0.0",
			expectError:  false,
		},
		{
			name: "dependency not found",
			fileContent: `apiVersion: v2
name: my-app
version: 1.0.0
dependencies:
  - name: postgres
    version: 12.1.5
    repository: https://charts.bitnami.com/bitnami
`,
			subchartName: "redis",
			newVersion:   "18.0.0",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "Chart.yaml")
			if err := os.WriteFile(tmpFile, []byte(tt.fileContent), 0644); err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}

			// Create target
			config := &configuration.Target{
				Name: "test-target",
				Type: configuration.TargetTypeSubchart,
				File: tmpFile,
				Items: []configuration.TargetItem{
					{
						SubchartName: tt.subchartName,
						Source:       "test-source",
					},
				},
			}

			target, err := NewSubchartTargetForUpdateItem(config, &config.Items[0])
			if err != nil {
				t.Fatalf("Failed to create target: %v", err)
			}

			// Test WriteVersion
			err = target.WriteVersion(tt.newVersion)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// Verify the version was actually written
				newVersion, err := target.ReadCurrentVersion()
				if err != nil {
					t.Errorf("Failed to read updated version: %v", err)
				}
				if newVersion != tt.newVersion {
					t.Errorf("Expected version '%s', got '%s'", tt.newVersion, newVersion)
				}

				// Verify file content to ensure formatting is preserved
				content, err := os.ReadFile(tmpFile)
				if err != nil {
					t.Errorf("Failed to read file: %v", err)
				}
				
				// Check that the new version is in the file
				if !strings.Contains(string(content), tt.newVersion) {
					t.Errorf("Updated version '%s' not found in file content", tt.newVersion)
				}
			}
		})
	}
}

func TestSubchartTarget_WriteVersion_MultipleSubcharts(t *testing.T) {
	// Test the specific case from the task: update only one dependency when multiple exist
	fileContent := `apiVersion: v2
name: my-app
description: My application deployment
type: application
version: 1.0.0
appVersion: "1.0.0"
dependencies:
  - name: backend-service
    version: 1.2.0
    repository: oci://registry.example.com/charts
  - name: postgres
    version: 12.1.5
    repository: https://charts.bitnami.com/bitnami
  - name: redis
    version: 17.3.7
    repository: https://charts.bitnami.com/bitnami
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "Chart.yaml")
	if err := os.WriteFile(tmpFile, []byte(fileContent), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Create target for updating only backend-service
	config := &configuration.Target{
		Name: "test-target",
		Type: configuration.TargetTypeSubchart,
		File: tmpFile,
		Items: []configuration.TargetItem{
			{
				SubchartName: "backend-service",
				Source:       "test-source",
			},
		},
	}

	target, err := NewSubchartTargetForUpdateItem(config, &config.Items[0])
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	// Update backend-service to 1.3.0
	if err := target.WriteVersion("1.3.0"); err != nil {
		t.Fatalf("Failed to write version: %v", err)
	}

	// Verify backend-service was updated
	version, err := target.ReadCurrentVersion()
	if err != nil {
		t.Fatalf("Failed to read updated version: %v", err)
	}
	if version != "1.3.0" {
		t.Errorf("Expected backend-service version '1.3.0', got '%s'", version)
	}

	// Verify other dependencies were not changed
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileStr := string(content)

	// Check postgres version is unchanged
	if !strings.Contains(fileStr, "postgres") || !strings.Contains(fileStr, "12.1.5") {
		t.Errorf("Postgres dependency was incorrectly modified")
	}

	// Check redis version is unchanged
	if !strings.Contains(fileStr, "redis") || !strings.Contains(fileStr, "17.3.7") {
		t.Errorf("Redis dependency was incorrectly modified")
	}

	// Check backend-service version was updated
	if !strings.Contains(fileStr, "backend-service") || !strings.Contains(fileStr, "1.3.0") {
		t.Errorf("backend-service dependency was not updated correctly")
	}

	// Ensure old version is gone
	if strings.Contains(fileStr, "1.2.0") {
		t.Errorf("Old version 1.2.0 still exists in file")
	}
}

func TestSubchartTarget_Validate(t *testing.T) {
	tests := []struct {
		name          string
		fileName      string
		fileContent   string
		subchartName  string
		expectError   bool
		errorContains string
	}{
		{
			name:     "valid Chart.yaml",
			fileName: "Chart.yaml",
			fileContent: `apiVersion: v2
name: my-app
version: 1.0.0
dependencies:
  - name: redis
    version: 17.3.7
    repository: https://charts.bitnami.com/bitnami
`,
			subchartName: "redis",
			expectError:  false,
		},
		{
			name:     "valid Chart.yml",
			fileName: "Chart.yml",
			fileContent: `apiVersion: v2
name: my-app
version: 1.0.0
dependencies:
  - name: redis
    version: 17.3.7
    repository: https://charts.bitnami.com/bitnami
`,
			subchartName: "redis",
			expectError:  false,
		},
		{
			name:          "invalid file name",
			fileName:      "chart.yaml",
			fileContent:   `apiVersion: v2\nname: my-app\nversion: 1.0.0`,
			subchartName:  "redis",
			expectError:   true,
			errorContains: "must be named Chart.yaml or Chart.yml",
		},
		{
			name:     "invalid file extension",
			fileName: "Chart.txt",
			fileContent: `apiVersion: v2
name: my-app
version: 1.0.0`,
			subchartName:  "redis",
			expectError:   true,
			errorContains: "must be named Chart.yaml or Chart.yml",
		},
		{
			name:         "dependency not found - permissive for wildcards",
			fileName:     "Chart.yaml",
			fileContent:  "apiVersion: v2\nname: my-app\nversion: 1.0.0\ndependencies:\n  - name: postgres\n    version: 12.1.5\n    repository: https://charts.bitnami.com/bitnami\n",
			subchartName: "redis",
			expectError:  false, // Validation passes even if dependency not found (for wildcard support)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, tt.fileName)
			if err := os.WriteFile(tmpFile, []byte(tt.fileContent), 0644); err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}

			// Create target
			config := &configuration.Target{
				Name: "test-target",
				Type: configuration.TargetTypeSubchart,
				File: tmpFile,
				Items: []configuration.TargetItem{
					{
						SubchartName: tt.subchartName,
						Source:       "test-source",
					},
				},
			}

			target, err := NewSubchartTargetForUpdateItem(config, &config.Items[0])
			if err != nil && !tt.expectError {
				t.Fatalf("Failed to create target: %v", err)
			}

			if target == nil {
				return // Creation failed as expected
			}

			// Test Validate
			err = target.Validate()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
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

func TestSubchartTarget_GetTargetInfo(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "Chart.yaml")
	fileContent := `apiVersion: v2
name: my-app
version: 1.0.0
dependencies:
  - name: backend-service
    version: 1.2.3
    repository: oci://registry.example.com/charts
`
	if err := os.WriteFile(tmpFile, []byte(fileContent), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	config := &configuration.Target{
		Name: "test-target",
		Type: configuration.TargetTypeSubchart,
		File: tmpFile,
		Items: []configuration.TargetItem{
			{
				SubchartName: "backend-service",
				Source:       "test-source",
			},
		},
	}

	target, err := NewSubchartTargetForUpdateItem(config, &config.Items[0])
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	info := target.GetTargetInfo()

	if info.Name != "test-target" {
		t.Errorf("Expected name 'test-target', got '%s'", info.Name)
	}
	if info.Type != configuration.TargetTypeSubchart {
		t.Errorf("Expected type 'subchart', got '%s'", info.Type)
	}
	if info.File != tmpFile {
		t.Errorf("Expected file '%s', got '%s'", tmpFile, info.File)
	}
	if info.Source != "test-source" {
		t.Errorf("Expected source 'test-source', got '%s'", info.Source)
	}
	if info.CurrentValue != "1.2.3" {
		t.Errorf("Expected current value '1.2.3', got '%s'", info.CurrentValue)
	}
}

func TestSubchartTarget_PreserveFormatting(t *testing.T) {
	// Test that comments and formatting are preserved when updating
	fileContent := `apiVersion: v2
name: my-app
description: My application deployment
type: application
version: 1.0.0
appVersion: "1.0.0"

# Chart dependencies
dependencies:
  # Main application
  - name: backend-service
    version: 1.2.0
    repository: oci://registry.example.com/charts
    condition: backend-service.enabled
  
  # Database
  - name: postgres
    version: 12.1.5
    repository: https://charts.bitnami.com/bitnami
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "Chart.yaml")
	if err := os.WriteFile(tmpFile, []byte(fileContent), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	config := &configuration.Target{
		Name: "test-target",
		Type: configuration.TargetTypeSubchart,
		File: tmpFile,
		Items: []configuration.TargetItem{
			{
				SubchartName: "backend-service",
				Source:       "test-source",
			},
		},
	}

	target, err := NewSubchartTargetForUpdateItem(config, &config.Items[0])
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	// Update the version
	if err := target.WriteVersion("1.3.0"); err != nil {
		t.Fatalf("Failed to write version: %v", err)
	}

	// Read the file and check that comments are preserved
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileStr := string(content)

	// Check that comments are still present
	if !strings.Contains(fileStr, "# Chart dependencies") {
		t.Errorf("Comment '# Chart dependencies' was removed")
	}
	if !strings.Contains(fileStr, "# Main application") {
		t.Errorf("Comment '# Main application' was removed")
	}
	if !strings.Contains(fileStr, "# Database") {
		t.Errorf("Comment '# Database' was removed")
	}

	// Check that the version was updated
	if !strings.Contains(fileStr, "1.3.0") {
		t.Errorf("Version was not updated to 1.3.0")
	}

	// Check that description is preserved
	if !strings.Contains(fileStr, "My application deployment") {
		t.Errorf("Description was removed")
	}
}