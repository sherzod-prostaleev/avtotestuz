# U-04 Refund → entitlement revoke — devops audit

**Date:** 2026-07-26  
**Scope:** Confirm Payme refund clamps VIP; harden unit tests; close inventory drift.

## Secrets / hygiene
- No payment merchant keys invented or committed.

## Implementation (already on main since `55815e2`)
- `billing.Service.RevokeEntitlementForPayment` clamps the payment-linked entitlement row.
- Payme `CancelTransaction` on performed txn → `refunded` + revoke.

## Tests
| Check | Result |
|-------|--------|
| `go test ./internal/billing/ -run RevokeEntitlement` | pass |
| `go test ./internal/billing/payme/ -run CancelTransaction_FromPaid` | pass |

## Residual
- Click has no Merchant refund RPC analogous to Payme Cancel; post-paid cabinet refunds need M3/ops path later.
