package pipelineconfig

import (
	_ "embed"
	"time"

	"gopkg.in/yaml.v3"
)

type Pipeline struct {
	MinWorkers      int           `yaml:"min_workers"`
	MaxWorkers      int           `yaml:"max_workers"`
	GrowThreshold   int           `yaml:"grow_threshold"`
	ShrinkThreshold int           `yaml:"shrink_threshold"`
	CheckInterval   time.Duration `yaml:"check_interval"`
	FailureRate     int           `yaml:"failure_rate"`

	// New fields for error-rate shutdown
	ErrorWindow    time.Duration `yaml:"error_window"`
	ErrorThreshold int           `yaml:"error_threshold"`
}

type Config struct {
	Pipeline Pipeline `yaml:"pipeline"`
}

//go:embed config.yml
var raw []byte

func Load() (*Config, error) {
	var c Config
	if err := yaml.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
