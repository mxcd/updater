package main

import (
	"context"
	"os"

	"github.com/joho/godotenv"
	"github.com/mxcd/updater/internal/actions"
	"github.com/mxcd/updater/internal/util"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

var version = "development"

func main() {

	cli.VersionFlag = &cli.BoolFlag{
		Name:    "version",
		Aliases: []string{},
		Usage:   "print only the version",
	}

	cmd := &cli.Command{
		Name:    "updater",
		Version: version,
		Usage:   "Updater for GitOps resources",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "debug output",
				Sources: cli.EnvVars("UPDATER_VERBOSE"),
			},
			&cli.BoolFlag{
				Name:    "very-verbose",
				Aliases: []string{"vv"},
				Usage:   "trace output",
				Sources: cli.EnvVars("UPDATER_VERY_VERBOSE"),
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			return initCli(ctx, cmd)
		},
		Commands: []*cli.Command{
			{
				Name:  "validate",
				Usage: "Validate configuration",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "config",
						Aliases: []string{"c"},
						Usage:   "Path to configuration file",
						Value:   ".updaterconfig.yml",
						Sources: cli.EnvVars("UPDATER_CONFIG"),
					},
					&cli.StringFlag{
						Name:  "output",
						Usage: "Output format: table, json, yaml, sarif",
						Value: "table",
					},
					&cli.BoolFlag{
						Name:  "probe-providers",
						Usage: "Verify provider connectivity and credentials",
						Value: false,
					},
				},
				Action: validateCommand,
			},
			{
				Name:  "load",
				Usage: "Load configuration and scrape all package sources",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "config",
						Aliases: []string{"c"},
						Usage:   "Path to configuration file",
						Value:   ".updaterconfig.yml",
						Sources: cli.EnvVars("UPDATER_CONFIG"),
					},
					&cli.StringFlag{
						Name:  "output",
						Usage: "Output format: table, json, yaml",
						Value: "table",
					},
					&cli.IntFlag{
						Name:  "limit",
						Usage: "Maximum number of versions to retrieve per source",
						Value: 10,
					},
				},
				Action: loadCommand,
			},
			{
				Name:  "compare",
				Usage: "Compare current versions in targets with latest available versions",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "config",
						Aliases: []string{"c"},
						Usage:   "Path to configuration file",
						Value:   ".updaterconfig.yml",
						Sources: cli.EnvVars("UPDATER_CONFIG"),
					},
					&cli.StringFlag{
						Name:  "output",
						Usage: "Output format: table, json, yaml",
						Value: "table",
					},
					&cli.IntFlag{
						Name:  "limit",
						Usage: "Maximum number of versions to retrieve per source",
						Value: 10,
					},
					&cli.StringFlag{
						Name:  "only",
						Usage: "Only show specific update types: major, minor, patch, all",
						Value: "all",
					},
				},
				Action: compareCommand,
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal().Err(err).Msg("command terminated with error")
	}
}

func initCli(ctx context.Context, cmd *cli.Command) (context.Context, error) {
	godotenv.Load()
	util.SetCliLoggerDefaults()
	util.SetCliLogLevel(cmd)
	log.Trace().Msg("Trace logging enabled")
	log.Debug().Msg("Debug logging enabled")
	log.Info().Msg("Info logging enabled")

	return ctx, nil
}

func validateCommand(ctx context.Context, cmd *cli.Command) error {
	options := &actions.ValidateOptions{
		ConfigPath:     cmd.String("config"),
		OutputFormat:   cmd.String("output"),
		ProbeProviders: cmd.Bool("probe-providers"),
	}

	if err := actions.Validate(options); err != nil {
		return cli.Exit(err.Error(), 3)
	}

	return nil
}

func loadCommand(ctx context.Context, cmd *cli.Command) error {
	options := &actions.LoadOptions{
		ConfigPath:   cmd.String("config"),
		OutputFormat: cmd.String("output"),
		Limit:        cmd.Int("limit"),
	}

	if err := actions.Load(options); err != nil {
		return cli.Exit(err.Error(), 1)
	}

	return nil
}

func compareCommand(ctx context.Context, cmd *cli.Command) error {
	options := &actions.CompareOptions{
		ConfigPath:   cmd.String("config"),
		OutputFormat: cmd.String("output"),
		Limit:        cmd.Int("limit"),
		Only:         cmd.String("only"),
	}

	result, err := actions.Compare(options)
	if err != nil {
		return cli.Exit(err.Error(), 1)
	}

	// Exit with code 1 if there are pending updates (for CI gating)
	if result.HasUpdates {
		return cli.Exit("", 1)
	}

	return nil
}