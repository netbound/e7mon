package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	web "net/http"
	"os"
	"strconv"
	"time"

	eth2client "github.com/attestantio/go-eth2-client"
	api "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/attestantio/go-eth2-client/http"
	"github.com/fatih/color"
	"github.com/jonasbostoen/e7mon/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"
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

	interval, err := time.ParseDuration(bm.Config.Settings.StatsInterval)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	bm.statLoop(interval)
}

func (bm BeaconMonitor) NewBlockHandler(block *api.Event) {
	bm.Logger.Info().Str("slot", fmt.Sprint(block.Data.(*api.BlockEvent).Slot)).Msg("New block received")
}

func (bm BeaconMonitor) statLoop(interval time.Duration) {
	log := bm.Logger
	for {
		connected, connecting, disconnected, disconnecting, err := bm.GetPeerCount()
		if err != nil {
			log.Fatal().Msg(err.Error())
		}

		if connected < 20 {
			log.Warn().Str("peer_count", fmt.Sprint(connected)).Msg("[P2P] Low peer count")
		} else {
			log.Info().Str(
				"connected", fmt.Sprint(connected)).Str(
				"connecting", fmt.Sprint(connecting)).Str(
				"disconnected", fmt.Sprint(disconnected)).Str(
				"disconnecting", fmt.Sprint(disconnecting)).Msg("[P2P] Network info")

		}
		time.Sleep(interval)
	}
}

type PeerCountResponse struct {
	Connected     string `json:"connected"`
	Connecting    string `json:"connecting"`
	Disconnected  string `json:"disconnected"`
	Disconnecting string `json:"disconnecting"`
}

var pcResponse PeerCountResponse

func (bm BeaconMonitor) GetPeerCount() (int, int, int, int, error) {
	req, err := web.NewRequest("GET", bm.Config.API+"/eth/v1/node/peer_count", nil)
	if err != nil {
		bm.Logger.Fatal().Msg(err.Error())
	}
	req.Header.Set("Accept", "application/json")
	res, err := web.DefaultClient.Do(req)
	if err != nil {
		bm.Logger.Fatal().Msg(err.Error())
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	data := gjson.GetBytes(body, "data")
	err = json.Unmarshal([]byte(data.String()), &pcResponse)
	if err != nil {
		bm.Logger.Fatal().Msg(err.Error())
	}

	connected, _ := strconv.Atoi(pcResponse.Connected)
	connecting, _ := strconv.Atoi(pcResponse.Connecting)
	disconnected, _ := strconv.Atoi(pcResponse.Disconnected)
	disconnecting, _ := strconv.Atoi(pcResponse.Disconnecting)

	return connected, connecting, disconnected, disconnecting, nil
}
