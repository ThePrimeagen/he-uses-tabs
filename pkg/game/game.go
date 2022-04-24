package game

import (
	"log"
	"sync"
	"time"

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
        var resp GameMessage;
        connection.ReadJSON(&resp)

        if resp.Type != ReadyUp {
            log.Fatalf("I am intentionally blowing up this program because this is completely wrong.")
        }

        wg.Done()
    }()

}

type NamedGameMessage struct {
    msg GameMessage
    name byte
}

func listenForFires(channel chan<- NamedGameMessage, c *websocket.Conn, name byte) {
    go func() {
        defer c.Close()
        for {
            var msg GameMessage
            err := c.ReadJSON(&msg)

            if err != nil {
                log.Println("We just had a json error while ready from socket???????? WDH")
            }

            if msg.Type == Fire {
                channel <- NamedGameMessage {msg, name}
            }
        }
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

            // 3. create stats and play message
            stats := NewGameStat()

            playerA.WriteJSON(CreateMessage(Fire))
            playerB.WriteJSON(CreateMessage(Fire))

            // Step 4.
            AddActiveGame()
            fires := make(chan NamedGameMessage, 10)
            listenForFires(fires, playerA, 'a')
            listenForFires(fires, playerB, 'b')

            // Steps 5. The rust version has a tokio::select
            ticker := time.NewTicker(time.Millisecond * 16)
            last_start := time.Now()

            var winner Player
            var loser Player

            game_me_daddy: for {
                select {
                case fire := <- fires:
                    player := game.Players[0]
                    if fire.name == 'b' {
                        player = game.Players[1]
                    }
                    game.Bullets = append(game.Bullets, CreateBulletFromPlayer(&player, 1.0))
                case <- ticker.C:
                    // 6. part 1 : calculate the time difference between each loop.
                    diff := time.Since(last_start).Microseconds()
                    last_start = time.Now()

                    // 6. do all the collision / updating
                    for _, bullet := range game.Bullets {
                        UpdateBullet(&bullet, diff)
                    }

                    // TODO: I may need to replace this because it sucks
                    loop_me_daddy: for idx1 := 0; idx1 < len(game.Bullets); idx1 += 1 {
                        bullet := game.Bullets[idx1]
                        for idx2 := idx1 + 1; idx2 < len(game.Bullets); idx2 += 1 {
                            bullet2 := game.Bullets[idx2]
                            if bullet.Geo.HasCollision(&bullet2.Geo) {
                                last := game.Bullets[len(game.Bullets) - 1]
                                second := game.Bullets[len(game.Bullets) - 2]

                                game.Bullets[idx1] = last;
                                game.Bullets[idx2] = second;

                                game.Bullets = game.Bullets[:len(game.Bullets) - 2]
                                break loop_me_daddy
                            }
                        }
                    }

                    for _, bullet := range game.Bullets {
                        if game.Players[0].Geo.HasCollision(&bullet.Geo) {
                            winner = game.Players[1]
                            loser = game.Players[0]
                            break game_me_daddy;
                        }
                        if game.Players[1].Geo.HasCollision(&bullet.Geo) {
                            winner = game.Players[0]
                            loser = game.Players[1]
                            break game_me_daddy;
                        }
                    }

                    stats.AddDelta(diff);
                }
            }

            // Part 7. Send out the winner / loser message and close down the
            // suckets

        }()

    }
}

