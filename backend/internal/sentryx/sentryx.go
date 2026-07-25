// Package sentryx wires optional Sentry error reporting (U-41 remainder).
// Empty DSN = no-op: no network, no Hub, Flush is a free return.
// This is SDK init only — no pager, no tracing product, no alerting stack.
package sentryx

import (
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
)

// Init enables Sentry when dsn is non-empty. Returns a Flush that blocks up to
// 2s (or a no-op when disabled). Call Flush before process exit.
func Init(dsn, env string) (flush func(), err error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return func() {}, nil
	}
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      env,
		TracesSampleRate: 0, // honest: no tracing product in this slice
	}); err != nil {
		return nil, err
	}
	return func() {
		sentry.Flush(2 * time.Second)
	}, nil
}
