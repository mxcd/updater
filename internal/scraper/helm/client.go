package helm

import (
	"github.com/mxcd/updater/internal/configuration"
	"github.com/rs/zerolog/log"
)

type ScrapeOptions struct {
	Limit int
}

type HelmProviderClient struct {
	Options *configuration.PackageSourceProvider
}

type HelmProviderOptions struct {
	Config *configuration.PackageSourceProvider
}

func (c *HelmProviderClient) ScrapePackageSource(source *configuration.PackageSource, opts *ScrapeOptions) ([]*configuration.PackageSourceVersion, error) {
	switch source.Type {
	case configuration.PackageSourceTypeHelmRepository:
		return scrapeHelmRepository(c.Options, source, opts)
	default:
		log.Fatal().Str("type", string(source.Type)).Msg("unsupported package source type for Helm provider")
		return nil, nil
	}
}