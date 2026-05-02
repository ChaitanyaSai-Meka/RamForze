package handshake

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Passphrase string `json:"passphrase"`
}

func ReadPassphrase() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".ramforze", "config.json")
	fileBytes, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("could not read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(fileBytes, &cfg); err != nil {
		return "", fmt.Errorf("could not parse config file: %w", err)
	}

	if cfg.Passphrase == "" {
		return "", fmt.Errorf("passphrase is empty in config file")
	}

	return cfg.Passphrase, nil
}
