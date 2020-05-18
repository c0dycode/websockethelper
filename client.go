package websockethelper

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Constant for the amount of items a channel can hold before blocking
	channelSize = 100
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
func ServeWS(hub *WebSocketHub, w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Upgrade(w, r, nil, 2048, 2048)
	if err != nil {
		log.Fatal(err)
	}
	client := &SocketClient{
		sendChannel: make(chan SocketMessage, channelSize),
		readChannel: make(chan SocketMessage, channelSize),
		callbacks:   make(map[string][]func(SocketMessage)),
		hub:         hub,
		conn:        ws,
	}
	for {
		if len(logBuffer) > 0 {
			if msg, ok := <-logBuffer; ok {
				client.sendChannel <- msg
			}
		} else {
			break
		}
	}
	client.hub.register <- client
	go client.readPump()
	go client.writePump()
}

func (s *SocketClient) writePump() {
	defer func() {
		s.hub.unregister <- s
		s.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-s.sendChannel:
			if len(msg.Content) > 0 && ok {
				err := s.conn.WriteJSON(msg)
				if err != nil {
					fmt.Printf("error while writing json: %v\n", err)
					return
				}
				break
			}
		default:
			break
		}
		time.Sleep(time.Millisecond * 50)
	}
}

func (s *SocketClient) readPump() {
	defer func() {
		s.hub.unregister <- s
		s.conn.Close()
	}()
	for {
		_, b, err := s.conn.ReadMessage()
		if err == nil {
			var msg SocketMessage
			err = json.Unmarshal(b, &msg)
			if err == nil && len(s.readChannel) < channelSize {
				sm.RLock()
				defer sm.RUnlock()
				if action, ok := s.callbacks[msg.EventName]; ok {
					for _, ac := range action {
						ac(msg)
					}
				}
			}
		} else {
			return
		}
		time.Sleep(time.Millisecond * 50)
	}
}
