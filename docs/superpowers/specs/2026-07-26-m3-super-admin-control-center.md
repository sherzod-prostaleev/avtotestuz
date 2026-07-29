# Driver Go — M3 Super Admin Control Center
**SOURCE OF TRUTH for Admin Panel requirements · 2026-07-26**

> Brand: **Driver Go**. Product domain may remain avtotest.uz.
> This document **supersedes** the thin M3 blurbs in master design §18 and roadmap §7 for **scope and UX depth**.
> Scheduling note: roadmap historically put M3 last; **requirements are locked now** so implementation can start when Growth (M4) critical path allows, without rediscovering scope.
> Content safety: Admin CRUD must never silently wipe licensed avtoimtihon corpus; delete/reimport follows session-referenced upsert rules (importer constraints).

---

## 0. Mission statement (non-negotiable)

Admin Panel is the system’s single **brain and hand**:

- **Brain:** observability, analytics, audit, feature flags, config.
- **Hand:** users, content, billing, CMS surfaces, investors, security.

**Rule:** If an operator needs SSH/SQL/Redis-CLI for a recurring business action, that action is an **Admin gap**. Exception: one-time infra disasters (restore drill) may stay in runbooks, but must be *linked* and *logged* from Admin.

**Identity model (product fact — updated 2026-07-26):** Learner auth is **phone + password** (`POST /auth/register`, `/auth/login`; bcrypt `profile.password_hash`). OTP routes may remain for legacy/bootstrap but are **not** the learner UI primary path. Admin staff accounts stay separate (`admin_user`).

| Learner wording | Behavior |
|-----------------|----------|
| View password | **Forbidden** — hashes only; never return plaintext |
| Reset password | Admin-triggered invalidate sessions + user sets new password via authenticated change-password (or one-shot `set-password` when hash was NULL for OTP-era accounts) |
| Login | Phone E.164 + password |

---

## 1. Technical stack (locked for M3)

| Layer | Choice | Why |
|-------|--------|-----|
| Admin SPA | **Next.js 14 App Router** under `frontend/` route group `[locale]/(admin)` **or** `apps/admin` package — **TypeScript** | Same monorepo as Driver Go web; one design-token pipeline |
| UI kit | **shadcn/ui** + Tailwind; tokens from **Asphalt & Signal** (`globals.css`) — amber CTA, asphalt surfaces | Consistency with public app; **no indigo SaaS default** |
| Charts | Recharts or Tremor (TS) | Dashboard density |
| Tables | TanStack Table + virtualization | 100k+ users |
| Realtime | **SSE first** for dashboards/logs; **WebSocket** for collaborative content locks & live ops feed | SSE simpler behind BFF; WS where bidirectional needed |
| API | Existing **Go chi** monolith: mount `/admin/v1/**` with `admin.Required` middleware | One deploy unit; sqlc + Redis already present |
| AuthN | Separate **admin session** (httpOnly JWT) ≠ learner OTP session; optional **passkey/TOTP 2FA** | Blast-radius isolation |
| AuthZ | **RBAC + permission matrix** in Postgres | Auditable |
| Secrets | Payment keys in **encrypted vault table** (AES-GCM, envelope key in env `ADMIN_SECRETS_KEK`) + optional sync display of masked env | Never show raw key twice after create |
| Docs | OpenAPI for `/admin/v1` + this spec | |

**Deviation from 2026-07-17 master design:** Admin was sketched as Flutter web. **User requirement (2026-07-26) locks TypeScript + shadcn.** Flutter admin is **cancelled** for M3.

---

## 2. Information architecture — sidebar + routes

Base path: `/{locale}/admin` (locale for operator UI: `uz-Latn` | `uz-Cyrl` | `ru`).

