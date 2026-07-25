# Admin audit log UI — devops audit

**Date:** 2026-07-26  
**Scope:** `GET /admin/v1/security/audit` (`security.audit.read`) over `admin_audit_log` + `/{locale}/admin/security/audit` list/filter/expand UI. Ops `/ops/audit` remains deprecated bridge to learner `audit_log`.

## Verdict
**Green** for admin audit read slice. Append-only read API; no UPDATE/DELETE. Sidebar stub cleared; Support nav stubs added for upcoming broadcast.

## Gates
| Check | Result |
|-------|--------|
| `go test ./internal/admin/ -run AdminAuditLog` | pass |
| Permission | `security.audit.read` (superadmin/admin); editor → 403 |
| Safety | no seed wipe; no mutation of audit rows |

## Remains
Support broadcast stub; wire feature flags into product gates; teacher dashboard; U-50 refresh.
