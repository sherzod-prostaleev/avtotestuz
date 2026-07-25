# Admin limits write — devops audit

**Date:** 2026-07-26  
**Scope:** Promote ops read-only `limit_config` to Admin `settings.config` list+PATCH with audit.

## Delivered
| Surface | Detail |
|---------|--------|
| API | `GET/PATCH /admin/v1/settings/limits` (`settings.config`) |
| UI | `/{locale}/admin/settings/limits` |
| Audit | `settings.limits.patch` |
| Ops | `/ops/limits` banner → admin settings/limits |

## Verdict
**Green.** Values ≥ -1 (-1 unlimited). No seed wipe of limit keys.

## Gates
| Check | Result |
|-------|--------|
| `go test ./internal/admin/ -run AdminLimitConfigs` | pass |
| Editor denied | 403 |

## Remains
Support inbox; homepage CMS; logs/alerts — larger invent / external.