```
Admin
├── Overview                    /admin
├── Monitoring
│   ├── System health           /admin/monitoring/health
│   ├── API & DB performance    /admin/monitoring/perf
│   ├── Live logs               /admin/monitoring/logs
│   ├── Jobs & workers          /admin/monitoring/jobs
│   └── Alerts                  /admin/monitoring/alerts
├── Users
│   ├── Directory               /admin/users
│   └── User detail             /admin/users/[id]
├── Content
│   ├── Questions               /admin/content/questions
│   ├── Tickets (bilets)        /admin/content/tickets
│   ├── Signs                   /admin/content/signs
│   ├── Categories & tags       /admin/content/taxonomy
│   ├── Explanations queue      /admin/content/explanations
│   ├── Media library           /admin/content/media
│   └── Import / export         /admin/content/io
├── Learning & Sessions
│   ├── Sessions browser        /admin/learning/sessions
│   ├── Mistakes / FSRS sample  /admin/learning/fsrs
│   └── Demo migrate audit      /admin/learning/demo-migrate
├── Payments
│   ├── Transactions            /admin/payments/transactions
│   ├── Refunds                 /admin/payments/refunds
│   ├── Webhooks                /admin/payments/webhooks
│   ├── Providers & keys        /admin/payments/providers
│   └── Tariffs / promo / limits /admin/payments/catalog
├── Growth
│   ├── Referrals               /admin/growth/referrals
│   ├── Leaderboard ops         /admin/growth/leaderboard
│   ├── Arena matches           /admin/growth/arena
│   └── Telegram bot            /admin/growth/telegram
├── CMS (site)
│   ├── Homepage                /admin/cms/home
│   ├── Header / footer / nav   /admin/cms/chrome
│   ├── Brand & theme           /admin/cms/brand
│   ├── Popups / banners        /admin/cms/surfaces
│   └── Legal pages             /admin/cms/legal
├── Investors                   /admin/investors
├── Analytics
│   ├── Product dashboard       /admin/analytics/product
│   ├── Revenue                 /admin/analytics/revenue
│   ├── Funnels                 /admin/analytics/funnels
│   └── Exports                 /admin/analytics/exports
├── Settings
│   ├── Feature flags           /admin/settings/flags
│   ├── Env & runtime config    /admin/settings/config
│   ├── i18n / timezone / money /admin/settings/locale
│   ├── Maintenance & limits    /admin/settings/ops
│   └── Cache                   /admin/settings/cache
├── Security
│   ├── Admins & RBAC           /admin/security/rbac
│   ├── Audit log               /admin/security/audit
│   ├── Sessions & 2FA          /admin/security/sessions
│   └── IP allowlist            /admin/security/ip
└── Support
    ├── Inbox                   /admin/support/inbox
    └── Broadcasts              /admin/support/broadcasts
```

**Density rule:** Overview = 1 screen, ≤6 KPI tiles + 3 charts + alert strip. No card forest.

---

## 3. Roles & permission matrix

### 3.1 Roles (seed)

| Role | Scope |
|------|--------|
| `superadmin` | All permissions; secrets create/reveal-once; hard-delete; KEK rotation trigger |
| `admin` | Ops + content + users + billing refund (policy-bound); no raw secret create |
| `editor` | Content CRUD + explanation queue; no billing keys; no hard-delete users |
| `support` | Users read, soft-block, entitlement view, support inbox; no content publish |
| `finance` | Payments read, refund initiate, export; no content write |
| `investor_viewer` | Investors + analytics **read-only** (no PII beyond aggregated) |
| `analyst` | Analytics + exports; no mutations |

### 3.2 Permission keys (examples — full matrix in DB)

```
monitoring.read | monitoring.alerts.manage
users.read | users.write | users.block | users.hard_delete | users.impersonate_forbid
users.sessions.revoke | users.entitlements.grant
content.questions.* | content.tickets.* | content.signs.* | content.publish | content.verify
payments.read | payments.refund | payments.keys.manage | payments.catalog.write
cms.* | investors.* | analytics.read | analytics.export
settings.flags | settings.config | settings.maintenance
security.rbac | security.audit.read | security.ip
support.inbox | support.broadcast
```

Every UI control and every `/admin/v1` route checks **permission**, not role name alone.

---

## 4. Module specifications (depth)

### 4.1 Monitoring (system brain)

**Health page**
- Components: API process, Postgres, Redis, MinIO, (future) arena WS, Telegram bot worker.
- Fields per component: status (`up`/`degraded`/`down`), latency p50/p95, last check, version/git SHA.
- Host metrics (node exporter or lightweight agent): CPU %, RAM %, disk %, load — poll ≤5s via SSE.

**Perf page**
- API: RPS, p50/p95/p99 latency by route group, 5xx rate, auth failure rate.
- DB: slow query feed (threshold config), connection pool in-use/max, lock waits.
- Redis: memory, hit rate, `arena:` vs `lb:` keyspace sizes (namespace-aware).

**Live logs**
- Tail structured zap JSON; filters: level, service, request_id, user_id, route.
- Click row → full JSON + correlate to audit/payment webhook if IDs present.
- Retention: hot 7d in searchable store; warm archive policy in Settings.

