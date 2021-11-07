package main

import (
	"fmt"
	"os"
	"time"

	"github.com/netbound/e7mon/config"
	"github.com/netbound/e7mon/monitor"

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
			mon.Start()
			return nil
		},
		Commands: []*cli.Command{
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
			{
				Name:    "client-versions",
				Aliases: []string{"cv"},
				Usage:   "prints client versions",
				Action: func(c *cli.Context) error {
					monitor.NewMonitor().PrintVersions()
					return nil
				},
			},
			{
				Name:  "execution",
				Usage: "monitors the execution client (eth1)",
				Action: func(c *cli.Context) error {
					monitor.NewExecutionMonitor().Start()
					return nil
				},
				Subcommands: []*cli.Command{
					{
						Name:  "p2pstat",
						Usage: "prints p2p network statistics",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "interface",
								Aliases: []string{"i"},
								Usage:   "connected interface, will try to find one if none are provided",
							},
						},
						Action: func(c *cli.Context) error {
							// TODO: only works with Geth, Erigon does not have `admin` namespace.
							// Need some way to check if admin namespace is enabled
							mon := monitor.NewExecutionMonitor()
							i := c.String("interface")

							_ = mon
							_ = i
							return nil
						},
					},
				},
			},
			{
				Name:  "beacon",
				Usage: "monitors the beacon node (eth2)",
				Action: func(c *cli.Context) error {
					mon := monitor.NewBeaconMonitor()
					mon.Start()
					return nil
				},
				Subcommands: []*cli.Command{
					{
						Name:  "p2pstat",
						Usage: "prints p2p network statistics",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "interface",
								Aliases: []string{"i"},
								Usage:   "connected interface, will try to find one if none are provided",
							},
						},
						Action: func(c *cli.Context) error {
							// TODO: only works with Geth, Erigon does not have `admin` namespace.
							// Need some way to check if admin namespace is enabled
							mon := monitor.NewBeaconMonitor()
							i := c.String("interface")
							res, err := mon.LatencyScan(i)
							if err != nil {
								log.Fatal().Err(err).Msg("")
							}

							log.Info().Str("high", res.High.String()).Str("low", res.Low.String()).Str("avg", res.Average.String()).Str("response_rate", fmt.Sprintf("%.2f%%", float64(res.Responses)/float64(res.Connected)*100)).Msg("[P2P] Latency scan results")

							return nil
						},
					},
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
}
