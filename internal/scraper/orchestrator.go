package scraper

import (
	"fmt"

	"github.com/mxcd/updater/internal/configuration"
	"github.com/rs/zerolog/log"

	"github.com/schollz/progressbar/v3"
)

// ScrapeError records a scraping failure for a single source
type ScrapeError struct {
	SourceName string
	Provider   string
	Err        error
}

func (e *ScrapeError) Error() string {
	return fmt.Sprintf("source %s (provider %s): %v", e.SourceName, e.Provider, e.Err)
}

// ScrapeResult holds the outcome of a ScrapeAllSources call
type ScrapeResult struct {
	Succeeded int
	Failed    int
	Errors    []*ScrapeError
}

// HasErrors returns true if any sources failed to scrape
func (r *ScrapeResult) HasErrors() bool {
	return len(r.Errors) > 0
}

type Orchestrator struct {
	config          *configuration.Config
	providerClients map[string]ProviderClient
}

func NewOrchestrator(config *configuration.Config) (*Orchestrator, error) {
	o := &Orchestrator{
		config:          config,
		providerClients: make(map[string]ProviderClient),
	}

	for _, provider := range config.PackageSourceProviders {
		client, err := o.createProviderClient(provider)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider client for %s: %w", provider.Name, err)
		}
		o.providerClients[provider.Name] = client
	}

	return o, nil
}

func (o *Orchestrator) createProviderClient(provider *configuration.PackageSourceProvider) (ProviderClient, error) {
	switch provider.Type {
	case configuration.PackageSourceProviderTypeGitHub:
		return NewGitHubProviderClient(provider), nil
	case configuration.PackageSourceProviderTypeDocker:
		return NewDockerProviderClient(provider), nil
	case configuration.PackageSourceProviderTypeHelm:
		return NewHelmProviderClient(provider), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", provider.Type)
	}
}

func (o *Orchestrator) ScrapeAllSources(options *ScrapeOptions) *ScrapeResult {
	log.Debug().Int("count", len(o.config.PackageSources)).Msg("Starting to scrape all package sources")

	bar := progressbar.NewOptions(len(o.config.PackageSources),
		progressbar.OptionSetDescription("Scraping package sources:"),
		progressbar.OptionSetItsString("pkg"),
		progressbar.OptionShowIts(),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	result := &ScrapeResult{}

	for _, source := range o.config.PackageSources {
		bar.Add(1)
		if err := o.scrapeSource(source, options); err != nil {
			log.Error().
				Err(err).
				Str("source", source.Name).
				Str("provider", source.Provider).
				Msg("Failed to scrape package source")
			result.Failed++
			result.Errors = append(result.Errors, &ScrapeError{
				SourceName: source.Name,
				Provider:   source.Provider,
				Err:        err,
			})
		} else {
			result.Succeeded++
		}
	}

	bar.Finish()
	fmt.Printf("\n")

	if result.HasErrors() {
		log.Warn().
			Int("succeeded", result.Succeeded).
			Int("failed", result.Failed).
			Msg("Scraped package sources with errors")
	} else {
		log.Debug().Msg("Successfully scraped all package sources")
	}
	return result
}

func (o *Orchestrator) scrapeSource(source *configuration.PackageSource, options *ScrapeOptions) error {
	log.Debug().
		Str("source", source.Name).
		Str("provider", source.Provider).
		Str("type", string(source.Type)).
		Str("uri", source.URI).
		Msg("Scraping package source")

	// Get the provider client
	client, exists := o.providerClients[source.Provider]
	if !exists {
		return fmt.Errorf("provider %s not found", source.Provider)
	}

	// Scrape the package source
	versions, err := client.ScrapePackageSource(source, options)
	if err != nil {
		return fmt.Errorf("failed to scrape package source: %w", err)
	}

	// Store versions in the source
	source.Versions = versions

	log.Debug().
		Str("source", source.Name).
		Int("versions", len(versions)).
		Msg("Successfully scraped package source")

	return nil
}

func (o *Orchestrator) GetConfig() *configuration.Config {
	return o.config
}
