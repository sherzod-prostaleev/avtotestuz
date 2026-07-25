# M3-1 Users management — devops audit

**Date:** 2026-07-26  
**Scope:** Learner directory under Super Admin — `/admin/v1/users` (list/search/pagination, detail, block/unblock, session revoke) + `/{locale}/admin/users` (+ detail) BFF via admin cookies.

## Verdict
**Green** for M3-1 practical slice (§4.2). Ops `/ops/users` stub remains as bridge; real home is `/admin/users`. Learner routes unchanged.

## Gates
| Check | Result |
|-------|--------|
| `go test ./internal/admin/` | pass (list/detail/block/sessions + RBAC deny for editor) |
| Permission gates | `users.read` / `users.block` / `users.sessions.revoke` |
| Audit | `admin_audit_log` on block/unblock/revoke |
| Password | never returned on detail |
| Phone | masked on directory; full on detail for staff |
| Status mapping | DB `banned` ↔ API/UI `blocked` |
| Secrets | no new secrets; admin JWT cookies only |

## Remains (out of this slice)
Full §4.2 tabs (activity/learning/billing/notes), bulk export, hard-delete, filters beyond search, CMS/payments/monitoring (M3-2+). B2B still deferred.
