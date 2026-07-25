# U-40 B2B orgs/seats/grant — devops audit

**Date:** 2026-07-26  
**Scope:** Thin M5-01 slice — mig `0033` `b2b_org` / `b2b_org_member` / `b2b_org_license` + Admin API/UI to create org, add member, issue seat license, grant `entitlement.source=b2b` (seat-capped). Teacher dashboard / learner B2B portal **not** invented.

## Verdict
**Green** for code-completable U-40 foundation. No school customer UX beyond admin grant.

## Gates
| Check | Result |
|-------|--------|
| `go test ./internal/admin/ -run B2B` | pass |
| Permissions | list/detail `users.read`; mutate/grant `users.entitlements.grant` |
| Seat rule | active license seats required; conflict when active b2b grants fill seats |
| Audit | `b2b.orgs.create`, `b2b.members.add`, `b2b.licenses.create`, `b2b.entitlements.grant` |
| Safety | no seed wipe; entitlement note tags `b2b_org=<uuid>` |

## Remains
Teacher dashboard, student class stats, seat billing/invoices. Next inventory: U-10 skip note, U-35 PDF/share, U-39, U-27, U-50.
