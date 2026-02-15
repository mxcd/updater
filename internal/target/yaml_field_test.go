package target

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mxcd/updater/internal/configuration"
)

func TestYamlFieldTarget_ReadCurrentVersion(t *testing.T) {
	tests := []struct {
		name          string
		fileContent   string
		yamlPath      string
		expectedVer   string
		expectError   bool
		errorContains string
	}{
		{
			name: "simple nested path",
			fileContent: `image:
  repository: nginx
  tag: "1.25.0"
`,
			yamlPath:    "image.tag",
			expectedVer: "1.25.0",
			expectError: false,
		},
		{
			name: "deeply nested path with image reference",
			fileContent: `spec:
  template:
    spec:
      containers:
        - name: myapp
          image: nginx:1.25.0
`,
			yamlPath:    "spec.template.spec.containers.0.image",
			expectedVer: "1.25.0", // extracts tag from Docker image reference
			expectError: false,
		},
		{
			name: "top-level key",
			fileContent: `version: "3.2.1"
name: my-app
`,
			yamlPath:    "version",
			expectedVer: "3.2.1",
			expectError: false,
		},
		{
			name: "unquoted value",
			fileContent: `image:
  tag: 1.25.0
`,
			yamlPath:    "image.tag",
			expectedVer: "1.25.0",
			expectError: false,
		},
		{
			name: "single-quoted value",
			fileContent: `image:
  tag: '1.25.0'
`,
			yamlPath:    "image.tag",
			expectedVer: "1.25.0",
			expectError: false,
		},
		{
			name: "path not found - missing leaf",
			fileContent: `image:
  repository: nginx
`,
			yamlPath:      "image.tag",
			expectError:   true,
			errorContains: "not found",
		},
		{
			name: "path not found - missing intermediate",
			fileContent: `name: my-app
`,
			yamlPath:      "image.tag",
			expectError:   true,
			errorContains: "not found",
		},
		{
			name: "array index access",
			fileContent: `items:
  - version: "1.0.0"
  - version: "2.0.0"
  - version: "3.0.0"
`,
			yamlPath:    "items.1.version",
			expectedVer: "2.0.0",
			expectError: false,
		},
		{
			name: "array index out of range",
			fileContent: `items:
  - version: "1.0.0"
`,
			yamlPath:      "items.5.version",
			expectError:   true,
			errorContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "values.yaml")
			if err := os.WriteFile(tmpFile, []byte(tt.fileContent), 0644); err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}

			config := &configuration.Target{
				Name: "test-target",
				Type: configuration.TargetTypeYamlField,
				File: tmpFile,
				Items: []configuration.TargetItem{
					{
						YamlPath: tt.yamlPath,
						Source:   "test-source",
					},
				},
			}

			target, err := NewYamlFieldTargetForUpdateItem(config, &config.Items[0])
			if err != nil {
				if tt.expectError {
					return
				}
				t.Fatalf("Failed to create target: %v", err)
			}

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

func TestYamlFieldTarget_WriteVersion(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		yamlPath    string
		newVersion  string
		expectError bool
	}{
		{
			name: "update double-quoted value",
			fileContent: `image:
  repository: nginx
  tag: "1.25.0"
`,
			yamlPath:    "image.tag",
			newVersion:  "1.26.0",
			expectError: false,
		},
		{
			name: "update single-quoted value",
			fileContent: `image:
  repository: nginx
  tag: '1.25.0'
`,
			yamlPath:    "image.tag",
			newVersion:  "1.26.0",
			expectError: false,
		},
		{
			name: "update unquoted value",
			fileContent: `image:
  repository: nginx
  tag: 1.25.0
`,
			yamlPath:    "image.tag",
			newVersion:  "1.26.0",
			expectError: false,
		},
		{
			name: "update deeply nested image reference",
			fileContent: `spec:
  template:
    spec:
      containers:
        - name: myapp
          image: nginx:1.25.0
`,
			yamlPath:    "spec.template.spec.containers.0.image",
			newVersion:  "1.26.0", // just the tag — image prefix is preserved
			expectError: false,
		},
		{
			name: "path not found",
			fileContent: `image:
  repository: nginx
`,
			yamlPath:    "image.tag",
			newVersion:  "1.26.0",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "values.yaml")
			if err := os.WriteFile(tmpFile, []byte(tt.fileContent), 0644); err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}

			config := &configuration.Target{
				Name: "test-target",
				Type: configuration.TargetTypeYamlField,
				File: tmpFile,
				Items: []configuration.TargetItem{
					{
						YamlPath: tt.yamlPath,
						Source:   "test-source",
					},
				},
			}

			target, err := NewYamlFieldTargetForUpdateItem(config, &config.Items[0])
			if err != nil {
				if tt.expectError {
					return
				}
				t.Fatalf("Failed to create target: %v", err)
			}

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

				// Verify the file on disk
				content, err := os.ReadFile(tmpFile)
				if err != nil {
					t.Errorf("Failed to read file: %v", err)
				}
				if !strings.Contains(string(content), tt.newVersion) {
					t.Errorf("Updated version '%s' not found in file content", tt.newVersion)
				}
			}
		})
	}
}

