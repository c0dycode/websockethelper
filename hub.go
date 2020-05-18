package websockethelper

import (
	"fmt"
	"sync"
	"time"
)

// LogType can be used to differentiate between different errorlevels
type LogType int

const (
	logBufferSize = 250

	// LogOk for general logs
	LogOk LogType = 0
	// LogInfo is used for warnings or information
	LogInfo LogType = 1
	// LogError is used for errors
	LogError LogType = 2
)

var (
	hub       *WebSocketHub
	logBuffer chan SocketMessage
	callbacks map[string][]func(SocketMessage)
	sm        sync.RWMutex
)

// WebSocketHub keeps track of the connections and registers and unregisters them
type WebSocketHub struct {
	clients    map[*SocketClient]bool
	register   chan *SocketClient
	unregister chan *SocketClient
	broadcast  chan SocketMessage
}

func init() {
	logBuffer = make(chan SocketMessage, logBufferSize)
	callbacks = make(map[string][]func(SocketMessage))
}

// NewHub returns a new WebSocketHub
func NewHub() *WebSocketHub {
	hub = &WebSocketHub{
		clients:    make(map[*SocketClient]bool),
		register:   make(chan *SocketClient),
		unregister: make(chan *SocketClient),
		broadcast:  make(chan SocketMessage, 250),
	}
	return hub
}

// Run starts the WebSocketHub and manages connections
func (wh *WebSocketHub) Run() {
	for {
		select {
		case client := <-wh.register:
			wh.clients[client] = true
			sm.Lock()
			client.callbacks = callbacks
			sm.Unlock()
		case client := <-wh.unregister:
			if _, ok := wh.clients[client]; ok {
				delete(wh.clients, client)
				close(client.sendChannel)
				close(client.readChannel)
			}
		case msg := <-wh.broadcast:
			if len(wh.clients) == 0 {
				logBuffer <- msg
			}
			for client := range wh.clients {
				select {
				case client.sendChannel <- msg:
				default:
					close(client.sendChannel)
					delete(wh.clients, client)
				}
			}
		}
	}
}

func broadcastMessages(msgToSend SocketMessage) {
	hub.broadcast <- msgToSend
}

// RegisterCallback allows you to register functions outside of this package to be called on specific eventnames
func RegisterCallback(eventName string, f func(SocketMessage)) {
	sm.Lock()
	defer sm.Unlock()
	for client := range hub.clients {
		var found bool = false
		if ac, _ := client.callbacks[eventName]; ac != nil {
			for _, a := range ac {
				if &a == &f {
					found = true
				}
			}
		}
		if !found {
			client.callbacks[eventName] = append(client.callbacks[eventName], f)
		}
	}
	callbacks[eventName] = append(callbacks[eventName], f)
}

// SendMessageToWS adds the message formatted to the SendChannel
func SendMessageToWS(message SocketMessage) error {
	broadcastMessages(message)
	return nil
}

// SendLogMessageToWS adds the message formatted to the SendChannel
func SendLogMessageToWS(message string, errorType LogType) {
	t := time.Now()
	y, mon, d := t.Date()
	h, m, sec := t.Clock()
	formattedMessage := fmt.Sprintf("%d-%d-%d %02d:%02d:%02d : %s", y, mon, d, h, m, sec, message)

	var msg SocketMessage
	msg.Content = formattedMessage
	msg.EventName = "LogMessage"

	switch errorType {
	case LogOk:
		msg.Error = "ok"
		break
	case LogInfo:
		msg.Error = "info"
		break
	case LogError:
		msg.Error = "error"
		break
	}
	broadcastMessages(msg)
}
