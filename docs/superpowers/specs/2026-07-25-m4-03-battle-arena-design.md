# M4-03: Battle Arena — Realtime Infra (Design Spec)

Sana: 2026-07-25 · Milestone: M4 (Growth) · Plan: M4-03 · Qatlam: backend
Manba: `docs/superpowers/2026-07-24-roadmap-m2-to-admin.md` §3 (M4-03 T1–T4), `AVTOTEST-MASTER-PROMPT.txt` (Battle Arena PvP)

> Bu hujjat **dizayn**, implementatsiya emas. Hech qanday WebSocket/matchmaking kodi bu to'lqinda yozilmaydi.
> Til: texnik qism ingliz tilida (kod izohlari va `plans/2026-07-25-m4-01-leaderboard.md` bilan bir xil), §0 va §12 —
> qaror talab qiladigan qismlar — o'zbekcha, chunki ular Sherzodning javobini kutadi.

---

## 0. Qisqa xulosa (o'zbekcha)

**Nima quriladi (M4-03):** ikki foydalanuvchi bir vaqtda bir xil savollarga javob beradigan **jonli 1v1 duel** uchun poydevor —
WebSocket transporti, JWT-asosidagi ulanish autentifikatsiyasi, Redis navbati orqali juftlash, va match'ning server tomonidagi
state-machine'i (savol sinxronlash, javob qabul qilish, taymer, uzilish/qayta ulanish, ochko hisoblash). Reyting/medal
(ELO, Bronza→Brilliant) **M4-04**da, UI **M4-05**da.

**Beshta asosiy qaror:**

1. **WebSocket kutubxonasi — `github.com/coder/websocket`** (eski nomi `nhooyr.io/websocket`), `gorilla/websocket` emas. Sabab: bu
   kod bazasidagi **har bir** servis metodi birinchi argument sifatida `context.Context` oladi; `coder/websocket` API'si
   context-asosida (`Accept`, `Read`, `Write`, `Ping` — hammasi ctx qabul qiladi), gorilla esa `SetReadDeadline`
   uslubida ishlaydi va har match goroutine'ida ctx→deadline ko'prigini qo'lda yozishga majbur qiladi. Batafsil: §2.1.
