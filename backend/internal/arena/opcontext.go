package arena

import (
	"context"
	"time"
)

// opTimeout bounds a single Redis or Postgres call made off the request path.
//
// Generous on purpose: these are small reads and writes that finish in
// milliseconds, and the point is not to enforce a latency budget but to make
// "forever" impossible. A value tight enough to trip during an ordinary load
// spike would turn a slow tick into a dropped match, which is worse than the
// leak it guards against.
const opTimeout = 10 * time.Second

// opContext returns the context for work that outlives the request or socket
// that triggered it -- a match loop, a queue timer, a WebSocket read pump.
//
// None of those can borrow a request context: the caller has already returned,
// so the work would be cancelled the moment it started. The previous answer was
// context.Background(), which has the opposite failure: it never cancels, so a
// single wedged query strands that goroutine for the life of the process, still
// holding its player's queue slot or match state. Bounded-but-detached is the
// shape that is actually wanted.
//
// The returned cancel func must be called; deferring it at the call site also
// releases the timer early on the normal path.
func opContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), opTimeout)
}