**Jobs**
- List background workers: OTP sender, entitlement grant, webhook retry, FSRS batch, arena matchmaker heartbeat, bot polling.
- Actions: pause/resume (feature-flagged), requeue failed job, view last N failures.

**Alerts**
- Rules: error_rate > X for Y min; disk > 85%; payment webhook fail streak; VIP grant lag.
- Channels: Telegram admin chat, email (config).
- Ack / snooze with audit.

**Realtime:** SSE `GET /admin/v1/monitoring/stream`.

---

### 4.2 User Management (highest priority vertical)

**Directory**
- Columns: phone (masked by permission), name, locale, VIP state, streak, created_at, last_seen_at, status (`active`/`blocked`/`deleted`).
- Search: phone E.164, name, user UUID, referral code.
- Filters: VIP, blocked, registered range, locale, has_payment, arena_played.
- Sort: any column; saved views per admin.
- Bulk: block, unblock, export CSV (permissioned), add tag.

**Detail — tabs**
1. **Profile** — identity, locale, region, created/updated, referral edges.
2. **Security** — sessions list (device UA, IP, created, last_active); revoke one/all; OTP lockout state; **no password**. Soft actions: force logout, require OTP re-verify.
3. **Activity** — timeline from `event` + session finishes + payment + referral (cursor pagination).
4. **Learning** — tickets progress, sessions, mistakes count, FSRS due count, readiness %, grand mock eligibility flags (read-only computation).
5. **Content refs** — saved questions, demo-migrate history.
6. **Billing** — entitlements, transactions, refunds.
7. **Devices / IP** — from auth/session tables + fingerprint if present (referral antifraud signals — view only until antifraud ships).
8. **Admin notes** — internal notes with author + timestamp.

**Destructive actions (modal + typed confirm + reason required)**
- Block / unblock (soft).
- Soft-delete (GDPR-oriented anonymize job).
- Hard-delete (**superadmin only**, dual confirmation, cool-down 5 min, full audit).
- Role assignment: **learner roles are not admin roles** — learners get `profile` flags; staff get `admin_user` link only for operator accounts.

**Impersonation:** **Forbidden** by default (`users.impersonate_forbid`). If ever enabled: time-boxed, watermarked UI, audit every API call.

---

### 4.3 Content & data control

**Questions / Tickets / Signs / Taxonomy**
- Full CRUD with draft → review → verified → archived workflow.
- Bulk: verify, unpublish, retag, reassign category.
- Version history: every publish creates immutable `content_revision` (diff UI).
- SEO fields where public (signs catalog): slug, title, description (locale rows).
- Media: MinIO browser, replace image with reference integrity check.
- **Import/export:** JSON/CSV/Excel; dry-run validation report; **never truncate** production corpus from UI without `superadmin` + maintenance mode + typed `DELETE CORPUS` (default: upsert-only).

**Explanations queue**
- Filter: unverified, low helpfulness, AI-draft.
- Actions: edit blocks, verify, reject, assign editor.

**Learning browser**
- Inspect any `exam_session` / practice session (answers, timing) for support — PII gated.

---

### 4.4 Payment & integrations

**Transactions**
- Status, provider (`payme`/`click`), amount, user, entitlement link, created/paid.
- Drill-down: raw provider payload (redact secrets), idempotency key, webhook chain.

**Refunds**
- Initiate refund → provider API → **entitlement revoke** (must be implemented; currently deferred in product — Admin UI blocked until BE revoke exists).
- Partial/full; reason codes; dual-approve if amount > threshold (config).

**Webhooks**
- Inbox of deliveries: success/fail, retries, replay button (idempotent).

**Providers & keys**
- Store encrypted keys; UI shows `pk_live_***`; reveal-once for superadmin with re-2FA.
- Rotate, disable provider, sandbox/prod toggle (env sync status badge).

**Catalog**
- Tariffs, promo codes, free limits, referral reward days, grand_mock thresholds — all editable without deploy (config rows + audit).

---

### 4.5 Investors

- Entities: investor, ownership_share, document (MinIO), contact_log, access to investor_viewer dashboards.
- Dashboard: MRR, growth, churn (from analytics) — **no raw user phones** for investor_viewer.
- Document versioning + NDA flag.

---

### 4.6 CMS — all dynamic site surfaces

Editable via typed blocks (JSON schema), not free HTML (XSS control):

