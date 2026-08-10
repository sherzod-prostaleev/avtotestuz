package supportchat

import (
	"sync"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

// Conn is one live WebSocket with a bounded outbound queue.
type Conn struct {
	ID     uuid.UUID
	Kind   string // user|admin
	UserID uuid.UUID
	ws     *websocket.Conn
	out    chan []byte
	closed bool
	mu     sync.Mutex
}

func newConn(kind string, userID uuid.UUID, ws *websocket.Conn) *Conn {
	return &Conn{
		ID:     uuid.New(),
		Kind:   kind,
		UserID: userID,
		ws:     ws,
		out:    make(chan []byte, 32),
	}
}

func (c *Conn) Enqueue(payload []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	select {
	case c.out <- payload:
	default:
		// Slow client: drop connection rather than unbounded memory.
		go c.Close(websocket.StatusPolicyViolation, "slow_client")
	}
}

func (c *Conn) Close(code websocket.StatusCode, reason string) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	close(c.out)
	c.mu.Unlock()
	_ = c.ws.Close(code, reason)
}

// Hub fans events to conversation subscribers and all admin inbox watchers.
type Hub struct {
	mu      sync.RWMutex
	byConv  map[uuid.UUID]map[*Conn]struct{}
	admins  map[*Conn]struct{}
	byUser  map[uuid.UUID]*Conn
	byAdmin map[uuid.UUID]map[*Conn]struct{}
}

func NewHub() *Hub {
	return &Hub{
		byConv:  make(map[uuid.UUID]map[*Conn]struct{}),
		admins:  make(map[*Conn]struct{}),
		byUser:  make(map[uuid.UUID]*Conn),
		byAdmin: make(map[uuid.UUID]map[*Conn]struct{}),
	}
}

// RegisterUser replaces any prior learner socket for the profile.
func (h *Hub) RegisterUser(profileID uuid.UUID, c *Conn) {
	h.mu.Lock()
	old := h.byUser[profileID]
	h.byUser[profileID] = c
	h.mu.Unlock()
	if old != nil && old != c {
		old.Close(websocket.StatusCode(4001), "replaced")
	}
}

// RegisterAdmin tracks an admin inbox/watch socket.
func (h *Hub) RegisterAdmin(adminID uuid.UUID, c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.admins[c] = struct{}{}
	if h.byAdmin[adminID] == nil {
		h.byAdmin[adminID] = make(map[*Conn]struct{})
	}
	h.byAdmin[adminID][c] = struct{}{}
}

// SubscribeConversation adds c to a conversation room.
func (h *Hub) SubscribeConversation(conversationID uuid.UUID, c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.byConv[conversationID] == nil {
		h.byConv[conversationID] = make(map[*Conn]struct{})
	}
	h.byConv[conversationID][c] = struct{}{}
}

// Unregister removes c from all rooms.
func (h *Hub) Unregister(c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c.Kind == "user" {
		if cur, ok := h.byUser[c.UserID]; ok && cur == c {
			delete(h.byUser, c.UserID)
		}
	}
	if c.Kind == "admin" {
		delete(h.admins, c)
		if set, ok := h.byAdmin[c.UserID]; ok {
			delete(set, c)
			if len(set) == 0 {
				delete(h.byAdmin, c.UserID)
			}
		}
	}
	for id, set := range h.byConv {
		delete(set, c)
		if len(set) == 0 {
			delete(h.byConv, id)
		}
	}
}

// BroadcastConversation sends to conversation subscribers plus all admin watchers.
func (h *Hub) BroadcastConversation(conversationID uuid.UUID, payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	seen := make(map[*Conn]struct{})
	for c := range h.byConv[conversationID] {
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		c.Enqueue(payload)
	}
	for c := range h.admins {
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		c.Enqueue(payload)
	}
}
