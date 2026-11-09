package main

import (
	"ChessLI/internal/config"
	"ChessLI/internal/gameplay"
	"ChessLI/internal/log"
	websockettransport "ChessLI/internal/websocket"
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	conf, err := config.Load()
	if err != nil {
		slog.SetDefault(log.NewLogger(log.LogFormatJSON))
		slog.Error("load configuration", "event", "server.configuration_failed", "error", err)
		os.Exit(1)
	}

	slog.SetDefault(log.NewLogger(conf.LogFormat))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	games := gameplay.NewGameService()

	server := websockettransport.NewServer(conf.ServerAddress, games)
	games.SetGameExpiredHandler(server.BroadcastGameState)

	slog.Info("starting websocket server", "event", "server.starting", "address", conf.ServerAddress)

	err = server.Run(ctx)
	games.Close()

	if err != nil {
		slog.Error("websocket server stopped", "event", "server.failed", "error", err)
		os.Exit(1)
	}
	slog.Info("websocket server stopped", "event", "server.stopped")
}
