package core

import (
	"errors"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the global config (<root>/config.yaml).
type Config struct {
	// Global maps tool -> active version used when no project
	// dem.yaml defines one. E.g. {"node": "22.5.1"}.
	Global map[string]string `yaml:"global"`
}

func LoadConfig(path string) (Config, error) {
	cfg := Config{Global: map[string]string{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Global == nil {
		cfg.Global = map[string]string{}
	}
	return cfg, nil
}

func (c Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
