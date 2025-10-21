package scraper

import (
	"fmt"

	"github.com/mxcd/updater/internal/configuration"
	"github.com/rs/zerolog/log"
)

type Orchestrator struct {
	config          *configuration.Config
	providerClients map[string]ProviderClient
}

func NewOrchestrator(config *configuration.Config) (*Orchestrator, error) {
	orchestrator := &Orchestrator{
		config:          config,
		providerClients: make(map[string]ProviderClient),
	}

	// Initialize provider clients
	for _, provider := range config.PackageSourceProviders {
		client, err := orchestrator.createProviderClient(provider)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider client for %s: %w", provider.Name, err)
		}
		orchestrator.providerClients[provider.Name] = client
	}

	return orchestrator, nil
}

func (orchestrator *Orchestrator) createProviderClient(provider *configuration.PackageSourceProvider) (ProviderClient, error) {
	switch provider.Type {
	case configuration.PackageSourceProviderTypeGitHub:
		return NewGitHubProviderClient(provider), nil
	case configuration.PackageSourceProviderTypeDocker:
		return NewDockerProviderClient(provider), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", provider.Type)
	}
}

func (orchestrator *Orchestrator) ScrapeAllSources(options *ScrapeOptions) error {
	log.Info().Int("count", len(orchestrator.config.PackageSources)).Msg("Starting to scrape all package sources")

	for _, source := range orchestrator.config.PackageSources {
		if err := orchestrator.scrapeSource(source, options); err != nil {
			log.Error().
				Err(err).
				Str("source", source.Name).
				Str("provider", source.Provider).
				Msg("Failed to scrape package source")
			return fmt.Errorf("failed to scrape source %s: %w", source.Name, err)
		}
	}

	log.Info().Msg("Successfully scraped all package sources")
	return nil
}

func (orchestrator *Orchestrator) scrapeSource(source *configuration.PackageSource, options *ScrapeOptions) error {
	log.Info().
		Str("source", source.Name).
		Str("provider", source.Provider).
		Str("type", string(source.Type)).
		Str("uri", source.URI).
		Msg("Scraping package source")

	// Get the provider client
	client, exists := orchestrator.providerClients[source.Provider]
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

	log.Info().
		Str("source", source.Name).
		Int("versions", len(versions)).
		Msg("Successfully scraped package source")

	return nil
}

func (orchestrator *Orchestrator) GetConfig() *configuration.Config {
	return orchestrator.config
}