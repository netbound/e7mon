package monitor

import (
	"context"
	"fmt"
	"os"
	"time"

	eth2client "github.com/attestantio/go-eth2-client"
	api "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/attestantio/go-eth2-client/http"
	"github.com/fatih/color"
	"github.com/jonasbostoen/e7mon/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type BeaconMonitor struct {
	Config *config.BeaconConfig
	Client eth2client.Service
	Logger zerolog.Logger
}

func NewBeaconMonitor() *BeaconMonitor {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05.000"}
	output.FormatMessage = func(i interface{}) string {
		p := color.New(color.FgMagenta).Add(color.Bold)
		return fmt.Sprintf("[%s] %-50s", p.Sprintf("%-9s", " BEACON"), i)
	}

	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := http.New(ctx,
		http.WithAddress(cfg.BeaconConfig.API),
		http.WithLogLevel(zerolog.WarnLevel),
	)

	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	return &BeaconMonitor{
		Config: cfg.BeaconConfig,
		Logger: log.Output(output),
		Client: client,
	}
}

func (bm BeaconMonitor) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bm.Logger.Info().Str("api", bm.Config.API).Msg("Starting beacon node monitor")
	// For events: no ws necessary, this API uses server streamed events (SSE)
	// https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events
	if provider, isProvider := bm.Client.(eth2client.EventsProvider); isProvider {
		err := provider.Events(ctx, []string{"block"}, bm.NewBlockHandler)
		if err != nil {
			log.Fatal().Msg(err.Error())
		}
	}

	select {}
}

func (bm BeaconMonitor) NewBlockHandler(block *api.Event) {
	bm.Logger.Info().Str("slot", fmt.Sprint(block.Data.(*api.BlockEvent).Slot)).Msg("New block received")
}
