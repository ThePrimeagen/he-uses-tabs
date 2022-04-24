package game

import (
	"fmt"
	"strings"
	"sync/atomic"
)

var ActiveGames int64

func AddActiveGame() {
    atomic.AddInt64(&ActiveGames, 1)
}

func RemoveActiveGame() {
    atomic.AddInt64(&ActiveGames, -1)
}

type GameStats struct {
    FrameBuckets [8]int64
}

func NewGameStat() *GameStats {
    return &GameStats {
        FrameBuckets: [8]int64{0, 0, 0, 0, 0, 0, 0, 0},
    }
}

func (g *GameStats) String() string {
    out := make([]string, 8)
    for idx, num := range g.FrameBuckets {
        out[idx] = fmt.Sprintf("%d", num)
    }

    return strings.Join(out, ",")
}

func (g *GameStats) AddDelta(delta int64) {
    if (delta > 40_999) {
        g.FrameBuckets[7] += 1
    } else if (delta > 30_999) {
        g.FrameBuckets[6] += 1
    } else if (delta > 25_999) {
        g.FrameBuckets[5] += 1
    } else if (delta > 23_999) {
        g.FrameBuckets[4] += 1
    } else if (delta > 21_999) {
        g.FrameBuckets[3] += 1
    } else if (delta > 19_999) {
        g.FrameBuckets[2] += 1
    } else if (delta > 17_999) {
        g.FrameBuckets[1] += 1
    } else {
        g.FrameBuckets[0] += 1
    }
}

