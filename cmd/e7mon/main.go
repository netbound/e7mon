package main

import (
	"fmt"
	"os"
	"time"

	"github.com/jonasbostoen/e7mon/config"
	"github.com/jonasbostoen/e7mon/core/monitor"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func main() {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05.000"}
	output.FormatMessage = func(i interface{}) string {
		return fmt.Sprintf("%s", i)
	}

	log := log.Output(output)

	app := &cli.App{
		Name:  "e7mon",
		Usage: "Monitors your Ethereum clients",
		Action: func(c *cli.Context) error {
			mon := monitor.NewMonitor()
			log.Info().Str(
				"execution_api", mon.Config.ExecutionConfig.API).Str(
				"consensus_api", mon.Config.ConsensusConfig.API).Msg("Starting")
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:  "execution",
				Usage: "monitors the execution client (eth1)",
				Action: func(c *cli.Context) error {
					mon := monitor.NewExecutionMonitor()
					mon.Start()
					return nil
				},
			},
			{
				Name:  "consensus",
				Usage: "monitors the consensus client (eth2)",
			},
			{
				Name:  "init",
				Usage: "initializes configs",
				Action: func(c *cli.Context) error {
					path, err := config.InitializeConfig()
					if err != nil {
						log.Info().Str("path", path).Msg("Config file already exists. e7mon is ready to go.")
					} else {
						log.Info().Str("path", path).Msg("Config file created. Ready to go.")
					}

					return nil
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
}
