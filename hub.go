package websockethelper

import (
	"fmt"
	"sync"
	"time"
)

// LogType can be used to differentiate between different errorlevels
type LogType int

const (
	// LogOk for general logs
	LogOk LogType = 0
	// LogInfo is used for warnings or information
	LogInfo LogType = 1
	// LogError is used for errors
	LogError LogType = 2
)

var (
	hub       *WebSocketHub
	callbacks map[string][]func(SocketMessage)
	shutdown  bool
)

// WebSocketHub keeps track of the connections and registers and unregisters them
type WebSocketHub struct {
	clients     map[*SocketClient]bool
	register    chan *SocketClient
	unregister  chan *SocketClient
	broadcast   chan SocketSendMessage
	logMessages []SocketSendMessage
	sync.RWMutex
}

func init() {
	// logBuffer = make(chan SocketMessage, logBufferSize)
	callbacks = make(map[string][]func(SocketMessage))
	hub = GetHub()
}

// GetHub returns a new WebSocketHub
func GetHub() *WebSocketHub {
	if hub == nil {
		hub = &WebSocketHub{
			clients:    make(map[*SocketClient]bool),
			register:   make(chan *SocketClient),
			unregister: make(chan *SocketClient),
			broadcast:  make(chan SocketSendMessage, 250),
		}
	}
	return hub
}

func shutdownHub() {
	shutdown = true
}

// Run starts the WebSocketHub and manages connections
func (wh *WebSocketHub) Run() {
	ticker := time.NewTicker(time.Millisecond * 20)
	for {
		select {
		case client := <-wh.register:
			wh.Lock()
			wh.clients[client] = true
			client.callbacks = callbacks

			for _, msg := range wh.logMessages {
				client.sendChannel <- msg
			}
			wh.Unlock()
		case client := <-wh.unregister:
			wh.Lock()
			if _, ok := wh.clients[client]; ok {
				delete(wh.clients, client)
				close(client.sendChannel)
				close(client.readChannel)
			}
			wh.Unlock()
		case msg := <-wh.broadcast:
			wh.Lock()
			if msg.EventName == "LogMessage" {
				wh.logMessages = append(wh.logMessages, msg)
			}
			for client := range wh.clients {
				select {
				case client.sendChannel <- msg:
				default:
					break
				}
			}
			wh.Unlock()
		case <-ticker.C:
			if shutdown {
				return
			}
		}
	}
}

func broadcastMessages(msgToSend *SocketSendMessage) {
	if hub != nil {
		hub.broadcast <- *msgToSend
	}
}

// RegisterCallback allows you to register functions outside of this package to be called on specific eventnames
func RegisterCallback(eventName string, f func(SocketMessage)) {
	for client := range hub.clients {
		var found bool = false
		if ac := client.callbacks[eventName]; ac != nil {
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
func SendMessageToWS(message *SocketSendMessage) {
	broadcastMessages(message)
}

// SendLogMessageToWS adds the message formatted to the SendChannel
func SendLogMessageToWS(message string, errorType LogType) {
	t := time.Now()
	y, mon, d := t.Date()
	h, m, sec := t.Clock()
	formattedMessage := fmt.Sprintf("%d-%d-%d %02d:%02d:%02d : %s", y, mon, d, h, m, sec, message)

	var msg SocketSendMessage
	msg.Content = []byte(formattedMessage)
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
	broadcastMessages(&msg)
}