2. **Ulanish auth — bir martalik "ws-ticket"**, WebSocket URL'ida JWT emas. Sabab **bu loyihaning aniq arxitekturasi**:
   frontend BFF naqshida ishlaydi — access-token `httpOnly` cookie'da (`frontend/src/lib/auth-cookies.ts`) va brauzer
   JavaScript'i uni **umuman o'qiy olmaydi** (`api-client.ts` hamma so'rovni `/api/proxy/*` orqali yuboradi). Ya'ni
   brauzer WS ochayotganda qo'lida token yo'q. Batafsil: §2.2.
3. **Arena o'z jadvallarini oladi** (`arena_match`, `arena_match_player`, `arena_answer`), `exam_session`ni
   qayta ishlatmaydi. Bu Grand Mock qaroriga (u aynan `exam_session`ni qayta ishlatgan) **teskari** — sabablari §5.1'da
   uchta konkret fakt bilan asoslangan, eng kuchlisi: `session_answer.answer_id` **NOT NULL**, ya'ni "javob bermadi,
   vaqti tugadi" holatini yozib bo'lmaydi, duelda esa bu eng ko'p uchraydigan holat.
4. **Arena ochkolari leaderboard'ga (M4-01) qo'shilmaydi** — o'z jadvallarida bo'lgani uchun bu **avtomatik**
   ta'minlanadi, qo'shimcha filtr shart emas. Agar arena `session_answer`ga yozganda, `RecordPoint` **va**
   `RebuildPeriod` so'rovining **ikkalasiga ham** mode-filtri qo'shish kerak bo'lardi — bittasini unutish M4-01
   Task-8'da topilgan xatoning **aynan o'zini** qaytaradi (jonli yo'l cheklangan, rebuild cheklanmagan). §6.1.
5. **Bir instansiya** (T1–T4), lekin ko'p instansiyaga o'tish **chok**i (seam) oldindan belgilangan: butun
   cross-instance holat Redis'da, xabar yo'naltirish esa `Transport` interfeysi ortida. **Muhim tuzatish:** "sticky
   session" bu muammoni **hal qilmaydi** — WS ulanishi tabiiy ravishda bitta instansiyaga yopishgan; haqiqiy muammo
   ikki **turli** o'yinchining ikki **turli** instansiyaga tushishi, buni faqat Redis pub/sub yo'naltirishi (yoki
   pool'ni bo'lib tashlash) hal qiladi. §8.

**Sherzod qarorini kutadigan savollar:** §12 — 11 ta savol, har biriga tavsiya bilan. Kod yozishdan oldin
**1, 2, 3, 6** raqamli savollarga javob **majburiy** (ular sxema va scoring'ga ta'sir qiladi); qolganlari T3/T4
davomida ham javob olishi mumkin.

---

## 1. Goals and non-goals

### 1.1 M4-03 (this plan) delivers

A playable, server-authoritative 1v1 duel:

- A WebSocket endpoint an authenticated profile can connect to, with a connection registry that enforces one live
  socket per profile.
- A Redis matchmaking queue with rating buckets, widening search, cancel, and timeout. Rating is consumed through
  an **interface** that M4-04 later implements for real (§3.4) — matchmaking code is written once, not twice.
- A match state machine: synchronized question delivery, server-side answer acceptance and scoring, per-question
  reveal, disconnect/reconnect/forfeit, idempotent finish, durable persistence of the result.
- Tests: pure-function unit tests (bucketing, widening, scoring, outcome), Redis integration tests (queue atomicity
  under concurrency), and full-match state-machine tests driven by an injected clock.

### 1.2 Explicitly NOT M4-03

| Not in scope | Where it belongs | Why the split works |
|---|---|---|
| ELO/rating math, medal tiers (Bronza→Brilliant), win streaks | **M4-04** | M4-03 defines `RatingProvider` + persists nullable `rating_before/after/delta` columns. M4-04 fills them in; no schema change needed later. |
| Match history API (`GET /me/arena/matches`), season resets | **M4-04** | The rows M4-04 reads are written by M4-03. Reading them is trivial once they exist and outcome semantics are settled. |
| Any UI: matchmaking screen, live duel, result, friend invite | **M4-05** | M4-03 ships a documented wire protocol (§2.4); M4-05 consumes it. |
| Private/friend matches ("do'stni chaqirish") | **M4-05** per roadmap, but the *transport* hook must be decided now — see §12 Q7 |
| Spectating, tournaments, 3+ player rooms | not planned | Protocol keeps `slot` as a smallint (not a boolean "is_player_a") so a 3rd slot is not schema-blocked, but nothing else accommodates it. |
| Leaderboard coupling | never (recommended) | §6.1 / §12 Q4 |

### 1.3 Non-goals of the design itself

This spec does **not** try to make Arena horizontally scalable on day one, does not introduce a message broker
(NATS/Kafka), and does not introduce a state store beyond the Postgres+Redis the project already runs
(`docker-compose.yml`). Anything that would require new infrastructure is called out as a future seam, not built.

---

## 2. Transport

### 2.1 WebSocket library: `github.com/coder/websocket`

The roadmap line says "gorilla/ws yoki nhooyr". `nhooyr.io/websocket` was renamed to `github.com/coder/websocket`
(v1.8.x) — same library, current import path. **Recommendation: `github.com/coder/websocket`.**

| Candidate | Verdict | Reasoning specific to this codebase |
|---|---|---|
| `github.com/coder/websocket` | **chosen** | Context-first API (`Accept(w, r, opts)`, `c.Read(ctx)`, `c.Write(ctx, ...)`, `c.Ping(ctx)`). Every service in `backend/internal/*` takes `ctx` as the first parameter; a match goroutine that already owns a `context.Context` for its lifetime can pass it straight through instead of translating to deadlines. Zero third-party dependencies — `backend/go.mod` currently has 33 modules and every single one is real (no bloat); adding a dependency-free module keeps that property. Built-in `AcceptOptions.OriginPatterns` gives handshake origin checking, which matters because **the WebSocket handshake is not covered by the CORS middleware** configured in `internal/server/server.go` (and that middleware is currently `AllowedOrigins: ["*"]`). Ships `wsjson.Read/Write` helpers, which is exactly our envelope shape. |
| `github.com/gorilla/websocket` | viable runner-up | Most battle-tested, most examples, maintained again since 2023. Rejected only on ergonomics: no `context` support, so every read/write in a match goroutine needs `SetReadDeadline(time.Now().Add(...))` bookkeeping, and cancelling a match (shutdown, forfeit) means closing the conn from another goroutine rather than cancelling a ctx. If T1's spike hits a blocker with `coder/websocket`, switching is a ~1-day change confined to `arena/conn.go` — the rest of the design is library-agnostic. |
| `golang.org/x/net/websocket` | rejected | Already an indirect dependency (`golang.org/x/net v0.53.0`), so it looks "free". It is deprecated by the Go team, has no ping/pong or close-code support, and its docs recommend gorilla. |
| `github.com/lxzan/gws`, `nbio` | rejected | epoll-based, optimize for 100k+ idle sockets. Arena's realistic peak is hundreds of concurrent sockets; goroutine-per-connection is not the bottleneck. Extra complexity buys nothing. |
| SSE (server-sent events) + `POST` for answers | rejected as primary, noted as fallback | Needs no new library and traverses hostile proxies, but it is two channels to keep in sync, has no client→server backpressure, and every answer pays a full HTTP round trip. Worth remembering as a degraded-mode fallback if Uzbek mobile carriers turn out to break WS (unlikely — Telegram/WhatsApp web work fine). Not built. |

**Concrete integration checks for the T1 spike** (each is a real, verified property of this repo, not hypothetical):

1. `cmd/api/main.go` sets **only** `ReadHeaderTimeout: 5 * time.Second` on the `http.Server` — no `ReadTimeout`,
   `WriteTimeout`, or `IdleTimeout`. Long-lived connections therefore survive. **Do not add** those timeouts later
   without exempting the arena route; a `WriteTimeout` would kill every duel mid-match.
2. `srv.Shutdown(ctx)` **does not wait for hijacked connections**, and a WebSocket upgrade hijacks. So graceful
   shutdown gives Arena nothing for free: the process will exit and drop live matches. Arena must own its own drain
   (§8.4), invoked before `srv.Shutdown`.
3. The chi middleware chain (`middleware.RequestID`, `middleware.Recoverer`, `middleware.Logger` in dev, `cors.Handler`)
   wraps the `ResponseWriter`. chi's wrapper forwards `http.Hijacker` when the underlying writer supports it, so the
   upgrade should work — but **verify it in dev with `middleware.Logger` active**, because that is the wrapper most
   likely to interfere and it is only enabled when `cfg.Env == "dev"` (i.e. a bug here would appear *only* locally, or
   *only* in production, depending on which way it breaks).

### 2.2 Authentication: single-use ws-ticket

**The constraint that decides this.** The frontend is a BFF: `frontend/src/lib/api-client.ts` sends every request to
`/api/proxy/<path>` on the Next.js origin; the Next route handler reads the `at` cookie
(`frontend/src/lib/auth-cookies.ts` — `httpOnly: true`) and adds `Authorization: Bearer <jwt>` when calling the Go
API. Consequences:

- Browser JS **has no access to the JWT**. Any design that says "send the token in the first WS message" or "put the
  token in the `Sec-WebSocket-Protocol` header" is impossible without first exposing the token to JS, which would
  discard the httpOnly protection deliberately built in M1.
- The `at` cookie is set on the **frontend** origin, not the API origin, so it is not sent on a cross-origin WS
  handshake to the API host. Making it work would require `Domain=.avtotest.uz` + `SameSite=None` and would expose the
  socket to cross-site hijacking (WS handshakes ignore CORS).
- Next.js App Router route handlers cannot proxy a WebSocket upgrade, so "route WS through the BFF too" is out
  without running a custom Next server.

**Design:**

```
POST /api/v1/arena/ws-ticket        (behind auth.Required — mounted with the other authed routes)
  → { "data": { "ticket": "<43-char base64url>", "expires_in_sec": 30, "ws_url": "wss://api.../api/v1/arena/ws" } }

GET  /api/v1/arena/ws?ticket=<...>  (NOT behind auth.Required — the ticket IS the credential)
```

- Ticket generation reuses the existing primitives: `crypto/rand` + `base64.RawURLEncoding` exactly as
  `auth.NewRefreshToken()` does (`internal/auth/jwt.go`), so there is one way to mint an opaque token in this codebase.
- Stored as `arena:ticket:<token>` → `profileID`, `TTL 30s`, redeemed with **`GETDEL`** (single Redis round trip,
  atomic, so a ticket replayed from a log or a shared screenshot is dead on second use).
- Ticket minting is rate-limited with the existing `auth.Limiter` (`internal/auth/ratelimit.go` — already generic:
  `Allow(ctx, key, limit, window)`), e.g. `arena:rl:ticket:<profileID>`, 30/minute. This is what stops socket-churn
  DoS; the WS endpoint itself cannot be rate-limited by profile before the ticket is read.
- The ticket is deliberately **not** the JWT: putting a 15-minute bearer token in a URL leaks it into access logs,
  `Referer` headers, and browser history. A 30-second single-use ticket that maps to nothing but a profile ID does not.
- `websocket.Accept` is called with `OriginPatterns` derived from `cfg.PublicBaseURL` (already in config, added for
  referral invite links). Defence in depth: with ticket auth a cross-origin page cannot obtain a ticket anyway
  (the ticket endpoint is cookie+CORS protected), but the origin check costs one line and closes the door if cookie
  auth is ever added.
- On successful redemption the server sends `hello` with `server_time_ms` so the client can compute its clock offset
  once (used only for rendering countdowns; **never** for scoring — §7.1).

`auth.Required` cannot wrap the WS route: it writes a JSON 401 body, which is correct for HTTP but meaningless for a
client expecting an upgrade. The WS handler returns `401` + `{"error":{"code":"bad_ticket"}}` before upgrading, or
closes with code `4002` if the ticket dies between check and upgrade.

### 2.3 Connection registry (`Hub`)

```go
type Hub struct {
    mu    sync.RWMutex
    conns map[uuid.UUID]*Conn   // one live socket per profile
}
```

- **One socket per profile, enforced.** A second connect closes the first with code `4001`
  (`replaced_by_new_connection`). This is not just hygiene: two tabs would let one profile queue twice, and — worse —
  answer the same duel from two windows. Kicking the old socket also makes reconnect (§4.5) trivial: reconnect *is*
  "connect again", no special path.
- Each `*Conn` owns two goroutines: a **read pump** (`ReadLimit(4096)`; a duel message is <200 bytes, so anything
  larger is either a bug or an attack) and a **write pump** draining a buffered `chan []byte` (cap 16). If the write
  buffer fills, the connection is closed rather than blocking — **the match goroutine must never block on a slow
  client**, or one player's bad Wi-Fi stalls the other player's timer.
- Liveness: `Ping(ctx)` every 20s, drop after 2 missed pongs (~45s). 20s is chosen to sit under Cloudflare's ~100s
  idle-connection timeout and under typical Nginx `proxy_read_timeout` (60s) so an idle-but-alive queue wait is never
  reaped by infrastructure.
- `arena:conn:<profileID>` → `instanceID`, TTL 30s, refreshed by the same heartbeat. Unused when single-instance;
  it is the routing table the multi-instance transport (§8.2) needs, and writing it from day one costs one `SET`.

### 2.4 Wire protocol

JSON envelope, versioned: `{"v":1,"t":"<type>","d":{...}}`. `v` exists so M4-05 and a future mobile client
(M6 PWA) can be upgraded independently; the server rejects unknown `v` with `error{code:"bad_protocol"}` rather
than guessing.

**Client → server**

| `t` | `d` | Notes |
|---|---|---|
| `queue.join` | `{}` | Eligibility (daily cap / VIP) is checked here, not at connect: a connected client may sit idle. |
| `queue.leave` | `{}` | Idempotent; `not_queued` is an `error`, not a failure. |
| `answer` | `{match_id, index, answer_id}` | `index` is required so a late answer for question 3 arriving during question 4 is rejected as `wrong_question` instead of silently scoring the wrong question. |
| `match.rejoin` | `{match_id}` | Sent after a reconnect; server replies `match.state`. |

**Server → client**

| `t` | `d` | Notes |
|---|---|---|
| `hello` | `{profile_id, server_time_ms, protocol}` | |
| `queue.joined` | `{queued_at_ms, timeout_ms}` | |
| `queue.timeout` | `{waited_ms}` | Client decides whether to re-join. |
| `match.found` | `{match_id, opponent:{name}, question_count, question_time_ms, starts_in_ms}` | `opponent.name` uses `leaderboard.DisplayName` — the existing helper that falls back to `Foydalanuvchi #<4 hex>` and **never** exposes a phone number. Reuse it; do not write a second one. |
| `question` | `{index, total, deadline_ms, server_time_ms, question:{...}}` | Full question content embedded — see below. |
| `answer.ack` | `{index, response_ms}` | **Carries no correctness.** |
| `question.result` | `{index, correct_answer_id, you:{...}, opponent:{...}, score:{you, opponent}}` | The only frame that reveals the answer key. |
| `opponent.status` | `{state:"disconnected"\|"connected"}` | |
| `match.end` | `{match_id, outcome:"won"\|"lost"\|"draw", reason, score, correct}` | |
| `error` | `{code, message}` | `code` is snake_case and localized by the client, exactly like `httpx.Error` codes elsewhere (`promo-input.tsx`/referral errors already do this). |

**Question content is embedded in the `question` frame, not fetched over HTTP.** Two reasons, both concrete:

1. A separate `GET /arena/matches/{id}/questions/{qid}` fetch adds a round trip *inside* a 15-second timer, and on a
   slow 3G connection that is a real fraction of the answer window — an unfair one, since it varies per player.
2. If the client can request question *N* by ID, it can request *N+1* early. Embedding means the server controls
   exactly when each question becomes knowable.

Content is built with the existing `content.Handler.LoadQuestionDetail(ctx, questionID, locale)` (already exported and
already used by `session.Handler.getSessionQuestion`). Two mandatory adjustments:

- `detail.Explanation` must be set to `nil` — `LoadQuestionDetail` attaches a verified explanation when one exists,
  and an explanation frequently states the correct answer. `session/handlers.go` already does exactly this when
  `!access.FeedbackAllowed`; arena does the same, unconditionally, until the reveal.
- Answer correctness is structurally safe: `content.AnswerDTO` has only `{id, position, text, image_url}` — no
  `is_correct` field exists to leak (`internal/content/dto.go`). Verified, not assumed.

### 2.5 Close codes

| Code | Meaning |
|---|---|
| `1000` | normal |
| `1008` | policy violation (message flood, oversize frame) |
| `1011` | internal error |
| `4001` | `replaced_by_new_connection` |
| `4002` | `ticket_invalid` |
| `4003` | `server_shutdown` |

---

## 3. Matchmaking

### 3.1 Why a Redis sorted set, not a list

`LPUSH`/`BRPOP` is the obvious queue, but Arena needs three things a list cannot do cheaply: **remove a specific
member** (cancel, disconnect-while-queued), **read wait time** (to widen the search and to time out), and **scan
several buckets at once**. A `ZSET` scored by enqueue time gives all three:

```
arena:q:<bucket>        ZSET   member = profileID, score = enqueuedAtUnixMs
arena:queued:<profileID> STRING → bucket, TTL 120s   (reverse index: cancel and double-queue detection)
arena:match:<profileID>  STRING → matchID, TTL matchDuration + grace   (reconnect routing, "already in a match")
```

All arena keys use the `arena:` prefix, mirroring the `lb:` convention M4-01 established, so a `KEYS`/`SCAN` during
an incident can tell the two subsystems apart in the shared Redis DB 0.

### 3.2 Pairing must be atomic — one Lua script

The failure mode to design against: two instances (or two goroutines) each pop the *same* waiting player and each
create a match, so that player is in two duels and one opponent is talking to nobody. Checking then removing in two
round trips does not prevent it.

**`arena_join.lua`** — a single script that does the whole join decision atomically (Redis is single-threaded, so the
script is a critical section):

```
KEYS: candidate bucket keys, own bucket first, then widened buckets (§3.3)
ARGV: selfProfileID, nowMs, ownBucketKey
1. for each KEYS[i]: read the oldest member (ZRANGE key 0 0 WITHSCORES), skipping selfProfileID
2. pick the globally oldest candidate across all keys
3. if found:  ZREM it from its key; DEL arena:queued:<candidate>; return {"paired", candidate}
4. if none:   ZADD ownBucketKey nowMs selfProfileID; SET arena:queued:<self> bucket EX 120; return {"queued"}
```

Because "look for an opponent" and "enqueue myself" happen in one atomic step, the symmetric race (A and B both look,
both find an empty queue, both enqueue, both wait) cannot happen. Whoever's script runs second sees the other and
pairs. This is the single most important correctness property in §3, and it is the one to test with an `-race`
N-goroutine test in the style of the existing `internal/billing/checkout_race_test.go`: N profiles joining
simultaneously must produce exactly `floor(N/2)` matches with no profile appearing twice.

**Stale-candidate handling.** A popped opponent may have a dead socket (disconnected without cleanup, or their
instance crashed). After a successful pop the matchmaker verifies liveness — locally via `Hub`, remotely via
`arena:conn:<opponent>` — and on failure discards the candidate and retries, bounded to 3 attempts before falling
back to enqueueing self. Without this, one crashed client poisons the front of the queue for everyone.

**Who drives pairing:** the joining connection calls `arena_join.lua` immediately (lowest latency for the common
"someone is already waiting" case), plus a per-instance 500ms ticker that re-runs the widened search for players
already queued and expires timed-out entries (`ZREMRANGEBYSCORE arena:q:* -inf <now-45000>`). The ticker is also the
only place widening happens, so widening logic lives in one function.

### 3.3 Rating buckets and widening

Both are **pure functions** in `arena/rules.go` (following the `session/rules.go` and `leaderboard/rules.go`
convention of putting all decision logic in ctx-free, DB-free functions that can be tested against reference values):

```go
// Bucket maps a rating to a matchmaking bucket. 100-point buckets.
func Bucket(rating int) int

// SearchBuckets returns the buckets to search, given the player's own bucket
// and how long they have waited. Widens over time; returns nil for "search all".
func SearchBuckets(bucket int, waited time.Duration) []int
```

Widening schedule (tunable constants, not magic numbers inline):

| Waited | Search |
|---|---|
| 0–5s | own bucket only |
| 5–15s | ±1 bucket |
| 15–30s | ±3 buckets |
| 30–45s | all buckets |
| >45s | `queue.timeout` |

45s total is chosen because it is roughly the limit of a user's patience on a "Finding opponent…" screen before it
reads as broken, and because at launch the concurrent-player pool will be tiny — the honest expectation for the first
months is that almost every match is found in the "all buckets" phase, which is precisely why §12 Q10 (bot opponent)
matters more than the bucket tuning does.

### 3.4 The rating seam (rating lands in M4-04)

M4-03 must not guess at ELO, and must not be rewritten when ELO arrives:

```go
// RatingProvider supplies the matchmaking rating for a profile. M4-03 ships
// FixedRating (everyone in one bucket, so pairing is pure FIFO); M4-04 drops
// in the real ELO-backed implementation with no matchmaking changes.
type RatingProvider interface {
    RatingFor(ctx context.Context, profileID uuid.UUID) (int, error)
}

type FixedRating struct{ Value int } // Value: 1000
```

`arena_match_player` already carries nullable `rating_before/rating_after/rating_delta` (§5.2), so M4-04 needs no
migration — only a new `RatingProvider` implementation and a post-match update. Deliberately **not** coupled to
M4-01's leaderboard score: duel rating and "correct answers this week" measure different things, and the leaderboard's
Redis sorted set is documented as a rebuildable cache, not a rating store.

### 3.5 Eligibility and daily cap

Checked at `queue.join`, not at connect (a socket may idle):

1. Not already in a live match (`arena:match:<profileID>` absent) → else `already_in_match`.
2. Not already queued → else `already_queued`.
3. Daily match budget from `limit_config` key `arena_daily_matches` (recommended `free_value=3, vip_value=-1`;
   `-1` is this schema's existing "unlimited" convention, see migration `0003`). Counted from
   `arena_match_player` rows joined today (UTC day boundary, matching `progress.todayUTC()` and
   `leaderboard.PeriodStart`). Exceeded → `daily_limit_reached`, and the client routes to `/premium` the same way
   `402 vip_required` already does. **This is Q1 in §12** — if Arena is fully free, this check disappears; if it is
   VIP-only, it becomes `billing.Service.Status` exactly as `session.StartSession`'s `case "exam"` does.
4. Rate limit on join churn via `auth.Limiter` (`arena:rl:join:<profileID>`, e.g. 20 per 5 minutes).

### 3.6 Locale is not a matchmaking constraint

Worth stating because it looks like one: questions are stored once and translated per locale
(`question_translation`, served via `LoadQuestionDetail(ctx, id, locale)`). Two players can therefore share one
question ID while each reads it in their own language — a `ru` player and a `uz-Cyrl` player are a perfectly fair
pair. Splitting the queue by locale would fragment an already-thin pool for no benefit. Each player's locale is
recorded on their own `arena_match_player` row (for support/debugging and for M4-05's replay screen).

---

## 4. Match state machine

### 4.1 Shape: one goroutine per match (actor)

```
pending ──countdown(3s)──> question_active[i] ──both answered | deadline──> question_reveal[i] (2.5s)
                                    ↑                                              │
                                    └──────────── i+1 ≤ N ─────────────────────────┘
                                                                                   │ i == N
                                                                            finished / aborted
```

All match state is owned by a single goroutine and mutated only there. Every input — an answer, a disconnect
notice, a reconnect, a timer firing, a shutdown signal — arrives on that goroutine's channel. Consequences:

- **No mutex on match state**, and therefore no lock-ordering hazard between the hub, the matchmaker, and the match.
  (Compare the money paths, where `pool.Begin` + `SELECT ... FOR UPDATE` is mandatory because the state lives in
  Postgres and is touched by concurrent HTTP requests. Match state lives in memory and has exactly one writer, which
  is a strictly stronger guarantee — but only if nothing else is ever allowed to touch it. That invariant must be
  stated in the package doc comment, because it is the kind of thing a later contributor silently breaks by adding
  "just one" accessor method.)
- The clock is injected: `Now func() time.Time` (the exact convention `session.Service` already uses) plus a small
  timer interface, so a full 7-question match can be tested in microseconds with zero `time.Sleep`. Any test that
  needs real sleeps to pass will be flaky in CI forever; this is the design decision that prevents that.
- Timers are per-match `time.Timer`s, not a global tick loop. At hundreds of concurrent matches this is cheaper and
  much simpler to reason about; a global 100ms tick loop would also make every match's timing quantized to the tick.

### 4.2 Constants (Go, not `limit_config`)

```go
ArenaQuestionCount     = 7
ArenaQuestionTimeSec   = 15
ArenaCountdownMs       = 3000
ArenaRevealMs          = 2500
ArenaLateGraceMs       = 400
ArenaQueueTimeoutSec   = 45
ArenaReconnectGraceSec = 20
ArenaBaseCorrectPoints = 100
ArenaMaxSpeedBonus     = 50
```

These live in `arena/rules.go` as constants, **not** in `limit_config`. `limit_config` is a two-column
free/VIP *limits* table (`free_value`, `vip_value`) — a question count has no free/VIP dimension, and forcing it in
would repeat the mistake migration `0018`'s comment describes (a seeded row that nothing reads, or a constant that
silently wins over the row). Only `arena_daily_matches` is genuinely free/VIP-shaped, so only that goes in
`limit_config`. Match length is Q2 in §12.

7×15s ≈ 2 minutes end to end including countdown and reveals. Rationale: the platform already has a 20-question /
25-minute exam and a Grand Mock; Arena's job is retention through a short, repeatable loop, not a second exam.

### 4.3 Question selection

`Q.RandomQuestionIDs(ctx, n)` — the exact query `case "exam"` and `case "grand_mock"` already use, so Arena inherits
the "valid questions only" filter for free. At match creation the server also loads the correct answer ID for all N
questions (one batched query, `ListCorrectAnswerIDsForQuestions` — new, trivial; `GetCorrectAnswerID` exists but is
per-question) and keeps them in the match goroutine's memory. **This is what keeps Postgres out of the hot path:**
scoring an answer is a map lookup and a pure function, so the only DB work during a duel is at creation and at finish.

Fairness caveat, accepted: one player may have seen a drawn question before. With a 1231-question bank and both
players drawing from the same pool this is symmetric in expectation, and "exclude questions either player has seen"
would need a two-profile anti-join against `question_memory` at match-creation latency. Revisit only if it shows up
in real complaints.

### 4.4 Answer acceptance and scoring

Accepted only if: the match is `question_active`, `d.index` equals the active index, the profile has not already
answered this index, the answer belongs to the question, and it arrived before `deadline + ArenaLateGraceMs`.
Otherwise: `wrong_question` / `already_answered` / `too_late` / `invalid_answer`.

The 400ms grace exists because the deadline is a *server* instant and a player on a 300ms mobile RTT would otherwise
be systematically robbed of their last moment. The grace affects *acceptance* only — the score uses
`min(responseMs, durationMs)`, so grace cannot produce a bonus.

```go
// AnswerPoints: 100 for correct, plus up to 50 scaled by remaining time.
// A wrong or missing answer scores 0 — speed is only ever a bonus on top of
// being right, never a consolation for being fast and wrong.
func AnswerPoints(correct bool, responseMs, durationMs int) int
```

**Both players answered → reveal immediately**, don't wait out the timer. This is the single biggest "feel" decision
in the duel: it turns dead time into pace, and shortens the average match well below the 2-minute worst case.

Match outcome (pure function, tested against reference values): higher total points wins; tie broken by more correct
answers; then by lower total response time; then it is a genuine `draw`. Whether a draw is acceptable or should
trigger a sudden-death question is Q5 in §12 — the schema supports both (`outcome` CHECK includes `'draw'`,
`arena_match.question_ids` is an array so an 8th question can be appended).

### 4.5 Disconnect, reconnect, forfeit

- **On disconnect** the match does not pause. The server clock keeps running; unanswered questions score 0. The
  opponent gets `opponent.status{state:"disconnected"}` — they must not be left staring at a frozen screen, which is
  indistinguishable from the *server* being broken.
- **Reconnect** = mint a new ticket, connect, send `match.rejoin{match_id}`. The server replies `match.state` with a
  full snapshot: current index, `deadline_ms`, both scores, and the already-revealed per-question results (never the
  active question's answer key). Reconnect is validated against `arena:match:<profileID>`, so a client cannot rejoin
  a match it is not a player in.
- **Grace `ArenaReconnectGraceSec = 20`.** If the player is not back within 20s *and* questions remain, the match ends
  as a forfeit: `arena_match.end_reason = 'forfeit'`, opponent `outcome='won'`, quitter `outcome='lost'`. Forfeits
  must count as losses in M4-04's rating, otherwise rage-quitting becomes the optimal strategy when losing — that is
  a hand-off requirement to M4-04, recorded here so it is not rediscovered later.
- If the disconnect happens after the last answer, the match simply finishes normally.
- **Both disconnected** → `end_reason='both_disconnected'`, both `outcome='draw'`, no rating change (M4-04).
- `finish()` is guarded by a state check and is idempotent — the same discipline as
  `session.finishInternal`'s `if row.Status != "in_progress"` early return, which exists precisely because two paths
  (3rd-mistake stop and explicit finish) can both reach it. Arena has *four* such paths (last question, forfeit,
  both-disconnected, shutdown), so the guard matters more, not less.

### 4.6 Persistence

One transaction at match end: `UPDATE arena_match` (status/finished_at/end_reason), `UPDATE arena_match_player` ×2
(score, correct_count, total_response_ms, outcome), `INSERT arena_answer` × (N×2) — including rows for questions that
timed out (`answer_id IS NULL`). Batched, off the timing path, and if it fails the players have already seen their
`match.end` frame (the socket result is derived from in-memory state, not from a read-back).

Deliberately **not** written incrementally per question: a per-answer write would put a DB round trip inside the reveal
window and would leave half-finished matches in the table on a crash. A lost match row on a crash is acceptable
(it is a game, not money); a duel that stutters because Postgres was slow is not.

---

## 5. Data model

### 5.1 Decision: new tables, `exam_session` is not reused

This reverses the Grand Mock precedent (M2-07 built a 5th *mode* on `exam_session` rather than a new subsystem), so it
needs real justification. Three concrete facts, in order of weight:

1. **`session_answer.answer_id` is `NOT NULL`** (migration `0004`). A duel's most common event is "the 15 seconds
   expired and the player answered nothing", and that row cannot be written. Absence-of-row could encode it, but then
   `arena_answer.response_ms` (needed for the speed bonus, for the tie-break, and for bot detection) has nowhere to
   live either — `session_answer` has only `answered_at`, and adding an arena-only nullable column to a table that
   five modes and four subsystems read is worse than a new table.
2. **Three CHECK constraints would each need widening, on a table five modes depend on:**
   - `exam_session.mode CHECK (mode IN ('variant','exam','practice','mistakes','grand_mock'))` → add `'arena'`.
   - `exam_session.status CHECK (status IN ('in_progress','passed','failed','abandoned'))` → a duel outcome is
     won/lost/draw. Mapping "won" onto "passed" would corrupt `GET /me/sessions` history and the stats screens, which
     read `status` as "did you pass the exam".
   - `exam_session.stopped_reason CHECK (... IN ('completed','time_up','too_many_errors'))` → needs
     `'forfeit'`/`'opponent_left'`.
     Widening three CHECKs on the busiest table in the schema, plus auditing every
     `row.Mode == ...` / `IsExamLike(row.Mode)` branch in `internal/session` (`finishInternal`'s switch, redaction,
     time-up handling) is more risk than one new table with no existing readers.
3. **Silent coupling into the leaderboard.** `CountCorrectAnswersByProfileByDayInRange` (used by
   `leaderboard.RebuildPeriod`) counts **every** `session_answer` row with no mode filter. Arena answers written there
   would enter the leaderboard through the rebuild path even if the live `RecordPoint` call site were filtered — the
   exact live-vs-rebuild divergence that M4-01's Task 8 was created to fix. Separate tables make the correct behaviour
   the default instead of something two independent code paths must both remember. §6.1.

**What is lost, and how it is recovered:** arena answers would no longer feed FSRS, the mistake bank, or mastery.
But `learning.Service.RecordReview(ctx, profileID, questionID, rating)` and
`progress.Service.RecordActivity(ctx, profileID)` are **session-independent** — neither takes a session ID, and
neither reads `exam_session`. Arena can therefore call them directly, per answer, with no `exam_session` row at all.
So the integration is a deliberate, switchable choice rather than a side effect of table reuse. Whether to switch it
on is Q3 in §12, and §6.2 explains why it is not obvious.

### 5.2 Migration `0020_battle_arena.up.sql` (shape, not final text)

```sql
CREATE TABLE arena_match (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  status            text NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','in_progress','finished','aborted')),
  question_ids      uuid[] NOT NULL CHECK (cardinality(question_ids) > 0),
  question_time_sec smallint NOT NULL CHECK (question_time_sec > 0),
  created_at        timestamptz NOT NULL DEFAULT now(),
  started_at        timestamptz,
  finished_at       timestamptz,
  end_reason        text CHECK (end_reason IS NULL OR end_reason IN
                    ('completed','forfeit','both_disconnected','server_shutdown'))
);

CREATE TABLE arena_match_player (
  match_id          uuid NOT NULL REFERENCES arena_match(id) ON DELETE CASCADE,
  profile_id        uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  slot              smallint NOT NULL CHECK (slot IN (1,2)),
  locale            locale_code NOT NULL,
  score             int      NOT NULL DEFAULT 0,
  correct_count     smallint NOT NULL DEFAULT 0,
  total_response_ms int      NOT NULL DEFAULT 0,
  outcome           text CHECK (outcome IS NULL OR outcome IN ('won','lost','draw')),
  -- Nullable, unused in M4-03, filled by M4-04's ELO. Present now so M4-04
  -- needs no migration and match history is never retroactively incomplete.
  rating_before     int,
  rating_after      int,
  rating_delta      int,
  disconnected_at   timestamptz,
  joined_at         timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (match_id, profile_id),
  UNIQUE (match_id, slot)
);
-- Mirrors exam_session_profile_idx / payment_profile_idx: history is always
-- "this profile's, most recent first".
CREATE INDEX arena_match_player_profile_idx ON arena_match_player(profile_id, joined_at DESC);

CREATE TABLE arena_answer (
  match_id    uuid NOT NULL REFERENCES arena_match(id) ON DELETE CASCADE,
  profile_id  uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  question_id uuid NOT NULL REFERENCES question(id),
  position    smallint NOT NULL,
  -- NULL = never answered (timed out). This is the column session_answer
  -- cannot provide, and the reason arena_answer exists (see spec 5.1).
  answer_id   uuid REFERENCES answer(id),
  is_correct  boolean NOT NULL DEFAULT false,
  response_ms int,
  points      smallint NOT NULL DEFAULT 0,
  answered_at timestamptz,
  PRIMARY KEY (match_id, profile_id, question_id),
  CHECK ((answer_id IS NULL) = (response_ms IS NULL)),
  CHECK (answer_id IS NOT NULL OR NOT is_correct)
);

INSERT INTO limit_config (key, free_value, vip_value) VALUES
  ('arena_daily_matches', 3, -1);   -- -1 = unlimited, existing convention
```

Down migration drops the three tables and deletes the `limit_config` row — and note the operational rule from the
handoff: editing a migration **in place** requires `make test-db-reset`, because per-package test databases are
reused.

### 5.3 Test-infrastructure registration (easy to miss, fails loudly)

`internal/redisx/testhelper.go` maps each test package to its own Redis logical DB via an **explicit** table. A new
package with no entry fails immediately with a clear message (deliberately — see the comment there). So
`"internal_arena": 5` must be added in the same task that creates the first arena Redis test. Postgres needs nothing:
`testdb.New(t)` derives `avtotest_test_internal_arena` automatically.

---

## 6. Interaction with existing subsystems

### 6.1 Leaderboard (M4-01): no coupling — and it is free

Because arena answers never touch `session_answer`, they are invisible to both `RecordPoint` (live) and
`RebuildPeriod` (rebuild). Nothing must be remembered, so nothing can be forgotten. Recommended: keep it that way
(Q4). The positive case for coupling — "duels should raise your weekly rank" — is better served by M4-04's rating,
which is the metric that actually means "good at duels".

### 6.2 FSRS / mistake bank / mastery — and the Grand Mock gate

`learning.RecordReview` writes `question_memory` **and** `category_mastery`. That reaches further than it looks:

- `question_memory` row count is `CountStudiedQuestions`, which is the **Grand Mock volume floor** (`≥25%` of the
  bank ≈ 308 questions, migration `0018`).
- `category_mastery` feeds `learning.Stats().ReadinessPct`, which is the **Grand Mock mastery gate** (`≥85%`).
- `question_memory.lapses > 0 AND due_at <= now()` is the **mistake bank** (`ListMistakeBankQuestionIDs`).

So if arena answers call `RecordReview`, a 2-minute duel becomes a way to advance the Grand Mock volume floor
(~44 duels ≈ 308 distinct questions), while free-tier practice is capped at `daily_practice_questions`. Grand Mock
is VIP-gated so the practical exposure is limited, but migration `0018` exists *specifically* because that gate was
gameable once already — introducing a second route into it without deciding on purpose would be repeating that
mistake. There is also a quality argument: FSRS treats a 15-second rushed answer identically to a considered one, so
duel answers arguably pollute scheduling.

Options (Q3): (a) no learning-engine integration at all — arena is pure competition; (b) mistake bank only — feed
`learning.Again` on wrong answers so duels surface weak spots, but never credit `Good`, so nothing can be farmed
upward; (c) full `RecordReview` both ways. **Recommendation: (b)** — it is the only option that is strictly
beneficial to the learner and cannot inflate any gate, and it needs no new query (`RecordReview` with
`learning.Again` is one call).

### 6.3 Streak (`progress.RecordActivity`)

Independent of `exam_session`, so arena can call it. Should a duel keep a streak alive? Probably yes (it is genuine
activity and streak protection is a retention feature), but the daily goal is measured in answered questions and a
duel is only 7 — folded into Q3.

### 6.4 Practice daily limit

`CountPracticeAnswersToday` filters `es.mode = 'practice'`, so arena cannot consume or be consumed by the practice
budget regardless of any other decision. Arena's own budget is `arena_daily_matches` (§3.5).

### 6.5 Logging: arena is the first service that needs a logger

M4-01 deliberately discarded `RecordPoint` errors, with the reasoning that "this codebase has no logging framework"
and a leaderboard point is low stakes. Both halves stop applying here: `zap` **is** available and already threaded
into `server.Deps.Log` (used by `auth.SenderFor`), and a match goroutine has **no HTTP request to return an error
to** — a silently dropped error in a background match loop is an invisible bug by construction. So `arena.Service`
takes a `*zap.Logger`, and it should be the only place in `internal/*` where that is true until someone else has an
equally concrete reason.

---

## 7. Anti-cheat and fairness

1. **The server is the only clock.** `deadline_ms` and `server_time_ms` are sent for rendering; scoring uses the
   instant the answer frame is read on the server. A client cannot claim a faster response time because it never
   reports one.
2. **The answer key is never sent early.** `answer.ack` carries no correctness; only `question.result` reveals
   `correct_answer_id`, and `LoadQuestionDetail`'s explanation is stripped (§2.4). This is the same discipline
   `IsExamLike` redaction enforces for exams, applied to a new transport.
3. **Prefetch is impossible** because question content is pushed, not fetched (§2.4).
4. **One socket per profile** (§2.3) blocks two-tab self-play. The harder problem is **win-trading across two
   accounts** on one device: both queue at the same moment, one deliberately loses, rating inflates. Mitigations, in
   order of cost: (a) refuse to pair two profiles whose enqueue timestamps are within a small window *and* which
   share a device fingerprint or asserted client IP — both already exist in this codebase (`device` table, migration
   `0002`; `auth.NewClientIPResolver`); (b) cap daily rated matches (§3.5); (c) diminishing rating returns for repeat
   opponents — M4-04, since it needs rating. Recommendation: (a) + (b) in M4-03, (c) handed to M4-04.
   **Caveat that must not be skipped:** IP-based rejection has a high false-positive rate in Uzbekistan — mobile CGNAT,
   internet cafés, and family/shared connections are normal. So (a) must require IP **and** device-fingerprint
   agreement **and** near-simultaneous enqueue, and even then it should re-queue rather than reject.
5. **Bot/superhuman detection: record, do not punish.** `arena_answer.response_ms` is persisted precisely so that a
   pattern of sub-250ms correct answers is analysable later. Auto-banning on a heuristic in M4-03 would produce
   false positives with no appeal path and no admin panel to review them (M3 is last). Record now, judge in M3.
6. **Flood control:** `ReadLimit(4096)` plus a per-connection token bucket (e.g. 20 messages / 10s) → close `1008`.
   Plus the two `auth.Limiter` budgets (ticket minting, queue joins).
7. **Symmetry, and its honest limit.** Both `question` frames are written back-to-back from the same goroutine with a
   single computed `deadline_ms`, so the *rules* are identical. Network latency is not: a player 80ms further away
   genuinely sees the question later. The alternative — start each player's timer when *they* acknowledge the question
   — is fairer on paper and trivially cheatable (delay the ack, think for free), so it is rejected. The speed bonus is
   kept deliberately coarse (50 points across 15,000ms ≈ 3.3 points per 100ms) so ordinary latency jitter moves the
   score by a rounding error rather than by a win.

---

## 8. Hosting implications

### 8.1 What breaks first

Arena is the project's first stateful, long-lived-connection subsystem. Everything else is request/response and
scales by adding processes. A match lives in **one process's memory**, so with two API instances behind a naive load
balancer, two paired players can land on different instances and neither can drive the match.

### 8.2 Sticky sessions do not fix this

Worth stating plainly because it is the reflexive answer: a WebSocket is a single long-lived TCP connection, so it is
already pinned to one instance for its whole life — stickiness adds nothing. The actual requirement is **match
affinity**: both players' sockets must be reachable from wherever the match goroutine runs. Two ways:

- **Redis pub/sub routing (recommended seam).** `arena:conn:<profileID>` → `instanceID` is the routing table (already
  written by the heartbeat, §2.3); each instance subscribes to `arena:inst:<instanceID>` once and forwards inbound
  frames to the owning instance and outbound frames to the socket's instance. Behind an interface so today's code is
  unaware:
  ```go
  type Transport interface { Send(ctx context.Context, profileID uuid.UUID, frame []byte) error }
  // LocalTransport (M4-03) → Hub lookup. RedisTransport (later) → PUBLISH.
  ```
- **Pair only players on the same instance.** Simple, and fragments an already-thin matchmaking pool by the number of
  instances. Rejected.

### 8.3 Recommendation for T1–T4: single instance, with the seam in place

The repository has **no deployment manifest at all** — no backend `Dockerfile`, no Kubernetes, no fly.toml;
`docker-compose.yml` runs only postgres/redis/minio and `run.sh` starts one Go process. Multi-instance is therefore
hypothetical today, and building `RedisTransport` now would be untestable speculation. So: `LocalTransport`,
plus `ARENA_INSTANCE_ID` config (default hostname) and a startup log line stating that arena is single-instance.
Q11 in §12 asks whether production launches with >1 instance; if yes, `RedisTransport` moves from "later" into T4.

### 8.4 Operational requirements to hand to whoever deploys this

- **Long-lived HTTP/1.1 upgrade support.** Rules out request/response-only serverless platforms for the API process.
- **Nginx:** `proxy_http_version 1.1;` + `proxy_set_header Upgrade $http_upgrade; Connection "upgrade";` and
  `proxy_read_timeout` above the 20s heartbeat (60s+). Without the first two, the upgrade returns 400 and Arena is
  simply dead in production while working perfectly in dev.
- **Cloudflare** proxying WS is fine; its ~100s idle timeout is covered by the 20s ping.
- **Redis** must be the same instance the leaderboard uses (it already is — one `REDIS_URL`, DB 0); arena keys are
  `arena:`-prefixed to stay distinguishable from `lb:`.
- **Graceful shutdown is Arena's own job** (§2.1 point 2): on SIGTERM, stop accepting joins, drain the queue, send
  `match.end{reason:"server_shutdown"}` to live matches, close sockets with `4003`, *then* `srv.Shutdown`. How a
  shutdown-interrupted match is scored is Q9.
- **Capacity, order-of-magnitude:** each connection ≈ 2 goroutines + ~8–16KB buffers; each match ≈ 1 goroutine +
  ~2KB state. 1,000 concurrent duels ≈ 3,000 goroutines and a few tens of MB — comfortably one modest instance. The
  binding constraint will be Postgres writes at match end (N×2 answer rows per match), not sockets.

---

## 9. Package and route layout

```
backend/internal/arena/
  rules.go       pure: Bucket, SearchBuckets, AnswerPoints, Outcome, constants  (no ctx, no DB — mirrors session/rules.go, leaderboard/rules.go)
  protocol.go    envelope types + message payload structs + close/error codes
  hub.go         connection registry, one socket per profile
  conn.go        read/write pumps, heartbeat, flood guard
  matchmaker.go  Redis ZSET queue + arena_join.lua + widening ticker
  match.go       the actor: state machine, timers, answer acceptance, finish
  service.go     persistence, eligibility, ticket mint/redeem, RatingProvider
  handlers.go    POST /arena/ws-ticket (authed), GET /arena/ws (ticket-authed)
```

Wiring in `internal/server/server.go`, inside the existing `if deps.Pool != nil && deps.Redis != nil {` block (where
`leaderboard`, `session`, and `auth` are already wired): the ticket route goes into the
`api.With(auth.Required(...))` group, the `/arena/ws` route goes on bare `api`.

---

## 10. Testing strategy

Following the project standard (`make test` / `make test-parallel` both green, `-race` for anything concurrent):

**Pure, no infrastructure** (`rules_test.go`) — the highest-value tests here, because all the decisions are pure:
`Bucket` boundaries; `SearchBuckets` at each widening step and at the timeout edge; `AnswerPoints` at
0ms/half/full/overtime and for wrong answers; `Outcome` for win/loss/each tie-break level/genuine draw.

**Redis** (`matchmaker_test.go`, `redisx.NewTest(t)` — needs the `internal_arena: 5` entry from §5.3):
join→pair happy path; join with empty queue enqueues; cancel removes; **N-goroutine `-race` atomicity test**: 20
profiles join simultaneously → exactly 10 matches, no profile in two matches (the `checkout_race_test.go` pattern);
stale-candidate skip; timeout eviction.

**State machine** (`match_test.go`, injected clock, no sleeps): full 7-question match; both-answer-early reveals
early; late answer past grace rejected; answer for the wrong index rejected; double answer rejected; a fully timed-out
question scores 0 for both; disconnect → opponent notified → reconnect replays correct snapshot; disconnect past grace
→ forfeit with correct outcomes; `finish()` called twice writes once.

**Transport** (`handlers_test.go`, `httptest.NewServer` + a real client): ticket single-use (second redemption
fails); expired ticket fails; missing ticket → 401; second connect closes the first with `4001`; oversize frame
closes `1008`; unknown `v` → `error{bad_protocol}`.

**End-to-end smoke** (T4, not CI): 200 concurrent sockets / 100 matches from a small Go driver, watching for goroutine
leaks (`runtime.NumGoroutine()` before/after) — the failure mode a unit test cannot see.

---

## 11. Phased delivery (T1–T4, aligned with roadmap §3)

Roadmap M4-03 is already split T1–T4; this refines each into a task with an exit criterion. Each phase ends green
(`make check`) and is independently reviewable.

**T1 — Transport foundation.** `internal/arena` skeleton, `protocol.go`, ws-ticket mint/redeem, `Hub`, `Conn` pumps,
heartbeat, origin check, flood guard, config flags, `redisx` test slot, server wiring.
*Exit:* two browsers can connect, `hello` arrives, a second connect kicks the first, an idle socket survives 5 minutes,
and the ticket cannot be replayed. **No matchmaking, no matches.**
*Risk retired:* the three integration unknowns in §2.1 (middleware/hijack, no write timeout, shutdown behaviour).

**T2 — Matchmaking.** ZSET queue, `arena_join.lua`, `Bucket`/`SearchBuckets`, `RatingProvider` + `FixedRating`,
widening ticker, join/leave/timeout, eligibility + daily cap, join rate limit.
*Exit:* two clients queue and both receive `match.found` with the same `match_id`; the 20-goroutine atomicity test
passes under `-race`; cancel and disconnect-while-queued both dequeue.

**T3 — Match state machine + persistence.** Migration `0020`, sqlc queries, the match actor with injected clock,
question preload, answer acceptance, reveal, scoring, outcome, disconnect/reconnect/forfeit, snapshot replay,
transactional finish.
*Exit:* a complete duel is playable end to end and its result is queryable in Postgres; every `match_test.go` case
above passes; a forfeit produces the right rows.

**T4 — Hardening and seams.** Graceful arena drain on SIGTERM, `Transport` interface + `LocalTransport` (and
`RedisTransport` only if Q11 says multi-instance at launch), win-trading guard (§7.4a), suspicion recording,
concurrency smoke test, whole-branch review.
*Exit:* SIGTERM during a live match ends it cleanly with no goroutine leak; the branch passes an independent deep
review — which the handoff (§6) requires before anything is marked done, and which found two real bugs in M4-01 that
task-level reviews missed.

**Handoff to M4-04:** `RatingProvider` implementation, `rating_*` column population, `GET /me/arena/matches`, medal
tiers, and the two requirements recorded above — forfeits count as losses (§4.5), repeat-opponent diminishing returns
(§7.4c).

---

## 12. Ochiq savollar — Sherzod qarorini kutadi

Kod yozishdan oldin **Q1, Q2, Q3, Q6** ga javob majburiy (sxema/scoring ularga bog'liq). Qolganlari T3/T4 davomida
javob olsa ham bo'ladi, lekin Q11 deploy rejasiga ta'sir qiladi.

| # | Savol | Tavsiya | Nimaga ta'sir qiladi |
|---|---|---|---|
| **Q1** | Arena kimga ochiq: faqat VIP, bepul lekin kunlik limit bilan, yoki to'liq bepul? | **Bepul, kuniga 3 ta duel; VIP — cheksiz** (`arena_daily_matches` 3/-1). Bepul foydalanuvchi mahsulotni ta'msiz sinab ko'radi, limit esa upsell yaratadi — `/premium`ga yo'l allaqachon `402 vip_required` orqali ishlaydi. | §3.5, migratsiya `0020`, T2 |
| **Q2** | Duel uzunligi va tempi: 7×15s (~2 daq), 10×20s (~3.5 daq), yoki 5×10s (~1 daq)? | **7×15s.** Platformada 20 savol/25 daqiqalik imtihon allaqachon bor; Arena — qisqa, qayta-qayta o'ynaladigan retention halqasi. | §4.2, scoring balansi |
| **Q3** | Arena javoblari o'quv dvigateliga (FSRS/xato-banki/mastery) ta'sir qilishi kerakmi? Streak'ni saqlab qoladimi? | **Faqat xato-banki:** noto'g'ri javobda `learning.Again`, to'g'ri javobda **hech narsa**. Bu duelni foydali qiladi, lekin hech qanday darvozani (Grand Mock 25%/85%) oshirib bo'lmaydi. Streak: ha. | §6.2/6.3 — **Grand Mock eligibility'ga bevosita ta'sir qiladi** |
| **Q4** | Arena ochkolari leaderboard'ga (M4-01) qo'shilsinmi? | **Yo'q.** Duelda kim kuchli — bu M4-04 reytingi; leaderboard "bu hafta nechta to'g'ri javob" degan boshqa o'lchov. Ajratilgan jadvallar buni bepul ta'minlaydi. | §6.1 |
| **Q5** | Durang (draw) qabul qilinadimi yoki qo'shimcha "sudden-death" savol berilsinmi? | **Durang qabul qilinadi** (M4-03), sudden-death M4-04'da reyting bilan birga ko'rib chiqiladi. Sxema ikkalasini ham qo'llaydi. | §4.4 |
| **Q6** | Uzilish siyosati: 20s'dan keyin forfeit (mag'lubiyat), yoki match nol ochko bilan tugaydi, yoki match bekor qilinadi? | **20s forfeit = mag'lubiyat.** Aks holda yutqizayotgan o'yinchi uchun eng yaxshi strategiya — internetni o'chirish. | §4.5, `end_reason` CHECK, M4-04 reytingi |
| **Q7** | "Do'stni chaqirish" (shaxsiy match kod bilan) — M4-03'da transport darajasida quriladimi, yoki M4-05'ga qoldiriladimi? | Roadmap uni M4-05 (UI)ga qo'ygan, **lekin transport qo'llab-quvvatlashi M4-03'da bo'lishi kerak** — keyin qo'shish matchmaking'ni qayta yozishni talab qiladi. Tavsiya: `arena:invite:<code>` kalitini hozir belgilab, T2'da **kichik** qo'shimcha task sifatida qurish (~yarim task). | §3, T2 hajmi |
| **Q8** | M4-04'dan oldin reyting ko'rsatilsinmi (masalan "1000" boshlang'ich raqam)? | **Yo'q** — ma'nosiz raqam ko'rsatish keyin ELO kelganda "reytingim o'zgarib ketdi" degan ishonchsizlik yaratadi. M4-05 UI reyting joyini bo'sh qoldiradi. | §3.4, M4-05 UI |
| **Q9** | Server restart (deploy) match o'rtasida bo'lsa: bekor (hisobga olinmaydi), durang, yoki jonli qolgan o'yinchi yutadi? | **Bekor** (`end_reason='server_shutdown'`, `outcome=NULL`, reyting o'zgarmaydi) — deploy foydalanuvchi xatosi emas, uni mag'lubiyat bilan jazolash noto'g'ri. | §8.4, T4 |
| **Q10** | Odam topilmasa (45s) bot raqib taklif qilinsinmi? | Launch'da o'yinchi hovuzi kichik bo'ladi, ya'ni bu retention uchun **muhim** — lekin bot'ni odam sifatida ko'rsatish ishonch masalasi. Tavsiya: **oshkora bot** ("Mashq roboti", reytingsiz), M4-05 bilan birga, M4-03'da emas. | Kelajakdagi Plan; T2 timeout xatti-harakati |
| **Q11** | Production launch'da API bir instansiyada ishlaydimi yoki bir nechtasida? | Agar bitta — `LocalTransport` yetarli va `RedisTransport` M7'ga qoladi. Agar bir nechta — `RedisTransport` T4'ga kiradi (+~1 task). Repozitoriyada hozircha **hech qanday deploy manifesti yo'q**, shuning uchun bu javobsiz savol. | §8.3, T4 hajmi |

---

## 13. Spike notes (optional, ~1–2 hours total, before T1 is planned)

**Spike A — upgrade through the real middleware chain (~30 min).** A 40-line branch: add
`GET /api/v1/arena/ws` to `internal/server/server.go` that calls `websocket.Accept` and echoes one message. Run with
`ENV=dev` (so `middleware.Logger` is active) and confirm the upgrade succeeds and the socket survives 5 minutes idle.
This retires the three §2.1 unknowns — chi's `ResponseWriter` wrapper vs `http.Hijacker`, absence of
`WriteTimeout`, and `srv.Shutdown` behaviour with a hijacked connection — before any design is committed to. Discard
the branch afterwards.

**Spike B — Lua pairing atomicity in `redis-cli` (~30 min).** Write `arena_join.lua` (§3.2) and drive it from two
`redis-cli --eval` invocations plus a 20-iteration shell loop. The point is not performance; it is confirming the
return-shape contract and that a paired member is gone from the ZSET, before Go code is written around it. Keep the
resulting script — it is the T2 deliverable.

---

## 14. Out of scope for M4-03 (recorded so it is not silently assumed)

- Rating/ELO, medals, seasons, match history endpoint → M4-04.
- All UI → M4-05.
- Bot opponents (Q10), spectating, tournaments, 3+ players, rematch flow.
- Multi-instance `RedisTransport` (unless Q11 says otherwise) → M7 alongside observability/load-testing.
- Push/Telegram notification of a duel invite → M4-07/M4-08.
- Arena metrics (queue wait histogram, match duration, forfeit rate) → M7-01 observability; the fields to derive them
  from are all persisted by T3.
