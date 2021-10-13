package monitor

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jonasbostoen/e7mon/config"
	"github.com/ledgerwatch/erigon/common/hexutil"
	"github.com/ledgerwatch/erigon/rpc"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type ExecutionMonitor struct {
	Config *config.ExecutionConfig
	Client *rpc.Client
	Logger zerolog.Logger
}

func NewExecutionMonitor() *ExecutionMonitor {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05.000"}
	output.FormatMessage = func(i interface{}) string {
		return fmt.Sprintf("[EXECUTION] %s", i)
	}

	cfg, err := config.NewConfig()

	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	client, err := rpc.Dial(cfg.ExecutionConfig.API)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	return &ExecutionMonitor{
		Config: cfg.ExecutionConfig,
		Client: client,
		Logger: log.Output(output),
	}
}

type Block struct {
	Number *hexutil.Big
}

func (em ExecutionMonitor) Start() {
	log := em.Logger
	log.Info().Str("api", em.Config.API).Msg("Starting execution client monitor")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	interval, err := time.ParseDuration(em.Config.Settings.StatsInterval)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	go em.statLoop(interval)

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
			log.Info().Str("block_number", fmt.Sprint(lastBlock)).Msg("New block header received")
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
				em.Logger.Warn().Msgf("%s since last block", lvls[0].Duration)
				lvls[0].Hit = true
			} else if new.After(start.Add(lvls[1].Duration)) && !lvls[1].Hit {
				em.Logger.Warn().Msgf("%s since last block", lvls[1].Duration)
				lvls[1].Hit = true
			} else if new.After(start.Add(lvls[2].Duration)) && !lvls[2].Hit {
				em.Logger.Warn().Msgf("%s since last block", lvls[2].Duration)
				lvls[2].Hit = true
			}
		}
	}
}

func (em ExecutionMonitor) statLoop(interval time.Duration) {
	log := em.Logger
	for {
		pc, err := em.GetPeerCount()
		if err != nil {
			log.Fatal().Msg(err.Error())
		}

		if pc < 20 {
			log.Warn().Str("peer_count", fmt.Sprint(pc)).Msg("[P2P] Low peer count")
		} else {
			log.Info().Str("peer_count", fmt.Sprint(pc)).Msg("[P2P] Network info")
		}
		time.Sleep(interval)
	}
}

func (em ExecutionMonitor) GetPeerCount() (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var peerCount hexutil.Big

	err := em.Client.CallContext(ctx, &peerCount, "net_peerCount")
	if err != nil {
		return 0, err
	}

	return peerCount.ToInt().Uint64(), nil
}
