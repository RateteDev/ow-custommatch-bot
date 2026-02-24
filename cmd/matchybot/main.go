package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"matchybot/internal/bot"
)

const (
	envFileName        = ".env"
	playerDataFileName = "player_data.json"
	rankDataFileName   = "rank.json"
)

func executableDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	return filepath.Dir(exePath), nil
}

func loadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open env file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			return fmt.Errorf("invalid env format at line %d", lineNo)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return fmt.Errorf("empty env key at line %d", lineNo)
		}

		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set env %s: %w", key, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan env file: %w", err)
	}
	return nil
}

func requiredEnv(key string) (string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func main() {
	exeDir, err := executableDir()
	if err != nil {
		log.Fatalf("failed to determine executable directory: %v", err)
	}

	envPath := filepath.Join(exeDir, envFileName)
	if err := loadEnvFile(envPath); err != nil {
		log.Fatalf("failed to load env (%s): %v", envPath, err)
	}

	botToken, err := requiredEnv("BOT_TOKEN")
	if err != nil {
		log.Fatalf("failed to read env: %v", err)
	}

	playerDataPath := filepath.Join(exeDir, playerDataFileName)
	rankDataPath := filepath.Join(exeDir, rankDataFileName)

	b, err := bot.New(playerDataPath, rankDataPath)
	if err != nil {
		log.Fatalf("failed to initialize bot: %v", err)
	}

	if err := b.Run(botToken); err != nil {
		log.Fatalf("bot error: %v", err)
	}
}
