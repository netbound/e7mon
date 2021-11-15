package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	web "net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/netbound/e7mon/config"
	"github.com/netbound/e7mon/net"

	eth2client "github.com/attestantio/go-eth2-client"
	api "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/attestantio/go-eth2-client/http"
	"github.com/fatih/color"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"
)

const (
	SLOTS_PER_EPOCH = 32
)

type BeaconMonitor struct {
	Config        *config.BeaconConfig
	Client        eth2client.Service
	Stats         []config.Stat
	Logger        zerolog.Logger
	InterfaceName string
	Reset         chan bool
	Scanner       *net.Scanner
}

func NewBeaconMonitor() *BeaconMonitor {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
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
		http.WithLogLevel(zerolog.TraceLevel),
	)

	logger := log.Output(output)

	if err != nil {
		logger.Fatal().Err(err).Msg("Can't connect to JSON-RPC API, is the endpoint correct and running?")
	}

	return &BeaconMonitor{
		Config:        cfg.BeaconConfig,
		Logger:        log.Output(output),
		Stats:         cfg.StatsConfig,
		InterfaceName: cfg.NetConfig.Interface,
		Client:        client,
	}
}

func (bm BeaconMonitor) Start() {
	log := bm.Logger

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reset := make(chan bool)
	bm.Reset = reset

	ver, err := bm.NodeVersion()
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	log.Info().Str("api", bm.Config.API).Str("node_version", ver).Msg("Starting beacon node monitor")

	bm.subscribeToEvents(ctx, []string{"block"}, bm.EventHandler)

	topics, err := parseTopics(bm.Stats, bm.Config.Settings.StatsConfig.Topics...)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	bm.statLoop(bm.Config.Settings.StatsConfig.Interval, topics)
}

var last time.Time = time.Time{}

func (bm *BeaconMonitor) subscribeToEvents(ctx context.Context, events []string, handler func(*api.Event)) {
	// For events: no ws necessary, this API uses server streamed events (SSE)
	// https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events
	if provider, isProvider := bm.Client.(eth2client.EventsProvider); isProvider {
		go bm.startBlockTimer()
		err := provider.Events(ctx, events, handler)
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}
	}
}

func (bm BeaconMonitor) EventHandler(event *api.Event) {
	log := bm.Logger

	switch event.Data.(type) {
	case *api.BlockEvent:
		block := event.Data.(*api.BlockEvent)
		var dur time.Duration
		if (last == time.Time{}) {
			dur, _ = time.ParseDuration("0s")
		} else {
			dur = time.Since(last).Round(time.Millisecond)
		}

		log.Info().Int("epoch", int(block.Slot/SLOTS_PER_EPOCH)).Str("slot", fmt.Sprint(block.Slot)).Str("last", dur.String()).Msg("New beacon block")
		bm.Reset <- true
		last = time.Now()
	case *api.FinalizedCheckpointEvent:
		cp := event.Data.(*api.FinalizedCheckpointEvent)
		log.Info().Str("epoch", fmt.Sprint(cp.Epoch)).Msg("Checkpoint finalized")
	case *api.ChainReorgEvent:
		ev := event.Data.(*api.ChainReorgEvent)
		log.Info().Uint64("depth", ev.Depth).Uint64("epoch", uint64(ev.Epoch)).Uint64("slot", uint64(ev.Slot)).Msg("Chain reorg")
	default:
		log.Warn().Str("event", event.String()).Msg("Unknown")
	}

}

func (bm BeaconMonitor) startBlockTimer() {
	log := bm.Logger

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
			Timer:    time.NewTimer(lvl1),
		},
		{
			Duration: lvl2,
			Timer:    time.NewTimer(lvl2),
		},
		{
			Duration: lvl3,
			Timer:    time.NewTimer(lvl2),
		},
	}

	for {
		select {
		case <-bm.Reset:
			lvls.Reset()
		case <-lvls[0].Timer.C:
			log.Warn().Msgf("%s since last block", lvls[0].Duration)
		case <-lvls[1].Timer.C:
			log.Warn().Msgf("%s since last block", lvls[1].Duration)
		case <-lvls[2].Timer.C:
			log.Warn().Msgf("%s since last block", lvls[2].Duration)
		}
	}
}

