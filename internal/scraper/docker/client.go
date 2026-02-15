package docker

import (
	"fmt"

	"github.com/mxcd/updater/internal/configuration"
)

type ScrapeOptions struct {
	Limit int
}

type DockerProviderClient struct {
	Options *configuration.PackageSourceProvider
}

func (c *DockerProviderClient) ScrapePackageSource(source *configuration.PackageSource, opts *ScrapeOptions) ([]*configuration.PackageSourceVersion, error) {
	switch source.Type {
	case configuration.PackageSourceTypeDockerImage:
		return scrapeDockerImage(c.Options, source, opts)
	default:
		return nil, fmt.Errorf("unsupported package source type for Docker provider: %s", source.Type)
	}
}
