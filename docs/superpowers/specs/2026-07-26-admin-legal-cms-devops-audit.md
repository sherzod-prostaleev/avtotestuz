# 2026-07-26 — Admin Legal CMS + nested Link/Button fix — devops audit

**Focus:** Close Admin «Huquqiy» stub (CMS legal) end-to-end; fix dead CTA clicks from `<Link><Button>`.

## Change

### Legal CMS
- `site_settings` key `legal` — multi-locale bundle (`uz-Latn` / `uz-Cyrl` / `ru`) with `oferta` / `privacy` / `refund` plain text
- Admin: `GET/PUT /admin/v1/cms/legal` (`cms.read` / `cms.write`) + audit `cms.legal.put`
- Public: `GET /api/v1/site/legal?locale=`
- FE admin page `/{locale}/admin/cms/legal` (locale tabs + save); sidebar stub removed
- BFF `/api/admin/cms/legal`; proxy public path includes `site/legal`
- Public `/oferta` + `/privacy`: CMS body when non-empty, else i18n sections

### Dead buttons
- `Button as="span"` for Link nesting (dashboard signs/saved, grand mock, saved, checkout success/failure)

## Gates

| Gate | Result |
|------|--------|
| `go test ./internal/site ./internal/admin -run 'Legal\|CMS'` | PASS |
| `npx tsc --noEmit` | PASS |
| vitest (site-legal, oferta, privacy, button, i18n-keysets) | PASS |

## Remaining (out of this slice)

- Admin content signs/tickets studios, runtime config, RBAC UI
- U-02/U-03/U-12/VAPID external blockers
- Refund CMS field stored but no dedicated public page yet
