package main

import (
	"ChessLI/internal/config"
	"ChessLI/internal/gameplay"
	"ChessLI/internal/log"
	"ChessLI/internal/persistence"
	websockettransport "ChessLI/internal/websocket"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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

	err = runServer(ctx, conf)
	if err != nil {
		slog.Error("websocket server stopped", "event", "server.failed", "error", err)
		os.Exit(1)
	}
	slog.Info("websocket server stopped", "event", "server.stopped")
}

func runServer(ctx context.Context, conf config.Config) error {
	startupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	pool, err := pgxpool.New(startupCtx, conf.DatabaseURL)
	if err != nil {
		cancel()
		return fmt.Errorf("configure database: %w", err)
	}
	defer pool.Close()
	err = pool.Ping(startupCtx)
	cancel()
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}

	games := gameplay.NewGameService(persistence.NewGameRepository(pool), persistence.NewProfileRepository(pool))
	defer games.Close()
	server := websockettransport.NewServer(conf.ServerAddress, games)
	games.SetGameExpiredHandler(server.BroadcastGameState)
	slog.Info("starting websocket server", "event", "server.starting", "address", conf.ServerAddress)
	return server.Run(ctx)
}
