# M3-4 CMS chrome (contacts) — devops audit

**Date:** 2026-07-26  
**Scope:** Admin CMS practical slice — `/admin/v1/cms/contacts` (GET `cms.read`, PUT `cms.write` + audit) + `/{locale}/admin/cms/chrome` editor. Public footer already reads `GET /site/contacts` (U-17). Ops `/ops/contacts` kept as deprecated bridge with link to Admin.

## Verdict
**Green** for M3-4 chrome/contacts slice (§ CMS Header/footer). No homepage/legal typed-block CMS invented. No fake business phone. No staging host. No seed wipe.

## Gates
| Check | Result |
|-------|--------|
| `go test ./internal/admin/ -run CMS` | pass (put+audit; editor read OK / write 403; finance no cms.read) |
| Permissions | `cms.read` / `cms.write` (seeded M3-0); editor has read-only |
| Audit | `cms.contacts.put` → `admin_audit_log` entity `site_settings`/`contacts` |
| BFF | cookie proxy `/api/admin/cms/contacts` |
| Public | unchanged `GET /api/v1/site/contacts` → landing footer |
| Safety | no truncate/seed; empty fields keep i18n placeholders |

## Remains (out of this slice)
Homepage/legal/brand/surfaces CMS documents, draft→publish, locks/SSE. Next: M3-5 Monitoring.
