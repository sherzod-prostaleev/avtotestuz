# M3-3 Payments UI — devops audit

**Date:** 2026-07-26  
**Scope:** Admin payments vertical — `/admin/v1/payments/transactions` (list filter status/provider/date + pagination, detail with redacted meta + provider txn + entitlement) + `/admin/v1/payments/providers` kill-switch (mirrors ops, admin JWT + RBAC) + `/{locale}/admin/payments/transactions` (+ detail) / `providers` / `refunds` (honest docs page).

## Verdict
**Green** for M3-3 practical slice (§ Payments: transactions + providers). No merchant prod keys invented. No staging host. No seed wipe.

## Refund honesty (critical)
| Provider | Admin-initiated refund? | What exists |
|----------|-------------------------|-------------|
| **Payme** | **No** outbound merchant HTTP refund from this app | Money refund is started in **Payme cabinet**; Payme calls our **CancelTransaction** (state=-2) → `payment.refunded` + `RevokeEntitlementForPayment` (**U-04 done**) |
| **Click** | **No** | Merchant API has **no** post-paid refund webhook/RPC in this codebase — not invented |

Detail JSON exposes `refund.action_available=false` + `provider_path` + note. UI Refunds page documents the same and links to `?status=refunded`.

## Gates
| Check | Result |
|-------|--------|
| `go test ./internal/admin/ -run Payments` | pass (list/detail/providers patch audit + finance read / editor deny / finance keys.manage deny) |
| Permissions | `payments.read` (list/detail/providers GET); `payments.keys.manage` (PATCH providers); `payments.refund` seeded but unused (no safe outbound path) |
| Audit | `payments.providers.patch` → `admin_audit_log` |
| BFF | cookie proxy `/api/admin/payments/*` |
| Safety | no truncate/seed; no Click refund invention; no Payme/Click prod keys |

## Remains (out of this slice)
Webhook inbox/replay, catalog/tariffs UI, dual-approve refund thresholds, CMS (M3-4), B2B.