func TestYamlFieldTarget_WriteVersion_PreserveFormatting(t *testing.T) {
	fileContent := `# Application configuration
name: my-app
version: "1.0.0"

# Image settings
image:
  repository: nginx  # The docker image
  tag: "1.25.0"      # Image tag
  pullPolicy: IfNotPresent

# Service configuration
service:
  type: ClusterIP
  port: 80
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "values.yaml")
	if err := os.WriteFile(tmpFile, []byte(fileContent), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	config := &configuration.Target{
		Name: "test-target",
		Type: configuration.TargetTypeYamlField,
		File: tmpFile,
		Items: []configuration.TargetItem{
			{
				YamlPath: "image.tag",
				Source:   "test-source",
			},
		},
	}

	target, err := NewYamlFieldTargetForUpdateItem(config, &config.Items[0])
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	if err := target.WriteVersion("1.26.0"); err != nil {
		t.Fatalf("Failed to write version: %v", err)
	}

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileStr := string(content)

	// Check that comments are preserved
	if !strings.Contains(fileStr, "# Application configuration") {
		t.Errorf("Comment '# Application configuration' was removed")
	}
	if !strings.Contains(fileStr, "# Image settings") {
		t.Errorf("Comment '# Image settings' was removed")
	}
	if !strings.Contains(fileStr, "# The docker image") {
		t.Errorf("Inline comment '# The docker image' was removed")
	}
	if !strings.Contains(fileStr, "# Image tag") {
		t.Errorf("Inline comment '# Image tag' was removed")
	}
	if !strings.Contains(fileStr, "# Service configuration") {
		t.Errorf("Comment '# Service configuration' was removed")
	}

	// Check that the version was updated
	if !strings.Contains(fileStr, "1.26.0") {
		t.Errorf("Version was not updated to 1.26.0")
	}

	// Check that double quotes are preserved
	if !strings.Contains(fileStr, `"1.26.0"`) {
		t.Errorf("Double-quote style was not preserved")
	}

	// Check that other values are unchanged
	if !strings.Contains(fileStr, "repository: nginx") {
		t.Errorf("Repository was incorrectly modified")
	}
	if !strings.Contains(fileStr, "pullPolicy: IfNotPresent") {
		t.Errorf("pullPolicy was incorrectly modified")
	}
	if !strings.Contains(fileStr, "port: 80") {
		t.Errorf("Service port was incorrectly modified")
	}

	// Check that blank lines are preserved
	lines := strings.Split(fileStr, "\n")
	blankCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blankCount++
		}
	}
	originalLines := strings.Split(fileContent, "\n")
	originalBlankCount := 0
	for _, line := range originalLines {
		if strings.TrimSpace(line) == "" {
			originalBlankCount++
		}
	}
	if blankCount != originalBlankCount {
		t.Errorf("Blank line count changed from %d to %d", originalBlankCount, blankCount)
	}
}

func TestYamlFieldTarget_WriteVersion_PreserveQuotingStyles(t *testing.T) {
	tests := []struct {
		name            string
		fileContent     string
		yamlPath        string
		newVersion      string
		expectedInFile  string
	}{
		{
			name: "preserve double quotes",
			fileContent: `image:
  tag: "1.25.0"
`,
			yamlPath:       "image.tag",
			newVersion:     "1.26.0",
			expectedInFile: `"1.26.0"`,
		},
		{
			name: "preserve single quotes",
			fileContent: `image:
  tag: '1.25.0'
`,
			yamlPath:       "image.tag",
			newVersion:     "1.26.0",
			expectedInFile: `'1.26.0'`,
		},
		{
			name: "preserve unquoted",
			fileContent: `image:
  tag: 1.25.0
`,
			yamlPath:       "image.tag",
			newVersion:     "1.26.0",
			expectedInFile: "tag: 1.26.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "values.yaml")
			if err := os.WriteFile(tmpFile, []byte(tt.fileContent), 0644); err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}

			config := &configuration.Target{
				Name: "test-target",
				Type: configuration.TargetTypeYamlField,
				File: tmpFile,
				Items: []configuration.TargetItem{
					{
						YamlPath: tt.yamlPath,
						Source:   "test-source",
					},
				},
			}

			target, err := NewYamlFieldTargetForUpdateItem(config, &config.Items[0])
			if err != nil {
				t.Fatalf("Failed to create target: %v", err)
			}

			if err := target.WriteVersion(tt.newVersion); err != nil {
				t.Fatalf("Failed to write version: %v", err)
			}

			content, err := os.ReadFile(tmpFile)
			if err != nil {
				t.Fatalf("Failed to read file: %v", err)
			}

			if !strings.Contains(string(content), tt.expectedInFile) {
				t.Errorf("Expected '%s' in file content, got:\n%s", tt.expectedInFile, string(content))
			}
		})
	}
}

func TestYamlFieldTarget_Validate(t *testing.T) {
	tests := []struct {
		name          string
		fileName      string
		fileContent   string
		yamlPath      string
		expectError   bool
		errorContains string
	}{
		{
			name:     "valid .yaml file",
			fileName: "values.yaml",
			fileContent: `image:
  tag: "1.25.0"
`,
			yamlPath:    "image.tag",
			expectError: false,
		},
		{
			name:     "valid .yml file",
			fileName: "values.yml",
			fileContent: `image:
  tag: "1.25.0"
`,
			yamlPath:    "image.tag",
			expectError: false,
		},
		{
			name:          "invalid file extension",
			fileName:      "values.txt",
			fileContent:   `image: {tag: "1.25.0"}`,
			yamlPath:      "image.tag",
			expectError:   true,
			errorContains: "must have .yaml or .yml extension",
		},
		{
			name:     "path not found - permissive for wildcards",
			fileName: "values.yaml",
			fileContent: `name: my-app
`,
			yamlPath:    "image.tag",
			expectError: false, // Validation passes even if path not found (for wildcard support)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, tt.fileName)
			if err := os.WriteFile(tmpFile, []byte(tt.fileContent), 0644); err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}

			config := &configuration.Target{
				Name: "test-target",
				Type: configuration.TargetTypeYamlField,
				File: tmpFile,
				Items: []configuration.TargetItem{
					{
						YamlPath: tt.yamlPath,
						Source:   "test-source",
					},
				},
			}

			target, err := NewYamlFieldTargetForUpdateItem(config, &config.Items[0])
			if err != nil && !tt.expectError {
				t.Fatalf("Failed to create target: %v", err)
			}
			if target == nil {
				return
			}

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

func TestYamlFieldTarget_GetTargetInfo(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "values.yaml")
	fileContent := `image:
  tag: "1.25.0"
`
	if err := os.WriteFile(tmpFile, []byte(fileContent), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	config := &configuration.Target{
		Name: "test-target",
		Type: configuration.TargetTypeYamlField,
		File: tmpFile,
		Items: []configuration.TargetItem{
			{
				YamlPath: "image.tag",
				Source:   "test-source",
			},
		},
	}

	target, err := NewYamlFieldTargetForUpdateItem(config, &config.Items[0])
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	info := target.GetTargetInfo()

	if info.Name != "test-target" {
		t.Errorf("Expected name 'test-target', got '%s'", info.Name)
	}
	if info.Type != configuration.TargetTypeYamlField {
		t.Errorf("Expected type 'yaml-field', got '%s'", info.Type)
	}
	if info.File != tmpFile {
		t.Errorf("Expected file '%s', got '%s'", tmpFile, info.File)
	}
	if info.Source != "test-source" {
		t.Errorf("Expected source 'test-source', got '%s'", info.Source)
	}
	if info.CurrentValue != "1.25.0" {
		t.Errorf("Expected current value '1.25.0', got '%s'", info.CurrentValue)
	}
}

func TestYamlFieldTarget_GetTargetInfo_WithItemName(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "values.yaml")
	fileContent := `image:
  tag: "1.25.0"
`
	if err := os.WriteFile(tmpFile, []byte(fileContent), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	config := &configuration.Target{
		Name: "test-target",
		Type: configuration.TargetTypeYamlField,
		File: tmpFile,
		Items: []configuration.TargetItem{
			{
				Name:     "custom-item-name",
				YamlPath: "image.tag",
				Source:   "test-source",
			},
		},
	}

	target, err := NewYamlFieldTargetForUpdateItem(config, &config.Items[0])
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	info := target.GetTargetInfo()

	// Item name should take precedence over target name
	if info.Name != "custom-item-name" {
		t.Errorf("Expected name 'custom-item-name', got '%s'", info.Name)
	}
}

func TestYamlFieldTarget_MultipleItemsSameFile(t *testing.T) {
	fileContent := `image:
  repository: nginx
  tag: "1.25.0"
sidecar:
  image:
    repository: envoy
    tag: "1.28.0"
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "values.yaml")
	if err := os.WriteFile(tmpFile, []byte(fileContent), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Update the main image tag
	config1 := &configuration.Target{
		Name: "test-target",
		Type: configuration.TargetTypeYamlField,
		File: tmpFile,
		Items: []configuration.TargetItem{
			{
				YamlPath: "image.tag",
				Source:   "nginx-source",
			},
		},
	}

	target1, err := NewYamlFieldTargetForUpdateItem(config1, &config1.Items[0])
	if err != nil {
		t.Fatalf("Failed to create target1: %v", err)
	}

	if err := target1.WriteVersion("1.26.0"); err != nil {
		t.Fatalf("Failed to write version for target1: %v", err)
	}

	// Update the sidecar image tag (re-read file since it was modified)
	config2 := &configuration.Target{
		Name: "test-target",
		Type: configuration.TargetTypeYamlField,
		File: tmpFile,
		Items: []configuration.TargetItem{
			{
				YamlPath: "sidecar.image.tag",
				Source:   "envoy-source",
			},
		},
	}

	target2, err := NewYamlFieldTargetForUpdateItem(config2, &config2.Items[0])
	if err != nil {
		t.Fatalf("Failed to create target2: %v", err)
	}

	if err := target2.WriteVersion("1.29.0"); err != nil {
		t.Fatalf("Failed to write version for target2: %v", err)
	}

	// Verify both versions were updated
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileStr := string(content)

	if !strings.Contains(fileStr, `"1.26.0"`) {
		t.Errorf("Main image tag was not updated to 1.26.0")
	}
	if !strings.Contains(fileStr, `"1.29.0"`) {
		t.Errorf("Sidecar image tag was not updated to 1.29.0")
	}
	// Old versions should be gone
	if strings.Contains(fileStr, "1.25.0") {
		t.Errorf("Old main image tag 1.25.0 still exists")
	}
	if strings.Contains(fileStr, "1.28.0") {
		t.Errorf("Old sidecar image tag 1.28.0 still exists")
	}
}

