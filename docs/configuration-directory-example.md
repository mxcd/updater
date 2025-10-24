# Configuration Directory Example

The updater now supports loading configuration from a directory containing multiple YAML files. This enables modular configuration where different aspects can be organized in separate files.

## Directory Structure

Instead of a single configuration file, you can use a directory like `.updater/`:

```
.updater/
├── providers.yml
├── sources.yml
└── targets.yml
```

## Example Configuration Files

### providers.yml
```yaml
packageSourceProviders:
  - name: github
    type: github
    authType: token
    token: ${GITHUB_TOKEN}
  
  - name: docker-hub
    type: docker
    authType: none
```

### sources.yml
```yaml
packageSources:
  - name: traefik-helm
    provider: github
    type: git-helm-chart
    uri: https://raw.githubusercontent.com/traefik/traefik-helm-chart/refs/heads/master/traefik/Chart.yaml
    versionConstraint: ">=10.0.0"
  
  - name: nginx-image
    provider: docker-hub
    type: docker-image
    uri: docker.io/library/nginx
    tagPattern: "^\\d+\\.\\d+\\.\\d+$"
```

### targets.yml
```yaml
targets:
  - name: production-traefik
    type: subchart
    file: environments/production/Chart.yaml
    items:
      - subchartName: traefik
        source: traefik-helm
  
  - name: staging-nginx
    type: terraform-variable
    file: environments/staging/variables.tf
    items:
      - terraformVariableName: nginx_version
        source: nginx-image

targetActor:
  name: Updater Bot
  email: updater@example.com
  username: updater-bot
  token: ${GITHUB_TOKEN}
```

## Usage

### With Directory
```bash
# Load configuration from .updater directory
updater validate --config .updater

# Or set it as default
export UPDATER_CONFIG=.updater
updater load
updater compare
updater apply --yes
```

### With Single File (still supported)
```bash
updater validate --config .updaterconfig.yml
```

## Configuration Merging Rules

When loading from a directory:

1. **All `.yml` and `.yaml` files** in the directory are loaded (subdirectories are not traversed)
2. **Files are merged** - all providers, sources, and targets are combined
3. **Duplicate names are detected** - an error is raised if:
   - Multiple files define the same provider name
   - Multiple files define the same source name
   - Multiple files define the same target name
4. **TargetActor** - if defined in multiple files, the last one encountered is used

## Benefits

- **Modularity**: Separate concerns into different files (providers, sources, targets)
- **Team collaboration**: Different team members can manage different files
- **Environment-specific**: Easy to override specific parts by adding/removing files
- **Version control**: Better diff visibility when changes affect only specific files
- **Organization**: Clearer structure for large configurations with many sources

## Migration from Single File

To migrate from a single configuration file to a directory:

1. Create the directory: `mkdir .updater`
2. Split your configuration into logical files:
   - Move `packageSourceProviders` to `providers.yml`
   - Move `packageSources` to `sources.yml`
   - Move `targets` and `targetActor` to `targets.yml`
3. Update your command to use the directory: `--config .updater`

## Validation

The same validation rules apply whether using a single file or directory:
- All referenced providers must be defined
- All source references in targets must exist
- Required fields must be present
- No duplicate names are allowed (enforced during merge)