# Helm Repository Implementation

## Overview

This document describes the implementation of the Helm Repository package source type and Helm provider for the updater project.

## Components Implemented

### 1. Helm Provider Client (`internal/scraper/helm/client.go`)
- Main client for the Helm provider
- Routes requests to appropriate scraping functions based on source type
- Supports `PackageSourceTypeHelmRepository`

### 2. Helm Repository Scraper (`internal/scraper/helm/repository.go`)
- Fetches and parses Helm repository `index.yaml` files
- Extracts chart versions for specified chart names
- Sorts versions semantically (newest first)
- Supports authentication (basic auth and token-based)
- Features:
  - Automatic index.yaml URL construction from base URL
  - Chart name lookup in repository index
  - Semantic version parsing and sorting
  - Limit support for returned versions
  - AppVersion metadata extraction

### 3. Helm Provider Adapter (`internal/scraper/provider-helm-adapter.go`)
- Adapter pattern implementation for Helm provider
- Integrates with the orchestrator's provider system

### 4. Configuration Types (`internal/configuration/types.go`)
- Already had `PackageSourceProviderTypeHelm` constant defined
- Already had `PackageSourceTypeHelmRepository` constant defined
- `ChartName` field in `PackageSource` for specifying Helm chart name

### 5. Validation (`internal/configuration/validator.go`)
Enhanced validation with:
- Added `PackageSourceProviderTypeHelm` to valid provider types
- Added `PackageSourceTypeHelmRepository` to valid source types
- New validation function `validateSourceProviderCombination()` to enforce:
  - `helm-repository` source type requires `helm` provider type
  - `git-*` source types require `github` provider type
  - `docker-image` source type requires `docker` or `harbor` provider type
- Validation that `chartName` is required for `helm-repository` sources
- Validation that `uri` is not required for `helm-repository` sources (uses provider's `baseUrl`)
- Validation that provider must have `baseUrl` configured for `helm-repository` sources

### 6. Orchestrator Integration (`internal/scraper/orchestrator.go`)
- Added Helm provider to `createProviderClient()` switch statement

## Usage Example

```yaml
packageSourceProviders:
  - name: bitnami
    type: helm
    baseUrl: https://charts.bitnami.com/bitnami
    authType: none

packageSources:
  - name: nginx
    provider: bitnami
    type: helm-repository
    chartName: nginx
    versionConstraint: ">=1.0.0"
```

**Note:** The `uri` field is not required for `helm-repository` sources. The implementation uses the provider's `baseUrl` to construct the index.yaml URL automatically.

## Tests

### Helm Repository Tests (`internal/scraper/helm/repository_test.go`)
- Test successful scraping with valid charts
- Test scraping with version limits
- Test scraping different charts from same repository
- Test missing chart name error handling
- Test chart not found in repository error handling
- Test version sorting (semantic, pre-release, mixed)
- Test version conversion from Helm format
- Test index URL construction

### Validation Tests (`internal/configuration/validator_helm_test.go`)
- Test valid Helm provider and helm-repository source combinations
- Test helm-repository without chartName (should fail)
- Test helm-repository without provider baseUrl (should fail)
- Test helm-repository with non-Helm provider (should fail)
- Test Helm provider with incompatible source types (should fail)
- Test Helm provider with various authentication methods
- Test source/provider type combination validation

## Technical Details

### Index.yaml Structure
The implementation parses standard Helm repository index.yaml files:
```yaml
apiVersion: v1
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
```

### Version Sorting
Versions are sorted by semantic version in descending order (newest first):
1. Compare major version
2. Compare minor version
3. Compare patch version
4. Lexicographically compare full version string (for pre-releases)

### Authentication Support
- **None**: No authentication required
- **Basic**: Username and password authentication
- **Token**: Bearer token authentication

## Integration Points

1. **Orchestrator**: Automatically creates Helm provider clients when configured
2. **Validator**: Ensures correct provider/source type combinations
3. **Configuration**: Loads and validates Helm provider and source configurations
4. **Scraping Pipeline**: Fetches versions from Helm repositories during scraping operations

## Error Handling

The implementation handles:
- Missing chart names
- Charts not found in repository
- Failed HTTP requests to index.yaml
- Invalid YAML parsing
- Authentication failures
- Network errors

All errors are propagated with descriptive messages for debugging.