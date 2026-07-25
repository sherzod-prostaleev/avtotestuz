# M3-2 Content studio — devops audit

**Date:** 2026-07-26  
**Scope:** First useful Content vertical — `/admin/v1/content/questions` (list/search/filter/pagination, detail + variants/translations, soft `validation_status` patch) + explanations queue (`draft`/`verified`) + mark verified (reuses CLI verify semantics) + `/{locale}/admin/content/questions` (+ detail) / `explanations` via admin BFF cookies.

## Verdict
**Green** for M3-2 practical slice (§4.3 read + verify). No corpus wipe/seed/truncate paths. Tickets/signs/taxonomy/revisions/import remain later Content depth.

## Gates
| Check | Result |
|-------|--------|
| `go test ./internal/admin/ -run Content` | pass (list/detail/queue/verify/patch + support RBAC deny) |
| Permission gates | `content.questions.read` / `content.questions.write` / `content.verify` (mig `0031`) |
| Audit | `content.questions.patch`, `content.verify` → `admin_audit_log` |
| Safety | no truncate/seed-real/import from Admin UI |
| BFF | cookie proxy under `/api/admin/content/*` |

## Remains (out of this slice)
Full CRUD + revisions, tickets/signs/media/IO, bulk verify, CMS/payments (M3-3+). B2B still deferred.
