package ws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Role differentiates a player connection from an admin's monitor connection.
type Role string

const (
	RolePlayer Role = "player"
	RoleAdmin  Role = "admin"
)

type Client struct {
	hub     *Hub
	conn    *websocket.Conn
	send    chan []byte
	GameID  string
	UserID  string // empty for admin
	Role    Role
	OnClose func()
}

type Hub struct {
	mu      sync.RWMutex
	rooms   map[string]map[*Client]struct{} // gameID -> set of clients
	OnRecv  func(c *Client, msg []byte)
	OnJoin  func(c *Client)
	OnLeave func(c *Client)
}

func NewHub() *Hub {
	return &Hub{rooms: make(map[string]map[*Client]struct{})}
}

func (h *Hub) add(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	room, ok := h.rooms[c.GameID]
	if !ok {
		room = make(map[*Client]struct{})
		h.rooms[c.GameID] = room
	}
	room[c] = struct{}{}
}

func (h *Hub) remove(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if room, ok := h.rooms[c.GameID]; ok {
		delete(room, c)
		if len(room) == 0 {
			delete(h.rooms, c.GameID)
		}
	}
}

// Broadcast sends a JSON-encoded envelope to every client in the room.
func (h *Hub) Broadcast(gameID string, msg any) {
	b, err := json.Marshal(msg)
	if err != nil {
		log.Printf("broadcast marshal: %v", err)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	room := h.rooms[gameID]
	for c := range room {
		select {
		case c.send <- b:
		default:
			// slow consumer; drop
		}
	}
}

// BroadcastTo sends a message only to clients matching the predicate.
func (h *Hub) BroadcastTo(gameID string, msg any, match func(c *Client) bool) {
	b, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.rooms[gameID] {
		if match(c) {
			select {
			case c.send <- b:
			default:
			}
		}
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// Serve upgrades the request and runs the connection loops. It blocks until
// the client disconnects.
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, gameID, userID string, role Role) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade: %v", err)
		return
	}
	c := &Client{
		hub:    h,
		conn:   conn,
		send:   make(chan []byte, 32),
		GameID: gameID,
		UserID: userID,
		Role:   role,
	}
	h.add(c)
	if h.OnJoin != nil {
		h.OnJoin(c)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go c.writeLoop(ctx)
	c.readLoop()

	h.remove(c)
	if h.OnLeave != nil {
		h.OnLeave(c)
	}
}

func (c *Client) writeLoop(ctx context.Context) {
	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-c.send:
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ping.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) readLoop() {
	defer c.conn.Close()
	c.conn.SetReadLimit(10 * 1024 * 1024)
	_ = c.conn.SetReadDeadline(time.Now().Add(75 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(75 * time.Second))
		return nil
	})
	for {
		_, b, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		if c.hub.OnRecv != nil {
			c.hub.OnRecv(c, b)
		}
	}
}

// Send sends a JSON message to this single client.
func (c *Client) Send(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	select {
	case c.send <- b:
	default:
	}
}
