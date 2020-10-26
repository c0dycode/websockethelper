// +build fast

package websockethelper

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/valyala/fasthttp"
)

const (
	// Constant for the amount of items a channel can hold before blocking
	channelSize = 50

	pongWait = 15 * time.Second
)

// SocketClient is the struct that stores the webSocket Connection
type SocketClient struct {
	sendChannel chan SocketMessage
	readChannel chan SocketMessage
	callbacks   map[string][]func(SocketMessage)
	hub         *WebSocketHub
	conn        *websocket.Conn
}

// SocketMessage is the struct that we send and receive from the javascript side
type SocketMessage struct {
	EventName string `json:"eventName"`
	Content   string `json:"content"`
	Error     string `json:"error"`
}

// ServeWS is the handler for requests to connect via websocket
func ServeWS(hub *WebSocketHub, ctx *fasthttp.RequestCtx) {
	var upgrader = websocket.FastHTTPUpgrader{
		ReadBufferSize:   2048,
		WriteBufferSize:  2048,
		HandshakeTimeout: 1000,
	}
	err := upgrader.Upgrade(ctx, wsLoop)
	if err != nil {
		log.Fatal(err)
	}
}

func wsLoop(ws *websocket.Conn) {
	defer ws.Close()
	client := &SocketClient{
		sendChannel: make(chan SocketMessage, channelSize),
		readChannel: make(chan SocketMessage, channelSize),
		callbacks:   make(map[string][]func(SocketMessage)),
		hub:         hub,
		conn:        ws,
	}
	client.hub.register <- client

	go client.writePump()
	client.readPump()

	client.hub.unregister <- client
}

func (s *SocketClient) writePump() {
	for {
		select {
		case msg, ok := <-s.sendChannel:
			if len(msg.Content) > 0 && ok {
				err := s.conn.WriteJSON(msg)
				if err != nil {
					fmt.Printf("error while writing json: %v\n", err)
					break
				}
				break
			}
		default:
			if err := s.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				break
			}
			break
		}
		time.Sleep(time.Millisecond * 50)
	}
}

func (s *SocketClient) readPump() {
	for {
		_, b, err := s.conn.ReadMessage()
		if err == nil {
			var msg SocketMessage
			err = json.Unmarshal(b, &msg)
			if err == nil && len(s.readChannel) < channelSize {
				hub.RLock()
				defer hub.RUnlock()
				if action, ok := s.callbacks[msg.EventName]; ok {
					for _, ac := range action {
						ac(msg)
					}
				}
			}
		} else {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		time.Sleep(time.Millisecond * 50)
	}
}