func TestYamlFieldTarget_KubernetesManifest(t *testing.T) {
	fileContent := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: my-app
          image: "myregistry.io/myapp:v1.2.3"
          ports:
            - containerPort: 8080
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "deployment.yaml")
	if err := os.WriteFile(tmpFile, []byte(fileContent), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	config := &configuration.Target{
		Name: "test-target",
		Type: configuration.TargetTypeYamlField,
		File: tmpFile,
		Items: []configuration.TargetItem{
			{
				YamlPath: "spec.template.spec.containers.0.image",
				Source:   "test-source",
			},
		},
	}

	target, err := NewYamlFieldTargetForUpdateItem(config, &config.Items[0])
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	// Read current version — should extract just the tag from the image reference
	version, err := target.ReadCurrentVersion()
	if err != nil {
		t.Fatalf("Failed to read current version: %v", err)
	}
	if version != "v1.2.3" {
		t.Errorf("Expected 'v1.2.3', got '%s'", version)
	}

	// Write new version (just the tag — image prefix is preserved)
	if err := target.WriteVersion("v1.3.0"); err != nil {
		t.Fatalf("Failed to write version: %v", err)
	}

	// Verify ReadCurrentVersion returns the new tag
	newVersion, err := target.ReadCurrentVersion()
	if err != nil {
		t.Fatalf("Failed to read updated version: %v", err)
	}
	if newVersion != "v1.3.0" {
		t.Errorf("Expected 'v1.3.0', got '%s'", newVersion)
	}

	// Verify file preserves structure and full image reference
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileStr := string(content)
	if !strings.Contains(fileStr, "myregistry.io/myapp:v1.3.0") {
		t.Errorf("Expected full image reference 'myregistry.io/myapp:v1.3.0' in file, got:\n%s", fileStr)
	}
	if !strings.Contains(fileStr, "replicas: 3") {
		t.Errorf("replicas field was incorrectly modified")
	}
	if !strings.Contains(fileStr, "containerPort: 8080") {
		t.Errorf("containerPort field was incorrectly modified")
	}
}

