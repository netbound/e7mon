package monitor

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/netbound/e7mon/config"
	"github.com/netbound/e7mon/net"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/fatih/color"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type ExecutionMonitor struct {
	Config  *config.ExecutionConfig
	Stats   []config.Stat
	Client  *rpc.Client
	Logger  zerolog.Logger
	Scanner *net.Scanner
}

func NewExecutionMonitor() *ExecutionMonitor {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05.000"}
	output.FormatMessage = func(i interface{}) string {
		p := color.New(color.FgBlue).Add(color.Bold)
		return fmt.Sprintf("| %s | %-50s", p.Sprintf("%-9s", "EXECUTION"), i)
	}

	cfg, err := config.NewConfig()

	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	client, err := rpc.Dial(cfg.ExecutionConfig.API)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	// TODO: build p2p scanner if latency stat is enabled

	return &ExecutionMonitor{
		Config: cfg.ExecutionConfig,
		Stats:  cfg.StatsConfig,
		Client: client,
		Logger: log.Output(output),
	}
}

type Block struct {
	Number *hexutil.Big
}

func (em ExecutionMonitor) Start() {
	log := em.Logger

	defer em.Client.Close()

	ver, err := em.NodeVersion()
	if err != nil {
		log.Error().Msg(err.Error())
	}
	log.Info().Str("api", em.Config.API).Str("node_version", ver).Msg("Starting execution client monitor")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	topics, err := parseTopics(em.Stats, em.Config.Settings.StatsConfig.Topics...)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	go em.statLoop(em.Config.Settings.StatsConfig.Interval, topics)

	c := make(chan Block)

	_, err = em.Client.EthSubscribe(ctx, c, "newHeads")
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	lastBlock := int64(0)
	reset := make(chan bool)

	go em.startBlockTimer(reset)
	for block := range c {
		tmp := block.Number.ToInt().Int64()
		if tmp > lastBlock {
			lastBlock = tmp
			log.Info().Int64("block_number", lastBlock).Msg("New execution block")
			reset <- true
		}
	}
}

type BlockTimeLevel struct {
	Duration time.Duration
	Hit      bool
}

type BlockTimeLevels [3]*BlockTimeLevel

func (l BlockTimeLevels) Reset() {
	for _, b := range l {
		b.Hit = false
	}
}

func (em ExecutionMonitor) startBlockTimer(reset <-chan bool) {
	start := time.Now()
	log := em.Logger

	lvl1, err := time.ParseDuration(em.Config.Settings.BlockTimeLevels[0])
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	lvl2, err := time.ParseDuration(em.Config.Settings.BlockTimeLevels[1])
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	lvl3, err := time.ParseDuration(em.Config.Settings.BlockTimeLevels[2])
	if err != nil {
		log.Fatal().Msg(err.Error())
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
		case <-reset:
			start = time.Now()
			lvls.Reset()
		default:
			time.Sleep(time.Millisecond * 500)
			new := time.Now()
			if new.After(start.Add(lvls[0].Duration)) && !lvls[0].Hit {
				log.Warn().Msgf("%s since last block", lvls[0].Duration)
				lvls[0].Hit = true
			} else if new.After(start.Add(lvls[1].Duration)) && !lvls[1].Hit {
				log.Warn().Msgf("%s since last block", lvls[1].Duration)
				lvls[1].Hit = true
			} else if new.After(start.Add(lvls[2].Duration)) && !lvls[2].Hit {
				log.Warn().Msgf("%s since last block", lvls[2].Duration)
				lvls[2].Hit = true
			}
		}
	}
}

func parseTopics(stats []config.Stat, topics ...string) (map[string]interface{}, error) {
	m := make(map[string]interface{})
	for _, topic := range topics {
		for _, stat := range stats {
			if stat.ID == topic {
				m[topic] = stat
				continue
			}

			return nil, fmt.Errorf("topic '%s' does not exist", topic)
		}
	}

	return m, nil
}

func getKeys(m map[string]interface{}) []string {
	keys := make([]string, len(m))

	i := 0
	for k := range m {
		keys[i] = k
		i++
	}

	return keys
}

func (em ExecutionMonitor) statLoop(interval time.Duration, topics map[string]interface{}) {
	log := em.Logger

	// No args
	if len(topics) == 0 {
		log.Info().Msg("No topics provided")
		return
	}

	log.Info().Strs("topics", getKeys(topics)).Msg("Subscribed to topics")

	for {
		time.Sleep(interval)
		if _, ok := topics["p2p"]; ok {
			pc, err := em.PeerCount()
			if err != nil {
				log.Fatal().Msg(err.Error())
			}

			if pc < 20 {
				log.Warn().Str("connected", fmt.Sprint(pc)).Msg("[P2P] Low peer count")
			} else {
				log.Info().Str("connected", fmt.Sprint(pc)).Msg("[P2P] Network info")
			}
		}
	}
}

func (em ExecutionMonitor) PeerCount() (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var peerCount hexutil.Big

	err := em.Client.CallContext(ctx, &peerCount, "net_peerCount")
	if err != nil {
		return 0, err
	}

	return peerCount.ToInt().Uint64(), nil
}

func (em ExecutionMonitor) NodeVersion() (version string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = em.Client.CallContext(ctx, &version, "web3_clientVersion")
	if err != nil {
		return
	}
	return
}

func (em ExecutionMonitor) P2PStat() {

}
