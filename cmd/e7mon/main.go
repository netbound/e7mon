package main

import (
	"os"

	"github.com/jonasbostoen/e7mon/core/monitor"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func main() {

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
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
}
