package configuration

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// SubstitutionContext holds the state for variable substitution
type SubstitutionContext struct {
	sopsCache map[string]map[string]interface{} // Cache for loaded SOPS files
}

// NewSubstitutionContext creates a new substitution context
func NewSubstitutionContext() *SubstitutionContext {
	return &SubstitutionContext{
		sopsCache: make(map[string]map[string]interface{}),
	}
}

// SubstituteVariables replaces environment variables and SOPS references in the input string
// Supports:
// - ${VAR_NAME} for environment variables
// - ${SOPS[path/to/file.yml].yaml.path.to.value} for SOPS encrypted files
func (ctx *SubstitutionContext) SubstituteVariables(input string) (string, error) {
	// Pattern to match ${...} placeholders
	pattern := regexp.MustCompile(`\$\{([^}]+)\}`)

	result := input
	matches := pattern.FindAllStringSubmatch(input, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		placeholder := match[0] // Full match: ${...}
		expression := match[1]  // Content inside: VAR_NAME or SOPS[...]...

		var value string
		var err error

		if strings.HasPrefix(expression, "SOPS[") {
			// Handle SOPS reference
			value, err = ctx.resolveSOPSReference(expression)
			if err != nil {
				return "", fmt.Errorf("failed to resolve SOPS reference %s: %w", placeholder, err)
			}
		} else {
			// Handle regular environment variable
			value = os.Getenv(expression)
			if value == "" {
				return "", fmt.Errorf("environment variable %s is not set", expression)
			}
		}

		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result, nil
}

// resolveSOPSReference resolves a SOPS reference like SOPS[file.yml].path.to.value
func (ctx *SubstitutionContext) resolveSOPSReference(expression string) (string, error) {
	// Extract file path and YAML path
	// Format: SOPS[path/to/file.yml].yaml.path.to.value
	if !strings.HasPrefix(expression, "SOPS[") {
		return "", fmt.Errorf("invalid SOPS reference format: %s", expression)
	}

	// Find the closing bracket
	closeBracketIdx := strings.Index(expression, "]")
	if closeBracketIdx == -1 {
		return "", fmt.Errorf("invalid SOPS reference format (missing ]): %s", expression)
	}

	filePath := expression[5:closeBracketIdx] // Extract path between SOPS[ and ]
	yamlPath := ""

	// Check if there's a YAML path after the bracket
	if closeBracketIdx+1 < len(expression) {
		if expression[closeBracketIdx+1] != '.' {
			return "", fmt.Errorf("invalid SOPS reference format (expected . after ]): %s", expression)
		}
		yamlPath = expression[closeBracketIdx+2:] // Skip ].
	}

	if yamlPath == "" {
		return "", fmt.Errorf("SOPS reference must include a YAML path: %s", expression)
	}

	// Load SOPS file (with caching)
	data, err := ctx.loadSOPSFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to load SOPS file %s: %w", filePath, err)
	}

	// Access the value using the YAML path
	value, err := GetYAMLValue(data, yamlPath)
	if err != nil {
		return "", fmt.Errorf("failed to access path %s in SOPS file %s: %w", yamlPath, filePath, err)
	}

	// Convert value to string
	return fmt.Sprintf("%v", value), nil
}

// loadSOPSFile loads and decrypts a SOPS file, with caching
func (ctx *SubstitutionContext) loadSOPSFile(filePath string) (map[string]interface{}, error) {
	// Check cache first
	if data, ok := ctx.sopsCache[filePath]; ok {
		return data, nil
	}

	// Decrypt the SOPS file
	data, err := DecryptSOPSFile(filePath)
	if err != nil {
		return nil, err
	}

	// Cache the decrypted data
	ctx.sopsCache[filePath] = data

	return data, nil
}

// SubstituteInConfig recursively substitutes variables in the entire config structure
func (ctx *SubstitutionContext) SubstituteInConfig(config *Config) error {
	// Substitute in providers
	for _, provider := range config.PackageSourceProviders {
		if err := ctx.substituteInProvider(provider); err != nil {
			return err
		}
	}

	// Substitute in sources
	for _, source := range config.PackageSources {
		if err := ctx.substituteInSource(source); err != nil {
			return err
		}
	}

	// Substitute in targetActor
	if config.TargetActor != nil {
		if err := ctx.substituteInTargetActor(config.TargetActor); err != nil {
			return err
		}
	}

	return nil
}

