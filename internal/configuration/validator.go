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
	sourceNames := make(map[string]bool)
	providerByName := make(map[string]*PackageSourceProvider)
	for _, provider := range config.PackageSourceProviders {
		providerByName[provider.Name] = provider
	}

	for i, source := range config.PackageSources {
		fieldPrefix := fmt.Sprintf("packageSources[%d]", i)

		// Validate name
		if strings.TrimSpace(source.Name) == "" {
			result.AddError(fmt.Sprintf("%s.name", fieldPrefix), "source name cannot be empty")
		} else {
			if sourceNames[source.Name] {
				result.AddError(fmt.Sprintf("%s.name", fieldPrefix), fmt.Sprintf("duplicate source name: %s", source.Name))
			}
			sourceNames[source.Name] = true
		}

		// Validate provider reference
		var provider *PackageSourceProvider
		if strings.TrimSpace(source.Provider) == "" {
			result.AddError(fmt.Sprintf("%s.provider", fieldPrefix), "provider reference cannot be empty")
		} else if !providerNames[source.Provider] {
			result.AddError(fmt.Sprintf("%s.provider", fieldPrefix), fmt.Sprintf("provider '%s' not found in packageSourceProviders", source.Provider))
		} else {
			provider = providerByName[source.Provider]
		}

		// Validate type
		if !isValidSourceType(source.Type) {
			result.AddError(fmt.Sprintf("%s.type", fieldPrefix), fmt.Sprintf("invalid source type: %s", source.Type))
		}

		// Validate source type and provider type combination
		if provider != nil {
			if err := validateSourceProviderCombination(source.Type, provider.Type); err != nil {
				result.AddError(fmt.Sprintf("%s.type", fieldPrefix), err.Error())
			}
		}

		// Validate URI (not required for helm-repository as it uses provider's baseUrl)
		if source.Type != PackageSourceTypeHelmRepository && strings.TrimSpace(source.URI) == "" {
			result.AddError(fmt.Sprintf("%s.uri", fieldPrefix), "URI cannot be empty")
		}

		// Validate helm-repository specific fields
		if source.Type == PackageSourceTypeHelmRepository {
			if strings.TrimSpace(source.ChartName) == "" {
				result.AddError(fmt.Sprintf("%s.chartName", fieldPrefix), "chartName is required for helm-repository source type")
			}
			// Validate that the provider has baseUrl configured
			if provider != nil && strings.TrimSpace(provider.BaseUrl) == "" {
				result.AddError(fmt.Sprintf("%s.provider", fieldPrefix), fmt.Sprintf("provider '%s' must have baseUrl configured for helm-repository source type", source.Provider))
			}
		}
	}

	// Validate targets
	for i, target := range config.Targets {
		fieldPrefix := fmt.Sprintf("targets[%d]", i)

		// Validate name
		if strings.TrimSpace(target.Name) == "" {
			result.AddError(fmt.Sprintf("%s.name", fieldPrefix), "target name cannot be empty")
		}

		// Validate type
		if !isValidTargetType(target.Type) {
			result.AddError(fmt.Sprintf("%s.type", fieldPrefix), fmt.Sprintf("invalid target type: %s", target.Type))
		}

		// Validate file
		if strings.TrimSpace(target.File) == "" {
			result.AddError(fmt.Sprintf("%s.file", fieldPrefix), "file path cannot be empty")
		}

		// Validate updateItems
		if len(target.Items) == 0 {
			result.AddError(fmt.Sprintf("%s.updateItems", fieldPrefix), "at least one updateItem is required")
		}

		for j, item := range target.Items {
			itemPrefix := fmt.Sprintf("%s.updateItems[%d]", fieldPrefix, j)

			// Validate source reference
			if strings.TrimSpace(item.Source) == "" {
				result.AddError(fmt.Sprintf("%s.source", itemPrefix), "source reference cannot be empty")
			} else if !sourceNames[item.Source] {
				result.AddError(fmt.Sprintf("%s.source", itemPrefix), fmt.Sprintf("source '%s' not found in packageSources", item.Source))
			}

			// Type-specific validation
			switch target.Type {
			case TargetTypeTerraformVariable:
				if strings.TrimSpace(item.TerraformVariableName) == "" {
					result.AddError(fmt.Sprintf("%s.terraformVariableName", itemPrefix), "terraformVariableName is required for terraform-variable target")
				}
			case TargetTypeSubchart:
				if strings.TrimSpace(item.SubchartName) == "" {
					result.AddError(fmt.Sprintf("%s.subchartName", itemPrefix), "subchartName is required for subchart target")
				}
			case TargetTypeYamlField:
				if strings.TrimSpace(item.YamlPath) == "" {
					result.AddError(fmt.Sprintf("%s.yamlPath", itemPrefix), "yamlPath is required for yaml-field target")
				}
			}
		}
	}

	// Validate targetActor (optional but if present, must have required fields)
	if config.TargetActor != nil {
		fieldPrefix := "targetActor"

		// Validate name
		if strings.TrimSpace(config.TargetActor.Name) == "" {
			result.AddError(fmt.Sprintf("%s.name", fieldPrefix), "targetActor name cannot be empty")
		}

		// Validate email
		if strings.TrimSpace(config.TargetActor.Email) == "" {
			result.AddError(fmt.Sprintf("%s.email", fieldPrefix), "targetActor email cannot be empty")
		}

		// Validate username
		if strings.TrimSpace(config.TargetActor.Username) == "" {
			result.AddError(fmt.Sprintf("%s.username", fieldPrefix), "targetActor username cannot be empty")
		}

		// Token is optional, so no validation needed
	}

	return result
}

// isValidProviderType checks if the provider type is valid
func isValidProviderType(providerType PackageSourceProviderType) bool {
	switch providerType {
	case PackageSourceProviderTypeGitHub,
		PackageSourceProviderTypeHarbor,
		PackageSourceProviderTypeDocker,
		PackageSourceProviderTypeHelm:
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
		PackageSourceTypeDockerImage,
		PackageSourceTypeHelmRepository:
		return true
	default:
		return false
	}
}

// validateSourceProviderCombination validates that the source type is compatible with the provider type
func validateSourceProviderCombination(sourceType PackageSourceType, providerType PackageSourceProviderType) error {
	switch sourceType {
	case PackageSourceTypeGitRelease, PackageSourceTypeGitTag, PackageSourceTypeGitHelmChart:
		if providerType != PackageSourceProviderTypeGitHub {
			return fmt.Errorf("source type '%s' requires provider type 'github', but provider type is '%s'", sourceType, providerType)
		}
	case PackageSourceTypeDockerImage:
		if providerType != PackageSourceProviderTypeDocker && providerType != PackageSourceProviderTypeHarbor {
			return fmt.Errorf("source type '%s' requires provider type 'docker' or 'harbor', but provider type is '%s'", sourceType, providerType)
		}
	case PackageSourceTypeHelmRepository:
		if providerType != PackageSourceProviderTypeHelm {
			return fmt.Errorf("source type '%s' requires provider type 'helm', but provider type is '%s'", sourceType, providerType)
		}
	}
	return nil
}

// isValidTargetType checks if the target type is valid
func isValidTargetType(targetType TargetType) bool {
	switch targetType {
	case TargetTypeTerraformVariable,
		TargetTypeSubchart,
		TargetTypeYamlField:
		return true
	default:
		return false
	}
}
