package scraper

import (
	"github.com/mxcd/updater/internal/configuration"
	"github.com/mxcd/updater/internal/scraper/docker"
)

type DockerProviderClientAdapter struct {
	client *docker.DockerProviderClient
}

func NewDockerProviderClient(provider *configuration.PackageSourceProvider) ProviderClient {
	return &DockerProviderClientAdapter{
		client: &docker.DockerProviderClient{
			Options: provider,
		},
	}
}

func (a *DockerProviderClientAdapter) NewClient(provider *configuration.PackageSourceProvider) (ProviderClient, error) {
	return NewDockerProviderClient(provider), nil
}

func (a *DockerProviderClientAdapter) ScrapePackageSource(source *configuration.PackageSource, opts *ScrapeOptions) ([]*configuration.PackageSourceVersion, error) {
	dockerOpts := &docker.ScrapeOptions{
		Limit: opts.Limit,
	}
	return a.client.ScrapePackageSource(source, dockerOpts)
}