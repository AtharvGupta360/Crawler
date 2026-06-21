package ws

import (
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024 // 512 KB
)

// Message is an outbound message destined for a specific user.
type Message struct {
	UserID  uuid.UUID
	Payload any
}

// Client represents a single WebSocket connection for a user.
type Client struct {
	UserID uuid.UUID
	hub    *Hub
	conn   *websocket.Conn
	send   chan any
}

// Hub maintains all active WebSocket clients and fans out messages.
type Hub struct {
	mu         sync.RWMutex
	clients    map[uuid.UUID]*Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan Message
	logger     *slog.Logger
}

// NewHub creates an initialised Hub. Call Run() in a goroutine.
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients:    make(map[uuid.UUID]*Client),
		register:   make(chan *Client, 32),
		unregister: make(chan *Client, 32),
		broadcast:  make(chan Message, 256),
		logger:     logger.With("component", "ws-hub"),
	}
}

// Run processes hub events. Must be called in its own goroutine.
func (h *Hub) Run() {
	h.logger.Info("WebSocket hub started")
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c.UserID] = c
			h.mu.Unlock()
			h.logger.Debug("client connected", "user_id", c.UserID)

		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c.UserID]; ok {
				delete(h.clients, c.UserID)
				close(c.send)
			}
			h.mu.Unlock()
			h.logger.Debug("client disconnected", "user_id", c.UserID)

		case msg := <-h.broadcast:
			h.mu.RLock()
			c, ok := h.clients[msg.UserID]
			h.mu.RUnlock()
			if ok {
				select {
				case c.send <- msg.Payload:
				default:
					// Slow consumer — drop and disconnect
					h.mu.Lock()
					delete(h.clients, msg.UserID)
					h.mu.Unlock()
					close(c.send)
				}
			}
		}
	}
}

// SendToUser enqueues a payload to be delivered to the given user's WebSocket.
// Safe to call from any goroutine. No-op if the user has no active connection.
func (h *Hub) SendToUser(userID uuid.UUID, payload any) {
	select {
	case h.broadcast <- Message{UserID: userID, Payload: payload}:
	default:
		h.logger.Warn("ws broadcast channel full, dropping message", "user_id", userID)
	}
}

// Register adds a new client to the hub.
func (h *Hub) Register(c *Client) {
	h.register <- c
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(c *Client) {
	h.unregister <- c
}

// NewClient creates a client bound to this hub and starts its read/write pumps.
// The caller should NOT write to the connection after this.
func (h *Hub) NewClient(userID uuid.UUID, conn *websocket.Conn) *Client {
	c := &Client{
		UserID: userID,
		hub:    h,
		conn:   conn,
		send:   make(chan any, 64),
	}
	h.Register(c)
	go c.writePump()
	go c.readPump()
	return c
}

// readPump discards inbound messages and handles pong/close frames.
func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))   //nolint:errcheck
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait)) //nolint:errcheck
		return nil
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			break
		}
	}
}

// writePump serialises JSON payloads and sends pings to keep the connection alive.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case payload, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait)) //nolint:errcheck
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{}) //nolint:errcheck
				return
			}
			if err := c.conn.WriteJSON(payload); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait)) //nolint:errcheck
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
