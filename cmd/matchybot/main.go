package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RateteDev/MatchyBot/internal/bot"
)

const (
	envFileName        = ".env"
	playerDataFileName = "player_data.json"
	vcConfigFileName   = "vc_config.json"
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

func setupLogger(exeDir string) (io.Closer, error) {
	logDir := filepath.Join(exeDir, ".logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}

	logPath := filepath.Join(logDir, time.Now().Format("2006-01-02T15-04-05")+".log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	log.SetOutput(io.MultiWriter(os.Stdout, f))
	return f, nil
}

func main() {
	exeDir, err := executableDir()
	if err != nil {
		log.Fatalf("failed to determine executable directory: %v", err)
	}
	closer, err := setupLogger(exeDir)
	if err != nil {
		log.Fatalf("failed to setup logger: %v", err)
	}
	defer closer.Close()

	envPath := filepath.Join(exeDir, envFileName)
	if err := loadEnvFile(envPath); err != nil {
		log.Fatalf("failed to load env (%s): %v", envPath, err)
	}

	botToken, err := requiredEnv("BOT_TOKEN")
	if err != nil {
		log.Fatalf("failed to read env: %v", err)
	}

	playerDataPath := filepath.Join(exeDir, playerDataFileName)
	vcConfigPath := filepath.Join(exeDir, vcConfigFileName)

	b, err := bot.New(playerDataPath, vcConfigPath)
	if err != nil {
		log.Fatalf("failed to initialize bot: %v", err)
	}

	if err := b.Run(botToken); err != nil {
		log.Fatalf("bot error: %v", err)
	}
}
