package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"agent-hub/internal/types"
)

const configFileName = "config.json"

func DefaultConfig() *types.Config {
	home, _ := os.UserHomeDir()
	return &types.Config{
		ProjectsDir: filepath.Join(home, "Documents", "projekte"),
		Agents: map[string]string{
			"opencode":    "opencode",
			"claude-code": "claude",
		},
	}
}

func Load(path string) (*types.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			if err := Save(path, cfg); err != nil {
				return nil, fmt.Errorf("save default config: %w", err)
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg types.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

func Save(path string, cfg *types.Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func ConfigPath() string {
	wd, _ := os.Getwd()
	// Walk up to find project root
	for {
		if _, err := os.Stat(filepath.Join(wd, configFileName)); err == nil {
			return filepath.Join(wd, configFileName)
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			break
		}
		wd = parent
	}
	return configFileName
}
