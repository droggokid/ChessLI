package main

import (
	"context"
	"fmt"
	"time"

	"ChessLI/internal/config"
	"ChessLI/internal/gameplay"
	"ChessLI/internal/identity"
	"ChessLI/internal/persistence"

	"github.com/corentings/chess/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()

	conf, err := config.Load()
	if err != nil {
		fmt.Printf("couldn't load configuration: %v\n", err)
		return
	}
	startupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(startupCtx, conf.DatabaseURL)
	if err != nil {
		fmt.Printf("couldn't configure database: %v\n", err)
		return
	}
	defer pool.Close()
	if err := pool.Ping(startupCtx); err != nil {
		fmt.Printf("couldn't connect to database: %v\n", err)
		return
	}
	service := gameplay.NewGameService(persistence.NewGameRepository(pool), persistence.NewProfileRepository(pool))
	defer service.Close()

	t := time.Minute * 10

	createCommand := gameplay.CreatePrivateCommand{ProfileID: identity.ProfileID("00000000-0000-4000-8000-000000000001"), Initial: t, Increment: 0, ColorPreference: gameplay.ColorRandom}

	createResult, err := service.CreatePrivateGame(ctx, createCommand)
	if err != nil {
		fmt.Printf("couldn't create private game: %v\n", err)
		return
	}

	joinCommand := gameplay.JoinPrivateCommand{ProfileID: identity.ProfileID("00000000-0000-4000-8000-000000000002"), GameID: createResult.GameID}

	joinResult, err := service.JoinPrivateGame(ctx, joinCommand)
	if err != nil {
		fmt.Printf("couldn't join private game: %v\n", err)
		return
	}

	var whitePlayerID, blackPlayerID identity.ProfileID

	if createResult.Color == chess.White {
		whitePlayerID = createCommand.ProfileID
		blackPlayerID = joinCommand.ProfileID
	} else {
		whitePlayerID = joinCommand.ProfileID
		blackPlayerID = createCommand.ProfileID
	}

	firstMoveCommand := gameplay.MoveCommand{GameID: createResult.GameID, ProfileID: whitePlayerID, Move: "e2e4", Notation: gameplay.MoveNotationUCI}
	firstMoveResult, err := service.MakeMove(ctx, firstMoveCommand)
	if err != nil {
		fmt.Printf("couldn't make first move: %v\n", err)
		return
	}

	fmt.Println("first move result: ", firstMoveResult)

	secondMoveCommand := gameplay.MoveCommand{
		GameID:          joinResult.GameID,
		ProfileID:       blackPlayerID,
		Move:            "e7e5",
		Notation:        gameplay.MoveNotationUCI,
		ExpectedVersion: firstMoveResult.Version,
	}
	secondMoveResult, err := service.MakeMove(ctx, secondMoveCommand)
	if err != nil {
		fmt.Printf("couldn't make second move: %v\n", err)
		return
	}

	fmt.Println("second move result: ", secondMoveResult)
}
