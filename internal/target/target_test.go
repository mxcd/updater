package target

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mxcd/updater/internal/configuration"
)

func TestTargetFactory_CreateTarget(t *testing.T) {
	tests := []struct {
		name        string
		targetType  configuration.TargetType
		expectError bool
	}{
		{
			name:        "terraform-variable target",
			targetType:  configuration.TargetTypeTerraformVariable,
			expectError: false,
		},
		{
			name:        "unsupported target type",
			targetType:  configuration.TargetType("unsupported"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file for terraform target
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test.tf")
			fileContent := `variable "version" { default = "1.0.0" }`
			if err := os.WriteFile(tmpFile, []byte(fileContent), 0644); err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}

			config := &configuration.Config{
				PackageSources: []*configuration.PackageSource{},
			}

			targetConfig := &configuration.Target{
				Name: "test-target",
				Type: tt.targetType,
				File: tmpFile,
				Items: []configuration.TargetItem{
					{
						TerraformVariableName: "version",
						Source:                "test-source",
					},
				},
			}

			factory := NewTargetFactory(config)
			target, err := factory.CreateTarget(targetConfig)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				if target != nil {
					t.Errorf("Expected nil target but got: %v", target)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if target == nil {
					t.Errorf("Expected target but got nil")
				}
			}
		})
	}
}

func TestTargetFactory_CreateAllTargets(t *testing.T) {
	// Create temp directory and files
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "test1.tf")
	file2 := filepath.Join(tmpDir, "test2.tf")

	fileContent := `variable "version" { default = "1.0.0" }`
	if err := os.WriteFile(file1, []byte(fileContent), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	if err := os.WriteFile(file2, []byte(fileContent), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	config := &configuration.Config{
		PackageSources: []*configuration.PackageSource{},
		Targets: []*configuration.Target{
			{
				Name: "target1",
				Type: configuration.TargetTypeTerraformVariable,
				File: file1,
				Items: []configuration.TargetItem{
					{
						TerraformVariableName: "version",
						Source:                "source1",
					},
				},
			},
			{
				Name: "target2",
				Type: configuration.TargetTypeTerraformVariable,
				File: file2,
				Items: []configuration.TargetItem{
					{
						TerraformVariableName: "version",
						Source:                "source2",
					},
				},
			},
		},
	}

	factory := NewTargetFactory(config)
	targets, err := factory.CreateAllTargets()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(targets) != 2 {
		t.Errorf("Expected 2 targets, got %d", len(targets))
	}
}

func TestUnsupportedTargetTypeError(t *testing.T) {
	err := &UnsupportedTargetTypeError{
		Type: configuration.TargetType("custom-type"),
	}

	expected := "unsupported target type: custom-type"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestFileNotFoundError(t *testing.T) {
	err := &FileNotFoundError{
		Path: "/path/to/missing.tf",
	}

	expected := "target file not found: /path/to/missing.tf"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestVariableNotFoundError(t *testing.T) {
	err := &VariableNotFoundError{
		Variable: "my_var",
		File:     "/path/to/file.tf",
	}

	expected := "variable 'my_var' not found in file: /path/to/file.tf"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestInvalidFileFormatError(t *testing.T) {
	err := &InvalidFileFormatError{
		File:   "test.txt",
		Reason: "must be .tf file",
	}

	expected := "invalid file format 'test.txt': must be .tf file"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}
