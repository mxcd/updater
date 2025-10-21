package github

import (
	"github.com/mxcd/updater/internal/configuration"
	"github.com/rs/zerolog/log"
)

type ScrapeOptions struct {
	Limit int
}

type GitHubProviderClient struct {
	Options *configuration.PackageSourceProvider
}

type GitHubProviderOptions struct {
	Config *configuration.PackageSourceProvider
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
		log.Fatal().Str("type", string(source.Type)).Msg("unsupported package source type for GitHub provider")
		return nil, nil
	}
}