package supportchat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"avtotest.uz/backend/internal/httpx"
)

func (h *Handler) serveUserWS(w http.ResponseWriter, r *http.Request) {
	h.serveWS(w, r, "user")
}

func (h *Handler) serveAdminWS(w http.ResponseWriter, r *http.Request) {
	h.serveWS(w, r, "admin")
}

func (h *Handler) serveWS(w http.ResponseWriter, r *http.Request, kind string) {
	ticket := r.URL.Query().Get("ticket")
	if ticket == "" {
		httpx.Error(w, http.StatusUnauthorized, "bad_ticket", "missing ticket")
		return
	}
	userID, err := h.Svc.RedeemTicket(r.Context(), kind, ticket)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "bad_ticket", "invalid or expired ticket")
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: originPatterns(h.Svc.PublicURL),
	})
	if err != nil {
		return
	}
	c := newConn(kind, userID, conn)
	if kind == "user" {
		h.Svc.Hub.RegisterUser(userID, c)
		// Auto-subscribe learner to their conversation room.
		if conv, err := h.Svc.Store.GetOrCreateConversation(r.Context(), userID); err == nil {
			h.Svc.Hub.SubscribeConversation(conv.ID, c)
		}
	} else {
		h.Svc.Hub.RegisterAdmin(userID, c)
	}

	hello, _ := json.Marshal(map[string]any{
		"t": "hello",
		"d": map[string]any{
			"kind":           kind,
			"user_id":        userID,
			"server_time_ms": time.Now().UTC().UnixMilli(),
		},
	})
	c.Enqueue(hello)

	ctx := r.Context()
	go h.writePump(ctx, c)
	h.readPump(ctx, c, kind)
	h.Svc.Hub.Unregister(c)
	c.Close(websocket.StatusNormalClosure, "bye")
}

func (h *Handler) writePump(ctx context.Context, c *Conn) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case payload, ok := <-c.out:
			if !ok {
				return
			}
			wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := c.ws.Write(wctx, websocket.MessageText, payload)
			cancel()
			if err != nil {
				c.Close(websocket.StatusGoingAway, "write_failed")
				return
			}
		case <-ticker.C:
			pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := c.ws.Ping(pctx)
			cancel()
			if err != nil {
				c.Close(websocket.StatusGoingAway, "ping_failed")
				return
			}
		}
	}
}

type clientEvent struct {
	T string          `json:"t"`
	D json.RawMessage `json:"d"`
}

func (h *Handler) readPump(ctx context.Context, c *Conn, kind string) {
	c.ws.SetReadLimit(4096)
	for {
		_, data, err := c.ws.Read(ctx)
		if err != nil {
			return
		}
		var ev clientEvent
		if err := json.Unmarshal(data, &ev); err != nil {
			continue
		}
		switch ev.T {
		case "subscribe":
			var d struct {
				ConversationID string `json:"conversation_id"`
			}
			if err := json.Unmarshal(ev.D, &d); err != nil {
				continue
			}
			id, err := uuid.Parse(d.ConversationID)
			if err != nil {
				continue
			}
			// Learners may only subscribe to their own conversation.
			if kind == "user" {
				conv, err := h.Svc.Store.GetByProfile(ctx, c.UserID)
				if err != nil || conv.ID != id {
					continue
				}
			}
			h.Svc.Hub.SubscribeConversation(id, c)
		case "ping":
			pong, _ := json.Marshal(map[string]any{"t": "pong", "d": map[string]any{}})
			c.Enqueue(pong)
		}
	}
}

func originPatterns(publicURL string) []string {
	if publicURL == "" {
		return []string{"*"}
	}
	u, err := url.Parse(publicURL)
	if err != nil || u.Host == "" {
		return []string{"*"}
	}
	host := u.Hostname()
	return []string{host, "localhost:*", "127.0.0.1:*"}
}

func (h *Handler) wsURL(r *http.Request, path string) string {
	if host, scheme, ok := publicWSBase(h.Svc.PublicURL); ok && internalAPIHost(r.Host) {
		return scheme + "://" + host + path
	}
	scheme := "ws"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "wss"
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return scheme + "://" + host + path
}

func publicWSBase(publicURL string) (host, scheme string, ok bool) {
	if publicURL == "" {
		return "", "", false
	}
	u, err := url.Parse(publicURL)
	if err != nil || u.Host == "" {
		return "", "", false
	}
	scheme = "ws"
	if u.Scheme == "https" {
		scheme = "wss"
	}
	return u.Host, scheme, true
}

func internalAPIHost(host string) bool {
	name := strings.ToLower(host)
	if i := strings.IndexByte(name, ':'); i >= 0 {
		name = name[:i]
	}
	switch name {
	case "", "api", "backend", "avtotest-api", "drivergo-api":
		return true
	}
	return name != "localhost" && name != "127.0.0.1" && !strings.Contains(name, ".")
}
