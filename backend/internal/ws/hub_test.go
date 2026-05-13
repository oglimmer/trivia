package ws

import (
	"encoding/json"
	"testing"
)

// fakeClient builds a Client suitable for unit tests. It has no underlying
// websocket connection — only the pieces the hub touches: send chan, GameID,
// UserID, Role.
func fakeClient(gameID string, role Role, userID string) *Client {
	return &Client{
		send:   make(chan []byte, 4),
		GameID: gameID,
		UserID: userID,
		Role:   role,
	}
}

func TestHubOnlinePlayersDeduplicatesAndIgnoresAdmin(t *testing.T) {
	h := NewHub()
	h.add(fakeClient("g", RolePlayer, "u1"))
	h.add(fakeClient("g", RolePlayer, "u2"))
	h.add(fakeClient("g", RolePlayer, "u1")) // second tab / reconnect
	h.add(fakeClient("g", RoleAdmin, ""))    // admins must not count

	got := h.OnlinePlayers("g")
	seen := map[string]bool{}
	for _, id := range got {
		seen[id] = true
	}
	if len(seen) != 2 || !seen["u1"] || !seen["u2"] {
		t.Fatalf("expected {u1,u2}, got %v", got)
	}
}

func TestHubIsPlayerOnline(t *testing.T) {
	h := NewHub()
	h.add(fakeClient("g", RolePlayer, "u1"))
	h.add(fakeClient("g", RoleAdmin, ""))

	if !h.IsPlayerOnline("g", "u1") {
		t.Fatalf("u1 should be online")
	}
	if h.IsPlayerOnline("g", "u-missing") {
		t.Fatalf("u-missing should not be online")
	}
	if h.IsPlayerOnline("other-game", "u1") {
		t.Fatalf("u1 should not be online in a different game")
	}
}

func TestHubOnlinePlayerCounts(t *testing.T) {
	h := NewHub()
	h.add(fakeClient("g", RolePlayer, "u1"))
	h.add(fakeClient("g", RolePlayer, "u2"))
	h.add(fakeClient("g", RolePlayer, "u1")) // duplicate connection
	h.add(fakeClient("h", RolePlayer, "u3"))
	h.add(fakeClient("admin-only", RoleAdmin, "")) // not surfaced

	counts := h.OnlinePlayerCounts()
	if counts["g"] != 2 {
		t.Errorf("g: want 2 distinct players, got %d", counts["g"])
	}
	if counts["h"] != 1 {
		t.Errorf("h: want 1, got %d", counts["h"])
	}
	if _, ok := counts["admin-only"]; ok {
		t.Errorf("admin-only room must not appear: %v", counts)
	}
}

func TestHubBroadcastToFiltersByPredicate(t *testing.T) {
	h := NewHub()
	player := fakeClient("g", RolePlayer, "u1")
	admin := fakeClient("g", RoleAdmin, "")
	h.add(player)
	h.add(admin)

	h.BroadcastTo("g", map[string]any{"type": "ping"}, func(c *Client) bool { return c.Role == RoleAdmin })

	select {
	case b := <-admin.send:
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil || m["type"] != "ping" {
			t.Fatalf("admin got unexpected message: %s err=%v", b, err)
		}
	default:
		t.Fatalf("admin should have received the message")
	}

	select {
	case b := <-player.send:
		t.Fatalf("player should not have received the message, got %s", b)
	default:
	}
}

func TestHubBroadcastToAllClients(t *testing.T) {
	h := NewHub()
	a := fakeClient("g", RolePlayer, "u1")
	b := fakeClient("g", RolePlayer, "u2")
	h.add(a)
	h.add(b)

	h.Broadcast("g", map[string]any{"type": "hello"})

	for _, c := range []*Client{a, b} {
		select {
		case <-c.send:
		default:
			t.Fatalf("client %s did not receive broadcast", c.UserID)
		}
	}
}

func TestHubRemoveCleansEmptyRoom(t *testing.T) {
	h := NewHub()
	c := fakeClient("g", RolePlayer, "u1")
	h.add(c)
	h.remove(c)

	h.mu.RLock()
	_, present := h.rooms["g"]
	h.mu.RUnlock()
	if present {
		t.Fatalf("expected empty room 'g' to be removed from rooms map")
	}
}

func TestHubBroadcastDropsOnFullSendBuffer(t *testing.T) {
	// A slow consumer's full send channel must not block the hub.
	h := NewHub()
	c := &Client{send: make(chan []byte), GameID: "g", UserID: "u1", Role: RolePlayer}
	h.add(c)
	// If Broadcast blocked, this test would deadlock under -timeout.
	h.Broadcast("g", map[string]any{"type": "x"})
}
