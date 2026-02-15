package github

import (
	"fmt"

	"github.com/mxcd/updater/internal/configuration"
)

type ScrapeOptions struct {
	Limit int
}

type GitHubProviderClient struct {
	Options *configuration.PackageSourceProvider
}

func (c *GitHubProviderClient) ScrapePackageSource(source *configuration.PackageSource, opts *ScrapeOptions) ([]*configuration.PackageSourceVersion, error) {
	switch source.Type {
	case configuration.PackageSourceTypeGitRelease:
		return scrapeRelease(c.Options, source, opts)
	case configuration.PackageSourceTypeGitTag:
		return scrapeTag(c.Options, source, opts)
	case configuration.PackageSourceTypeGitHelmChart:
		return scrapeHelmChart(c.Options, source, opts)
	default:
		return nil, fmt.Errorf("unsupported package source type for GitHub provider: %s", source.Type)
	}
}
