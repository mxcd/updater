package scraper

import (
	"github.com/mxcd/updater/internal/configuration"
	"github.com/mxcd/updater/internal/scraper/github"
)

type GitHubProviderClientAdapter struct {
	client *github.GitHubProviderClient
}

func NewGitHubProviderClient(provider *configuration.PackageSourceProvider) ProviderClient {
	return &GitHubProviderClientAdapter{
		client: &github.GitHubProviderClient{
			Options: provider,
		},
	}
}

func (a *GitHubProviderClientAdapter) NewClient(provider *configuration.PackageSourceProvider) (ProviderClient, error) {
	return NewGitHubProviderClient(provider), nil
}

func (a *GitHubProviderClientAdapter) ScrapePackageSource(source *configuration.PackageSource, opts *ScrapeOptions) ([]*configuration.PackageSourceVersion, error) {
	githubOpts := &github.ScrapeOptions{
		Limit: opts.Limit,
	}
	return a.client.ScrapePackageSource(source, githubOpts)
}