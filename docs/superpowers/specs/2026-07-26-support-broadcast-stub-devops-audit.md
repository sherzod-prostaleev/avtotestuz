# Support broadcast stub — devops audit

**Date:** 2026-07-26  
**Scope:** Smallest honest support broadcast slice: in-app banner via `site_settings.support_banner` + web-push broadcast on existing M4-08 push infra (`support.broadcast`).

## Delivered
| Surface | Detail |
|---------|--------|
| Admin UI | `/{locale}/admin/support/broadcasts` |
| Banner API | `GET/PUT /admin/v1/support/banner` → `site_settings` |
| Public banner | `GET /api/v1/site/banner` + app-shell `SupportBanner` |
| Web-push | `POST /admin/v1/support/broadcasts/webpush` (`dry_run` + live, cap 500) |
| Audit | `support.banner.put`, `support.broadcast.webpush` |

## Verdict
**Green** for stub. No Telegram broadcast. Live push needs VAPID (honest `web_push_unconfigured`). No invented campaign scheduler/segments.

## Gates
| Check | Result |
|-------|--------|
| `go test ./internal/admin/ -run AdminSupportBroadcast` | pass |
| `go test ./internal/push/ -run Broadcast` | pass |
| `go test ./internal/site/ -run SupportBanner` | pass |
| vitest `support-banner.test.tsx` | pass |
| Safety | relative href only; no seed wipe; dry-run default path in UI |

## Remains
Wire feature flags into product gates; teacher/B2B dashboard; inbox; U-50 refresh.
