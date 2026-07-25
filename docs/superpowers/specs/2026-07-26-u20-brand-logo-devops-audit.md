# Driver Go — DevOps Audit (U-20 · BrandLogo / no-img chrome)

**Date:** 2026-07-26  
**Auditor:** Cursor agent  
**Repo:** `/home/sher/Рабочий стол/avtotest` · branch `main`  
**Scope:** N4 chrome debt — migrate static `/logo.svg` chrome marks to `next/image` via `BrandLogo`.  
**Constraint:** Do not touch dynamic MinIO/CDN question/sign media `<img>` paths.

---

## 0. Executive verdict

**Green.** Static brand logos clear `@next/next/no-img-element` without changing remote media URL handling.

---

## 1. Change surface

- `frontend/src/components/brand/brand-logo.tsx` (+ vitest)
- Chrome/auth/public/legal/exam header consumers swap `<img src="/logo.svg">` → `<BrandLogo />`
- Inventory U-20 note updated (content media still accepted as `<img>`)

---

## 2. Quality gates

| Check | Result |
|-------|--------|
| `vitest …/brand-logo.test.tsx` | pass (2) |
| `tsc --noEmit` | pass |
| eslint (BrandLogo + consumers) | pass |
| Seed / migrations | untouched |

---

## 3–8. Ops / safety

- SVG served `unoptimized` (Next does not rasterize SVG by default) — no `dangerouslyAllowSVG` needed
- No `images.remotePatterns` change — remote question media stays raw `<img>`
- No backend / env / compose changes

---

## 9. Findings

| Sev | Finding | Action |
|-----|---------|--------|
| Info | Content `no-img-element` (S3/CDN) still eslint-disabled | Accept (dynamic hosts) |
| Info | Footer contact placeholders (U-17) | Separate |

---

## 12. E0 gate

Safe to commit + push.
