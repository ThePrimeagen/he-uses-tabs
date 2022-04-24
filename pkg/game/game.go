package game

import (
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type Game struct {
	Players [2]Player
	Bullets []Bullet
}

func readyUp(wg sync.WaitGroup, connection *websocket.Conn) {
    wg.Add(1)
    go func() {
        connection.WriteJSON(CreateMessage(ReadyUp))
        var resp MessageEnvelope;
        connection.ReadJSON(&resp)

        if resp.Message.Type != ReadyUp {
            log.Fatalf("I am intentionally blowing up this program because this is completely wrong.")
        }

        wg.Done()
    }()
}


func RunGame(connections chan *websocket.Conn) {

    for {
        playerA := <- connections
        playerB := <- connections

        go func() {
            // 1. Wait for ready
            wg := sync.WaitGroup{}
            readyUp(wg, playerA)
            readyUp(wg, playerB)
            wg.Wait()

            // 2.  Game state
            game := Game {
                Players: [2]Player {
                    NewPlayer(Vector2D{2500.0, 0.0}, Vector2D{-1.0, 0.0}, 180),
                    NewPlayer(Vector2D{-2500.0, 0.0}, Vector2D{-1.0, 0.0}, 300),
                },
                Bullets: []Bullet{},
            }


        }()
    }
}

