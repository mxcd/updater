package target

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mxcd/updater/internal/configuration"
)

func TestTerraformVariableTarget_ReadCurrentVersion(t *testing.T) {
	tests := []struct {
		name          string
		fileContent   string
		variableName  string
		expectedVer   string
		expectError   bool
		errorContains string
	}{
		{
			name: "simple variable declaration",
			fileContent: `variable "app_version" {
  default = "1.2.3"
}`,
			variableName: "app_version",
			expectedVer:  "1.2.3",
			expectError:  false,
		},
		{
			name: "variable with description",
			fileContent: `variable "app_version" {
  description = "Application version"
  type        = string
  default     = "2.0.0"
}`,
			variableName: "app_version",
			expectedVer:  "2.0.0",
			expectError:  false,
		},
		{
			name:         "single line variable",
			fileContent:  `variable "version" { default = "3.4.5" }`,
			variableName: "version",
			expectedVer:  "3.4.5",
			expectError:  false,
		},
		{
			name: "variable not found",
			fileContent: `variable "other_version" {
  default = "1.0.0"
}`,
			variableName:  "app_version",
			expectError:   true,
			errorContains: "not found",
		},
		{
			name: "multiple variables",
			fileContent: `variable "first_version" {
  default = "1.0.0"
}

variable "second_version" {
  default = "2.0.0"
}

variable "third_version" {
  default = "3.0.0"
}`,
			variableName: "second_version",
			expectedVer:  "2.0.0",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test.tf")
			if err := os.WriteFile(tmpFile, []byte(tt.fileContent), 0644); err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}

			// Create target
			config := &configuration.Target{
				Name: "test-target",
				Type: configuration.TargetTypeTerraformVariable,
				File: tmpFile,
				Items: []configuration.TargetItem{
					{
						TerraformVariableName: tt.variableName,
						Source:                "test-source",
					},
				},
			}

			target, err := NewTerraformVariableTarget(config)
			if err != nil {
				t.Fatalf("Failed to create target: %v", err)
			}

			// Test ReadCurrentVersion
			version, err := target.ReadCurrentVersion()

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
				if version != tt.expectedVer {
					t.Errorf("Expected version '%s', got '%s'", tt.expectedVer, version)
				}
			}
		})
	}
}

func TestTerraformVariableTarget_WriteVersion(t *testing.T) {
	tests := []struct {
		name         string
		fileContent  string
		variableName string
		newVersion   string
		expectError  bool
	}{
		{
			name: "update simple variable",
			fileContent: `variable "app_version" {
  default = "1.0.0"
}`,
			variableName: "app_version",
			newVersion:   "2.0.0",
			expectError:  false,
		},
		{
			name: "update variable with description",
			fileContent: `variable "app_version" {
  description = "Application version"
  type        = string
  default     = "1.0.0"
}`,
			variableName: "app_version",
			newVersion:   "3.5.2",
			expectError:  false,
		},
		{
			name:         "update single line variable",
			fileContent:  `variable "version" { default = "1.0.0" }`,
			variableName: "version",
			newVersion:   "4.0.0",
			expectError:  false,
		},
		{
			name: "variable not found",
			fileContent: `variable "other_version" {
  default = "1.0.0"
}`,
			variableName: "app_version",
			newVersion:   "2.0.0",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test.tf")
			if err := os.WriteFile(tmpFile, []byte(tt.fileContent), 0644); err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}

			// Create target
			config := &configuration.Target{
				Name: "test-target",
				Type: configuration.TargetTypeTerraformVariable,
				File: tmpFile,
				Items: []configuration.TargetItem{
					{
						TerraformVariableName: tt.variableName,
						Source:                "test-source",
					},
				},
			}

			target, err := NewTerraformVariableTarget(config)
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
			}
		})
	}
}

func TestTerraformVariableTarget_Validate(t *testing.T) {
	tests := []struct {
		name          string
		fileName      string
		fileContent   string
		variableName  string
		expectError   bool
		errorContains string
	}{
		{
			name:     "valid .tf file",
			fileName: "test.tf",
			fileContent: `variable "version" {
  default = "1.0.0"
}`,
			variableName: "version",
			expectError:  false,
		},
		{
			name:     "valid .tfvars file",
			fileName: "test.tfvars",
			fileContent: `variable "version" {
  default = "1.0.0"
}`,
			variableName: "version",
			expectError:  false,
		},
		{
			name:          "invalid file extension",
			fileName:      "test.txt",
			fileContent:   `variable "version" { default = "1.0.0" }`,
			variableName:  "version",
			expectError:   true,
			errorContains: "must have .tf or .tfvars extension",
		},
		{
			name:          "variable not found",
			fileName:      "test.tf",
			fileContent:   `variable "other" { default = "1.0.0" }`,
			variableName:  "version",
			expectError:   true,
			errorContains: "not found",
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
				Type: configuration.TargetTypeTerraformVariable,
				File: tmpFile,
				Items: []configuration.TargetItem{
					{
						TerraformVariableName: tt.variableName,
						Source:                "test-source",
					},
				},
			}

			target, err := NewTerraformVariableTarget(config)
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

func TestTerraformVariableTarget_GetTargetInfo(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.tf")
	fileContent := `variable "app_version" {
  default = "1.2.3"
}`
	if err := os.WriteFile(tmpFile, []byte(fileContent), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	config := &configuration.Target{
		Name: "test-target",
		Type: configuration.TargetTypeTerraformVariable,
		File: tmpFile,
		Items: []configuration.TargetItem{
			{
				TerraformVariableName: "app_version",
				Source:                "test-source",
			},
		},
	}

	target, err := NewTerraformVariableTarget(config)
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	info := target.GetTargetInfo()

	if info.Name != "test-target" {
		t.Errorf("Expected name 'test-target', got '%s'", info.Name)
	}
	if info.Type != configuration.TargetTypeTerraformVariable {
		t.Errorf("Expected type 'terraform-variable', got '%s'", info.Type)
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

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