func (ctx *SubstitutionContext) substituteInProvider(provider *PackageSourceProvider) error {
	var err error

	if provider.BaseUrl != "" {
		provider.BaseUrl, err = ctx.SubstituteVariables(provider.BaseUrl)
		if err != nil {
			return fmt.Errorf("failed to substitute BaseUrl in provider %s: %w", provider.Name, err)
		}
	}

	if provider.Username != "" {
		provider.Username, err = ctx.SubstituteVariables(provider.Username)
		if err != nil {
			return fmt.Errorf("failed to substitute Username in provider %s: %w", provider.Name, err)
		}
	}

	if provider.Password != "" {
		provider.Password, err = ctx.SubstituteVariables(provider.Password)
		if err != nil {
			return fmt.Errorf("failed to substitute Password in provider %s: %w", provider.Name, err)
		}
	}

	if provider.Token != "" {
		provider.Token, err = ctx.SubstituteVariables(provider.Token)
		if err != nil {
			return fmt.Errorf("failed to substitute Token in provider %s: %w", provider.Name, err)
		}
	}

	return nil
}

func (ctx *SubstitutionContext) substituteInSource(source *PackageSource) error {
	var err error

	if source.URI != "" {
		source.URI, err = ctx.SubstituteVariables(source.URI)
		if err != nil {
			return fmt.Errorf("failed to substitute URI in source %s: %w", source.Name, err)
		}
	}

	if source.VersionConstraint != "" {
		source.VersionConstraint, err = ctx.SubstituteVariables(source.VersionConstraint)
		if err != nil {
			return fmt.Errorf("failed to substitute VersionConstraint in source %s: %w", source.Name, err)
		}
	}

	return nil
}

func (ctx *SubstitutionContext) substituteInTargetActor(targetActor *TargetActor) error {
	var err error

	if targetActor.Name != "" {
		targetActor.Name, err = ctx.SubstituteVariables(targetActor.Name)
		if err != nil {
			return fmt.Errorf("failed to substitute Name in targetActor: %w", err)
		}
	}

	if targetActor.Email != "" {
		targetActor.Email, err = ctx.SubstituteVariables(targetActor.Email)
		if err != nil {
			return fmt.Errorf("failed to substitute Email in targetActor: %w", err)
		}
	}

	if targetActor.Username != "" {
		targetActor.Username, err = ctx.SubstituteVariables(targetActor.Username)
		if err != nil {
			return fmt.Errorf("failed to substitute Username in targetActor: %w", err)
		}
	}

	if targetActor.Token != "" {
		targetActor.Token, err = ctx.SubstituteVariables(targetActor.Token)
		if err != nil {
			return fmt.Errorf("failed to substitute Token in targetActor: %w", err)
		}
	}

	return nil
}

// GetYAMLValue retrieves a value from a nested YAML structure using dot notation
// Example: "credentials.token" accesses data["credentials"]["token"]
func GetYAMLValue(data map[string]interface{}, path string) (interface{}, error) {
	if path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}

	parts := strings.Split(path, ".")
	current := interface{}(data)

	for i, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("invalid path: empty segment at position %d", i)
		}

		switch v := current.(type) {
		case map[string]interface{}:
			value, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("path not found: %s (missing key '%s')", path, part)
			}
			current = value
		case map[interface{}]interface{}:
			// YAML sometimes uses interface{} keys
			value, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("path not found: %s (missing key '%s')", path, part)
			}
			current = value
		default:
			return nil, fmt.Errorf("path not found: %s (cannot traverse into non-map at '%s')", path, part)
		}
	}

	return current, nil
}

// DecryptSOPSFile decrypts a SOPS-encrypted YAML file and returns the parsed data
func DecryptSOPSFile(filePath string) (map[string]interface{}, error) {
	return DecryptSOPSFileWithLib(filePath)
}
