# U-41 Sentry SDK init — devops audit

**Date:** 2026-07-26  
**Scope:** Optional Sentry SDK on API + browser; init only when DSN set; empty = no-op.  
**Honest:** no pager, no tracing product, no on-call stack invented.

## Secrets / hygiene
| Env | Where | Behavior |
|-----|-------|----------|
| `SENTRY_DSN` | API (`config.Load` → `sentryx.Init`) | empty → no Hub / no flush work |
| `NEXT_PUBLIC_SENTRY_DSN` | FE client (`lib/sentry.ts`) | empty → no `@sentry/browser` init |
| Both | `.env.example` / `deploy/app.env.example` | documented placeholders only |

`NEXT_PUBLIC_*` is baked at Next **build** time for production images — set the
build arg / CI secret when you have a real browser DSN. Local `next dev` reads
`.env.local`.

## Code
| Piece | Detail |
|-------|--------|
| `backend/internal/sentryx` | thin wrapper; `TracesSampleRate: 0` |
| `backend/cmd/api` | Init after logger; Flush on shutdown |
| `frontend/src/lib/sentry.ts` | client init; once-guard |
| `InitSentry` in Providers | client-only effect |

## Gates
| Check | Result |
|-------|--------|
| `go test ./internal/sentryx/ ./internal/config/` | pass |
| `go build ./cmd/api` | pass |
| vitest `src/lib/sentry.test.ts` | pass |

## Remains (not this slice)
- Distributed tracing / performance product
- Pager / PagerDuty / Opsgenie wiring
- Server-side Next instrumentation (`@sentry/nextjs` webpack plugin)
- Auto `CaptureException` middleware everywhere (Hub is live when DSN set; call sites can grow later)
