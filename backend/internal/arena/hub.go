package arena

import (
	"sync"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

// Hub keeps at most one live Conn per profile.
type Hub struct {
	mu      sync.RWMutex
	conns   map[uuid.UUID]*Conn
	inMatch map[uuid.UUID]uuid.UUID // profile → match
}

func NewHub() *Hub {
	return &Hub{
		conns:   make(map[uuid.UUID]*Conn),
		inMatch: make(map[uuid.UUID]uuid.UUID),
	}
}

// Register replaces any existing connection for profileID (closes old with 4001).
func (h *Hub) Register(profileID uuid.UUID, c *Conn) {
	h.mu.Lock()
	old := h.conns[profileID]
	h.conns[profileID] = c
	h.mu.Unlock()
	if old != nil && old != c {
		old.Close(websocket.StatusCode(CloseReplaced), "replaced_by_new_connection")
	}
}

func (h *Hub) Unregister(profileID uuid.UUID, c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if cur, ok := h.conns[profileID]; ok && cur == c {
		delete(h.conns, profileID)
	}
}

func (h *Hub) Get(profileID uuid.UUID) *Conn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.conns[profileID]
}

func (h *Hub) Alive(profileID uuid.UUID) bool {
	return h.Get(profileID) != nil
}

func (h *Hub) Send(profileID uuid.UUID, payload []byte) error {
	c := h.Get(profileID)
	if c == nil {
		return nil
	}
	return c.Enqueue(payload)
}

func (h *Hub) SetMatch(profileID, matchID uuid.UUID) {
	h.mu.Lock()
	h.inMatch[profileID] = matchID
	h.mu.Unlock()
}

func (h *Hub) ClearMatch(profileID uuid.UUID) {
	h.mu.Lock()
	delete(h.inMatch, profileID)
	h.mu.Unlock()
}

func (h *Hub) InMatch(profileID uuid.UUID) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.inMatch[profileID]
	return ok
}

func (h *Hub) MatchOf(profileID uuid.UUID) (uuid.UUID, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	id, ok := h.inMatch[profileID]
	return id, ok
}

func (h *Hub) CloseAll(code int) {
	h.mu.Lock()
	list := make([]*Conn, 0, len(h.conns))
	for _, c := range h.conns {
		list = append(list, c)
	}
	h.conns = make(map[uuid.UUID]*Conn)
	h.mu.Unlock()
	for _, c := range list {
		c.Close(websocket.StatusCode(code), "server_shutdown")
	}
}
