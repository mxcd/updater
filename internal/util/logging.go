package util

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

func SetCliLoggerDefaults() {
	zerolog.TimeFieldFormat = "2006-01-02T15:04:05.000Z"
	log.Logger = log.Logger.Output(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		NoColor:    false,
		TimeFormat: time.RFC3339,
	}).With().Logger()
}

func SetCliLogLevel(c *cli.Command) {
	if c.Bool("very-verbose") {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	} else if c.Bool("verbose") {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}
