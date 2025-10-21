package configuration

import (
	"fmt"
	"strings"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationResult contains the results of configuration validation
type ValidationResult struct {
	Valid  bool
	Errors []*ValidationError
}

// AddError adds a validation error to the result
func (r *ValidationResult) AddError(field, message string) {
	r.Valid = false
	r.Errors = append(r.Errors, &ValidationError{
		Field:   field,
		Message: message,
	})
}

// ValidateConfiguration performs validation on the configuration
func ValidateConfiguration(config *Config) *ValidationResult {
	result := &ValidationResult{
		Valid:  true,
		Errors: make([]*ValidationError, 0),
	}

	// Validate package source providers
	providerNames := make(map[string]bool)
	for i, provider := range config.PackageSourceProviders {
		fieldPrefix := fmt.Sprintf("packageSourceProviders[%d]", i)

		// Validate name
		if strings.TrimSpace(provider.Name) == "" {
			result.AddError(fmt.Sprintf("%s.name", fieldPrefix), "provider name cannot be empty")
		} else {
			if providerNames[provider.Name] {
				result.AddError(fmt.Sprintf("%s.name", fieldPrefix), fmt.Sprintf("duplicate provider name: %s", provider.Name))
			}
			providerNames[provider.Name] = true
		}

		// Validate type
		if !isValidProviderType(provider.Type) {
			result.AddError(fmt.Sprintf("%s.type", fieldPrefix), fmt.Sprintf("invalid provider type: %s", provider.Type))
		}

		// Validate auth type
		if provider.AuthType != "" && !isValidAuthType(provider.AuthType) {
			result.AddError(fmt.Sprintf("%s.authType", fieldPrefix), fmt.Sprintf("invalid auth type: %s", provider.AuthType))
		}

		// Validate auth configuration
		if provider.AuthType == PackageSourceProviderAuthTypeBasic {
			if strings.TrimSpace(provider.Username) == "" {
				result.AddError(fmt.Sprintf("%s.username", fieldPrefix), "username is required for basic auth")
			}
			if strings.TrimSpace(provider.Password) == "" {
				result.AddError(fmt.Sprintf("%s.password", fieldPrefix), "password is required for basic auth")
			}
		}

		if provider.AuthType == PackageSourceProviderAuthTypeToken {
			if strings.TrimSpace(provider.Token) == "" {
				result.AddError(fmt.Sprintf("%s.token", fieldPrefix), "token is required for token auth")
			}
		}
	}

	// Validate package sources
	for i, source := range config.PackageSources {
		fieldPrefix := fmt.Sprintf("packageSources[%d]", i)

		// Validate name
		if strings.TrimSpace(source.Name) == "" {
			result.AddError(fmt.Sprintf("%s.name", fieldPrefix), "source name cannot be empty")
		}

		// Validate provider reference
		if strings.TrimSpace(source.Provider) == "" {
			result.AddError(fmt.Sprintf("%s.provider", fieldPrefix), "provider reference cannot be empty")
		} else if !providerNames[source.Provider] {
			result.AddError(fmt.Sprintf("%s.provider", fieldPrefix), fmt.Sprintf("provider '%s' not found in packageSourceProviders", source.Provider))
		}

		// Validate type
		if !isValidSourceType(source.Type) {
			result.AddError(fmt.Sprintf("%s.type", fieldPrefix), fmt.Sprintf("invalid source type: %s", source.Type))
		}

		// Validate URI
		if strings.TrimSpace(source.URI) == "" {
			result.AddError(fmt.Sprintf("%s.uri", fieldPrefix), "URI cannot be empty")
		}
	}

	return result
}

// isValidProviderType checks if the provider type is valid
func isValidProviderType(providerType PackageSourceProviderType) bool {
	switch providerType {
	case PackageSourceProviderTypeGitHub,
		PackageSourceProviderTypeHarbor,
		PackageSourceProviderTypeDocker:
		return true
	default:
		return false
	}
}

// isValidAuthType checks if the auth type is valid
func isValidAuthType(authType PackageSourceProviderAuthType) bool {
	switch authType {
	case PackageSourceProviderAuthTypeNone,
		PackageSourceProviderAuthTypeBasic,
		PackageSourceProviderAuthTypeToken:
		return true
	default:
		return false
	}
}

// isValidSourceType checks if the source type is valid
func isValidSourceType(sourceType PackageSourceType) bool {
	switch sourceType {
	case PackageSourceTypeGitRelease,
		PackageSourceTypeGitTag,
		PackageSourceTypeGitHelmChart,
		PackageSourceTypeDockerImage:
		return true
	default:
		return false
	}
}