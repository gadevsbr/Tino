package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DataDir string      `yaml:"data_dir"`
	Profile string      `yaml:"profile"`
	Batch   BatchConfig `yaml:"batch"`
	Flow    FlowConfig  `yaml:"flow"`
}

type BatchConfig struct {
	MinInterval time.Duration `yaml:"-"`
	MaxInterval time.Duration `yaml:"-"`
	MinRaw      string        `yaml:"min_interval"`
	MaxRaw      string        `yaml:"max_interval"`
	MaxPerRun   int           `yaml:"max_per_run"`
}

type FlowConfig struct {
	RulesFile string `yaml:"rules_file"`
}

func Default() Config {
	return Config{
		DataDir: "data", Profile: "default",
		Batch: BatchConfig{MinInterval: 30 * time.Second, MaxInterval: 60 * time.Second, MinRaw: "30s", MaxRaw: "60s", MaxPerRun: 100},
		Flow:  FlowConfig{RulesFile: "config/flows.yaml"},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("ler configuração: %w", err)
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("decodificar configuração: %w", err)
	}
	if cfg.DataDir == "" || cfg.Profile == "" {
		return Config{}, errors.New("data_dir e profile são obrigatórios")
	}
	if cfg.Batch.MinRaw != "" {
		cfg.Batch.MinInterval, err = time.ParseDuration(cfg.Batch.MinRaw)
		if err != nil {
			return Config{}, fmt.Errorf("batch.min_interval: %w", err)
		}
	}
	if cfg.Batch.MaxRaw != "" {
		cfg.Batch.MaxInterval, err = time.ParseDuration(cfg.Batch.MaxRaw)
		if err != nil {
			return Config{}, fmt.Errorf("batch.max_interval: %w", err)
		}
	}
	if cfg.Batch.MinInterval <= 0 || cfg.Batch.MaxInterval < cfg.Batch.MinInterval {
		return Config{}, errors.New("intervalos do batch inválidos")
	}
	if cfg.Batch.MaxPerRun <= 0 {
		return Config{}, errors.New("batch.max_per_run deve ser positivo")
	}
	return cfg, nil
}
