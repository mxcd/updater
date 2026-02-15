# Updater

A declarative GitOps dependency updater CLI. Updater discovers the latest package versions from multiple sources (GitHub releases/tags, Docker registries, Helm repositories) and applies updates to target files (Helm Chart.yaml, Terraform variables, arbitrary YAML) via Git commits and pull requests with staged rollouts.

## Installation

```bash
go install github.com/mxcd/updater/cmd/updater@latest
```

Or build from source:

```bash
go build -ldflags="-s -w -X 'main.version=dev'" -o updater cmd/updater/main.go
```

## Quick Start

1. Create a configuration file (`.updater/config.yaml` or a single `.updater.yaml`):

```yaml
packageSourceProviders:
  - name: docker-hub
    type: docker

packageSources:
  - name: nginx
    provider: docker-hub
    type: docker-image
    uri: nginx
    tagPattern: "^\\d+\\.\\d+\\.\\d+$"

targets:
  - name: update-nginx
    type: yaml-field
    file: values.yaml
    items:
      - yamlPath: image.tag
        source: nginx

targetActor:
  name: "Updater Bot"
  email: "updater@example.com"
  username: "updater-bot"
  token: "${GITHUB_TOKEN}"
```

2. Validate the configuration:

```bash
updater validate
```

3. Check for available updates:

```bash
updater compare
```

4. Apply updates (creates branches and PRs):

```bash
updater apply
```

## Commands

### `validate`

Validates configuration syntax, field completeness, and cross-references.

```bash
updater validate [--config .updater] [--output table|json|yaml|sarif] [--probe-providers]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--config`, `-c` | Path to configuration file or directory | `.updater` |
| `--output` | Output format | `table` |
| `--probe-providers` | Verify provider connectivity and credentials | `false` |

### `load`

Loads configuration and scrapes all package sources to display available versions.

```bash
updater load [--config .updater] [--output table|json|yaml] [--limit 10]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--config`, `-c` | Path to configuration file or directory | `.updater` |
| `--output` | Output format | `table` |
| `--limit` | Maximum versions to retrieve per source | `10` |

### `compare`

Compares current versions in target files with the latest available versions. Exits with code 1 if updates are available (useful for CI gating).

```bash
updater compare [--config .updater] [--output table|json|yaml] [--limit 10] [--only all|major|minor|patch]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--config`, `-c` | Path to configuration file or directory | `.updater` |
| `--output` | Output format | `table` |
| `--limit` | Maximum versions to retrieve per source | `10` |
| `--only` | Filter by update type | `all` |

### `apply`

Applies updates by creating Git branches, commits, and pull requests.

```bash
updater apply [--config .updater] [--output table|json|yaml] [--dry-run] [--limit 10] [--only all|major|minor|patch]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--config`, `-c` | Path to configuration file or directory | `.updater` |
| `--output` | Output format | `table` |
| `--dry-run`, `-d` | Show what would be done without making changes | `false` |
| `--limit` | Maximum versions to retrieve per source | `10` |
| `--only` | Only apply specific update types | `all` |

### Global Flags

| Flag | Description | Environment Variable |
|------|-------------|---------------------|
| `--verbose`, `-v` | Enable debug output | `UPDATER_VERBOSE` |
| `--very-verbose`, `-vv` | Enable trace output | `UPDATER_VERY_VERBOSE` |
| `--version` | Print version | |

## Configuration

Configuration can be provided as:
- A single YAML file (e.g., `.updater.yaml`)
- A directory containing multiple YAML files (e.g., `.updater/*.yaml`) — they are automatically merged

Environment variable substitution is supported using `${ENV_VAR}` syntax. SOPS-encrypted files are automatically decrypted.

### Package Source Providers

Providers define connection and authentication settings for package registries.

```yaml
packageSourceProviders:
  - name: github
    type: github
    authType: token
    token: "${GITHUB_TOKEN}"

  - name: docker-hub
    type: docker
    # No auth needed for public images

  - name: private-registry
    type: docker
    authType: basic
    username: "${REGISTRY_USER}"
    password: "${REGISTRY_PASS}"

  - name: helm-repo
    type: helm
    baseUrl: "https://charts.example.com"
    authType: basic
    username: "${HELM_USER}"
    password: "${HELM_PASS}"
```