func TestYamlFieldTarget_FileNotFound(t *testing.T) {
	config := &configuration.Target{
		Name: "test-target",
		Type: configuration.TargetTypeYamlField,
		File: "/nonexistent/values.yaml",
		Items: []configuration.TargetItem{
			{
				YamlPath: "image.tag",
				Source:   "test-source",
			},
		},
	}

	_, err := NewYamlFieldTargetForUpdateItem(config, &config.Items[0])
	if err == nil {
		t.Errorf("Expected error for nonexistent file, got none")
	}

	var fileNotFound *FileNotFoundError
	if !isFileNotFoundError(err) {
		t.Errorf("Expected FileNotFoundError, got: %T - %v", err, err)
	}
	_ = fileNotFound
}

func isFileNotFoundError(err error) bool {
	_, ok := err.(*FileNotFoundError)
	return ok
}

func TestYamlFieldTarget_MissingYamlPath(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "values.yaml")
	if err := os.WriteFile(tmpFile, []byte("key: value\n"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	config := &configuration.Target{
		Name: "test-target",
		Type: configuration.TargetTypeYamlField,
		File: tmpFile,
		Items: []configuration.TargetItem{
			{
				YamlPath: "",
				Source:   "test-source",
			},
		},
	}

	_, err := NewYamlFieldTargetForUpdateItem(config, &config.Items[0])
	if err == nil {
		t.Errorf("Expected error for empty yamlPath, got none")
	}
}

func TestYamlFieldNotFoundError(t *testing.T) {
	err := &YamlFieldNotFoundError{
		Path: "image.tag",
		File: "/path/to/values.yaml",
	}

	expected := "yaml path 'image.tag' not found in file: /path/to/values.yaml"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestIsDockerImageReference(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"nginx:1.25.0", true},
		{"ghcr.io/immich-app/immich-server:v2.5.3", true},
		{"quay.io/keycloak/keycloak:26.4.7", true},
		{"quay.io/minio/minio:RELEASE.2025-09-07T16-13-09Z", true},
		{"mariadb:12.1.2", true},
		{"restic/rest-server:0.14.0", true},
		{"wordpress:6.2.2-php8.2-apache", true},
		{"redis:8.4.0", true},
		{"myregistry.com:5000/myimage:v1.0", true}, // registry with port
		{"v1.2.3", false},                           // just a version
		{"1.25.0", false},                           // just a number
		{"hello world", false},                      // not an image
		{"https://example.com", false},              // URL
		{"", false},                                 // empty
		{":tag", false},                             // no image name
		{"Hello: World", false},                     // space in tag
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			result := isDockerImageReference(tt.value)
			if result != tt.expected {
				t.Errorf("isDockerImageReference(%q) = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

func TestExtractTagFromImageReference(t *testing.T) {
	tests := []struct {
		value    string
		expected string
	}{
		{"nginx:1.25.0", "1.25.0"},
		{"ghcr.io/immich-app/immich-server:v2.5.3", "v2.5.3"},
		{"quay.io/minio/minio:RELEASE.2025-09-07T16-13-09Z", "RELEASE.2025-09-07T16-13-09Z"},
		{"wordpress:6.2.2-php8.2-apache", "6.2.2-php8.2-apache"},
		{"myregistry.com:5000/myimage:v1.0", "v1.0"},
		{"v1.2.3", "v1.2.3"}, // no colon, returns as-is
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			result := extractTagFromImageReference(tt.value)
			if result != tt.expected {
				t.Errorf("extractTagFromImageReference(%q) = %q, want %q", tt.value, result, tt.expected)
			}
		})
	}
}

func TestReplaceTagInImageReference(t *testing.T) {
	tests := []struct {
		value    string
		newTag   string
		expected string
	}{
		{"nginx:1.25.0", "1.26.0", "nginx:1.26.0"},
		{"ghcr.io/immich-app/immich-server:v2.5.3", "v2.5.4", "ghcr.io/immich-app/immich-server:v2.5.4"},
		{"wordpress:6.2.2-php8.2-apache", "6.9.1-php8.5-apache", "wordpress:6.9.1-php8.5-apache"},
		{"myregistry.com:5000/myimage:v1.0", "v2.0", "myregistry.com:5000/myimage:v2.0"},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			result := replaceTagInImageReference(tt.value, tt.newTag)
			if result != tt.expected {
				t.Errorf("replaceTagInImageReference(%q, %q) = %q, want %q", tt.value, tt.newTag, result, tt.expected)
			}
		})
	}
}

