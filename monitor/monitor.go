package monitor

import (
	"github.com/jonasbostoen/e7mon/config"
	"github.com/rs/zerolog/log"
)

type Monitor struct {
	Config    *config.Config
	Execution *ExecutionMonitor
	Consensus *BeaconMonitor
}

func NewMonitor() *Monitor {
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	exec := NewExecutionMonitor()
	consensus := NewBeaconMonitor()

	return &Monitor{
		Config:    cfg,
		Execution: exec,
		Consensus: consensus,
	}
}

func (m Monitor) Start() {
	go m.Execution.Start()
	m.Consensus.Start()
}
