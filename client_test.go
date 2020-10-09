package websockethelper

import (
	"log"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func wsHandler(w http.ResponseWriter, r *http.Request) {
	ServeWS(hub, w, r)
}

func LaunchServer() {
	http.HandleFunc("/ws", wsHandler)
	log.Fatal(http.ListenAndServe(":9998", nil))
}

const (
	maxClientCons = 200
)

func TestMultipleClientConnections(t *testing.T) {
	go LaunchServer()
	go hub.Run()
	for count := 0; count < maxClientCons; count++ {
		u := url.URL{Scheme: "ws", Host: "localhost:9998", Path: "/ws"}
		log.Printf("Connecting to %s #%d\n", u.String(), count+1)
		c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			t.Error(err)
		}
		c.Close()
		time.Sleep(time.Millisecond * 50)
	}
}