| Field | Description | Required |
|-------|-------------|----------|
| `name` | Unique identifier for the provider | Yes |
| `type` | Provider type: `github`, `docker`, `harbor`, `helm` | Yes |
| `baseUrl` | Base URL (required for `helm` providers, optional for others) | Depends |
| `authType` | Authentication type: `none`, `basic`, `token` | No |
| `username` | Username for basic auth | When `authType: basic` |
| `password` | Password for basic auth | When `authType: basic` |
| `token` | Token for token auth | When `authType: token` |

### Package Sources

Sources define what packages to track and how to discover versions.

#### GitHub Release

Fetches the latest release from a GitHub repository.

```yaml
packageSources:
  - name: my-tool
    provider: github
    type: git-release
    uri: https://github.com/owner/repo
```

#### GitHub Tag

Fetches tags from a GitHub repository with filtering and sorting.

```yaml
packageSources:
  - name: my-tool-tags
    provider: github
    type: git-tag
    uri: https://github.com/owner/repo
    tagPattern: "^v\\d+\\.\\d+\\.\\d+$"
    excludePattern: "rc|beta|alpha"
    sortBy: semantic
```

#### GitHub Helm Chart

Fetches a Helm chart version from a GitHub repository.

```yaml
packageSources:
  - name: my-helm-chart
    provider: github
    type: git-helm-chart
    uri: https://github.com/owner/repo
    branch: main
    path: charts/my-chart
```

#### Docker Image

Fetches tags from a Docker registry (Docker Hub, GCR, ECR, private registries).

```yaml
packageSources:
  - name: nginx
    provider: docker-hub
    type: docker-image
    uri: nginx
    tagPattern: "^\\d+\\.\\d+\\.\\d+$"
    excludePattern: "alpine|slim"
    sortBy: semantic
    tagLimit: 100
```

Supported URI formats:
- `nginx` — Docker Hub official image
- `myorg/myapp` — Docker Hub with namespace
- `gcr.io/myproject/myapp` — Google Container Registry
- `registry.example.com:5000/myorg/myapp` — Private registry with port

#### Helm Chart

Fetches a chart version from a Helm repository.

```yaml
packageSources:
  - name: ingress-nginx
    provider: helm-repo
    type: helm-chart
    chartName: ingress-nginx
    versionConstraint: ">=4.0.0"
```

#### Common Source Fields

| Field | Description | Applies To |
|-------|-------------|-----------|
| `name` | Unique identifier | All |
| `provider` | References a provider by name | All |
| `type` | Source type (see above) | All |
| `uri` | Repository or registry URI | All except `helm-chart` |
| `branch` | Git branch | `git-helm-chart` |
| `path` | File path in repository | `git-helm-chart` |
| `chartName` | Chart name in Helm repo | `helm-chart` |
| `versionConstraint` | SemVer constraint for filtering | All |
| `tagPattern` | Regex to match desired tags | `git-tag`, `docker-image` |
| `excludePattern` | Regex to exclude unwanted tags | `git-tag`, `docker-image`, `helm-chart` |
| `tagLimit` | Max tags to fetch before filtering | `docker-image` |
| `sortBy` | Sort order: `semantic`, `date`, `alphabetical` | `git-tag`, `docker-image` |

### Targets

Targets define which files to update and how to locate version values within them.

#### Subchart (`subchart`)

Updates Helm subchart dependency versions in `Chart.yaml` files.

```yaml
targets:
  - name: helm-deps
    type: subchart
    file: Chart.yaml
    items:
      - subchartName: postgresql
        source: bitnami-postgresql
      - subchartName: redis
        source: bitnami-redis
```

| Item Field | Description | Required |
|-----------|-------------|----------|
| `subchartName` | Name of the dependency in `Chart.yaml` | Yes |
| `source` | References a package source by name | Yes |

