# Homepage CMS stub — devops audit

**Date:** 2026-07-26  
**Scope:** `site_settings.home_hero` (headline/subtitle/CTA) + admin `/{locale}/admin/cms/home` + landing i18n fallback.

## Delivered
| Surface | Detail |
|---------|--------|
| Public | `GET /api/v1/site/home` |
| Admin | `GET/PUT /admin/v1/cms/home` (`cms.read` / `cms.write`) |
| UI | `/{locale}/admin/cms/home` |
| Landing | Reads hero; empty fields → Landing i18n (accent split preserved when CMS headline empty) |
| Audit | `cms.home.put` |
| Safety | Relative `ctaHref` only; length caps |

## Verdict
**Green** for thin stub. Single-locale CMS strings (same honesty as contacts). No per-locale JSON, no full page builder.

## Gates
| Check | Result |
|-------|--------|
| `go test ./internal/site/ ./internal/admin/ -run 'Home\|CMS'` | pass |
| vitest `site-home.test.ts` + landing page | pass |

## Remains
Monitoring logs/alerts; U-39 done-enough; U-50 refresh.
