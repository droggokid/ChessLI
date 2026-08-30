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

	service := gameplay.NewGameService()
	defer service.Close()

	t := time.Minute * 10

	createCommand := gameplay.CreatePrivateCommand{ProfileID: identity.NewProfileID(), Initial: t, Increment: 0, ColorPreference: gameplay.ColorRandom}

	createResult, err := service.CreatePrivateGame(ctx, createCommand)
	if err != nil {
		fmt.Printf("couldn't create private game: %v\n", err)
		return
	}

	joinCommand := gameplay.JoinPrivateCommand{ProfileID: identity.NewProfileID(), GameID: createResult.GameID}

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