func (bm BeaconMonitor) statLoop(interval time.Duration, topics map[string]interface{}) {
	var (
		res P2PScanResult
		err error

		log = bm.Logger
	)

	// No args
	if len(topics) == 0 {
		log.Info().Msg("No topics provided")
		return
	}

	log.Info().Strs("topics", getKeys(topics)).Msg("Subscribed to topics")
	for {
		if settings, ok := topics["p2p"]; ok {
			time.Sleep(interval - 5*time.Second)
			if settings.(config.Stat).Latency {
				str := ""
				if bm.InterfaceName != "" {
					str = bm.InterfaceName
				}

				go func() {
					res, err = bm.LatencyScan(str)
					if err != nil {
						log.Err(err).Msg("")
					}

				}()
			}

			connected, connecting, disconnected, disconnecting, err := bm.PeerCount()
			if err != nil {
				log.Fatal().Err(err).Msg("")
			}

			time.Sleep(5 * time.Second)

			if connected < 20 {
				log.Warn().Int("peer_count", connected).Msg("[P2P] Low peer count")
			} else {
				log.Info().Int(
					"connected", connected).Int(
					"connecting", connecting).Int(
					"disconnected", disconnected).Int(
					"disconnecting", disconnecting).Msg("[P2P] Network info")
			}

			if settings.(config.Stat).Latency {
				log.Info().Str("high", res.High.String()).Str("low", res.Low.String()).Str("avg", res.Average.String()).Str("response_rate", fmt.Sprintf("%.2f%%", float64(res.Responses)/float64(res.Connected)*100)).Msg("[P2P] Latency scan results")
			}
		}
	}
}

func (bm BeaconMonitor) PeerCount() (int, int, int, int, error) {
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

// NodeVersion returns the node version.
func (bm BeaconMonitor) NodeVersion() (string, error) {
	res, err := web.Get(bm.Config.API + "/eth/v1/node/version")
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return gjson.GetBytes(body, "data.version").String(), nil
}

// FinalityCheckpoints returns the finality checkpoints as a JSON string.
func (bm BeaconMonitor) FinalityCheckpoints() (string, error) {
	res, err := web.Get(bm.Config.API + "/eth/v1/beacon/states/head/finality_checkpoints")
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return gjson.GetBytes(body, "data").String(), nil
}

type Peer struct {
	MultiAddress string `json:"last_seen_p2p_address"`
}

type PeersResponse struct {
	Data []Peer `json:"data"`
}

type P2PScanResult struct {
	High      time.Duration
	Low       time.Duration
	Average   time.Duration
	Connected int
	Responses int
}

func (bm *BeaconMonitor) Peers(state string) ([]Peer, error) {
	var peers PeersResponse
	res, err := web.Get(bm.Config.API + "/eth/v1/node/peers?state=" + state)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	err = json.Unmarshal(body, &peers)
	if err != nil {
		return nil, err
	}

	return peers.Data, nil
}

func (bm BeaconMonitor) LatencyScan(iface string) (P2PScanResult, error) {
	log := bm.Logger
	var (
		addrs []string
	)

	peers, err := bm.Peers("connected")
	if err != nil {
		return P2PScanResult{}, err
	}

	for _, p := range peers {
		// multiaddr format: /ip4/188.166.75.68/tcp/13000
		tmp := strings.Split(p.MultiAddress, "/")
		addrs = append(addrs, fmt.Sprintf("%s:%s", tmp[2], tmp[4]))
	}
	log.Trace().Int("connected", len(addrs)).Msg("[P2P] Starting latency scan")

	if bm.InterfaceName != "" {
		iface = bm.InterfaceName
	}

	if bm.Scanner == nil {
		// No scanner created yet
		bm.Scanner = net.NewScanner(iface)
	}

	results, err := bm.Scanner.StartLatencyScan(addrs)
	if err != nil {
		return P2PScanResult{}, err
	}

	hi := time.Nanosecond
	lo := time.Minute
	var total time.Duration
	for _, v := range results {
		if v > hi {
			hi = v
		}

		if v < lo {
			lo = v
		}

		total += v
	}

	avg, err := time.ParseDuration(fmt.Sprintf("%dus", int(total.Microseconds())/len(results)))
	if err != nil {
		return P2PScanResult{}, err
	}

	return P2PScanResult{
		High:      hi,
		Low:       lo,
		Average:   avg,
		Connected: len(addrs),
		Responses: len(results),
	}, nil
}