func TestYamlFieldTarget_DockerImageReference_ReadAndWrite(t *testing.T) {
	tests := []struct {
		name            string
		fileContent     string
		yamlPath        string
		expectedRead    string
		writeVersion    string
		expectedInFile  string
	}{
		{
			name: "simple image:tag",
			fileContent: `spec:
  template:
    spec:
      containers:
        - name: app
          image: redis:8.2.2
`,
			yamlPath:       "spec.template.spec.containers.0.image",
			expectedRead:   "8.2.2",
			writeVersion:   "8.4.0",
			expectedInFile: "image: redis:8.4.0",
		},
		{
			name: "registry/image:tag",
			fileContent: `spec:
  template:
    spec:
      containers:
        - name: app
          image: ghcr.io/immich-app/immich-server:v2.5.3
`,
			yamlPath:       "spec.template.spec.containers.0.image",
			expectedRead:   "v2.5.3",
			writeVersion:   "v2.5.4",
			expectedInFile: "image: ghcr.io/immich-app/immich-server:v2.5.4",
		},
		{
			name: "quoted image reference",
			fileContent: `spec:
  template:
    spec:
      containers:
        - name: app
          image: "quay.io/keycloak/keycloak:26.4.7"
`,
			yamlPath:       "spec.template.spec.containers.0.image",
			expectedRead:   "26.4.7",
			writeVersion:   "27.0.0",
			expectedInFile: `"quay.io/keycloak/keycloak:27.0.0"`,
		},
		{
			name: "image tag field (not a reference)",
			fileContent: `image:
  tag: "v1.2.3"
`,
			yamlPath:       "image.tag",
			expectedRead:   "v1.2.3",
			writeVersion:   "v1.3.0",
			expectedInFile: `"v1.3.0"`,
		},
		{
			name: "spec.image field",
			fileContent: `spec:
  image: quay.io/minio/minio:RELEASE.2025-09-07T16-13-09Z
`,
			yamlPath:       "spec.image",
			expectedRead:   "RELEASE.2025-09-07T16-13-09Z",
			writeVersion:   "RELEASE.2026-01-01T00-00-00Z",
			expectedInFile: "image: quay.io/minio/minio:RELEASE.2026-01-01T00-00-00Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "manifest.yaml")
			if err := os.WriteFile(tmpFile, []byte(tt.fileContent), 0644); err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}

			config := &configuration.Target{
				Name: "test",
				Type: configuration.TargetTypeYamlField,
				File: tmpFile,
				Items: []configuration.TargetItem{
					{YamlPath: tt.yamlPath, Source: "test-source"},
				},
			}

			target, err := NewYamlFieldTargetForUpdateItem(config, &config.Items[0])
			if err != nil {
				t.Fatalf("Failed to create target: %v", err)
			}

			// Test read
			version, err := target.ReadCurrentVersion()
			if err != nil {
				t.Fatalf("ReadCurrentVersion failed: %v", err)
			}
			if version != tt.expectedRead {
				t.Errorf("ReadCurrentVersion = %q, want %q", version, tt.expectedRead)
			}

			// Test write
			if err := target.WriteVersion(tt.writeVersion); err != nil {
				t.Fatalf("WriteVersion failed: %v", err)
			}

			// Verify file content
			content, err := os.ReadFile(tmpFile)
			if err != nil {
				t.Fatalf("Failed to read file: %v", err)
			}
			if !strings.Contains(string(content), tt.expectedInFile) {
				t.Errorf("Expected %q in file, got:\n%s", tt.expectedInFile, string(content))
			}

			// Verify read-after-write returns new tag
			newVersion, err := target.ReadCurrentVersion()
			if err != nil {
				t.Fatalf("ReadCurrentVersion after write failed: %v", err)
			}
			if newVersion != tt.writeVersion {
				t.Errorf("ReadCurrentVersion after write = %q, want %q", newVersion, tt.writeVersion)
			}
		})
	}
}

