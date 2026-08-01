package arena

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

const (
	writeBufCap  = 16
	readLimit    = 4096
	pingInterval = 20 * time.Second
	floodWindow  = 10 * time.Second
	floodMaxMsgs = 20
	connKeyTTL   = 30 * time.Second
)

// Conn wraps one WebSocket with bounded write queue and flood guard.
type Conn struct {
	ProfileID uuid.UUID
	ws        *websocket.Conn
	out       chan []byte
	svc       *Service
	closed    atomic.Bool
	closeOnce sync.Once
	onClose   func()
}

func NewConn(profileID uuid.UUID, ws *websocket.Conn, svc *Service, onClose func()) *Conn {
	return &Conn{
		ProfileID: profileID,
		ws:        ws,
		out:       make(chan []byte, writeBufCap),
		svc:       svc,
		onClose:   onClose,
	}
}

func (c *Conn) Enqueue(payload []byte) error {
	if c.closed.Load() {
		return nil
	}
	select {
	case c.out <- payload:
		return nil
	default:
		c.Close(websocket.StatusPolicyViolation, "slow_client")
		return nil
	}
}

func (c *Conn) Close(code websocket.StatusCode, reason string) {
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		_ = c.ws.Close(code, reason)
		close(c.out)
		if c.onClose != nil {
			c.onClose()
		}
	})
}

func (c *Conn) Run(ctx context.Context) {
	go c.writePump(ctx)
	c.readPump(ctx)
}

func (c *Conn) writePump(ctx context.Context) {
	ticker := time.NewTicker(pingInterval)
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
			if c.svc != nil && c.svc.R != nil {
				_ = c.svc.R.Set(ctx, "arena:conn:"+c.ProfileID.String(), c.svc.Instance, connKeyTTL).Err()
			}
		}
	}
}

func (c *Conn) readPump(ctx context.Context) {
	c.ws.SetReadLimit(readLimit)
	var windowStart time.Time
	var count int
	for {
		_, data, err := c.ws.Read(ctx)
		if err != nil {
			c.Close(websocket.StatusGoingAway, "read_failed")
			return
		}
		now := time.Now()
		if windowStart.IsZero() || now.Sub(windowStart) > floodWindow {
			windowStart = now
			count = 0
		}
		count++
		if count > floodMaxMsgs {
			c.Close(websocket.StatusPolicyViolation, "flood")
			return
		}
		env, err := Decode(data)
		if err != nil {
			_ = c.Enqueue(mustEncode("error", ErrorData{Code: "bad_protocol", Message: err.Error()}))
			continue
		}
		if c.svc != nil {
			c.svc.HandleClient(c.ProfileID, env)
		}
	}
}

func mustEncode(t string, d any) []byte {
	b, err := Encode(t, d)
	if err != nil {
		return []byte(`{"v":1,"t":"error","d":{"code":"internal","message":"encode"}}`)
	}
	return b
}
