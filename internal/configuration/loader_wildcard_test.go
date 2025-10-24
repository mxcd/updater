package configuration

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestExpandWildcardTargets(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()
	
	// Create test files
	env1Dir := filepath.Join(tmpDir, "environments", "dev")
	env2Dir := filepath.Join(tmpDir, "environments", "prod")
	env3Dir := filepath.Join(tmpDir, "environments", "staging")
	
	if err := os.MkdirAll(env1Dir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	if err := os.MkdirAll(env2Dir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	if err := os.MkdirAll(env3Dir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	
	// Create Chart.yaml files
	chartContent := `apiVersion: v2
name: test-app
version: 1.0.0
dependencies:
  - name: backend
    version: 1.0.0
    repository: oci://registry.example.com/charts
`
	
	devChart := filepath.Join(env1Dir, "Chart.yaml")
	prodChart := filepath.Join(env2Dir, "Chart.yaml")
	stagingChart := filepath.Join(env3Dir, "Chart.yaml")
	
	if err := os.WriteFile(devChart, []byte(chartContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	if err := os.WriteFile(prodChart, []byte(chartContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	if err := os.WriteFile(stagingChart, []byte(chartContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	tests := []struct {
		name           string
		config         *Config
		expectedCount  int
		checkWildcard  bool
		wildcardCount  int
	}{
		{
			name: "expand wildcard pattern",
			config: &Config{
				Targets: []*Target{
					{
						Name: "helm-charts",
						Type: TargetTypeSubchart,
						File: filepath.Join(tmpDir, "environments", "*", "Chart.yaml"),
						Items: []TargetItem{
							{
								SubchartName: "backend",
								Source:       "backend-source",
							},
						},
					},
				},
			},
			expectedCount: 3,
			checkWildcard: true,
			wildcardCount: 3,
		},
		{
			name: "no wildcard - keep as is",
			config: &Config{
				Targets: []*Target{
					{
						Name: "single-chart",
						Type: TargetTypeSubchart,
						File: devChart,
						Items: []TargetItem{
							{
								SubchartName: "backend",
								Source:       "backend-source",
							},
						},
					},
				},
			},
			expectedCount: 1,
			checkWildcard: false,
			wildcardCount: 0,
		},
		{
			name: "mixed wildcard and non-wildcard",
			config: &Config{
				Targets: []*Target{
					{
						Name: "wildcard-charts",
						Type: TargetTypeSubchart,
						File: filepath.Join(tmpDir, "environments", "*", "Chart.yaml"),
						Items: []TargetItem{
							{
								SubchartName: "backend",
								Source:       "backend-source",
							},
						},
					},
					{
						Name: "specific-chart",
						Type: TargetTypeSubchart,
						File: devChart,
						Items: []TargetItem{
							{
								SubchartName: "frontend",
								Source:       "frontend-source",
							},
						},
					},
				},
			},
			expectedCount: 4, // 3 from wildcard + 1 specific
			checkWildcard: true,
			wildcardCount: 3,
		},
		{
			name: "wildcard with no matches - keep original",
			config: &Config{
				Targets: []*Target{
					{
						Name: "no-matches",
						Type: TargetTypeSubchart,
						File: filepath.Join(tmpDir, "nonexistent", "*", "Chart.yaml"),
						Items: []TargetItem{
							{
								SubchartName: "backend",
								Source:       "backend-source",
							},
						},
					},
				},
			},
			expectedCount: 1, // Original target kept
			checkWildcard: false,
			wildcardCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Expand wildcards
			if err := ExpandWildcardTargets(tt.config); err != nil {
				t.Fatalf("ExpandWildcardTargets failed: %v", err)
			}

			// Check total count
			if len(tt.config.Targets) != tt.expectedCount {
				t.Errorf("Expected %d targets, got %d", tt.expectedCount, len(tt.config.Targets))
			}

			// Check wildcard metadata
			if tt.checkWildcard {
				wildcardMatches := 0
				for _, target := range tt.config.Targets {
					if target.IsWildcardMatch {
						wildcardMatches++
						if target.WildcardPattern == "" {
							t.Errorf("Target %s is marked as wildcard match but has no pattern", target.File)
						}
					}
				}
				
				if wildcardMatches != tt.wildcardCount {
					t.Errorf("Expected %d wildcard matches, got %d", tt.wildcardCount, wildcardMatches)
				}
			}
		})
	}
}

func TestExpandWildcardTargets_PreservesTargetProperties(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create test structure
	env1Dir := filepath.Join(tmpDir, "env1")
	env2Dir := filepath.Join(tmpDir, "env2")
	
	if err := os.MkdirAll(env1Dir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	if err := os.MkdirAll(env2Dir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	
	chart1 := filepath.Join(env1Dir, "Chart.yaml")
	chart2 := filepath.Join(env2Dir, "Chart.yaml")
	
	chartContent := `apiVersion: v2
name: test
version: 1.0.0
`
	
	if err := os.WriteFile(chart1, []byte(chartContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	if err := os.WriteFile(chart2, []byte(chartContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	config := &Config{
		Targets: []*Target{
			{
				Name:       "test-charts",
				Type:       TargetTypeSubchart,
				File:       filepath.Join(tmpDir, "*", "Chart.yaml"),
				PatchGroup: "my-group",
				Labels:     []string{"label1", "label2"},
				Items: []TargetItem{
					{
						SubchartName: "backend",
						Source:       "backend-source",
					},
				},
			},
		},
	}

	if err := ExpandWildcardTargets(config); err != nil {
		t.Fatalf("ExpandWildcardTargets failed: %v", err)
	}

	// Verify properties are preserved
	if len(config.Targets) != 2 {
		t.Fatalf("Expected 2 expanded targets, got %d", len(config.Targets))
	}

	for _, target := range config.Targets {
		if target.Name != "test-charts" {
			t.Errorf("Name not preserved, got: %s", target.Name)
		}
		if target.Type != TargetTypeSubchart {
			t.Errorf("Type not preserved, got: %s", target.Type)
		}
		if target.PatchGroup != "my-group" {
			t.Errorf("PatchGroup not preserved, got: %s", target.PatchGroup)
		}
		if len(target.Labels) != 2 || target.Labels[0] != "label1" || target.Labels[1] != "label2" {
			t.Errorf("Labels not preserved, got: %v", target.Labels)
		}
		if len(target.Items) != 1 || target.Items[0].SubchartName != "backend" {
			t.Errorf("Items not preserved, got: %v", target.Items)
		}
		if !target.IsWildcardMatch {
			t.Errorf("IsWildcardMatch should be true")
		}
		if target.WildcardPattern == "" {
			t.Errorf("WildcardPattern should be set")
		}
	}
}
func TestExpandWildcardTargets_RecursiveGlob(t *testing.T) {
	// Create a nested directory structure with Chart.yaml files
	tmpDir := t.TempDir()
	
	// Create: envs/dev/app1/Chart.yaml
	devApp1Dir := filepath.Join(tmpDir, "envs", "dev", "app1")
	if err := os.MkdirAll(devApp1Dir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	devApp1File := filepath.Join(devApp1Dir, "Chart.yaml")
	if err := os.WriteFile(devApp1File, []byte("apiVersion: v2\nname: app1\nversion: 1.0.0"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Create: envs/dev/app2/Chart.yaml
	devApp2Dir := filepath.Join(tmpDir, "envs", "dev", "app2")
	if err := os.MkdirAll(devApp2Dir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	devApp2File := filepath.Join(devApp2Dir, "Chart.yaml")
	if err := os.WriteFile(devApp2File, []byte("apiVersion: v2\nname: app2\nversion: 1.0.0"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Create: envs/prod/app1/Chart.yaml
	prodApp1Dir := filepath.Join(tmpDir, "envs", "prod", "app1")
	if err := os.MkdirAll(prodApp1Dir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	prodApp1File := filepath.Join(prodApp1Dir, "Chart.yaml")
	if err := os.WriteFile(prodApp1File, []byte("apiVersion: v2\nname: app1\nversion: 1.0.0"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Create: other-file.txt (should not match)
	otherFile := filepath.Join(tmpDir, "envs", "dev", "other-file.txt")
	if err := os.WriteFile(otherFile, []byte("not a chart"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Test recursive glob with **
	pattern := filepath.Join(tmpDir, "envs", "**", "Chart.yaml")
	config := &Config{
		Targets: []*Target{
			{
				Name: "test-recursive",
				Type: TargetTypeSubchart,
				File: pattern,
				Items: []TargetItem{
					{
						SubchartName: "my-chart",
						Source:       "test-source",
					},
				},
				PatchGroup: "test-group",
				Labels:     []string{"test-label"},
			},
		},
	}

	err := ExpandWildcardTargets(config)
	if err != nil {
		t.Fatalf("ExpandWildcardTargets failed: %v", err)
	}

	// Should expand to 3 targets (one for each Chart.yaml)
	if len(config.Targets) != 3 {
		t.Fatalf("Expected 3 targets, got %d", len(config.Targets))
	}

	// Collect and sort file paths for consistent comparison
	matchedFiles := make([]string, len(config.Targets))
	for i, target := range config.Targets {
		matchedFiles[i] = target.File
	}
	sort.Strings(matchedFiles)

	expectedFiles := []string{devApp1File, devApp2File, prodApp1File}
	sort.Strings(expectedFiles)

	// Verify all expected files were matched
	for i, expected := range expectedFiles {
		if matchedFiles[i] != expected {
			t.Errorf("Expected file %s, got %s", expected, matchedFiles[i])
		}
	}

	// Verify all targets have correct properties
	for _, target := range config.Targets {
		if target.Name != "test-recursive" {
			t.Errorf("Expected name 'test-recursive', got '%s'", target.Name)
		}
		if target.Type != TargetTypeSubchart {
			t.Errorf("Expected type 'subchart', got '%s'", target.Type)
		}
		if !target.IsWildcardMatch {
			t.Errorf("Expected IsWildcardMatch to be true")
		}
		if target.WildcardPattern != pattern {
			t.Errorf("Expected WildcardPattern '%s', got '%s'", pattern, target.WildcardPattern)
		}
		if target.PatchGroup != "test-group" {
			t.Errorf("Expected PatchGroup 'test-group', got '%s'", target.PatchGroup)
		}
		if len(target.Labels) != 1 || target.Labels[0] != "test-label" {
			t.Errorf("Expected Labels ['test-label'], got %v", target.Labels)
		}
	}
}

func TestExpandWildcardTargets_RecursiveGlobSingleLevel(t *testing.T) {
	// Test that ** also works for matching files in the immediate directory
	tmpDir := t.TempDir()
	
	// Create: Chart.yaml in root
	rootFile := filepath.Join(tmpDir, "Chart.yaml")
	if err := os.WriteFile(rootFile, []byte("apiVersion: v2\nname: root\nversion: 1.0.0"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Create: subdir/Chart.yaml
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	subFile := filepath.Join(subDir, "Chart.yaml")
	if err := os.WriteFile(subFile, []byte("apiVersion: v2\nname: sub\nversion: 1.0.0"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Test ** matches files at all levels
	pattern := filepath.Join(tmpDir, "**", "Chart.yaml")
	config := &Config{
		Targets: []*Target{
			{
				Name: "test-all-levels",
				Type: TargetTypeSubchart,
				File: pattern,
				Items: []TargetItem{
					{
						SubchartName: "my-chart",
						Source:       "test-source",
					},
				},
			},
		},
	}

	err := ExpandWildcardTargets(config)
	if err != nil {
		t.Fatalf("ExpandWildcardTargets failed: %v", err)
	}

	// Should match both files
	if len(config.Targets) != 2 {
		t.Fatalf("Expected 2 targets, got %d", len(config.Targets))
	}

	// Collect file paths
	matchedFiles := make(map[string]bool)
	for _, target := range config.Targets {
		matchedFiles[target.File] = true
	}

	if !matchedFiles[rootFile] {
		t.Errorf("Expected to match %s", rootFile)
	}
	if !matchedFiles[subFile] {
		t.Errorf("Expected to match %s", subFile)
	}
}