package websockethelper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vmihailenco/msgpack/v5"
)

const (
	// Constant for the amount of items a channel can hold before blocking
	channelSize = 50

	pongWait = 15 * time.Second
)

// SocketClient is the struct that stores the webSocket Connection
type SocketClient struct {
	sendChannel chan SocketSendMessage
	readChannel chan SocketMessage
	callbacks   map[string][]func(SocketMessage)
	hub         *WebSocketHub
	conn        *websocket.Conn
}

// SocketMessage is the struct that we send and receive from the javascript side
//easyjson:json
type SocketMessage struct {
	EventName string `json:"eventName"`
	Content   string `json:"content"`
	Error     string `json:"error"`
}

// SocketMessage is the struct that we send and receive from the javascript side
//easyjson:json
type SocketSendMessage struct {
	EventName string `json:"eventName"`
	Content   []byte `json:"content"`
	Error     string `json:"error"`
}

// ServeWS is the handler for requests to connect via websocket
func ServeWS(hub *WebSocketHub, w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{
		// ReadBufferSize:   2048,
		// WriteBufferSize:  2048,
		HandshakeTimeout: 1000,
		CheckOrigin:      func(r *http.Request) bool { return true },
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	client := &SocketClient{
		sendChannel: make(chan SocketSendMessage, channelSize),
		readChannel: make(chan SocketMessage, channelSize),
		callbacks:   make(map[string][]func(SocketMessage)),
		hub:         hub,
		conn:        ws,
	}
	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}

func (s *SocketClient) writePump() {
	ticker := time.NewTicker(time.Millisecond * 30)
	defer func() {
		s.conn.Close()
		s.hub.unregister <- s
	}()
	for {
		select {
		case msg, ok := <-s.sendChannel:
			if len(msg.Content) > 0 && ok {
				w, err := s.conn.NextWriter(websocket.BinaryMessage)
				if err != nil {
					fmt.Printf("failed to retrieve next writer: %v\n", err)
					break
				}
				// enc := msgpack.NewEncoder(w)
				// enc.SetCustomStructTag("json")
				//
				// enc.Encode(&msg)

				b, err := msgpack.Marshal(&msg)
				if err != nil {
					fmt.Printf("failed to encode message: %s\n", err)
					break
				}
				r := bytes.NewReader(b)
				for {
					_, err = io.Copy(w, r)
					if err == nil {
						break
					}
				}
				if err != nil {
					fmt.Printf("error while writing json: %v\n", err)
					break
				}
				err = w.Close()
				if err != nil {
					fmt.Printf("failed to close writer: %s\n", err)
				}
			}
		case <-ticker.C:
			if err := s.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func (s *SocketClient) readPump() {
	defer func() {
		s.conn.Close()
		s.hub.unregister <- s
	}()
	for {
		_, b, err := s.conn.ReadMessage()
		if err == nil {
			msg := SocketMessage{}
			err = json.Unmarshal(b, &msg)
			if err == nil && len(s.readChannel) < channelSize {
				hub.RLock()
				if action, ok := s.callbacks[msg.EventName]; ok {
					for _, ac := range action {
						ac(msg)
					}
				}
				hub.RUnlock()
			}
		} else {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			return
		}
		time.Sleep(time.Millisecond * 25)
	}
}
