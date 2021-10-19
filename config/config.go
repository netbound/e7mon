package config

import (
	_ "embed"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

//go:embed config.yml
var cfg []byte

type ExecutionConfig struct {
	API      string   `yaml:"api"`
	Settings Settings `yaml:"settings"`
}

type Settings struct {
	BlockTimeLevels []string     `yaml:"block_time_levels"`
	StatsConfig     *StatsConfig `yaml:"stats"`
}

type StatsConfig struct {
	Interval time.Duration `yaml:"interval"`
	Topics   []string      `yaml:"topics"`
}

type BeaconConfig struct {
	API      string   `yaml:"api"`
	Settings Settings `yaml:"settings"`
}

type Config struct {
	ExecutionConfig *ExecutionConfig `yaml:"execution"`
	BeaconConfig    *BeaconConfig    `yaml:"beacon"`
	StatsConfig     []Stat           `yaml:"stats"`
}

type Stat struct {
	ID      string `yaml:"id"`
	Latency bool   `yaml:"latency,omitempty"`
}

func NewConfig() (*Config, error) {
	c := Config{}
	configPath, err := os.UserConfigDir()
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	configPath = path.Join(configPath, "e7mon/config.yml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	err = yaml.Unmarshal([]byte(data), &c)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	return &c, nil
}

func InitializeConfig() (string, error) {
	configPath, err := os.UserConfigDir()
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	configPath = path.Join(configPath, "e7mon/config.yml")
	_, err = os.Stat(configPath)
	if os.IsNotExist(err) {
		os.WriteFile(configPath, cfg, 0644)
	} else {
		return configPath, fmt.Errorf("%s already exists", configPath)
	}

	return configPath, nil
}
