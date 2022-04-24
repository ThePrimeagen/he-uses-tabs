FROM golang:1.18rc1
WORKDIR /app
COPY cmd cmd
COPY pkg pkg
COPY go.mod go.mod
COPY go.sum go.sum
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o server -a -ldflags '-extldflags "-static"' cmd/server/main.go
CMD ["sh", "-c", "./server"]
