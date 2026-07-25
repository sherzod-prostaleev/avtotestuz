# U-50 handoff/inventory refresh — devops audit (wave #5)

**Date:** 2026-07-26  
**Scope:** Align SESSION-HANDOFF §⚡ + inventory + roadmap after Sentry SDK + U-39 variant detail cache.  
**M4-07:** re-checked — still skipped (U-10; design has no tiny cron+quiz vertical).

## Wave SHAs (this sequence)
| SHA | Item |
|-----|------|
| `e0e7233` | U-41 optional Sentry SDK (DSN-gated; no pager) |
| `a003bb0` | U-39 recently-opened variant detail SW cache |
| (this) | U-50 refresh #5 |

## Status flips
| ID | Before | After |
|----|--------|-------|
| U-41 | Prometheus + DSN stub | + Sentry SDK init (still no pager/tracing) |
| U-39 | CMS list cache | + variants/{n} recently-opened cache; exam still large |
| U-10 / M4-07 | skipped | **still skipped** (inventory note only) |
| U-50 | refresh #4 | **refresh #5** |

## Remains that need user/secrets/host
U-02 staging host/registry, U-03 Payme/Click prod keys, U-12 LLM key, VAPID ops keys if push delivery wanted, real Sentry project DSNs for prod capture.

## Remains that are still multi-week product
Full offline exam sync, Metabase/Grafana, TG daily quiz product, B2B school sales,
tracing/pager, soak load-test / off-site WAL, Admin legal CMS / deeper studios,
FE Next majors / dep-scan high findings.