| Surface | Fields |
|---------|--------|
| Homepage | Hero title/subtitle/CTAs, proof facts, method copy, FAQ, bottom CTA, contact strip |
| Chrome | Header links, footer columns, social URLs, phone, email, address, hours |
| Brand | Product name, logo, favicon, theme tokens override (constrained to Asphalt palette vars) |
| Surfaces | Banners, popups, notification toasts (targeting rules) |
| Legal | Offer, privacy, refund policy (locale markdown/MDX sanitized) |

**Publish workflow:** draft → preview link → publish → CDN/cache purge hook.
**Realtime:** operators see “someone else editing” locks via WS room `cms:{surface}`.

---

### 4.7 Global settings

- Feature flags (boolean/percentage/user allowlist).
- Runtime config key-value with typed schema (number/string/json) + validation.
- Maintenance mode (global or path-prefix) with bypass IP allowlist.
- Rate limits (per route class).
- Cache: purge by tag (`content`, `signs`, `tariffs`).
- Locale defaults, timezone (`Asia/Tashkent`), currency (`UZS`).

---

### 4.8 Security & audit

**Audit log (append-only)**
- Who (`admin_user_id`), when, action, entity_type, entity_id, before/after JSON diff, IP, UA, request_id.
- Immutable: no UPDATE/DELETE via API; retention job only archives.

**2FA:** TOTP required for `superadmin` and `payments.keys.manage`.
**IP allowlist:** optional per admin user or global.
**Session management:** list/revoke admin sessions; absolute TTL + idle TTL.

---

### 4.9 Analytics & reporting

- Real-time: signups, active sessions, checkout starts/paid, arena matches, error rate.
- Funnels: visit → OTP → first practice → paywall → paid.
- Content: hardest questions (error %), ticket completion.
- Export jobs: CSV/XLSX/PDF — async with download when ready.
- Investor-ready metrics: MRR, DAU/MAU, D1/D7/D30, ARPU, LTV proxies, churn.

Data source: existing `event` stream + SQL rollups; ClickHouse only if M7 scale triggers.

---

### 4.10 Support & broadcasts

- Inbox: user tickets / Telegram forwards (when bot linked).
- Broadcast: in-app + Telegram — segment by VIP/locale; dry-run count; schedule; audit.

---

## 5. Data model (core entities)

New / extended tables (names indicative; sqlc + migrate `00xx_admin_*`):

```
admin_user (id, email, phone?, display_name, status, totp_secret_enc, created_at)
admin_role (id, code, name)
admin_permission (id, code, description)
admin_role_permission (role_id, permission_id)
admin_user_role (admin_user_id, role_id)
admin_session (id, admin_user_id, refresh_hash, ip, ua, expires_at)
admin_audit_log (id, admin_user_id, action, entity_type, entity_id, before_json, after_json, ip, ua, created_at)
admin_ip_allowlist (id, cidr, label, created_by)

feature_flag (key, type, value_json, updated_by, updated_at)
runtime_config (key, value_json, schema_version, updated_by, updated_at)

cms_document (id, surface, locale, status, body_json, version, published_at, updated_by)
cms_document_revision (id, document_id, body_json, editor_id, created_at)
cms_lock (surface, locale, admin_user_id, expires_at)

content_revision (id, entity_type, entity_id, snapshot_json, editor_id, created_at)

payment_provider_secret (id, provider, kind, ciphertext, key_version, created_by, created_at, revoked_at)
admin_note (id, subject_type, subject_id, body, author_id, created_at)

investor (id, name, share_bps, status, ...)
investor_document (id, investor_id, media_key, title, ...)
investor_contact_log (id, investor_id, body, author_id, created_at)

alert_rule / alert_event
export_job (id, type, params_json, status, result_media_key, created_by)
```

Reuse existing: `profile`, `entitlement`, `payment`, `question`, `variant`, `sign`, `exam_session`, `event`, etc. Admin APIs join — do not duplicate learner PII into admin tables.

---

## 6. Security architecture

1. **Network:** Admin only on VPN / IP allowlist in prod; separate subdomain `admin.drivergo.uz` recommended.
2. **AuthN:** email+password **or** email+OTP for operators (staff), not learner phone OTP — staff accounts are `admin_user`, not `profile`.
3. **AuthZ:** middleware loads permission set once per request; deny by default.
4. **Secrets:** envelope encryption; audit every reveal; UI never persists plaintext in browser storage.
5. **CSRF:** SameSite cookies + origin check for cookie sessions.
6. **XSS:** CMS sanitized; CSP strict on admin origin.
7. **SQL:** sqlc only.
8. **Rate limit:** stricter on login, refund, secret reveal.
9. **Separation:** Learner JWT cannot call `/admin/v1`; admin JWT cannot call learner mutating APIs unless explicitly bridged (default: no).

