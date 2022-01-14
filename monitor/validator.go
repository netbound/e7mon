package monitor

import (
	"context"
	"fmt"
	"os"
	"time"

	eth2client "github.com/attestantio/go-eth2-client"
	api "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/attestantio/go-eth2-client/http"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/fatih/color"
	"github.com/netbound/e7mon/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type ValidatorMonitor struct {
	API    string
	Config *config.ValidatorConfig
	Client eth2client.Service
	Logger zerolog.Logger
}

func NewValidatorMonitor() *ValidatorMonitor {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05.000"}
	output.FormatMessage = func(i interface{}) string {
		p := color.New(color.FgYellow).Add(color.Bold)
		return fmt.Sprintf("| %s | %-50s", p.Sprintf("%-9s", "VALIDATOR"), i)
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
	return &ValidatorMonitor{
		API:    cfg.BeaconConfig.API,
		Config: cfg.ValidatorConfig,
		Client: client,
		Logger: logger,
	}
}

func (vm *ValidatorMonitor) Start() {
	log := vm.Logger

	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()

	balance, err := vm.validatorBalance(vm.Config.Index)
	if err != nil {
		log.Fatal().Err(err).Msg("Error getting balance")
	}

	log.Info().Uint64("validator_index", vm.Config.Index).Uint64("balance", balance).Msg("Starting validator monitor")

	// TODO: only subscribe to attestations OUR validator produces
	// vm.subscribeToAttestations(ctx)

	select {}
}

func (vm *ValidatorMonitor) subscribeToAttestations(ctx context.Context) {
	if provider, isProvider := vm.Client.(eth2client.EventsProvider); isProvider {
		err := provider.Events(ctx, []string{"attestation"}, func(event *api.Event) {
			attestation := event.Data.(*phase0.Attestation)

			vm.Logger.Info().Uint64("committee", uint64(attestation.Data.Index)).Msg("New attestation")
		})
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}
	}
}

func (vm *ValidatorMonitor) validatorBalance(index uint64) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if provider, ok := vm.Client.(eth2client.ValidatorBalancesProvider); ok {
		res, err := provider.ValidatorBalances(ctx, "head", []phase0.ValidatorIndex{phase0.ValidatorIndex(index)})
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}

		if len(res) == 0 {
			log.Warn().Msg("Validator not found")
			return 0, nil
		}

		return uint64(res[phase0.ValidatorIndex(index)]), nil
	}

	return 0, nil
}
