package monitor

import (
	"fmt"
	"os"

	"github.com/netbound/e7mon/config"
	"github.com/rs/zerolog/log"
)

type Monitor struct {
	Config    *config.Config
	Execution *ExecutionMonitor
	Consensus *BeaconMonitor
	Validator *ValidatorMonitor
}

func NewMonitor() *Monitor {
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	exec := NewExecutionMonitor()
	consensus := NewBeaconMonitor()
	validator := NewValidatorMonitor()

	return &Monitor{
		Config:    cfg,
		Execution: exec,
		Consensus: consensus,
		Validator: validator,
	}
}

func (m Monitor) Start() {
	go m.Execution.Start()
	go m.Consensus.Start()
	m.Validator.Start()
}

func (m Monitor) PrintVersions() {
	exec := NewExecutionMonitor()
	beacon := NewBeaconMonitor()
	execVersion, err := exec.NodeVersion()
	if err != nil {
		fmt.Println("Unable to get execution client version")
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("Execution client version:\t%s\n", execVersion)
	beaconVersion, err := beacon.NodeVersion()
	if err != nil {
		fmt.Println("Unable to get beacon client version")
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("Beacon client version:\t\t%s\n", beaconVersion)
}
