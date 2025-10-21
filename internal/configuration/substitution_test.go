package configuration

import (
	"os"
	"strings"
	"testing"
)

func TestGetYAMLValue(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string]interface{}
		path      string
		want      interface{}
		wantError bool
	}{
		{
			name: "simple top-level access",
			data: map[string]interface{}{
				"token": "secret123",
			},
			path:      "token",
			want:      "secret123",
			wantError: false,
		},
		{
			name: "nested access",
			data: map[string]interface{}{
				"credentials": map[string]interface{}{
					"token": "nested-secret",
				},
			},
			path:      "credentials.token",
			want:      "nested-secret",
			wantError: false,
		},
		{
			name: "deeply nested access",
			data: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"level3": map[string]interface{}{
							"value": "deep-value",
						},
					},
				},
			},
			path:      "level1.level2.level3.value",
			want:      "deep-value",
			wantError: false,
		},
		{
			name: "access number value",
			data: map[string]interface{}{
				"config": map[string]interface{}{
					"port": 8080,
				},
			},
			path:      "config.port",
			want:      8080,
			wantError: false,
		},
		{
			name: "access boolean value",
			data: map[string]interface{}{
				"settings": map[string]interface{}{
					"enabled": true,
				},
			},
			path:      "settings.enabled",
			want:      true,
			wantError: false,
		},
		{
			name: "path not found",
			data: map[string]interface{}{
				"token": "secret",
			},
			path:      "nonexistent",
			wantError: true,
		},
		{
			name: "nested path not found",
			data: map[string]interface{}{
				"credentials": map[string]interface{}{
					"username": "user",
				},
			},
			path:      "credentials.token",
			wantError: true,
		},
		{
			name: "empty path",
			data: map[string]interface{}{
				"token": "secret",
			},
			path:      "",
			wantError: true,
		},
		{
			name: "path with empty segment",
			data: map[string]interface{}{
				"credentials": map[string]interface{}{
					"token": "secret",
				},
			},
			path:      "credentials..token",
			wantError: true,
		},
		{
			name: "traverse into non-map",
			data: map[string]interface{}{
				"value": "string",
			},
			path:      "value.nested",
			wantError: true,
		},
		{
			name: "interface{} keyed map",
			data: map[string]interface{}{
				"data": map[interface{}]interface{}{
					"key": "value",
				},
			},
			path:      "data.key",
			want:      "value",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetYAMLValue(tt.data, tt.path)

			if tt.wantError {
				if err == nil {
					t.Errorf("GetYAMLValue() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("GetYAMLValue() unexpected error: %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("GetYAMLValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubstituteVariables_EnvVars(t *testing.T) {
	// Set up test environment variables
	os.Setenv("TEST_VAR", "test-value")
	os.Setenv("API_KEY", "secret-key-123")
	os.Setenv("BASE_URL", "https://api.example.com")
	defer func() {
		os.Unsetenv("TEST_VAR")
		os.Unsetenv("API_KEY")
		os.Unsetenv("BASE_URL")
	}()

	tests := []struct {
		name      string
		input     string
		want      string
		wantError bool
	}{
		{
			name:      "simple substitution",
			input:     "${TEST_VAR}",
			want:      "test-value",
			wantError: false,
		},
		{
			name:      "substitution in middle of string",
			input:     "prefix-${TEST_VAR}-suffix",
			want:      "prefix-test-value-suffix",
			wantError: false,
		},
		{
			name:      "multiple substitutions",
			input:     "${TEST_VAR} and ${API_KEY}",
			want:      "test-value and secret-key-123",
			wantError: false,
		},
		{
			name:      "URL with substitution",
			input:     "${BASE_URL}/v1/endpoint",
			want:      "https://api.example.com/v1/endpoint",
			wantError: false,
		},
		{
			name:      "no substitution needed",
			input:     "plain string",
			want:      "plain string",
			wantError: false,
		},
		{
			name:      "undefined variable",
			input:     "${UNDEFINED_VAR}",
			wantError: true,
		},
		{
			name:      "empty string",
			input:     "",
			want:      "",
			wantError: false,
		},
		{
			name:      "escaped dollar sign (not a variable)",
			input:     "price: $100",
			want:      "price: $100",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewSubstitutionContext()
			got, err := ctx.SubstituteVariables(tt.input)

			if tt.wantError {
				if err == nil {
					t.Errorf("SubstituteVariables() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("SubstituteVariables() unexpected error: %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("SubstituteVariables() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveSOPSReference(t *testing.T) {
	tests := []struct {
		name      string
		expr      string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid format",
			expr:      "SOPS[path/to/file.yml].credentials.token",
			wantError: false,
		},
		{
			name:      "missing closing bracket",
			expr:      "SOPS[path/to/file.yml.credentials.token",
			wantError: true,
			errorMsg:  "missing ]",
		},
		{
			name:      "missing dot after bracket",
			expr:      "SOPS[path/to/file.yml]credentials",
			wantError: true,
			errorMsg:  "expected . after ]",
		},
		{
			name:      "missing YAML path",
			expr:      "SOPS[path/to/file.yml]",
			wantError: true,
			errorMsg:  "must include a YAML path",
		},
		{
			name:      "empty YAML path",
			expr:      "SOPS[path/to/file.yml].",
			wantError: true,
		},
		{
			name:      "invalid prefix",
			expr:      "INVALID[path/to/file.yml].token",
			wantError: true,
			errorMsg:  "invalid SOPS reference format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewSubstitutionContext()
			_, err := ctx.resolveSOPSReference(tt.expr)

			if tt.wantError {
				if err == nil {
					t.Errorf("resolveSOPSReference() expected error but got none")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("resolveSOPSReference() error = %q, want it to contain %q", err.Error(), tt.errorMsg)
				}
			} else {
				// For valid format test, we expect an error about file not found
				// since we're not creating actual SOPS files here
				if err == nil {
					t.Errorf("resolveSOPSReference() expected file error but got none (file should not exist)")
				}
			}
		})
	}
}

func TestSubstituteInConfig(t *testing.T) {
	// Set up test environment variables
	os.Setenv("TEST_BASE_URL", "https://test.example.com")
	os.Setenv("TEST_TOKEN", "test-token-123")
	os.Setenv("TEST_URI", "https://github.com/test/repo")
	defer func() {
		os.Unsetenv("TEST_BASE_URL")
		os.Unsetenv("TEST_TOKEN")
		os.Unsetenv("TEST_URI")
	}()

	config := &Config{
		PackageSourceProviders: []*PackageSourceProvider{
			{
				Name:     "test-provider",
				Type:     PackageSourceProviderTypeGitHub,
				BaseUrl:  "${TEST_BASE_URL}",
				AuthType: PackageSourceProviderAuthTypeToken,
				Token:    "${TEST_TOKEN}",
			},
		},
		PackageSources: []*PackageSource{
			{
				Name:     "test-source",
				Provider: "test-provider",
				Type:     PackageSourceTypeGitRelease,
				URI:      "${TEST_URI}",
			},
		},
	}

	ctx := NewSubstitutionContext()
	err := ctx.SubstituteInConfig(config)
	if err != nil {
		t.Fatalf("SubstituteInConfig() unexpected error: %v", err)
	}

	// Verify substitutions
	if config.PackageSourceProviders[0].BaseUrl != "https://test.example.com" {
		t.Errorf("BaseUrl = %q, want %q", config.PackageSourceProviders[0].BaseUrl, "https://test.example.com")
	}

	if config.PackageSourceProviders[0].Token != "test-token-123" {
		t.Errorf("Token = %q, want %q", config.PackageSourceProviders[0].Token, "test-token-123")
	}

	if config.PackageSources[0].URI != "https://github.com/test/repo" {
		t.Errorf("URI = %q, want %q", config.PackageSources[0].URI, "https://github.com/test/repo")
	}
}

func TestSubstitutionContext_Caching(t *testing.T) {
	ctx := NewSubstitutionContext()

	// Verify cache is initialized
	if ctx.sopsCache == nil {
		t.Error("sopsCache should be initialized")
	}

	// Verify cache is empty initially
	if len(ctx.sopsCache) != 0 {
		t.Errorf("sopsCache should be empty initially, got %d entries", len(ctx.sopsCache))
	}
}

// No helper needed - we'll use strings.Contains from standard library