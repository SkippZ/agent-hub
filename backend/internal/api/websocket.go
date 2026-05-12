package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WSMessage struct {
	Type    string `json:"type"`
	Data    string `json:"data,omitempty"`
	Status  string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
}

type WSHub struct {
	mu      sync.RWMutex
	conns   map[string]map[*websocket.Conn]bool
}

func NewWSHub() *WSHub {
	return &WSHub{conns: make(map[string]map[*websocket.Conn]bool)}
}

func (h *WSHub) Broadcast(sessionID string, msg WSMessage) {
	data, _ := json.Marshal(msg)
	h.mu.RLock()
	conns := h.conns[sessionID]
	h.mu.RUnlock()

	for conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("ws write error: %v", err)
			conn.Close()
			h.removeConn(sessionID, conn)
		}
	}
}

func (h *WSHub) Subscribe(sessionID string, conn *websocket.Conn) {
	h.mu.Lock()
	if h.conns[sessionID] == nil {
		h.conns[sessionID] = make(map[*websocket.Conn]bool)
	}
	h.conns[sessionID][conn] = true
	h.mu.Unlock()
}

func (h *WSHub) Unsubscribe(sessionID string, conn *websocket.Conn) {
	h.removeConn(sessionID, conn)
}

func (h *WSHub) removeConn(sessionID string, conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.conns[sessionID], conn)
	if len(h.conns[sessionID]) == 0 {
		delete(h.conns, sessionID)
	}
	h.mu.Unlock()
}

func (h *WSHub) HandleWS(w http.ResponseWriter, r *http.Request, sessionID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade: %v", err)
		return
	}
	h.Subscribe(sessionID, conn)

	defer func() {
		h.Unsubscribe(sessionID, conn)
		conn.Close()
	}()

	// Read loop — receives messages from client to forward to agent
	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var msg WSMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			continue
		}
		// Messages from the UI are forwarded via the message queue channel
		// The handler will need to pick these up
		_ = msg
	}
}
