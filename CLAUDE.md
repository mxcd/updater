# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Updater is a declarative GitOps dependency updater CLI written in Go. It discovers latest package versions from multiple sources (GitHub releases/tags, Docker registries, Helm repositories) and applies updates to target files (Helm Chart.yaml, Terraform variables) via Git commits and PRs with staged rollouts.

## Build & Development Commands

```bash
# Install binary to $GOBIN
just install
# or directly:
go install ./cmd/updater

# Run all tests
go test ./...

# Run tests for a specific package
go test ./internal/configuration/...
go test ./internal/target/...
go test ./internal/scraper/docker/...

# Build with version info
go build -ldflags="-s -w -X 'main.version=dev'" -o updater cmd/updater/main.go

# Format code
go fmt ./...
```

## Architecture

The codebase follows a layered architecture:

1. **CLI Layer** (`cmd/updater/main.go`): urfave/cli/v3 command definitions with shared flags. Five commands: `validate`, `load`, `compare`, `apply`, and a hidden `version`.

2. **Actions Layer** (`internal/actions/`): Each CLI command maps to an action function. The `apply` action is split across multiple files handling execution (`apply_executor.go`), PR creation (`apply_pr.go`), and Git operations.

3. **Configuration Layer** (`internal/configuration/`): YAML config loading from single file or `.updater` directory. Supports `${ENV_VAR}` substitution and SOPS-encrypted secrets. Key types are in `types.go`.

4. **Scraper Layer** (`internal/scraper/`): Provider-based architecture with a `ProviderClient` interface (`provider.go`) and an orchestrator that routes to implementations in `docker/`, `github/`, and `helm/` subdirectories. Source types: `git-release`, `git-tag`, `git-helm-chart`, `docker-image`, `helm-chart`.

5. **Target Layer** (`internal/target/`): Mutators that modify version references in files. Currently supports `subchart` (Helm Chart.yaml dependencies) and `terraform-variable` (.tf files).

6. **Git Layer** (`internal/git/`): Repository cloning, branch management, committing, and PR creation/reconciliation.

## Key Design Patterns

- Configuration can be a single YAML file or a directory of YAML files (loaded and merged by `loader.go`)
- Provider credentials are configured once at the provider level and referenced by sources
- Updates flow through stages with ordered rollout and patch grouping
- PR reconciliation prevents duplicate PRs by updating existing ones
- The `compare` action classifies updates as major/minor/patch using semver
