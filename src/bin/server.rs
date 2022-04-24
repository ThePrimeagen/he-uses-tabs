use futures::{
	sink::SinkExt,
	stream::{SplitSink, SplitStream},
	StreamExt as FuturesStreamExt,
};
use serde::{Deserialize, Serialize};
use serde_repr::{Deserialize_repr, Serialize_repr};
use std::{ops::{Add, Mul, Sub}, sync::{Arc, atomic::{AtomicUsize, Ordering}}};
use tokio::{
	io::AsyncWriteExt,
	net::{TcpListener, TcpStream},
	sync::mpsc::channel,
	time::{interval, Duration, Instant},
};
use tokio_stream::StreamExt as TokioStreamExt;
use tokio_tungstenite::{accept_async, tungstenite::Message, WebSocketStream};

const PLAYER_WIDTH: f32 = 100.0;
const PLAYER_HEIGHT: f32 = 100.0;
const BULLET_WIDTH: f32 = 35.0;
const BULLET_HEIGHT: f32 = 3.0;

#[derive(Debug, Clone, Serialize_repr, Deserialize_repr, PartialEq)]
#[repr(u8)]
pub enum MessageType {
	ReadyUp = 0,
	Play = 1,
	Fire = 2,
	GameOver = 3,
	Kill = 4,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct GameMessage {
	#[serde(rename = "type")]
	pub msg_type: MessageType,
	pub msg: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct AABB {
	pub x: f32,
	pub y: f32,
	pub w: f32,
	pub h: f32,
}

impl AABB {
	// Duplicating the logic in the go code
	pub fn collides(&self, other: &Self) -> bool {
		if self.x > other.x + other.w || other.x > self.x + self.w {
			return false;
		}

		if self.y > other.y + other.h || other.y > self.y + self.h {
			return false;
		}
		true
	}
}

#[derive(Debug, Copy, Clone, PartialEq)]
pub struct Vec2D(pub f32, pub f32);

impl Add for Vec2D {
	type Output = Vec2D;

	fn add(self, other: Vec2D) -> Vec2D { Vec2D(self.0 + other.0, self.1 + other.1) }
}

impl Sub for Vec2D {
	type Output = Vec2D;

	fn sub(self, other: Vec2D) -> Vec2D { Vec2D(self.0 - other.0, self.1 - other.1) }
}

impl Mul<f32> for Vec2D {
	type Output = Vec2D;

	fn mul(self, rhs: f32) -> Self::Output { Vec2D(self.0 * rhs, self.1 * rhs) }
}

#[derive(Debug)]
pub struct Player {
    id: usize,
	geo: AABB,
	dir: Vec2D,
	reload_time: u32,
	last_fired: Option<Instant>,
}

impl Player {
	pub fn create_bullet(&self, speed: f32) -> Bullet {
		// Sus logic
		let aabb = if self.dir.0 == 1.0 {
			AABB {
				x: self.geo.x + self.geo.w + 1.0,
				y: 0.0,
				w: BULLET_WIDTH,
				h: BULLET_HEIGHT,
			}
		}
		else {
			AABB {
				x: self.geo.x - BULLET_WIDTH - 1.0,
				y: 0.0,
				w: BULLET_WIDTH,
				h: BULLET_HEIGHT,
			}
		};
		Bullet {
			aabb,
			velocity: self.dir * speed,
		}
	}
}

#[derive(Debug)]
pub struct Bullet {
	aabb: AABB,
	velocity: Vec2D,
}

impl Bullet {
	pub fn update(&mut self, dt: Duration) {
		self.aabb.x += self.velocity.0 * dt.as_millis() as f32;
		self.aabb.y += self.velocity.1 * dt.as_millis() as f32;
	}
}

#[derive(Debug)]
pub struct GameState {
	player_a: Player,
	player_b: Player,
	// Using a Vec here is really jank
	bullets: Vec<Bullet>,
}

async fn wait_for_ready(
	stream: &mut SplitStream<WebSocketStream<TcpStream>>,
	sink: &mut SplitSink<WebSocketStream<TcpStream>, Message>,
) {
	sink.send(Message::Text(
		serde_json::to_string(&GameMessage {
			msg_type: MessageType::ReadyUp,
			msg: None,
		})
		.unwrap(),
	))
	.await
	.unwrap();
	while let Some(m) = futures::StreamExt::next(stream).await {
		if let Ok(Message::Text(t)) = m {
			let msg = serde_json::from_str::<GameMessage>(&t).unwrap();
			if let MessageType::ReadyUp = msg.msg_type {
				break;
			}
		}
	}
}

fn fire(now: Instant, player: &mut Player, speed: f32) -> Option<Bullet> {
	let can_fire = match player.last_fired {
		Some(last) => now - last >= Duration::from_millis(player.reload_time as u64),
		None => true,
	};

	if !can_fire {
		return None;
	}

    player.last_fired = Some(now);
	Some(player.create_bullet(speed))
}

pub struct GameStats {
    frame_buckets: [u128; 8]
}

impl GameStats {
    pub fn new() -> GameStats {
        return GameStats {
            frame_buckets: [0; 8],
        };
    }

    pub fn add_delta(&mut self, delta: u128) {
        if delta > 40_999 {
            self.frame_buckets[7] += 1;
        } else if delta > 30_999 {
            self.frame_buckets[6] += 1;
        } else if delta > 25_999 {
            self.frame_buckets[5] += 1;
        } else if delta > 23_999 {
            self.frame_buckets[4] += 1;
        } else if delta > 21_999 {
            self.frame_buckets[3] += 1;
        } else if delta > 19_999 {
            self.frame_buckets[2] += 1;
        } else if delta > 17_999 {
            self.frame_buckets[1] += 1;
        } else {
            self.frame_buckets[0] += 1;
        }
    }
}

impl Into<String> for GameStats {
    fn into(self) -> String {
        return self.frame_buckets
            .iter()
            .map(|x| x.to_string())
            .collect::<Vec<String>>()
            .join(",");
    }
}

#[tokio::main]
async fn main() {
	let socket_addr = "0.0.0.0:42069";
	let listener = TcpListener::bind(socket_addr).await.unwrap();
    let active_games = Arc::new(AtomicUsize::new(0));

	// Accept pairs of conections
	loop {
		let (conna, _) = listener.accept().await.unwrap();
		let (connb, _) = listener.accept().await.unwrap();

        let active_games = active_games.clone();
		tokio::spawn(async move {
			let wsa = accept_async(conna).await.unwrap();
			let wsb = accept_async(connb).await.unwrap();

			let (mut sinka, mut streama) = wsa.split();
			let (mut sinkb, mut streamb) = wsb.split();

            // 1. Wait for ready
			// Exchange ready-up messages
			tokio::join!(
				wait_for_ready(&mut streama, &mut sinka),
				wait_for_ready(&mut streamb, &mut sinkb)
			);

            // 2. create game initial state
			let mut state = GameState {
                player_a: Player {
                    id: 1,
                    geo: AABB {
                        x: 2500.0,
                        y: 0.0,
                        w: PLAYER_WIDTH,
                        h: PLAYER_HEIGHT,
                    },
                    dir: Vec2D(-1.0, 0.0),
                    reload_time: 180,
                    last_fired: None,
                },
                player_b: Player {
                    id: 2,
                    geo: AABB {
                        x: -2500.0,
                        y: 0.0,
                        w: PLAYER_WIDTH,
                        h: PLAYER_HEIGHT,
                    },
                    dir: Vec2D(1.0, 0.0),
                    reload_time: 300,
                    last_fired: None,
                },
                bullets: Vec::new(),
			};

            let mut stats = GameStats::new();

			// Send "play" message
			let play = GameMessage {
				msg_type: MessageType::Play,
				msg: None,
			};
			sinka
				.send(Message::Text(serde_json::to_string(&play).unwrap()))
				.await
				.unwrap();
			sinkb
				.send(Message::Text(serde_json::to_string(&play).unwrap()))
				.await
				.unwrap();

			// Combine both message sources into one
			let streama = futures::StreamExt::map(streama, |x| {
                return (b'a', x);
            });
			let streamb = futures::StreamExt::map(streamb, |x| {
                return (b'b', x);
            });
			let mut merged_messages = streama.merge(streamb);

			let (s, mut r) = channel(32);

            active_games.fetch_add(1, Ordering::Relaxed);

			tokio::select! {
				// Infinite message relay
				_ = async {
					while let Some((player, Ok(Message::Text(t)))) =
					futures::StreamExt::next(&mut merged_messages).await {
						s.send((player, t)).await.unwrap();
					}
				} => (),
				// Game loop
				_ = async {
					let mut interval = interval(Duration::from_micros(16000));
					let mut lastloop = Instant::now();
					let mut first_frame = true;
					let winner = 'game_loop: loop {
						let start = interval.tick().await;
						let delta = start - lastloop;

						if !first_frame && delta < Duration::from_millis(1) {
							tokio::io::stdout().write("FRAME DROPPED\n".as_bytes()).await.unwrap();
						}

						// let time = format!("FPS: {}\n", 1.0 / delta.as_secs_f32());
						// tokio::io::stdout().write(time.as_bytes()).await.unwrap();

						// Process messages
						while let Ok((player, t)) =
							r.try_recv()
						{
							let msg = serde_json::from_str::<GameMessage>(&t).unwrap();
							if let MessageType::Fire = msg.msg_type {
								match player {
									b'a' => {
										if let Some(b) = fire(start, &mut state.player_a, 1.0) {
											state.bullets.push(b);
										}
									}
									b'b' => {
										if let Some(b) = fire(start, &mut state.player_b, 1.0) {
											state.bullets.push(b);
										}
									}
									_ => (),
								}
							}
						}

						// Update all bullets
						for bullet in &mut state.bullets {
							bullet.update(delta);
						}

						// Check for bullet collisions
						// This is duplicating the logic of the Go code - which can only handle max 1 collision per game loop...
						'outer: for a in 0..state.bullets.len() {
							for b in a + 1..state.bullets.len() {
								let bullet_a = &state.bullets[a];
								let bullet_b = &state.bullets[b];
								if !bullet_a.aabb.collides(&bullet_b.aabb) {
									continue;
								}
								state.bullets.swap_remove(b);
								state.bullets.swap_remove(a);
								break 'outer;
							}
						}

						// Check for player collisions
						for bullet in &mut state.bullets {
							if bullet.aabb.collides(&state.player_a.geo) {
								break 'game_loop (b'a');
							}
							if bullet.aabb.collides(&state.player_b.geo) {
								break 'game_loop (b'b');
							}
						}

						let finish = Instant::now();

						lastloop = start;
						first_frame = false;
                        stats.add_delta(delta.as_millis());
					};

                    let ags = active_games.load(Ordering::Relaxed);
                    let stats = <GameStats as Into<std::string::String>>::into(stats);
                    let winner_msg = format!("winner({})___{}", ags, stats);

					// Send game over messages
					let winner_msg = serde_json::to_string(&GameMessage {
						msg_type: MessageType::GameOver,
						msg: Some(winner_msg),
					})
					.unwrap();

                    active_games.fetch_sub(1, Ordering::Relaxed);

					let loser_msg = serde_json::to_string(&GameMessage {
						msg_type: MessageType::GameOver,
						msg: Some("loser".to_string()),
					})
					.unwrap();
					match winner {
						b'a' => {
							sinka.send(message::text(winner_msg)).await.unwrap();
							sinkb.send(message::text(loser_msg)).await.unwrap();
						}
						b'b' => {
							sinkb.send(message::text(winner_msg)).await.unwrap();
							sinka.send(Message::Text(loser_msg)).await.unwrap();
						}
						_ => (),
					}
				} => (),
			}
		});
	}
}

