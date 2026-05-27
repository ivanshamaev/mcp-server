package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all server configuration.
type Config struct {
	// AccessToken is the Yandex Metrika OAuth token (required).
	AccessToken string

	// ClientID and ClientSecret are for OAuth app (optional).
	ClientID     string
	ClientSecret string

	// MetrikaBaseURL is the Yandex Metrika API base URL.
	MetrikaBaseURL string

	// LogLevel controls log verbosity: debug, info, error.
	LogLevel slog.Level

	// LogFile is the path to a log file. Empty means stderr.
	LogFile string
}

// Load reads configuration from environment variables and optional .env file.
func Load() (*Config, error) {
	// Load .env file if present (ignore error — file may not exist).
	_ = godotenv.Load()

	token := os.Getenv("ACCESS_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("ACCESS_TOKEN environment variable is required")
	}

	cfg := &Config{
		AccessToken:    token,
		ClientID:       os.Getenv("CLIENT_ID"),
		ClientSecret:   os.Getenv("CLIENT_SECRET"),
		MetrikaBaseURL: "https://api-metrika.yandex.net",
		LogLevel:       parseLogLevel(os.Getenv("LOG_LEVEL")),
		LogFile:        os.Getenv("LOG_FILE"),
	}

	if base := os.Getenv("METRIKA_BASE_URL"); base != "" {
		cfg.MetrikaBaseURL = base
	}

	return cfg, nil
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
