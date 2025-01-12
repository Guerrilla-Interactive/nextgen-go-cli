package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadConfig reads config from ~/.ngc/config.json (or returns a default if missing).
func LoadConfig() (Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("could not find home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".ngc")
	configPath := filepath.Join(configDir, "config.json")

	// If file doesn't exist, return defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return Config{}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return cfg, nil
}

// SaveConfig writes Config to ~/.ngc/config.json
func SaveConfig(cfg Config) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not find home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".ngc")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "config.json")

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		// 0o600 ensures the config file is only readable by the owner.
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
