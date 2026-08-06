package main

import (
	"context"
	"fmt"
	"time"

	"ChessLI/internal/gameplay"
	"ChessLI/internal/identity"

	"github.com/corentings/chess/v2"
)

func main() {
	ctx := context.Background()

	service := gameplay.NewMemoryService()

	t := time.Minute * 10

	createCommand := gameplay.CreatePrivateCommand{ProfileID: identity.NewProfileID(), Initial: t, Increment: 0, ColorPreference: gameplay.ColorRandom}

	createResult, err := service.CreatePrivateGame(ctx, createCommand)
	if err != nil {
		fmt.Println("couldn't create private game: %w", err)
	}

	joinCommand := gameplay.JoinPrivateCommand{ProfileID: identity.NewProfileID(), GameID: createResult.GameID}

	joinResult, err := service.JoinPrivateGame(ctx, joinCommand)
	if err != nil {
		fmt.Println("couldn't join private game: %w", err)
	}

	var whitePlayerID, blackPlayerID identity.ProfileID

	if createResult.Color == chess.White {
		whitePlayerID = createCommand.ProfileID
		blackPlayerID = joinCommand.ProfileID
	} else {
		whitePlayerID = joinCommand.ProfileID
		blackPlayerID = createCommand.ProfileID
	}

	firstMoveCommand := gameplay.MoveCommand{GameID: createResult.GameID, ProfileID: whitePlayerID, Move: "e2e4"}
	firstMoveResult, err := service.MakeMove(ctx, firstMoveCommand)

	fmt.Println("first move result: ", firstMoveResult)

	secondMoveCommand := gameplay.MoveCommand{GameID: joinResult.GameID, ProfileID: blackPlayerID, Move: "e7e5"}
	secondMoveResult, err := service.MakeMove(ctx, secondMoveCommand)

	fmt.Println("second move result: ", secondMoveResult)
}