func TestYamlFieldTarget_MultiDocumentYAML(t *testing.T) {
	// Simulates a multi-document YAML file like Kubernetes manifests
	fileContent := `apiVersion: v1
kind: Namespace
metadata:
  name: test-ns
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-sa
  namespace: test-ns
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: my-app
          image: ghcr.io/org/my-app:v1.0.0
          ports:
            - containerPort: 8080
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "manifest.yaml")
	if err := os.WriteFile(tmpFile, []byte(fileContent), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	config := &configuration.Target{
		Name: "test",
		Type: configuration.TargetTypeYamlField,
		File: tmpFile,
		Items: []configuration.TargetItem{
			{YamlPath: "spec.template.spec.containers.0.image", Source: "test-source"},
		},
	}

	target, err := NewYamlFieldTargetForUpdateItem(config, &config.Items[0])
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	// Read should find the image in the 3rd document
	version, err := target.ReadCurrentVersion()
	if err != nil {
		t.Fatalf("ReadCurrentVersion failed: %v", err)
	}
	if version != "v1.0.0" {
		t.Errorf("ReadCurrentVersion = %q, want %q", version, "v1.0.0")
	}

	// Write should update only the tag in the 3rd document
	if err := target.WriteVersion("v1.1.0"); err != nil {
		t.Fatalf("WriteVersion failed: %v", err)
	}

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	fileStr := string(content)

	// Verify the image was updated
	if !strings.Contains(fileStr, "ghcr.io/org/my-app:v1.1.0") {
		t.Errorf("Image not updated in file:\n%s", fileStr)
	}

	// Verify document separators are preserved
	if strings.Count(fileStr, "---") < 2 {
		t.Errorf("Document separators were lost")
	}

	// Verify other documents are intact
	if !strings.Contains(fileStr, "kind: Namespace") {
		t.Errorf("First document was modified")
	}
	if !strings.Contains(fileStr, "kind: ServiceAccount") {
		t.Errorf("Second document was modified")
	}
}

func TestYamlFieldTarget_MultiDocumentYAML_PathNotInFirstDoc(t *testing.T) {
	// The path exists only in the 2nd document
	fileContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: config
data:
  key: value
---
spec:
  imageName: ghcr.io/org/postgres:16.1
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "cluster.yaml")
	if err := os.WriteFile(tmpFile, []byte(fileContent), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	config := &configuration.Target{
		Name: "test",
		Type: configuration.TargetTypeYamlField,
		File: tmpFile,
		Items: []configuration.TargetItem{
			{YamlPath: "spec.imageName", Source: "test-source"},
		},
	}

	target, err := NewYamlFieldTargetForUpdateItem(config, &config.Items[0])
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	version, err := target.ReadCurrentVersion()
	if err != nil {
		t.Fatalf("ReadCurrentVersion failed: %v", err)
	}
	if version != "16.1" {
		t.Errorf("ReadCurrentVersion = %q, want %q", version, "16.1")
	}
}
