# syntax=docker/dockerfile:1
FROM rust:latest AS FETCH_THE_EFFIN_RUST
WORKDIR /app
COPY Cargo.toml ./Cargo.toml
COPY Cargo.lock ./Cargo.lock
COPY src ./src
RUN rustup default nightly
RUN cargo build --release --bin server
RUN cargo install --path .

FROM debian:latest
EXPOSE 42069
WORKDIR /app
RUN apt update && apt install -y ca-certificates
COPY --from=FETCH_THE_EFFIN_RUST /usr/local/cargo/bin/server /app
CMD ["sh", "-c", "/app/server"]



