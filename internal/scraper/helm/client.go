package helm

import (
	"fmt"

	"github.com/mxcd/updater/internal/configuration"
)

type ScrapeOptions struct {
	Limit int
}

type HelmProviderClient struct {
	Options *configuration.PackageSourceProvider
}

func (c *HelmProviderClient) ScrapePackageSource(source *configuration.PackageSource, opts *ScrapeOptions) ([]*configuration.PackageSourceVersion, error) {
	switch source.Type {
	case configuration.PackageSourceTypeHelmRepository:
		return scrapeHelmRepository(c.Options, source, opts)
	default:
		return nil, fmt.Errorf("unsupported package source type for Helm provider: %s", source.Type)
	}
}
