# M4-03 Battle Arena — Realtime Infra Implementation Plan (SKELETON)

> **STATUS: ALL PRODUCT DECISIONS LOCKED.** Expand into a full TDD plan with `superpowers:writing-plans`
> before coding T1. Soft opens closed: Q7=yes (invite in T2), Q10=no bot in M4-03, Q11=single-instance LocalTransport.

**Spec:** `docs/superpowers/specs/2026-07-25-m4-03-battle-arena-design.md`

**Locked:** VIP-only (`402` if free); `ArenaQuestionCount=10`, `ArenaQuestionTimeSec=15`,
`ArenaReconnectGraceSec=20`; learning wrong→`Again` / correct→noop / streak→yes; friend-invite transport in T2;
no bot in M4-03; launch on one API instance.

**Goal:** Server-authoritative live 1v1 duel infrastructure: WebSocket transport with ticket auth, Redis matchmaking,
and a match state machine with persistence. No rating/medals (M4-04), no UI (M4-05).

**Architecture in one paragraph:** A new `internal/arena` package. Browsers authenticate by exchanging their BFF
session for a single-use 30-second Redis ticket (the JWT is in an httpOnly cookie the browser cannot read), then open
one WebSocket per profile. A Redis sorted-set queue paired by one atomic Lua script assigns two players to a match.
Each match runs as a single goroutine owning all its state, with question content pushed to both players and all
timing and scoring computed server-side. Results land in three new tables (`arena_match`, `arena_match_player`,
`arena_answer`) — **not** in `exam_session`, for the three reasons in spec §5.1.

**Tech stack:** Go, `github.com/coder/websocket` (**new dependency** — the only one this plan adds),
`github.com/redis/go-redis/v9` + `internal/redisx` (existing), Postgres via sqlc, `go.uber.org/zap` via
`server.Deps.Log` (existing but not yet used by any service — spec §6.5).

## Global constraints

