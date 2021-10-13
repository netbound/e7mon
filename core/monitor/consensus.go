package monitor

import (
	"fmt"
	"os"
	"time"

	"github.com/jonasbostoen/e7mon/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type ConsensusMonitor struct {
	Config *config.ConsensusConfig
	Logger zerolog.Logger
}

func NewConsensusMonitor() *ConsensusMonitor {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05.000"}
	output.FormatMessage = func(i interface{}) string {
		return fmt.Sprintf("[CONSENSUS] %s", i)
	}

	cfg, err := config.NewConfig()

	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	return &ConsensusMonitor{
		Config: cfg.ConsensusConfig,
		Logger: log.Output(output),
	}
}
