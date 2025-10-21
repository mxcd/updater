package docker

import (
	"github.com/mxcd/updater/internal/configuration"
	"github.com/rs/zerolog/log"
)

type ScrapeOptions struct {
	Limit int
}

type DockerProviderClient struct {
	Options *configuration.PackageSourceProvider
}

type DockerProviderOptions struct {
	Config *configuration.PackageSourceProvider
}

func (c *DockerProviderClient) ScrapePackageSource(source *configuration.PackageSource, opts *ScrapeOptions) ([]*configuration.PackageSourceVersion, error) {
	switch source.Type {
	case configuration.PackageSourceTypeDockerImage:
		return scrapeDockerImage(c.Options, source, opts)
	default:
		log.Fatal().Str("type", string(source.Type)).Msg("unsupported package source type for Docker provider")
		return nil, nil
	}
}