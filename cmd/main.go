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
		slog.Error("load configuration", "error", err)
		os.Exit(1)
	}

	slog.SetDefault(log.NewLogger(conf.LogFormat))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	games := gameplay.NewGameService()

	server := websockettransport.NewServer(conf.ServerAddress, games)

	slog.Info("starting websocket server", "address", conf.ServerAddress)

	if err = server.Run(ctx); err != nil {
		slog.Error("websocket server stopped", "error", err)
		os.Exit(1)
	}
}
