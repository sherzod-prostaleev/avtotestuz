# Driver Go / AvtoTest — Full-Project Unfinished Work Inventory

**Date:** 2026-07-26  
**Scope:** Entire product (M1–M7 + M3 Admin + chrome/J-wave + ops) — **not** only Asphalt J7→J10.  
**Method:** Read-only audit of `docs/superpowers/**`, root handoffs/prompts, Makefile/`run.sh`/compose/CI, `backend/{cmd,internal}`, `frontend` routes.  
**Constraint:** Report only. No product code. Prefer not colliding with sibling J7/N2 work.  
**Sources of truth (may disagree — code wins over stale handoff rows):**  
`AVTOTEST-MASTER-PROMPT.txt`, `2026-07-24-roadmap-m2-to-admin.md`, `2026-07-24-SESSION-HANDOFF.md`, design/plans under `docs/superpowers/`, live packages/routes.

---

## 1. Executive summary — how big is remaining work?

**J10 Arena UI is one growth feature, not “project done.”** After M1 learning core + M2 monetization (sandbox) + Leaderboard BE/UI + Telegram bot **foundation**, the remaining surface is still **multiple product waves** (~roadmap’s own estimate was ~41 sessions to Admin, plus Admin ~13).

| Wave band | What’s left (high level) | Rough size |
|-----------|--------------------------|------------|
| **A — Habit / chrome close** | Demo→account migrate (N2); any residual J7; optional J8 Figma; chrome tech-debt | S–M |
| **B — Money production** | Payme/Click **prod** keys + legal entity; refund→entitlement revoke; referral antifraud; promo pro-rate UX; payment recon job | M–L (external blockers) |
| **C — Growth incomplete** | Arena infra→rating→UI (M4-03…05 / J10); TG quiz+notif (M4-07); FE Telegram link; web push (M4-08) | **L** (Arena alone ~6.5 sessions in roadmap) |
| **D — Content / explanations** | Real LLM (not stub); expert verify at scale; LegalRefs extraction; leftover bilets; signs licensing pipeline hardening | M–L |
| **E — Learning depth** | FE use of `GET /learn/next` / due-FSRS practice UX; weak-area surfacing beyond stats bars | S–M |
| **F — Ship / ops** | Dockerfiles, staging host (D18), Playwright in CI, Redis in CI, observability, backup/DR, load-test | L |
| **G — PWA (M6)** | Manifest, SW, offline shell, offline content sync | M |
| **H — B2B (M5)** | Orgs, seats, teacher dashboard — **requirements thin until a school customer** | L when scoped |
| **I — Super Admin (M3)** | Contenteditor, users, billing/refund UI, investor metrics, RBAC, audit — **intentionally last** | **L** (~13 sessions) |

**Bottom line:** Treating “Arena UI last” as project completion **understates** Admin, production payments, bot completeness, content quality, ops/staging, PWA, B2B, and antifraud. Honest “launchable B2C MVP” ≠ “platform complete.”

**Doc drift callout:** `SESSION-HANDOFF` §⚡ says Wave 1 shipped Leaderboard UI + TG bot foundation + checkout `returnURL`, but §4 still lists M4-02/M4-06 as “Navbatda.” **Code confirms** `/leaderboard`, `internal/bot`, and checkout `returnURL` exist. Prefer code + this inventory over stale §4 rows.

---

## 2. Table of ALL unfinished items (P0–P3)

Status legend: **missing** = no usable implementation · **partial** = exists but incomplete/unsafe for prod · **deferred** = explicitly postponed in spec/plan · **stale-doc** = docs claim unfinished but code exists (verify, don’t rebuild).

