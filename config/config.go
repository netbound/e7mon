package config

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"gopkg.in/yaml.v2"
)

//go:embed config.yml
var cfg []byte

type Config struct {
	ExecutionConfig *ExecutionConfig `yaml:"execution"`
	BeaconConfig    *BeaconConfig    `yaml:"beacon"`
	StatsConfig     []Stat           `yaml:"stats"`
	NetConfig       *NetConfig       `yaml:"net"`
}

type ExecutionConfig struct {
	API      string   `yaml:"api"`
	Settings Settings `yaml:"settings"`
}

type BeaconConfig struct {
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

type Stat struct {
	ID      string `yaml:"id"`
	Latency bool   `yaml:"latency,omitempty"`
}

type NetConfig struct {
	Interface string `yaml:"interface,omitempty"`
	Backup    string `yaml:"backup,omitempty"`
}

func NewConfig() (*Config, error) {
	c := Config{}
	configPath, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}

	configPath = path.Join(configPath, "e7mon/config.yml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatal(fmt.Errorf("error reading config file at %s, try running e7mon init first", configPath))
	}

	err = yaml.Unmarshal([]byte(data), &c)
	if err != nil {
		log.Fatal(err)
	}

	return &c, nil
}

func InitializeConfig() (string, error) {
	configPath, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}

	dirPath := path.Join(configPath, "e7mon")
	_, err = os.Stat(dirPath)
	if os.IsNotExist(err) {
		os.MkdirAll(dirPath, 0744)
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
