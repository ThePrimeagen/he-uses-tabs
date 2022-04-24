package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "0.0.0.0:42069", "http service address")
var upgrader = websocket.Upgrader{} // use default options

func main() {
    flag.Parse()
	log.SetFlags(0)

    conns := make(chan *websocket.Conn, 10)// *server.Conn, 10)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        c, err := upgrader.Upgrade(w, r, nil)
        if err != nil {
            log.Println("Connection upgrade failed.")
            return;
        }

        conns <- c
    })

	log.Fatal(http.ListenAndServe(*addr, nil))
}


