# Support inbox stub — devops audit

**Date:** 2026-07-26  
**Scope:** Honest support inbox (not Zendesk): `support_ticket` migration, learner/public create, admin list+detail+status (`support.inbox`).

## Delivered
| Surface | Detail |
|---------|--------|
| Migration | `0034_support_ticket` |
| Public | `POST /api/v1/support/tickets` + `/{locale}/support` |
| Learner | `POST /api/v1/me/support/tickets` + profile card |
| Admin UI | `/{locale}/admin/support/inbox` + `…/inbox/{id}` |
| Admin API | `GET/PATCH /admin/v1/support/tickets[/{id}]` |
| Audit | `support.ticket.patch` |
| Permission | `support.inbox` (already seeded in 0030) |

## Verdict
**Green** for stub. Status machine: open / in_progress / resolved / closed. Public create requires email or phone. No Telegram forward, no SLA, no attachments.

## Gates
| Check | Result |
|-------|--------|
| `go test ./internal/support/ ./internal/admin/ -run 'Support\|Inbox'` | pass |
| `go test ./internal/db/ -run MigrateCreates` | pass |
| vitest `support-ticket-card.test.tsx` | pass |
| Safety | no seed wipe; body/subject length caps; editor role denied |

## Remains
Homepage CMS; monitoring logs/alerts thin slice; U-39 done-enough; U-50 refresh.
