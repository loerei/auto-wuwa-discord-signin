package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	AppVersion             = "v1.0.0"
	AppDirName             = "WuWaDiscordAuto"
	DefaultConfigFileName  = "config.json"
)

var configMu sync.Mutex

type Config struct {
	DiscordToken    string `json:"discord_token"`
	GuildID         string `json:"guild_id"`
	ChannelID       string `json:"channel_id"`
	CheckIntervalHr int    `json:"check_interval_hr"`
}

func DefaultConfig() Config {
	return Config{
		DiscordToken:    "",
		GuildID:         "963760374543450182",
		ChannelID:       "1352684570683641929",
		CheckIntervalHr: 1,
	}
}

// GetAppDataDir returns %APPDATA%\WuWaDiscordAuto
func GetAppDataDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user directory: %w", err)
		}
		appData = filepath.Join(home, "AppData", "Roaming")
	}

	targetDir := filepath.Join(appData, AppDirName)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create app data directory: %w", err)
	}
	return targetDir, nil
}

func GetConfigPath() (string, error) {
	dir, err := GetAppDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, DefaultConfigFileName), nil
}

func LoadConfig() (Config, error) {
	configMu.Lock()
	defer configMu.Unlock()

	cfgPath, err := GetConfigPath()
	if err != nil {
		return DefaultConfig(), err
	}

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		cfg := DefaultConfig()
		_ = saveConfigUnsafe(cfg, cfgPath)
		return cfg, nil
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return DefaultConfig(), fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), fmt.Errorf("failed to parse config file: %w", err)
	}

	if cfg.GuildID == "" {
		cfg.GuildID = "963760374543450182"
	}
	if cfg.ChannelID == "" {
		cfg.ChannelID = "1352684570683641929"
	}
	if cfg.CheckIntervalHr <= 0 {
		cfg.CheckIntervalHr = 1
	}

	return cfg, nil
}

func SaveConfig(cfg Config) error {
	configMu.Lock()
	defer configMu.Unlock()

	cfgPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	return saveConfigUnsafe(cfg, cfgPath)
}

func saveConfigUnsafe(cfg Config, cfgPath string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	tmpPath := cfgPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return os.WriteFile(cfgPath, data, 0644)
	}

	if err := os.Rename(tmpPath, cfgPath); err != nil {
		_ = os.Remove(tmpPath)
		return os.WriteFile(cfgPath, data, 0644)
	}

	return nil
}
