package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"matchybot/internal/bot"
)

type Config struct {
	BotToken       string `json:"bot_token"`
	PlayerDataPath string `json:"player_data_path"`
	RankDataPath   string `json:"rank_data_path"`
}

func loadConfig(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	var cfg Config
	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	if cfg.BotToken == "" {
		return Config{}, fmt.Errorf("bot_token is required")
	}
	if cfg.PlayerDataPath == "" {
		return Config{}, fmt.Errorf("player_data_path is required")
	}
	if cfg.RankDataPath == "" {
		return Config{}, fmt.Errorf("rank_data_path is required")
	}

	return cfg, nil
}

func executableDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	return filepath.Dir(exePath), nil
}

func resolvePath(baseDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(baseDir, path)
}

func main() {
	exeDir, err := executableDir()
	if err != nil {
		log.Fatalf("failed to determine executable directory: %v", err)
	}

	configPath := filepath.Join(exeDir, "config.json")
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("failed to load config (%s): %v", configPath, err)
	}

	cfg.PlayerDataPath = resolvePath(exeDir, cfg.PlayerDataPath)
	cfg.RankDataPath = resolvePath(exeDir, cfg.RankDataPath)

	b, err := bot.New(cfg.PlayerDataPath, cfg.RankDataPath)
	if err != nil {
		log.Fatalf("failed to initialize bot: %v", err)
	}

	if err := b.Run(cfg.BotToken); err != nil {
		log.Fatalf("bot error: %v", err)
	}
}
