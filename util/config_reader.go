package pipelineconfig

import (
	_ "embed"
	"time"

	"gopkg.in/yaml.v3"
)

// ------------------------------ YAML schema ------------------------------

type Pipeline struct {
	MinWorkers      int           `yaml:"min_workers"`
	MaxWorkers      int           `yaml:"max_workers"`
	GrowThreshold   int           `yaml:"grow_threshold"`
	ShrinkThreshold int           `yaml:"shrink_threshold"`
	CheckInterval   time.Duration `yaml:"check_interval"`
	FailureRate     int           `yaml:"failure_rate"`
}

type Config struct {
	Pipeline Pipeline `yaml:"pipeline"`
}

// ------------------------------ loader ------------------------------

//go:embed config.yml
var raw []byte

// Load unmarshals the embedded YAML into a Config struct.
func Load() (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
