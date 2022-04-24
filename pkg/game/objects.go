package game

import (
	"time"
)

type Vector2D = [2]float64;

/*
type Updatable interface {
    Update(xDelta, yDelta float64)
    GetVelocity() *Vector2D
}

func UpdateItems[T Updatable](items []T, delta uint) {
    for _, item := range items {
        vel := item.GetVelocity()
        item.Update(vel[0] * float64(delta), vel[1] * float64(delta))
    }
}
*/

type Player struct {
    Geo AABB
    Dir Vector2D
    FireRate int64
    lastFireTime int64
}

type Bullet struct {
    Geo AABB
    Vel Vector2D
}

const PLAYER_WIDTH = 100
const PLAYER_HEIGHT = 100
const BULLET_WIDTH = 35
const BULLET_HEIGHT = 3

func NewPlayer(pos, dir Vector2D, fireRate int64) *Player {
    return &Player {
        Geo: AABB {
            X: pos[0],
            Y: pos[1],
            Width: PLAYER_WIDTH,
            Height: PLAYER_HEIGHT,
        },
        Dir: dir,
        FireRate: fireRate,
        lastFireTime: 0,
    }
}

func PlayerFire(p *Player) bool {
    now := time.Now().UnixMilli()

    if p.FireRate > now - p.lastFireTime {
        return false
    }

    p.lastFireTime = now
    return true
}

func NewBullet() Bullet {
    return Bullet {
        AABB {0, 0, BULLET_WIDTH, BULLET_HEIGHT},
        Vector2D {0, 0},
    }
}

func CreateBulletFromPlayer(player *Player, speed float64) *Bullet {
    bullet := NewBullet()

    if player.Dir[0] == 1 {
        bullet.Geo.SetPosition(
            player.Geo.X + player.Geo.Width + 1,
            0);
    } else {
        bullet.Geo.SetPosition(
            player.Geo.X - BULLET_WIDTH - 1,
            0);
    }

    bullet.Vel[0] = player.Dir[0] * speed;
    bullet.Vel[1] = player.Dir[1] * speed;

    return &bullet
}

func UpdateBullet(b *Bullet, delta int64) {
    millis := delta / 1000;

    b.Geo.X += float64(millis) * b.Vel[0]
    b.Geo.Y += float64(millis) * b.Vel[1]
}

