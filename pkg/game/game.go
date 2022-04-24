package game

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Game struct {
	Players [2]*Player
	Bullets []*Bullet
}

type Debug struct {
    Time int64
    Place string
}

func readyUp(wg *sync.WaitGroup, connection *websocket.Conn) {
	go func() {
        defer func() {
            wg.Done()
        }()

		connection.WriteJSON(CreateMessage(ReadyUp))
		var resp GameMessage
        msgType, msg, err := connection.ReadMessage()

        if msgType != websocket.TextMessage {
			log.Fatalf("WHERE AM I??")
        }

        if err != nil {
			log.Fatalf("I think that I have died and gone to JSON")
        }

        json.Unmarshal(msg, &resp)
		if resp.Type != ReadyUp {
			log.Fatalf("I am intentionally blowing up this program because this is completely wrong.")
		}
	}()

}

type NamedGameMessage struct {
	msg  GameMessage
	name byte
}

func listenForFires(channel chan<- NamedGameMessage, c *websocket.Conn, name byte) {
	go func() {
		for {
			var cmd GameMessage
            msgType, msg, err := c.ReadMessage()

            if err != nil {
                return
            }

            if msgType != websocket.TextMessage {
                continue
            }

            json.Unmarshal(msg, &cmd)

			if cmd.Type == Fire {
                channel <- NamedGameMessage{msg: cmd, name: name}
			} else {
                log.Fatalf("WHAT IS HAPPENING TO ME ???? %+v\n", cmd)
            }
		}
	}()
}

func checkBulletCollisions(g *Game) {
    loop_me_daddy: for idx1 := 0; idx1 < len(g.Bullets); {
        bullet := g.Bullets[idx1]
        for idx2 := idx1 + 1; idx2 < len(g.Bullets); idx2 += 1 {
            bullet2 := g.Bullets[idx2]
            if bullet.Geo.HasCollision(&bullet2.Geo) {
                // that is also very crappy code.  Why would I ever do this...
                g.Bullets = append(g.Bullets[:idx2], g.Bullets[(idx2 + 1):]...)
                g.Bullets = append(g.Bullets[:idx1], g.Bullets[(idx1 + 1):]...)
                break loop_me_daddy
            }
        }

        idx1 += 1
    }
}

func RunGame(connections chan *websocket.Conn) {

	for {
		playerA := <-connections
		playerB := <-connections

		go func() {
            defer playerA.Close()
            defer playerB.Close()

            starting_of_game := time.Now()
            hack := []Debug{}

			// 1. Wait for ready
			wg := sync.WaitGroup{}
            wg.Add(1)
            wg.Add(1)
			readyUp(&wg, playerA)
			readyUp(&wg, playerB)
			wg.Wait()

			// 2.  Game state
			game := Game{
				Players: [2]*Player{
					NewPlayer(Vector2D{2500.0, 0.0}, Vector2D{-1.0, 0.0}, 180),
					NewPlayer(Vector2D{-2500.0, 0.0}, Vector2D{1.0, 0.0}, 300),
				},
				Bullets: []*Bullet{},
			}

			// 3. create stats and play message
			stats := NewGameStat()

            playMsg, err := json.Marshal(CreateMessage(Play))
            if err != nil {
                log.Fatalf("WHY WOULD THIS EVER DIE ON ME???")
            }

			playerA.WriteMessage(websocket.TextMessage, playMsg)
			playerB.WriteMessage(websocket.TextMessage, playMsg)

			// Step 4.
			AddActiveGame()
			fires := make(chan NamedGameMessage, 10)
			listenForFires(fires, playerA, 'a')
			listenForFires(fires, playerB, 'b')

			// Steps 5. The rust version has a tokio::select
			ticker := time.NewTicker(time.Millisecond * 16)
			last_start := time.Now()

			var winner *websocket.Conn
			var loser *websocket.Conn

		game_me_daddy:
			for {
				select {
				case fire := <-fires:
					player := game.Players[0]
					if fire.name == 'b' {
						player = game.Players[1]
                    }

                    if PlayerFire(player) {
                        game.Bullets = append(game.Bullets, CreateBulletFromPlayer(player, 1.0))
                        hack = append(hack, Debug{
                            int64(time.Since(starting_of_game).Milliseconds()),
                            fmt.Sprintf("fire%v", fire.name),
                        })
                    }

				case <-ticker.C:
					// 6. part 1 : calculate the time difference between each loop.
					diff := time.Since(last_start).Microseconds()
					last_start = time.Now()

					// 6. do all the collision / updating
                    for i := 0; i < len(game.Bullets); i += 1 {
						UpdateBullet(game.Bullets[i], diff)
					}

                    checkBulletCollisions(&game)

                    for i := 0; i < len(game.Bullets); i += 1 {
						if game.Players[0].Geo.HasCollision(&game.Bullets[i].Geo) {
							winner = playerA
							loser = playerB
							break game_me_daddy
						}
						if game.Players[1].Geo.HasCollision(&game.Bullets[i].Geo) {
							winner = playerB
							loser = playerA
							break game_me_daddy
						}
					}

					stats.AddDelta(diff)
				}
			}

			// Part 7. Send out the winner / loser message and close down the
			// suckets
			winnerMsg := CreateWinnerMessage(stats)
			loserMsg := CreateLoserMessage()

            winner.WriteJSON(winnerMsg)
            loser.WriteJSON(loserMsg)

            if stats.FrameBuckets[0] > 1000 {
                log.Printf("COLLISIONS\n")
                for _, debug := range hack {
                    log.Printf("%+v\n", debug)
                }
                log.Println()
            }

            RemoveActiveGame()
		}()
	}
}