---

## 7. UX / UI principles (Admin)

- Professional dense **Linear/Raycast** density + Asphalt tokens (not marketing hero).
- shadcn: DataTable, Command palette (⌘K), Sheet, Dialog, Toast.
- Dark/Light via existing `next-themes`.
- Mobile: responsive tables → card list; **critical ops** usable on tablet.
- **Undo/Redo:** command stack for CMS + content editors (in-session); server-side “revert to revision” for published content.
- Bulk ops + keyboard shortcuts documented in `/admin/help/shortcuts`.
- Optimistic UI only where realtime confirms; destructive = confirm.

**Component examples**
- `<MetricTile label value delta sparkline />`
- `<AuditDiff before after />`
- `<SecretField masked onReveal={reauth} />`
- `<PublishBar draft preview publish />`
- `<PermissionGate perm children />`

---

## 8. Realtime contract

| Channel | Transport | Payload |
|---------|-----------|---------|
| `monitoring.stream` | SSE | health, metrics sample |
| `logs.stream` | SSE | log lines |
| `cms.lock` | WS | presence |
| `ops.feed` | WS | payment paid, alert fired |
| `export.ready` | SSE or push | job id |

All realtime auth via admin session cookie / short-lived ticket (same pattern as arena WS ticket).

---

## 9. Implementation phases (inside M3)

| Phase | Deliverable | Sessions (est.) |
|-------|-------------|-----------------|
| **M3-0** | `admin_user`, RBAC, audit_log, admin login+2FA, empty shell | 2 |
| **M3-1** | Users directory + detail + block/sessions | 2 |
| **M3-2** | Content studio + revisions + explanations queue | 3 |
| **M3-3** | Payments UI + refund+revoke + webhook tools + catalog | 2 |
| **M3-4** | CMS surfaces + brand | 2 |
| **M3-5** | Monitoring + jobs + alerts | 2 |
| **M3-6** | Analytics + exports + investors | 2 |
| **M3-7** | Support, broadcasts, flags, hardening | 1–2 |

**Dependencies before M3-3:** BE entitlement revoke on refund.  
**Dependencies before full monitoring:** metrics exporters (can stub with `/healthz` + process metrics first).

---

## 10. Acceptance criteria (“100% control center”)

- [ ] Every recurring operator action has an Admin screen + permission + audit entry.
- [ ] Learner password N/A; session/OTP controls cover account takeover response.
- [ ] Content edits versioned; corpus wipe path is superadmin-only and logged.
- [ ] Payment keys never stored in plaintext at rest; refund revokes VIP.
- [ ] CMS can change landing contacts/footer without deploy.
- [ ] Monitoring shows API/DB/Redis/MinIO + error rate with alert path.
- [ ] RBAC matrix enforced on UI and API identically (integration tests).
- [ ] Realtime feeds for monitoring and CMS locks work behind admin auth.
- [ ] Type-safe TS admin + Go `/admin/v1` OpenAPI published.
- [ ] Dark/light, responsive, ⌘K, bulk actions on users/content.

---

## 11. Explicit non-goals / anti-patterns

- Admin is **not** a second marketing site.
- No indigo/purple “AI admin” theme — Asphalt & Signal.
- No embedding raw SQL console for editors.
- No learner impersonation without separate security review.
- No `make seed-real` / truncate from Admin without the dual-confirm nuclear path.
- Do not block Growth (Arena) forever on full M3 — ship M3-0/M3-1 early if needed for support, then deepen.

---

## 12. Traceability

| Source | Relationship |
|--------|----------------|
| User brief 2026-07-26 | This document expands 1:1 |
| Master design §18 | Superseded in depth; stack updated to TS |
| Roadmap M3 | Phases above map to ~13 sessions |
| `2026-07-26-full-project-unfinished-inventory.md` U-45…U-47 | Tracked here as M3-* |

---

*Status: REQUIREMENTS LOCKED for planning. Implementation starts with M3-0 when scheduled. Not part of Asphalt J7 / Arena E5 critical path unless explicitly pulled forward.*
