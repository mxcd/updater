package scraper

import "github.com/mxcd/updater/internal/configuration"

type ScrapeOptions struct {
	Limit int
}

type ProviderClient interface {
	ScrapePackageSource(*configuration.PackageSource, *ScrapeOptions) ([]*configuration.PackageSourceVersion, error)
}
