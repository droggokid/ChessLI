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
	defaultServerAddress     = ":8080"
)

// Config contains configuration loaded from the environment.
type Config struct {
	LogFormat     log.LogFormat
	ServerAddress string
}

// Load reads and validates the application/server configuration from environment variables.
func Load() (Config, error) {
	logFormat, err := parseLogFormat(os.Getenv(logFormatEnvironment))
	if err != nil {
		return Config{}, err
	}

	serverAddress := strings.TrimSpace(os.Getenv(serverAddressEnvironment))
	if serverAddress == "" {
		serverAddress = defaultServerAddress
	}

	return Config{
		LogFormat:     logFormat,
		ServerAddress: serverAddress,
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
