package config

import (
	"ChessLI/internal/log"
	"fmt"
	"os"
	"strings"
)

const (
	logFormatEnvironment     = "CHESSLI_LOG_FORMAT"
	serverAddressEnvironment = "CHESSLI_SERVER_ADDRESS"
	databaseURLEnvironment   = "DATABASE_URL"
	defaultServerAddress     = ":8080"
)

// Config contains configuration loaded from the environment.
type Config struct {
	LogFormat     log.LogFormat
	ServerAddress string
	DatabaseURL   string
}

// Load reads environment configuration, applying defaults and validating the log format.
func Load() (Config, error) {
	logFormat, err := parseLogFormat(os.Getenv(logFormatEnvironment))
	if err != nil {
		return Config{}, err
	}

	serverAddress := strings.TrimSpace(os.Getenv(serverAddressEnvironment))
	if serverAddress == "" {
		serverAddress = defaultServerAddress
	}
	databaseURL := strings.TrimSpace(os.Getenv(databaseURLEnvironment))
	if databaseURL == "" {
		return Config{}, fmt.Errorf("%s is required", databaseURLEnvironment)
	}

	return Config{
		LogFormat:     logFormat,
		ServerAddress: serverAddress,
		DatabaseURL:   databaseURL,
	}, nil
}

func parseLogFormat(value string) (log.LogFormat, error) {
	switch log.LogFormat(strings.ToLower(strings.TrimSpace(value))) {
	case "", log.LogFormatJSON:
		return log.LogFormatJSON, nil
	case log.LogFormatText:
		return log.LogFormatText, nil
	default:
		return "", fmt.Errorf("%s must be %q or %q", logFormatEnvironment, log.LogFormatJSON, log.LogFormatText)
	}
}
