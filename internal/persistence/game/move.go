package game

import "time"

type Move struct {
	Ply            int    `db:"ply"`
	UCI            string `db:"uci"`
	WhiteRemaining time.Duration
	BlackRemaining time.Duration
}
