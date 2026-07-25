# U-17 admin-editable site contacts — devops audit

**Date:** 2026-07-26  
**Scope:** `site_settings` contacts JSON + public `GET /site/contacts` + ops `GET|PUT /ops/site-contacts` + FE `/{locale}/ops/contacts` + landing footer read with i18n fallback.

## Executive verdict
Footer contacts are no longer blocked on marketing paste. Sherzod can set phone/address/TG/IG/email from ops admin (`OPS_ADMIN_TOKEN`). Empty DB fields keep existing `Landing.footer*` placeholders.

## Change surface
| Layer | Path |
|-------|------|
| Migration | `0029_site_settings` |
| BE | `internal/site`, ops handlers, CORS PUT |
| FE BFF | `/api/ops/site-contacts` |
| FE UI | `/{locale}/ops/contacts` |
| Public | proxy allowlist `site/contacts`; landing footer |

## Secrets / hygiene
- Ops token required for write/list via ops routes.
- Public GET returns only non-secret contact chrome (no payment/LLM keys).
- Field length capped at 500 runes.

## Quality gates
| Check | Result |
|-------|--------|
| `go test ./internal/site/... ./internal/ops/...` | pass |
| vitest `site-contacts.test.ts` + `ops-nav` + landing page | pass (9) |

## Remains
- Full M3 CMS/RBAC (U-45) still open; this is a thin ops precursor.
- **Real home:** `/{locale}/admin/cms/chrome` (M3-4); `/ops/contacts` is temporary bridge (inventory notes this).
- Locale-specific address copy still via i18n fallback until ops fills values (single JSON, not per-locale CMS).