| ID | Priority | Area | Item | Status | Depends on | Notes |
|----|----------|------|------|--------|------------|-------|
| **U-01** | P0 | FE/BE | **N2 Demo→account investment migrate** | **done** | Auth, progress/learning | `POST /me/demo-progress/migrate` — incorrect→FSRS Again; correct skipped (no mastery inflate). |
| **U-02** | P0 | Ops | **Staging / production deploy path** | **partial** | D18 host decision | Dockerfiles + `deploy/` overlay/smoke + **STAGING-RUNBOOK** (registry push, host layout, health/**readyz**, rollback) + hardened compose (restart, log rotate, web healthcheck, `API_IMAGE`/`WEB_IMAGE`). Remote host + registry credentials still blocked on D18. |
| **U-03** | P0 | Ops | **Payme/Click production merchant keys + legal entity** | deferred | External / yuridik shaxs | Sandbox adapters **done**. ENV empty → webhooks reject. Master prompt: sandbox until legal entity. |
| **U-04** | P0 | BE | **Refund → entitlement revoke** | **done** | Billing audit | Payme `CancelTransaction` state=-2 → `payment.refunded` + `RevokeEntitlementForPayment` (clamp ends_at). Unit + Payme RPC tests. Click Merchant API has no post-paid refund webhook — cabinet refunds remain ops/M3 follow-up. |
| **U-05** | P0 | BE | **Referral antifraud (retroactive attach)** | **done** | Design locked | Attach requires no prior `paid` payment + `created_at` within `referral_attach_window_days` (30). Terminal FE codes clear localStorage. |
| **U-06** | P1 | BE | **M4-03 Arena realtime infra** | **done** | Spec ✅ | `internal/arena` + mig `0021_battle_arena` + Redis `arena:` + WS ticket/match hub. |
| **U-07** | P1 | BE | **M4-04 Arena rating / medals / history API** | **done** | U-06 | `GET /me/arena/rating`, `GET /me/arena/matches` + ELO/medals in arena package. |
| **U-08** | P1 | FE | **J10 / M4-05 Arena UI** | **done** | U-06 (+ U-07) | `/(app)/arena` + sidebar + protocol client; VIP gate → premium. |
| **U-09** | P1 | FE | **Telegram “bog‘lash” UI** | **done** | M4-06 BE ✅ | Profile `TelegramLinkCard`: `GET /me/telegram` + `POST /me/telegram/link-token` deep link. |
| **U-10** | P1 | Bot | **M4-07 TG daily quiz + notifications** | deferred | M4-06 ✅ | Spec defers quiz, outbound cron, rich keyboards, multi-locale bot copy, `/unlink`, flood limits. |
| **U-11** | P1 | BE/FE | **M4-08 Web push / campaigns** | **partial** | VAPID keys (ops) | Foundation + harden + **FSRS due digest `-send`** (`RunFSRSDueDigest`, 20h cooldown, dead-sub prune). Campaigns/admin broadcast still open (M3); delivery needs `VAPID_*` on cron host. |
| **U-12** | P1 | Content | **AI explanation = real LLM** | stub | Budget/API key | `TemplateDraftGenerator` / `ai-stub` only. Real legal analysis deferred since M1 Plan 05. |
| **U-13** | P1 | Content/Admin | **Explanation expert verify at product scale** | partial | U-12, M3 UI | CLI `gendraft` / `verifyexplanation` exist; **no** admin queue UI (M3). User sees only `verified`. |
| **U-14** | P1 | Ops/CI | **Playwright e2e in CI** | **partial** | Staging or compose in GHA | CI `e2e` job runs Chromium smoke; optional `secrets.E2E_AUTH_TOKEN` injects `at` cookie for session-gate shells (skip when absent). Full-stack API journeys still need backend/compose + real JWT. |
| **U-15** | P1 | Ops/CI | **Redis service in GitHub Actions** | **done** | — | Backend CI job now runs `redis:7-alpine` + `TEST_REDIS_URL` so `redisx.NewTest` packages exercise a real Redis. |
| **U-16** | P1 | FE | **FSRS due practice UX (`GET /learn/next`)** | **done** | Learning BE ✅ | Session `mode=review` uses `learning.NextDue`; Practice “Takrorlash” source + dashboard CTA. |
| **U-17** | P1 | Docs/FE | **Landing footer real contacts** | partial | Marketing inputs | Placeholder phone/address/TG/IG in `messages/*` (`+998 71 200 00 00`, etc.). |
| **U-18** | P2 | Design | **J8 Figma SoT** | deferred | Optional | Explicitly optional in next-wave; not a gate for Arena or N2. |
| **U-19** | P2 | Design/FE | **J7 residual / visual QA depth** | partial | Sibling | Checklist largely signed in `visual-qa-checklist.md`; deeper dark×locale pixel walk open; handoff J-table may still lag. Do not thrash sibling hot files. |
| **U-20** | P2 | FE | **N4 chrome tech debt** | partial | U-19 | Sticky-CTA gaps closed (Premium mobile buy + Stats due→`mode=review`). Provider picker dots on Asphalt tokens. Static `/logo.svg` chrome → `BrandLogo` (`next/image`). Remaining: content `no-img-element` on dynamic MinIO/CDN media (accepted); footer contacts = U-17. |
| **U-21** | P2 | BE | **Promo pro-rate user communication** | **done** | Billing | `me/entitlement.proration` + checkout success notice when promo exhausted mid-flight. |
| **U-22** | P2 | BE | **Leaderboard rebuild cap approximation** | partial | M4-01 ✅ | Documented low-risk: rebuild uses current VIP/cap, not perfect historical fidelity. |
| **U-23** | P2 | BE | **Drop or document dead `referral_attribution`** | deferred | U-05 | Parallel unused table vs live `referral` (0015). Antifraud design recommends drop later. |
| **U-24** | P2 | FE | **Demo multi-question strength** | **done** | Demo BE | `demoQuestionCount=5` (first 5 of free bilet 1); random draw + whitelist enforcement tests updated. |
| **U-25** | P2 | BE | **SMS OTP (Eskiz/PlayMobile)** | deferred | Config | `OTP_CHANNEL=sms` rejected — “no sender implementation”. Telegram Gateway + sandbox only. |
| **U-26** | P2 | BE | **Anonymous / pre-login event capture** | deferred | Events M1 | Authenticated-only `POST /events`; anon deferred in Plan 05. |
| **U-27** | P2 | BE | **Payment provider reconciliation job** | **partial** | Prod payments | Local dry-run skeleton: `cmd/payrecon` + `internal/billing/recon` (payment vs payme/click txn consistency; mirrors GetStatement window). Live outbound provider APIs + admin findings queue still need prod keys / M3. |
| **U-28** | P2 | Content | **LegalRefs machine extraction** | partial | Import | Comments imported as prose; structured `legal_refs` largely empty (honest gap in import handoff). |
| **U-29** | P2 | Content | **15 biletsiz leftover questions** | partial | Import design | Valid for practice/FSRS; not in numbered bilets — product copy/UX may under-explain. |
| **U-30** | P2 | Content | **Signs catalog licensing pipeline** | partial | Research ✅ | Live catalog exists (gensigns / seed path); research still flags lex.uz extraction **UNVERIFIED** pieces — harden provenance for legal comfort. |
| **U-31** | P2 | Content | **Category name native review** | partial | Taxonomy | Older handoff: user said “edits coming” for 13 category names — confirm closed. |
| **U-32** | P2 | FE | **SEO public pages (jarimalar, oferta, privacy, narxlar)** | **done** | Marketing | Public `/{locale}/oferta`, `/privacy`, `/narxlar`, `/jarimalar` (+ footer links, i18n, vitest/e2e). Jarimalar is an **honest SEO shell** (no fines API/catalog yet — no invented amounts); full lex.uz-backed reference remains a content follow-up. |
| **U-33** | P2 | FE | **`exam-mockup` route** | **done** | Legacy | Kept as auth-gated **dev/visual QA** component playground (`QuestionCard` states); real exam = session engine. Not linked from sidebar. |
| **U-34** | P2 | i18n | **Backend `kaa` locale** | partial | Product decision | BE `i18n.Supported` includes `kaa`; FE messages = uz-Latn/uz-Cyrl/ru only. Incomplete Karakalpak. |
| **U-35** | P2 | FE | **Grand Mock “certificate”** | partial | M2-07 ✅ | UI dialog + confetti only — **no** persisted certificate, PDF, shareable ID, or admin-issued credential. |
| **U-36** | P2 | Bot | **`/unlink` + bot i18n** | deferred | U-10 | Documented TODO in M4-06 design. |
| **U-37** | P2 | Ops | **Makefile frontend targets** | **done** | — | `make fe-install` / `fe-lint` / `fe-typecheck` / `fe-test` / `fe-build` / `fe-e2e` / `fe-check`. |
| **U-38** | P3 | M6 | **PWA foundation (manifest, SW, install)** | **done** | — | Manifest + SW + appleWebApp + Asphalt SVG mark + PNG `logo-512` / `apple-touch-icon` + BrandLogo chrome. Install prompt UX polish still optional. |
| **U-39** | P3 | M6 | **Offline content cache + sync** | **partial** | U-38 | Shell slice done: `sw.js` precache + network-first nav → `offline.html`; static cache-first. Full offline exam/content sync (bilets/signs) still open. |
| **U-40** | P3 | M5 | **B2B orgs / seats / teacher dashboard** | missing | Customer + entitlement `b2b` source already in CHECK | Schema allows `source='b2b'`; no org tables/packages. |
| **U-41** | P3 | M7 | **Observability (metrics, tracing, alerting)** | partial | — | Liveness `/healthz` + readiness `/readyz` (Postgres/Redis checks) documented in README + STAGING-RUNBOOK + smoke. FE ops stub `/{locale}/ops/health` aggregates both via BFF. No Prometheus/Sentry yet. |
| **U-42** | P3 | M7 | **Load-test (k6) + perf audit** | missing | Staging | |
| **U-43** | P3 | M7 | **Security audit + dependency scan** | **partial** | — | Standing CI `dependency-scan` job: `govulncheck ./...` (hard gate) + `npm audit` JSON artifact/warnings + Dependabot (npm/gomod/actions) + `make dep-scan`. Bumped `golang.org/x/text` for GO-2026-5970. FE critical/high (Next 14 / next-intl 3 majors) deferred; full security checklist / pen-test still open. |
| **U-44** | P3 | M7 | **Backup + DR drill** | missing | Host | Compose volumes local-only. |
| **U-45** | P3 | M3 | **Super Admin entire vertical** | partial | Most product features | **SoT locked:** [`2026-07-26-m3-super-admin-control-center.md`](./2026-07-26-m3-super-admin-control-center.md). Thin ops precursors: `/{locale}/ops/providers` (kill-switch UX) + `/{locale}/ops/health` (healthz/readyz stub). Full admin RBAC/CMS/monitoring still missing. |
| **U-46** | P3 | M3 | **Investor / Metabase–Grafana dashboards** | missing | Events + U-45 | Events ingestion exists; no BI layer. |
| **U-47** | P3 | M3 | **Feature flags / support inbox** | missing | U-45 | Master admin scope. |
| **U-48** | P3 | Arena | **RedisTransport multi-instance** | deferred | U-06 single-instance | Locked Q11: LocalTransport at launch. |
| **U-49** | P3 | Arena | **Practice bot opponent** | deferred | Product Q10 | Deferred to M4-05 decision; not in M4-03. |
| **U-50** | P3 | Docs | **Handoff / roadmap status refresh** | partial | After each wave | SESSION-HANDOFF §4, design-system J-table, roadmap M4 rows drift vs code. |

### Explicitly beyond Arena UI (must not be forgotten)

- **Admin (M3)** — largest intentional backlog; content verify + billing ops cannot scale without it.
- **Production payments** — keys, legal entity, refund revoke, recon.
- **Telegram** — foundation ≠ product: FE link **done** (U-09); M4-07 quiz/notif still deferred.
- **FSRS gaps** — engine + due-queue FE (**U-16 done**); mistakes UX depends on FSRS timing (documented, easy to misread as bugs).
- **Arena** — BE infra + rating/history + FE UI **done** (U-06…08); RedisTransport multi-instance + practice bot still deferred (U-48/U-49).
- **Referral antifraud** — **done** (U-05).
- **Grand Mock** — eligibility/session **done**; certificate is **theater**, not a credential system.
- **i18n** — 3 UI locales mostly; `kaa` half-supported; bot copy single-locale; some historical hardcode risks.
- **E2E CI / staging / Docker / monitoring / backup** — ship blockers independent of Arena.
- **Content pipeline** — stub AI, LegalRefs, leftover questions, signs provenance.
- **PWA + B2B** — PWA foundation + offline shell **partial** (U-38/U-39); full offline content sync + B2B still open.

---

## 3. Recommended revised end-to-end sequence

**Do not** treat “J10 green” as project complete. Suggested true sequence:

```
A0  Stabilize: merge sibling J7/N2 safely; refresh SESSION-HANDOFF statuses
A1  N2 demo→account migrate (mistakes/FSRS-aligned; no Grand Mock inflation without OK)
A2  Money hardening (can parallel A1): referral antifraud (U-05); refund revoke (U-04);
    promo pro-rate UX (U-21); confirm returnURL + sandbox E2E manually
A3  Ops MVP (can parallel): Dockerfiles + D18 staging; add Redis + Playwright to CI
 │
 ├─ Growth track
 │   B1  M4-03 Arena infra (expand plan → T1–T4; mig 0021)
 │   B2  M4-04 rating/medals/history
 │   B3  J10 / M4-05 Arena UI (Asphalt tokens)
 │   B4  FE Telegram link (U-09) → M4-07 quiz/notif → M4-08 push
 │
 ├─ Learning/content track (parallel with Growth)
 │   C1  FE due-FSRS / learn/next UX (U-16)
 │   C2  Real LLM draft OR expert batch-verify path (U-12/U-13)
 │   C3  SEO/legal pages + real footer contacts (U-32 done shell; U-17 contacts still partial)
 │
 ├─ Prod money flip (external gate)
 │   D1  Legal entity → Payme/Click prod keys (U-03) + recon job (U-27)
 │
 ├─ Distribution
 │   E1  PWA M6-01…03 when retention metrics justify
 │
 ├─ B2B when a school customer appears (M5)
 │
 └─ M3 Super Admin LAST — build against the real feature set above (U-45…)
     then M7 scale hardening continuous with prod traffic
```

**Anti-patterns:** Arena UI before wire protocol; Admin before monetization ops; claiming M2 “prod-ready” without revoke/antifraud/keys; wiping/reseeding content DBs; redesigning locked official exam desktop.

---

## 4. What is already DONE (short contrast)

| Area | Done |
|------|------|
| **M1 BE** | Auth OTP (sandbox/Telegram Gateway), JWT+refresh, sessions/scoring/unlock, FSRS engine + stats APIs, entitlement gating, content API, explanations model + stub draft, saved/streak/events, demo endpoints |
| **Content** | Real import ~1231–1235 Q / ~61–62 bilets / 13 categories / signs catalog present; 3 locales |
| **M1 FE** | Landing+demo, login/verify BFF cookies, dashboard, tickets, practice, signs, mistakes, saved, stats, profile, session engine, official exam view (desktop locked) |
| **M2** | Tariffs, Payme+Click **sandbox**, checkout+promo, payments history, referral BE+UI (+ capture fixes), Grand Mock eligibility+session+UI card/dialog, guest demo funnel |
| **M4 partial** | Leaderboard BE+UI; Telegram bot **foundation** (link-token, `/start`/`/link`/`status`, webhook/longpoll wiring) |
| **Chrome** | Driver Go Asphalt & Signal J0–J6/J6b/J9 largely landed; visual QA checklist largely signed (verify after sibling merge) |
| **Dev ergonomics** | `docker compose`, `run.sh`, `make test`/`test-parallel`, sqlc, seed/seed-real, `.env.example` |

---

## 5. Proposed “project complete” definitions (pick one — don’t conflate)

| Label | Definition | Arena UI? |
|-------|------------|-----------|
| **MVP launch (B2C)** | Real OTP channel; sandbox or prod payments with revoke+antifraud; demo migrate; staging URL; e2e smoke in CI; legal pages; no critical money bugs | Optional |
| **Growth-complete** | MVP + Arena playable end-to-end + TG quiz/link FE + push | **Required** |
| **Platform-complete** | Growth-complete + Admin (M3) + PWA + monitoring/backup + (optional) B2B | Arena is only one chapter |

**Recommendation for parent messaging:**  
“Arena UI oxirgi emas — u Growth to‘lqini ichidagi oxirgi FE qadam. Loyiha tugashi = Admin + prod to‘lov + ops + kontent sifati + (ixtiyoriy) PWA/B2B.”

---

## 6. Sibling / collision notes

- Sibling agent may commit **J7 / N2** — treat U-01 / U-19 as **owned elsewhere** until merge; this inventory must not rewrite those hot paths.
- Safe follow-ups independent of that merge: Arena **plan expansion** (docs), antifraud implementation (billing), Docker/CI, Admin brainstorming, M4-07 design, SEO pages.

---

*Inventory only. Next implementer: pick a wave from §3; do not start from J10 alone.*
