package config

import (
	"os"
	"path"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

type ExecutionConfig struct {
	API      string            `yaml:"api"`
	Settings ExecutionSettings `yaml:"settings"`
}

type ExecutionSettings struct {
	BlockTimeLevels []string `yaml:"block_time_levels"`
}

type ConsensusConfig struct {
	API string `yaml:"api"`
}

type Config struct {
	ExecutionConfig *ExecutionConfig `yaml:"execution"`
	ConsensusConfig *ConsensusConfig `yaml:"consensus"`
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
