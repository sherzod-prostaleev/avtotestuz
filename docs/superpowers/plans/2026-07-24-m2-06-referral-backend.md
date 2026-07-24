# M2-06 Referral Program Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Steps use `- [ ]`.

**Goal:** Implement backend support for referral program: referral code generation/lookup, referral code application with anti-fraud rules, referral stats retrieval, and automatic reward distribution (+7 VIP days to referrer) upon referee's first successful payment.

**Architecture:** Go (`internal/billing/referral.go`, `internal/db/queries/referral.sql`), PostgreSQL migration `0004_referral.up.sql`, sqlc code generation.

## Global Constraints
- `go build ./...` must succeed.
- `go test ./... -p 1 -count=1` must pass cleanly across all 27+ packages.

---

### Task 1: Migration & SQL Queries
**Files:**
- Create `backend/internal/db/migrations/0004_referral.up.sql`
- Create `backend/internal/db/migrations/0004_referral.down.sql`
- Create `backend/internal/db/queries/referral.sql`
- Run `make generate` to update `sqlc` models and queries

- [ ] Add `user_referral_code` and `referral` tables with unique indexes and anti-self-referral constraint.
- [ ] Add sqlc queries: `GetOrCreateUserReferralCode`, `GetReferralCodeOwner`, `CreateReferral`, `GetPendingReferralForReferee`, `MarkReferralRewarded`, `GetReferralStatsForUser`.
- [ ] Run `make generate`.

---

### Task 2: Referral Service & Webhook Integration
**Files:**
- Create `backend/internal/billing/referral.go`
- Create `backend/internal/billing/referral_test.go`
- Modify `backend/internal/billing/entitlement.go` (integrate referral reward into `ProcessPaymentGrant`)
- Modify `backend/internal/billing/handlers.go` (add `GET /api/v1/me/referral` and `POST /api/v1/referral/apply`)

- [ ] Implement `ApplyReferralCode`, `GetReferralStats`, and reward processing in `ProcessPaymentGrant`.
- [ ] Add HTTP handlers for referral stats and code application.
- [ ] Write TDD unit and integration tests for referral application, anti-fraud rules, and payment reward grant.

---

## Verification Plan
- Run `go test ./... -p 1 -count=1` across all Go packages.