- All Redis keys use the `arena:` prefix (spec §3.1), never `lb:` (that is M4-01's namespace in the same DB 0).
- All decision logic goes in `arena/rules.go` as pure, ctx-free, DB-free functions — the convention
  `session/rules.go` and `leaderboard/rules.go` already establish. If a rule cannot be unit-tested without Redis or
  Postgres, it is in the wrong file.
- Match state has exactly **one** writer (its own goroutine). No mutex, and no accessor method that would let another
  goroutine read or write it — state a package-level invariant in the doc comment (spec §4.1).
- The server is the only clock. No score, deadline, or ordering decision ever derives from a client-supplied
  timestamp (spec §7.1).
- Never send `correct_answer_id` or an explanation before the reveal frame (spec §2.4, §7.2).
- Inject the clock as `Now func() time.Time` (matching `session.Service.Now`). **No test may use `time.Sleep`** to
  advance match state.
- `internal_arena: 5` must be added to `testDBByPackage` in `internal/redisx/testhelper.go` before the first arena
  Redis test, or it fails loudly by design (spec §5.3).
- `go`/`sqlc` commands need `export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"`. `sqlc generate` = `make generate`
  from the repo root. Tests: `make test` or `make test-parallel` (both must be green); `-race` for every concurrency
  test. Editing migration `0020` in place requires `make test-db-reset`.
- Concurrency tests follow `internal/billing/checkout_race_test.go`: reproduce the bug first, then fix it.

---

## T1 — Transport foundation

*Exit criterion:* two clients connect and receive `hello`; a second connect from the same profile closes the first
with `4001`; an idle socket survives 5 minutes; a redeemed ticket cannot be replayed. No matchmaking yet.

- [ ] **T1.1** Add `github.com/coder/websocket`; run Spike A (spec §13) to confirm the upgrade survives the real chi
      middleware chain with `ENV=dev` (`middleware.Logger` active). *Blocking:* if it fails, decide gorilla here, not
      in T3.
- [ ] **T1.2** `arena/protocol.go` — envelope (`{v,t,d}`), every message payload struct from spec §2.4, error codes,
      close codes. Tests: round-trip encode/decode, unknown `v` rejected, unknown `t` rejected.
- [ ] **T1.3** `arena/service.go` ticket mint/redeem — `crypto/rand` + `base64.RawURLEncoding` (same as
      `auth.NewRefreshToken`), `arena:ticket:<token>` with `GETDEL`, 30s TTL, `auth.Limiter` on minting. Tests:
      single-use, expiry, wrong token.
- [ ] **T1.4** `arena/hub.go` + `arena/conn.go` — registry (one socket per profile, `4001` replacement), read pump
      with `ReadLimit(4096)`, write pump with a bounded channel that closes rather than blocks, 20s ping / ~45s
      liveness, per-connection flood guard (20 msg/10s → `1008`), `arena:conn:<profileID>` heartbeat.
- [ ] **T1.5** `arena/handlers.go` + wiring in `internal/server/server.go`: `POST /arena/ws-ticket` inside the
      `auth.Required` group, `GET /arena/ws` on bare `api`, `websocket.Accept` with `OriginPatterns` from
      `cfg.PublicBaseURL`. Config: `ARENA_INSTANCE_ID` (default hostname) + `backend/.env.example` entry.
- [ ] **T1.6** Handler tests via `httptest.NewServer` per spec §10 (ticket replay, expiry, missing ticket → 401,
      second-connect kick, oversize frame, bad protocol version).

## T2 — Matchmaking

*Exit criterion:* two queued clients both get `match.found` with the same `match_id`; 20 simultaneous joins produce
exactly 10 matches with no profile twice, under `-race`; cancel and disconnect-while-queued both dequeue.

- [ ] **T2.1** `arena/rules.go` — `Bucket`, `SearchBuckets`, and the constants from spec §4.2. **Pure-function tests
      first** (bucket boundaries, every widening step, the timeout edge).
- [ ] **T2.2** `RatingProvider` interface + `FixedRating{1000}` (spec §3.4). Nothing else may read a rating.
- [ ] **T2.3** `arena_join.lua` (spec §3.2) — the atomic look-for-opponent-else-enqueue script (Spike B output), and
      the Go wrapper. **The atomicity test is the deliverable here**, not the happy path.
- [ ] **T2.4** `arena/matchmaker.go` — `queue.join`/`queue.leave`, reverse index `arena:queued:<profileID>`,
      500ms widening ticker, 45s timeout eviction, stale-candidate skip with bounded retry, dequeue on disconnect.
- [ ] **T2.5** Eligibility gate (spec §3.5) — **VIP required** (`billing.Status` → `ErrRequiresVIP`),
      already-in-match, already-queued, join rate limit. No daily match budget.
- [ ] **T2.6** Friend-invite codes (**locked Q7**): `arena:invite:<code>` create/redeem, VIP check on both sides,
      bypasses public buckets, same match actor as ranked queue.

## T3 — Match state machine and persistence

*Exit criterion:* a full duel is playable end to end and its result is queryable in Postgres; every `match_test.go`
case in spec §10 passes; a forfeit writes the correct rows.

- [ ] **T3.1** Migration `0020_battle_arena` (spec §5.2) + down migration + sqlc queries
      (`ListCorrectAnswerIDsForQuestions`, match/player/answer writes, today's match count). **Depends on spec
      Q1/Q6** (the `limit_config` row and the `end_reason` CHECK values).
- [ ] **T3.2** `AnswerPoints` + `Outcome` pure functions with reference-value tests. **Depends on spec Q2/Q5.**
- [ ] **T3.3** `arena/match.go` actor — injected clock, per-match timers, `pending → question_active →
      question_reveal → …  → finished`, question + correct-answer-key preload at creation, both-answered-early reveal.
- [ ] **T3.4** Answer acceptance rules (spec §4.4): active index only, one per index, answer-belongs-to-question,
      `deadline + 400ms` grace, score clamped to the window.
- [ ] **T3.5** Disconnect / reconnect / forfeit (spec §4.5): `opponent.status`, `match.rejoin` → `match.state`
      snapshot (never leaking the active question's key), 20s grace, forfeit outcomes, both-disconnected draw.
      **Depends on spec Q6.**
- [ ] **T3.6** Transactional finish (spec §4.6), idempotent — including `arena_answer` rows for timed-out questions
      (`answer_id IS NULL`).
- [ ] **T3.7** Learning-engine integration per spec Q3 (recommended: `learning.Again` on wrong answers only, plus
      `progress.RecordActivity`). Test that a duel **cannot** raise `CountStudiedQuestions` if the answer is "no
      credit for correct" — this is the Grand Mock gate, so it needs an explicit test, not an assumption.

## T4 — Hardening and seams

*Exit criterion:* SIGTERM during a live match ends it cleanly with no goroutine leak; an independent whole-branch
review passes (required by the handoff §6 before anything is marked done).

- [ ] **T4.1** Graceful arena drain on SIGTERM before `srv.Shutdown` (spec §8.4) — hijacked connections are invisible
      to `srv.Shutdown`, so this does not come for free. **Depends on spec Q9** for how the match is scored.
- [ ] **T4.2** `Transport` interface + `LocalTransport` (spec §8.2). `RedisTransport` deferred (locked Q11:
      single-instance launch).
- [ ] **T4.3** Win-trading guard (spec §7.4a): refuse to pair on simultaneous enqueue + shared device fingerprint +
      shared asserted IP, and **re-queue rather than reject** (false positives are expected — CGNAT, shared family
      connections, internet cafés).
- [ ] **T4.4** Suspicion recording only — persist `response_ms` and flag sub-250ms correct-answer patterns for M3
      admin review. **No auto-ban** (spec §7.5).
- [ ] **T4.5** Concurrency smoke driver: 200 sockets / 100 matches, `runtime.NumGoroutine()` before and after. Not in CI.
- [ ] **T4.6** Whole-branch review, then update `2026-07-24-SESSION-HANDOFF.md` and the roadmap M4-03 row.

## Handoff to M4-04 (record, do not build here)

- Implement `RatingProvider` with real ELO; populate `rating_before/after/delta` (columns already exist, no migration).
- **Forfeits must count as losses** (spec §4.5) — otherwise rage-quitting is optimal when losing.
- Diminishing rating returns for repeat opponents (spec §7.4c).
- `GET /me/arena/matches` history reads rows T3 already writes.