The file must be named `Chart.yaml` or `Chart.yml`.

#### Terraform Variable (`terraform-variable`)

Updates default values of Terraform variables in `.tf` or `.tfvars` files.

```yaml
targets:
  - name: infra-versions
    type: terraform-variable
    file: versions.tf
    items:
      - terraformVariableName: nginx_version
        source: nginx
      - terraformVariableName: redis_version
        source: redis
```

| Item Field | Description | Required |
|-----------|-------------|----------|
| `terraformVariableName` | Name of the Terraform variable | Yes |
| `source` | References a package source by name | Yes |

Matches the pattern `variable "name" { default = "value" }` in both single-line and multi-line forms.

#### YAML Field (`yaml-field`)

Updates any scalar value in an arbitrary YAML file using a dot-notation path. This is the most flexible target type — it works with Helm `values.yaml`, Kubernetes manifests, or any YAML document.

```yaml
targets:
  - name: app-images
    type: yaml-field
    file: values.yaml
    items:
      - yamlPath: image.tag
        source: my-app-image
      - yamlPath: sidecar.image.tag
        source: sidecar-image
```

| Item Field | Description | Required |
|-----------|-------------|----------|
| `yamlPath` | Dot-notation path to the YAML field | Yes |
| `source` | References a package source by name | Yes |

**Path syntax:**
- Dot-separated keys navigate nested mappings: `image.tag` navigates to `image:` then `tag:`
- Numeric segments index into arrays: `containers.0.image` accesses the first element of the `containers` list

**Examples:**

Update an image tag in `values.yaml`:
```yaml
# values.yaml
image:
  repository: nginx
  tag: "1.25.0"    # ← updated by yamlPath: image.tag
```

Update a container image in a Kubernetes manifest:
```yaml
# deployment.yaml
spec:
  template:
    spec:
      containers:
        - name: my-app
          image: "myregistry.io/myapp:v1.2.3"  # ← updated by yamlPath: spec.template.spec.containers.0.image
```

**Formatting preservation:** Comments, blank lines, indentation, and quoting style (double-quoted, single-quoted, unquoted) are all preserved when updating values.

The file must have a `.yaml` or `.yml` extension.

#### Common Target Fields

| Field | Description | Required |
|-------|-------------|----------|
| `name` | Display name for the target | Yes |
| `type` | Target type: `subchart`, `terraform-variable`, `yaml-field` | Yes |
| `file` | Path to the target file (supports wildcards `*` and `**`) | Yes |
| `items` | List of update items | Yes (at least one) |
| `patchGroup` | Group name for batching updates into a single PR | No |
| `labels` | Labels to apply to the PR | No |

#### Common Item Fields

| Field | Description | Required |
|-------|-------------|----------|
| `source` | References a package source by name | Yes |
| `name` | Custom display name for this item | No |
| `patchGroup` | Override the target's patch group | No |
| `labels` | Additional labels (merged with target labels) | No |

### Target Actor

The target actor configures Git commit author and GitHub credentials for creating PRs.

```yaml
targetActor:
  name: "Updater Bot"
  email: "updater@example.com"
  username: "updater-bot"
  token: "${GITHUB_TOKEN}"
```

| Field | Description | Required |
|-------|-------------|----------|
| `name` | Git commit author name | Yes |
| `email` | Git commit author email | Yes |
| `username` | GitHub username for push and PR creation | Yes |
| `token` | GitHub personal access token | No (required for `apply`) |

## Patch Groups and Staged Rollouts

Updates can be grouped into patch groups. Each patch group gets its own branch and PR, enabling staged rollouts.

```yaml
targets:
  - name: staging-images
    type: yaml-field
    file: staging/values.yaml
    patchGroup: staging
    items:
      - yamlPath: image.tag
        source: my-app

  - name: production-images
    type: yaml-field
    file: production/values.yaml
    patchGroup: production
    items:
      - yamlPath: image.tag
        source: my-app
```

This creates separate PRs:
- Branch `chore/update/staging` with a PR for staging updates
- Branch `chore/update/production` with a PR for production updates

