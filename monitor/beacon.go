package monitor

import (
	"context"
	"fmt"
	"io"
	web "net/http"
	"os"
	"strconv"
	"time"

	eth2client "github.com/attestantio/go-eth2-client"
	api "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/attestantio/go-eth2-client/http"
	"github.com/eth-tools/e7mon/config"
	"github.com/fatih/color"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"
)

const (
	SLOTS_PER_EPOCH = 32
)

type BeaconMonitor struct {
	Config *config.BeaconConfig
	Client eth2client.Service
	Logger zerolog.Logger
	Reset  chan bool
}

func NewBeaconMonitor() *BeaconMonitor {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05.000"}
	output.FormatMessage = func(i interface{}) string {
		p := color.New(color.FgMagenta).Add(color.Bold)
		return fmt.Sprintf("| %s | %-50s", p.Sprintf("%-9s", "BEACON"), i)
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

	reset := make(chan bool)
	bm.Reset = reset

	ver, err := bm.GetNodeVersion()
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	bm.Logger.Info().Str("api", bm.Config.API).Str("node_version", ver).Msg("Starting beacon node monitor")
	// For events: no ws necessary, this API uses server streamed events (SSE)
	// https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events
	if provider, isProvider := bm.Client.(eth2client.EventsProvider); isProvider {
		go bm.startBlockTimer()
		err := provider.Events(ctx, []string{"block"}, bm.NewBlockHandler)
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}
	}

	interval, err := time.ParseDuration(bm.Config.Settings.StatsInterval)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	bm.statLoop(interval)
}

var last time.Time = time.Time{}

func (bm BeaconMonitor) NewBlockHandler(event *api.Event) {
	block := event.Data.(*api.BlockEvent)
	var dur time.Duration
	if (last == time.Time{}) {
		dur, _ = time.ParseDuration("0s")
	} else {
		dur = time.Since(last).Round(time.Millisecond)
	}

	bm.Logger.Info().Int("epoch", int(block.Slot/SLOTS_PER_EPOCH)).Str("slot", fmt.Sprint(block.Slot)).Str("last", dur.String()).Msg("New block received")
	bm.Reset <- true
	last = time.Now()
}

func (bm BeaconMonitor) startBlockTimer() {
	start := time.Now()

	lvl1, err := time.ParseDuration(bm.Config.Settings.BlockTimeLevels[0])
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	lvl2, err := time.ParseDuration(bm.Config.Settings.BlockTimeLevels[1])
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	lvl3, err := time.ParseDuration(bm.Config.Settings.BlockTimeLevels[2])
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	var lvls BlockTimeLevels = [3]*BlockTimeLevel{
		{
			Duration: lvl1,
			Hit:      false,
		},
		{
			Duration: lvl2,
			Hit:      false,
		},
		{
			Duration: lvl3,
			Hit:      false,
		},
	}

	for {
		select {
		case <-bm.Reset:
			start = time.Now()
			lvls.Reset()
		default:
			time.Sleep(time.Millisecond * 500)
			new := time.Now()
			if new.After(start.Add(lvls[0].Duration)) && !lvls[0].Hit {
				bm.Logger.Warn().Msgf("%s since last block", lvls[0].Duration)
				lvls[0].Hit = true
			} else if new.After(start.Add(lvls[1].Duration)) && !lvls[1].Hit {
				bm.Logger.Warn().Msgf("%s since last block", lvls[1].Duration)
				lvls[1].Hit = true
			} else if new.After(start.Add(lvls[2].Duration)) && !lvls[2].Hit {
				bm.Logger.Warn().Msgf("%s since last block", lvls[2].Duration)
				lvls[2].Hit = true
			}
		}
	}
}

func (bm BeaconMonitor) statLoop(interval time.Duration) {
	log := bm.Logger
	for {
		connected, connecting, disconnected, disconnecting, err := bm.GetPeerCount()
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}

		if connected < 20 {
			log.Warn().Int("peer_count", connected).Msg("[P2P] Low peer count")
		} else {
			log.Info().Int(
				"connected", connected).Int(
				"connecting", connecting).Int(
				"disconnected", disconnected).Int(
				"disconnecting", disconnecting).Msg("[P2P] Network info")

		}
		time.Sleep(interval)
	}
}

func (bm BeaconMonitor) GetPeerCount() (int, int, int, int, error) {
	res, err := web.Get(bm.Config.API + "/eth/v1/node/peer_count")
	if err != nil {
		bm.Logger.Fatal().Err(err).Msg("")
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	data := gjson.GetManyBytes(body, "data.connected", "data.connecting", "data.disconnected", "data.disconnecting")

	connected, _ := strconv.Atoi(data[0].String())
	connecting, _ := strconv.Atoi(data[1].String())
	disconnected, _ := strconv.Atoi(data[2].String())
	disconnecting, _ := strconv.Atoi(data[3].String())

	return connected, connecting, disconnected, disconnecting, nil
}

// GetNodeVersion returns the node version.
func (bm BeaconMonitor) GetNodeVersion() (string, error) {
	res, err := web.Get(bm.Config.API + "/eth/v1/node/version")
	if err != nil {
		return "", nil
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return gjson.GetBytes(body, "data.version").String(), nil
}

// GetFinalityCheckpoints returns the finality checkpoints as a JSON string.
func (bm BeaconMonitor) GetFinalityCheckpoints() (string, error) {
	res, err := web.Get(bm.Config.API + "/eth/v1/beacon/states/head/finality_checkpoints")
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return gjson.GetBytes(body, "data").String(), nil
}