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

// OnlinePlayers returns the set of distinct player userIDs currently connected
// to the given room.
func (h *Hub) OnlinePlayers(gameID string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	seen := make(map[string]struct{})
	for c := range h.rooms[gameID] {
		if c.Role == RolePlayer && c.UserID != "" {
			seen[c.UserID] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	return out
}

// IsPlayerOnline reports whether the given player has at least one live
// connection in the room.
func (h *Hub) IsPlayerOnline(gameID, userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.rooms[gameID] {
		if c.Role == RolePlayer && c.UserID == userID {
			return true
		}
	}
	return false
}

// OnlinePlayerCounts returns gameID -> number of distinct online players, for
// all rooms with at least one player connection.
func (h *Hub) OnlinePlayerCounts() map[string]int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make(map[string]int)
	for gameID, room := range h.rooms {
		seen := make(map[string]struct{})
		for c := range room {
			if c.Role == RolePlayer && c.UserID != "" {
				seen[c.UserID] = struct{}{}
			}
		}
		if len(seen) > 0 {
			out[gameID] = len(seen)
		}
	}
	return out
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
		send:   make(chan []byte, 256),
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
