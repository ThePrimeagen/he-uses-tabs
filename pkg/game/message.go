package game

import (
	"fmt"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

const (
    ReadyUp int = iota
    Play
    Fire
    GameOver
)

type GameMessage struct {
    Type int `json:"type"`
    Msg string `json:"msg,omitempty"` // how to optionally specify?
}

func CreateMessage(messageType int) GameMessage {
    return GameMessage{
        messageType, "",
    }
}

func CreateWinnerMessage(gameStats *GameStats) GameMessage {
    return GameMessage{
        Type: GameOver,
        Msg: fmt.Sprintf("winner(%v)___%v", atomic.LoadInt64(&ActiveGames), gameStats),
    }
}

func CreateLoserMessage() GameMessage {
    return GameMessage{
        Type: GameOver,
        Msg: "loser",
    }
}

func ErrorGameOver(msg string) GameMessage {
    return GameMessage{
        Type: GameOver,
        Msg: msg,
    }
}


