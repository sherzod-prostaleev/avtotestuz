# U-39 offline — done-enough note (shell + meta lists)

**Date:** 2026-07-26  
**Scope:** Confirm intentional U-39 ceiling after meta-list cache + public `/support` shell path. Full exam/question sync remains open and **large**.

## Done (code-complete for this wave)
| Layer | What |
|-------|------|
| Shell | Precache offline.html + icons; network-first nav for public shells |
| Public shells | login, oferta, privacy, narxlar, jarimalar, **support** |
| Meta lists | network-first `/api/proxy/{variants,me/variants,categories,signs}` |
| Honesty | SW comment + tests: no IndexedDB question catalog |

## Explicitly NOT done (large / separate plan)
- Offline exam session / question bodies / images catalog
- Background sync / conflict resolution
- Full bilets content offline package

## Verdict
**Done-enough** for U-39 thin slice. Inventory stays **partial** until exam sync ships.

## Gates
| Check | Result |
|-------|--------|
| vitest `pwa-sw-shell.test.ts` | pass |
