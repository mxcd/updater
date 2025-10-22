package scraper

import (
	"github.com/mxcd/updater/internal/configuration"
	"github.com/mxcd/updater/internal/scraper/helm"
)

type HelmProviderClientAdapter struct {
	client *helm.HelmProviderClient
}

func NewHelmProviderClient(provider *configuration.PackageSourceProvider) ProviderClient {
	return &HelmProviderClientAdapter{
		client: &helm.HelmProviderClient{
			Options: provider,
		},
	}
}

func (a *HelmProviderClientAdapter) NewClient(provider *configuration.PackageSourceProvider) (ProviderClient, error) {
	return NewHelmProviderClient(provider), nil
}

func (a *HelmProviderClientAdapter) ScrapePackageSource(source *configuration.PackageSource, opts *ScrapeOptions) ([]*configuration.PackageSourceVersion, error) {
	helmOpts := &helm.ScrapeOptions{
		Limit: opts.Limit,
	}
	return a.client.ScrapePackageSource(source, helmOpts)
}