If no `patchGroup` is specified, updates are grouped under `default`.

Item-level `patchGroup` overrides the target-level setting:

```yaml
targets:
  - name: all-images
    type: yaml-field
    file: values.yaml
    patchGroup: minor-updates
    items:
      - yamlPath: image.tag
        source: my-app
        patchGroup: critical  # This item goes into a separate "critical" PR
```

## Wildcard Targets

Target file paths support glob wildcards to match multiple files:

```yaml
targets:
  - name: all-charts
    type: subchart
    file: "services/*/Chart.yaml"
    items:
      - subchartName: common-lib
        source: common-lib-chart
```

Recursive matching with `**`:

```yaml
targets:
  - name: all-values
    type: yaml-field
    file: "**/values.yaml"
    items:
      - yamlPath: image.tag
        source: my-app
```

When using wildcards, validation is permissive — it does not require every matched file to contain the specified path or dependency. Only files that actually contain the target field are updated.

## PR Reconciliation

Updater automatically detects and updates existing PRs. If a branch `chore/update/<patchGroup>` already exists with an open PR, the PR title and body are updated rather than creating a duplicate. This makes updater safe to run repeatedly (e.g., in a cron job).

## Full Configuration Example

```yaml
packageSourceProviders:
  - name: github
    type: github
    authType: token
    token: "${GITHUB_TOKEN}"

  - name: docker-hub
    type: docker

  - name: private-registry
    type: docker
    authType: basic
    username: "${REGISTRY_USER}"
    password: "${REGISTRY_PASS}"

  - name: helm-repo
    type: helm
    baseUrl: "https://charts.bitnami.com/bitnami"

packageSources:
  - name: nginx-image
    provider: docker-hub
    type: docker-image
    uri: nginx
    tagPattern: "^\\d+\\.\\d+\\.\\d+$"
    excludePattern: "alpine|slim|perl"
    sortBy: semantic

  - name: redis-chart
    provider: helm-repo
    type: helm-chart
    chartName: redis

  - name: my-app
    provider: private-registry
    type: docker-image
    uri: registry.example.com/myorg/myapp
    tagPattern: "^v\\d+\\.\\d+\\.\\d+$"
    sortBy: semantic

  - name: terraform-aws
    provider: github
    type: git-release
    uri: https://github.com/hashicorp/terraform-provider-aws

targets:
  # Update Helm subchart dependencies
  - name: helm-dependencies
    type: subchart
    file: Chart.yaml
    patchGroup: helm-deps
    items:
      - subchartName: redis
        source: redis-chart

  # Update image tags in values.yaml
  - name: app-images
    type: yaml-field
    file: values.yaml
    patchGroup: images
    labels:
      - dependencies
      - docker
    items:
      - yamlPath: image.tag
        source: my-app
      - yamlPath: nginx.image.tag
        source: nginx-image

  # Update Terraform provider versions
  - name: terraform-providers
    type: terraform-variable
    file: versions.tf
    patchGroup: terraform
    items:
      - terraformVariableName: aws_provider_version
        source: terraform-aws

targetActor:
  name: "Dependency Bot"
  email: "deps@example.com"
  username: "dep-bot"
  token: "${GITHUB_TOKEN}"
```

## CI/CD Integration

### GitHub Actions

```yaml
name: Update Dependencies
on:
  schedule:
    - cron: '0 6 * * 1'  # Weekly on Monday at 6am
  workflow_dispatch:

jobs:
  update:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - run: go install github.com/mxcd/updater/cmd/updater@latest

      - run: updater apply
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### Check for Updates (CI Gate)

The `compare` command exits with code 1 if updates are available:

```yaml
- run: updater compare --only patch
  continue-on-error: true
  id: check

- run: echo "Patch updates available!"
  if: steps.check.outcome == 'failure'
```

## Development

```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./internal/target/...
go test ./internal/configuration/...

# Build
go build -o updater cmd/updater/main.go

# Install to $GOBIN
go install ./cmd/updater

# Format
go fmt ./...
```

## License

See [LICENSE](LICENSE) for details.
