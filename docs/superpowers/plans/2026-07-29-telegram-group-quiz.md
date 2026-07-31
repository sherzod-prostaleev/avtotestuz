# Telegram guruh quizi — implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `/quiz` ni bir kishilik savol-javobdan 10 savollik, vaqt chegarali,
ko'p kishilik guruh musobaqasiga aylantirish — har bir ishtirokchining hisobi
alohida yuritiladi va yakunda to'liq reyting chiqadi.

**Architecture:** Har bir savol ikki xabar bo'lib yuboriladi — rasm
(`sendPhoto`), keyin unga reply qilingan Telegram **native quiz poll**
(`sendPoll`, `type: "quiz"`). Poll 100 belgilik variantlarni, o'rnatilgan
sanoqni (`open_period`) va konfettini beradi. Javoblar `callback_query`
o'rniga `poll_answer` update'i orqali keladi — u har bir foydalanuvchini
alohida ko'rsatadi, shu sababli guruh reytingi mumkin bo'ladi.

**Tech Stack:** Go 1.26.5, chi, pgx/v5, sqlc 1.31.1, Postgres. Telegram Bot
API — mavjud `internal/bot.Client` (tashqi SDK yo'q, oddiy REST).

## Global Constraints

- Spec: `docs/superpowers/specs/2026-07-29-telegram-group-quiz-design.md`
- **Migratsiya faqat qo'shimcha (additive).** Mavjud jadval/ustun o'chirilmaydi,
  tipi o'zgartirilmaydi. Jonli bazada pul to'lagan foydalanuvchilar bor.
- **Tegilmaydi:** ilova ichidagi mashq/bilet/imtihon oqimlari, to'lov,
  premium, referral, GRAND MOCK, umumiy CSS tokenlar
  (`--success`, `--danger`, `--accent`, `--ring`).
- Locale: `quizLocale = "uz-Latn"` o'zgarmaydi (spec §9).
- Poll cheklovlari: savol ≤300 belgi, variant ≤100 belgi, 2–10 variant,
  `explanation` ≤200 belgi, `open_period` 5–600 sekund.
- `message_effect_id` **faqat shaxsiy chatda** yuboriladi.
- Boshlang'ich qiymatlar: 10 savol, 10 sekund. Ikkalasi ham `limit_config`
  dan o'qiladi, kodga qotirilmaydi.
- Commit uslubi: `type(scope): imperativ sarlavha`, bo'sh qator, **nima uchun**
  qilinganini tushuntiruvchi tan. Har bir commit oxirida:
  `Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>`
- `main` ga to'g'ridan-to'g'ri push yo'q. Tarmoq: `feat/m4-07-telegram-group-quiz`.

### Muhit (har bir terminal sessiyada)

Go va sqlc PATH'da emas — har bir buyruqdan oldin:

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
```

Postgres kerak (bir marta):

```bash
cd "/home/sher/Рабочий стол/avtotest" && docker compose up -d --wait
```

Test buyrug'i (shu reja davomida hamma joyda shu ishlatiladi):

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && \
TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
go test ./internal/bot/... -count=1
```

**Tekshirilgan baseline (2026-07-29):** `go build ./...` OK,
`go vet ./internal/bot/...` OK, `go test ./internal/bot/...` → `ok 33.6s`.

### Test yozish qoidasi — majburiy

**Har bir testni avval SINDIRIB ko'r.** Implementatsiyadan oldin ishga
tushir va aynan kutilgan sabab bilan qizil bo'lishiga ishonch hosil qil.
"O'tdi" degani "tekshirdi" degani emas.

---

## File Structure

| Fayl | Mas'uliyati |
|---|---|
| `internal/db/migrations/0046_telegram_quiz_multiplayer.up.sql` | Yangi jadvallar + ustunlar + sozlama qatorlari |
| `internal/db/migrations/0046_telegram_quiz_multiplayer.down.sql` | Rollback |
| `internal/db/queries/telegram_quiz.sql` | **Modify** — yangi so'rovlar qo'shiladi |
| `internal/bot/types.go` | **Modify** — `Poll`, `PollAnswer`, `Update.PollAnswer` |
| `internal/bot/client.go` | **Modify** — `SendPoll`, `SetMessageReaction`, `SendSticker`, `SendTextWithEffect`, `allowed_updates` |
| `internal/bot/quizconfig.go` | **Create** — `limit_config` dan sozlamalarni o'qish |
| `internal/bot/quizpoll.go` | **Create** — poll yasash, savol tanlash filtri, uzunlik kesish |
| `internal/bot/quizscore.go` | **Create** — ishtirokchi hisobi, reyting matni |
| `internal/bot/quiz.go` | **Modify** — `sendNextQuestion` poll'ga o'tadi, `HandlePollAnswer`, `finishGame` |
| `internal/bot/dispatcher.go` | **Modify** — `poll_answer` update'ini yo'naltirish |

`quiz.go` hozir 415 qator. Yangi mantiq unga to'liq qo'shilsa ~800 qatorga
chiqadi — shuning uchun poll yasash, hisob va sozlama uchta alohida faylga
ajratiladi.

---

## Task 1: Migratsiya 0046 va sqlc so'rovlari

**Files:**
- Create: `backend/internal/db/migrations/0046_telegram_quiz_multiplayer.up.sql`
- Create: `backend/internal/db/migrations/0046_telegram_quiz_multiplayer.down.sql`
- Modify: `backend/internal/db/queries/telegram_quiz.sql`
- Test: `backend/internal/bot/quizscore_test.go`

**Interfaces:**
- Consumes: mavjud `telegram_quiz_session` (migratsiya 0039), `question`, `answer`, `answer_translation`, `limit_config`
- Produces: sqlc funksiyalari —
  `CreateQuizPoll(ctx, CreateQuizPollParams{PollID string, SessionID uuid.UUID, QuestionID uuid.NullUUID, QuestionNo int32, CorrectIdx int32}) error`
  `GetQuizPoll(ctx, pollID string) (TelegramQuizPoll, error)`
  `CloseQuizPoll(ctx, pollID string) error`
  `UpsertQuizParticipant(ctx, UpsertQuizParticipantParams{SessionID uuid.UUID, TgUserID int64, DisplayName string, CorrectDelta int32, ElapsedMs int64}) error`
  `ListQuizRanking(ctx, sessionID uuid.UUID) ([]ListQuizRankingRow, error)`
  `AdvanceQuizSessionQuestion(ctx, sessionID uuid.UUID) (int32, error)`
  `SetQuizSessionMode(ctx, SetQuizSessionModeParams{ID uuid.UUID, Mode string, TotalQuestions int32}) error`
  `RandomPollableQuestionIDs(ctx, RandomPollableQuestionIDsParams{HasImage bool, MaxAnswerLen int32, LimitCount int32}) ([]uuid.UUID, error)`
  `GetLimitConfigValue(ctx, key string) (int32, error)`

- [ ] **Step 1: Migratsiya up faylini yoz**

`backend/internal/db/migrations/0046_telegram_quiz_multiplayer.up.sql`:

```sql
-- Ko'p kishilik Telegram quizi: har bir ishtirokchi alohida hisoblanadi.
-- Spec: docs/superpowers/specs/2026-07-29-telegram-group-quiz-design.md

ALTER TABLE telegram_quiz_session
  ADD COLUMN total_questions int  NOT NULL DEFAULT 10,
  ADD COLUMN question_no     int  NOT NULL DEFAULT 0,
  ADD COLUMN mode            text NOT NULL DEFAULT 'solo';

CREATE TABLE telegram_quiz_participant (
  session_id     uuid   NOT NULL REFERENCES telegram_quiz_session(id) ON DELETE CASCADE,
  tg_user_id     bigint NOT NULL,
  display_name   text   NOT NULL DEFAULT '',
  answered_count int    NOT NULL DEFAULT 0,
  correct_count  int    NOT NULL DEFAULT 0,
  total_ms       bigint NOT NULL DEFAULT 0,
  first_seen_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (session_id, tg_user_id)
);

CREATE INDEX telegram_quiz_participant_rank_idx
  ON telegram_quiz_participant (session_id, correct_count DESC, total_ms ASC);

-- poll_answer update'i faqat poll_id beradi — savolga qaytib bog'lash uchun.
CREATE TABLE telegram_quiz_poll (
  poll_id     text PRIMARY KEY,
  session_id  uuid NOT NULL REFERENCES telegram_quiz_session(id) ON DELETE CASCADE,
  question_id uuid REFERENCES question(id) ON DELETE SET NULL,
  question_no int  NOT NULL,
  correct_idx int  NOT NULL,
  opened_at   timestamptz NOT NULL DEFAULT now(),
  closed      boolean NOT NULL DEFAULT false
);

CREATE INDEX telegram_quiz_poll_session_idx
  ON telegram_quiz_poll (session_id, question_no);

-- Sozlamalar: /settings/limits admin sahifasidan deploy'siz o'zgartiriladi.
-- limit_config free/vip juftligi bu kalitlar uchun ma'nosiz — ikkalasi ham
-- bir xil qiymatni saqlaydi, o'qiyotgan kod faqat free_value ni oladi.
INSERT INTO limit_config (key, free_value, vip_value) VALUES
  ('tg_quiz_seconds',   10, 10),
  ('tg_quiz_questions', 10, 10)
ON CONFLICT (key) DO NOTHING;
```

- [ ] **Step 2: Migratsiya down faylini yoz**

`backend/internal/db/migrations/0046_telegram_quiz_multiplayer.down.sql`:

```sql
DELETE FROM limit_config WHERE key IN ('tg_quiz_seconds', 'tg_quiz_questions');

DROP TABLE IF EXISTS telegram_quiz_poll;
DROP TABLE IF EXISTS telegram_quiz_participant;

ALTER TABLE telegram_quiz_session
  DROP COLUMN IF EXISTS mode,
  DROP COLUMN IF EXISTS question_no,
  DROP COLUMN IF EXISTS total_questions;
```

- [ ] **Step 3: sqlc so'rovlarini qo'sh**

`backend/internal/db/queries/telegram_quiz.sql` **oxiriga** qo'shiladi:

```sql
-- name: SetQuizSessionMode :exec
UPDATE telegram_quiz_session
SET mode = sqlc.arg(mode), total_questions = sqlc.arg(total_questions)
WHERE id = sqlc.arg(id);

-- name: AdvanceQuizSessionQuestion :one
UPDATE telegram_quiz_session
SET question_no = question_no + 1, last_activity_at = now()
WHERE id = $1 AND active = true
RETURNING question_no;

-- name: CreateQuizPoll :exec
INSERT INTO telegram_quiz_poll
  (poll_id, session_id, question_id, question_no, correct_idx)
VALUES ($1, $2, $3, $4, $5);

-- name: GetQuizPoll :one
SELECT poll_id, session_id, question_id, question_no, correct_idx, opened_at, closed
FROM telegram_quiz_poll WHERE poll_id = $1;

-- name: CloseQuizPoll :exec
UPDATE telegram_quiz_poll SET closed = true WHERE poll_id = $1;

-- name: UpsertQuizParticipant :exec
INSERT INTO telegram_quiz_participant
  (session_id, tg_user_id, display_name, answered_count, correct_count, total_ms)
VALUES (
  sqlc.arg(session_id), sqlc.arg(tg_user_id), sqlc.arg(display_name),
  1, sqlc.arg(correct_delta), sqlc.arg(elapsed_ms)
)
ON CONFLICT (session_id, tg_user_id) DO UPDATE SET
  answered_count = telegram_quiz_participant.answered_count + 1,
  correct_count  = telegram_quiz_participant.correct_count + EXCLUDED.correct_count,
  total_ms       = telegram_quiz_participant.total_ms + EXCLUDED.total_ms,
  display_name   = CASE WHEN EXCLUDED.display_name <> ''
                        THEN EXCLUDED.display_name
                        ELSE telegram_quiz_participant.display_name END;

-- name: ListQuizRanking :many
-- Reyting: to'g'ri javob soni, tenglikda o'rtacha javob vaqti tezrog'i.
SELECT tg_user_id, display_name, answered_count, correct_count, total_ms
FROM telegram_quiz_participant
WHERE session_id = $1
ORDER BY correct_count DESC,
         (total_ms::numeric / GREATEST(answered_count, 1)) ASC,
         first_seen_at ASC;

-- name: RandomPollableQuestionIDs :many
-- Telegram poll varianti 100 belgidan oshmasligi kerak: uzun javobli
-- savollar tanlanmaydi (kesish o'rniga chetlab o'tiladi).
SELECT q.id FROM question q
WHERE q.validation_status = 'valid'
  AND (q.image_id IS NOT NULL) = sqlc.arg(has_image)::boolean
  AND (SELECT COUNT(*) FROM answer a WHERE a.question_id = q.id) BETWEEN 2 AND 10
  AND NOT EXISTS (
    SELECT 1 FROM answer a
    JOIN answer_translation at
      ON at.answer_id = a.id AND at.locale = 'uz-Latn' AND at.status = 'verified'
    WHERE a.question_id = q.id
      AND char_length(at.text) > sqlc.arg(max_answer_len)::int
  )
ORDER BY random()
LIMIT sqlc.arg(limit_count);

-- name: GetLimitConfigValue :one
SELECT free_value FROM limit_config WHERE key = $1;
```

- [ ] **Step 4: sqlc generate**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && sqlc generate
```

Expected: xatosiz tugaydi; `internal/db/sqlc/telegram_quiz.sql.go` da yangi
funksiyalar paydo bo'ladi.

- [ ] **Step 5: Migratsiya va so'rovlar ishlashini isbotlovchi testni yoz**

`backend/internal/bot/quizscore_test.go` (yangi fayl):

```go
package bot

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

// Ranking must order by correct_count first and break ties by the faster
// average — not by insertion order, which is what a naive query returns.
func TestListQuizRankingOrdersByCorrectThenSpeed(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	session, err := q.CreateQuizSession(ctx, sqlc.CreateQuizSessionParams{
		ChatID: -7001, StartedByTgUserID: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	// slow: 3 correct, 30s total over 3 answers (10s avg)
	// fast: 3 correct, 9s total over 3 answers (3s avg)
	// weak:  1 correct
	for _, p := range []struct {
		id      int64
		name    string
		correct int32
		ms      int64
	}{
		{101, "slow", 1, 10000},
		{102, "fast", 1, 3000},
		{103, "weak", 0, 1000},
	} {
		for i := 0; i < 3; i++ {
			correct := p.correct
			if p.name == "weak" && i > 0 {
				correct = 0
			}
			if p.name == "weak" && i == 0 {
				correct = 1
			}
			if err := q.UpsertQuizParticipant(ctx, sqlc.UpsertQuizParticipantParams{
				SessionID: session.ID, TgUserID: p.id, DisplayName: p.name,
				CorrectDelta: correct, ElapsedMs: p.ms,
			}); err != nil {
				t.Fatal(err)
			}
		}
	}

	rows, err := q.ListQuizRanking(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 participants, got %d", len(rows))
	}
	if rows[0].DisplayName != "fast" {
		t.Fatalf("want fast first (same score, quicker average), got %q", rows[0].DisplayName)
	}
	if rows[1].DisplayName != "slow" {
		t.Fatalf("want slow second, got %q", rows[1].DisplayName)
	}
	if rows[2].DisplayName != "weak" {
		t.Fatalf("want weak last, got %q", rows[2].DisplayName)
	}
	if rows[0].AnsweredCount != 3 || rows[0].CorrectCount != 3 {
		t.Fatalf("upsert did not accumulate: %+v", rows[0])
	}
}

// A second poll_answer for the same (session, user) must accumulate, never
// insert a duplicate row.
func TestUpsertQuizParticipantAccumulates(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	session, err := q.CreateQuizSession(ctx, sqlc.CreateQuizSessionParams{
		ChatID: -7002, StartedByTgUserID: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := q.UpsertQuizParticipant(ctx, sqlc.UpsertQuizParticipantParams{
			SessionID: session.ID, TgUserID: 555, DisplayName: "Aziz",
			CorrectDelta: 1, ElapsedMs: 2000,
		}); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := q.ListQuizRanking(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if rows[0].AnsweredCount != 2 || rows[0].CorrectCount != 2 || rows[0].TotalMs != 4000 {
		t.Fatalf("accumulation wrong: %+v", rows[0])
	}
}

// The poll registry is what maps an inbound poll_answer back to a question.
func TestQuizPollRoundTrip(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	session, err := q.CreateQuizSession(ctx, sqlc.CreateQuizSessionParams{
		ChatID: -7003, StartedByTgUserID: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.CreateQuizPoll(ctx, sqlc.CreateQuizPollParams{
		PollID: "poll-abc", SessionID: session.ID,
		QuestionID: uuid.NullUUID{}, QuestionNo: 1, CorrectIdx: 2,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := q.GetQuizPoll(ctx, "poll-abc")
	if err != nil {
		t.Fatal(err)
	}
	if got.CorrectIdx != 2 || got.QuestionNo != 1 || got.SessionID != session.ID {
		t.Fatalf("round trip lost data: %+v", got)
	}
	if got.Closed {
		t.Fatal("new poll should not be closed")
	}
	if err := q.CloseQuizPoll(ctx, "poll-abc"); err != nil {
		t.Fatal(err)
	}
	got, _ = q.GetQuizPoll(ctx, "poll-abc")
	if !got.Closed {
		t.Fatal("CloseQuizPoll did not stick")
	}
}
```

- [ ] **Step 6: Testni ishga tushirib, QIZIL bo'lishini tasdiqla**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && \
TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
go test ./internal/bot/... -run 'TestListQuizRanking|TestUpsertQuizParticipant|TestQuizPollRoundTrip' -count=1
```

Expected: **FAIL** — kompilyatsiya xatosi `undefined: sqlc.UpsertQuizParticipantParams`
(agar Step 4 hali bajarilmagan bo'lsa) yoki migratsiya qo'llanmagani uchun
`relation "telegram_quiz_participant" does not exist`.

⚠️ Agar test **o'tib ketsa** — to'xta. Bu testdb eski migratsiya keshidan
foydalanayotganini bildiradi. Tuzatish: `make test-db-reset`, keyin qayta.

- [ ] **Step 7: Migratsiyani qo'llab, testni YASHIL qil**

`testdb` migratsiyalarni o'zi qo'llaydi, lekin bazalar qayta ishlatiladi —
0046 yangi bo'lgani uchun eski test bazalarini tashlash kerak:

```bash
cd "/home/sher/Рабочий стол/avtotest" && make test-db-reset
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd backend && \
TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
go test ./internal/bot/... -run 'TestListQuizRanking|TestUpsertQuizParticipant|TestQuizPollRoundTrip' -count=1
```

Expected: **PASS** (3 test).

- [ ] **Step 8: To'liq bot to'plamini ishga tushir — regressiya yo'qligini tasdiqla**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && go build ./... && go vet ./... && \
TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
go test ./internal/bot/... -count=1
```

Expected: `ok avtotest.uz/backend/internal/bot`.

- [ ] **Step 9: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest"
git add backend/internal/db/migrations/0046_telegram_quiz_multiplayer.up.sql \
        backend/internal/db/migrations/0046_telegram_quiz_multiplayer.down.sql \
        backend/internal/db/queries/telegram_quiz.sql \
        backend/internal/db/sqlc/ \
        backend/internal/bot/quizscore_test.go
git commit -F - <<'EOF'
feat(bot): give the Telegram quiz a per-participant score table

The group quiz cannot rank anyone today because telegram_quiz_session keeps
one asked/correct pair for the whole chat — there is nowhere to record that
Aziz got seven and Malika got five. This adds the participant table and the
poll registry that an inbound poll_answer needs to find its way back to a
question, plus the ranking query that breaks ties on average speed rather
than on insertion order.

Question selection also gains a corpus filter: 20% of questions carry an
answer past Telegram's 100-character poll option limit, and skipping those is
honest where truncating them is not.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
```

---

## Task 2: Telegram mijoziga poll va effekt metodlari

**Files:**
- Modify: `backend/internal/bot/types.go`
- Modify: `backend/internal/bot/client.go`
- Test: `backend/internal/bot/client_poll_test.go`

**Interfaces:**
- Consumes: mavjud `Client.call`, `fakeTelegram` (`dispatcher_test.go:31`)
- Produces:
  - `type PollRequest struct { Question string; Options []string; CorrectIdx int; Explanation string; OpenPeriod int; ReplyTo int64 }`
  - `type PollAnswer struct { PollID string; User User; OptionIDs []int }`
  - `Update.PollAnswer *PollAnswer`
  - `Client.SendPoll(ctx context.Context, chatID int64, req PollRequest) (messageID int64, pollID string, err error)`
  - `Client.SendTextWithEffect(ctx context.Context, chatID int64, text, effectID string, markup *InlineKeyboardMarkup) (int64, error)`
  - `Client.SetMessageReaction(ctx context.Context, chatID, messageID int64, emoji string) error`
  - `Client.SendSticker(ctx context.Context, chatID int64, fileID string) error`

- [ ] **Step 1: Testni yoz**

`backend/internal/bot/client_poll_test.go` (yangi fayl):

```go
package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type capturedPoll struct {
	mu      sync.Mutex
	path    string
	payload map[string]any
}

func newPollCapture(t *testing.T) (*capturedPoll, *Client) {
	t.Helper()
	c := &capturedPoll{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		c.mu.Lock()
		c.path = r.URL.Path
		c.payload = body
		c.mu.Unlock()
		_, _ = fmt.Fprint(w,
			`{"ok":true,"result":{"message_id":42,"poll":{"id":"poll-xyz"}}}`)
	}))
	t.Cleanup(srv.Close)
	return c, NewClient(srv.URL, "T", srv.Client())
}

func TestSendPollBuildsQuizPayload(t *testing.T) {
	cap, client := newPollCapture(t)
	msgID, pollID, err := client.SendPoll(context.Background(), -900, PollRequest{
		Question:    "Bu belgi nimani bildiradi?",
		Options:     []string{"Birinchi", "Ikkinchi", "Uchinchi"},
		CorrectIdx:  1,
		Explanation: "Izoh",
		OpenPeriod:  10,
		ReplyTo:     41,
	})
	if err != nil {
		t.Fatalf("SendPoll: %v", err)
	}
	if msgID != 42 || pollID != "poll-xyz" {
		t.Fatalf("msgID=%d pollID=%q", msgID, pollID)
	}
	if !strings.HasSuffix(cap.path, "/sendPoll") {
		t.Fatalf("path = %q", cap.path)
	}
	if cap.payload["type"] != "quiz" {
		t.Fatalf("type = %v, want quiz", cap.payload["type"])
	}
	// Without this the bot never learns who answered — poll_answer only
	// carries a user for non-anonymous polls.
	if cap.payload["is_anonymous"] != false {
		t.Fatalf("is_anonymous = %v, want false", cap.payload["is_anonymous"])
	}
	if cap.payload["correct_option_id"] != float64(1) {
		t.Fatalf("correct_option_id = %v", cap.payload["correct_option_id"])
	}
	if cap.payload["open_period"] != float64(10) {
		t.Fatalf("open_period = %v", cap.payload["open_period"])
	}
	if cap.payload["reply_to_message_id"] != float64(41) {
		t.Fatalf("reply_to_message_id = %v", cap.payload["reply_to_message_id"])
	}
	opts, _ := cap.payload["options"].([]any)
	if len(opts) != 3 {
		t.Fatalf("options = %v", cap.payload["options"])
	}
}

// Telegram rejects a poll whose question exceeds 300 chars or whose option
// exceeds 100. The client must not send one.
func TestSendPollRejectsOversizeFields(t *testing.T) {
	_, client := newPollCapture(t)
	ctx := context.Background()

	_, _, err := client.SendPoll(ctx, -900, PollRequest{
		Question: strings.Repeat("a", 301),
		Options:  []string{"bir", "ikki"}, CorrectIdx: 0, OpenPeriod: 10,
	})
	if err == nil {
		t.Fatal("want error for a 301-char question")
	}

	_, _, err = client.SendPoll(ctx, -900, PollRequest{
		Question: "ok",
		Options:  []string{"bir", strings.Repeat("b", 101)}, CorrectIdx: 0, OpenPeriod: 10,
	})
	if err == nil {
		t.Fatal("want error for a 101-char option")
	}

	_, _, err = client.SendPoll(ctx, -900, PollRequest{
		Question: "ok", Options: []string{"yolg'iz"}, CorrectIdx: 0, OpenPeriod: 10,
	})
	if err == nil {
		t.Fatal("want error for a single option")
	}
}

func TestPollAnswerDecodesFromUpdate(t *testing.T) {
	raw := []byte(`{"update_id":7,"poll_answer":{"poll_id":"p1",
		"user":{"id":88,"first_name":"Aziz"},"option_ids":[2]}}`)
	var u Update
	if err := json.Unmarshal(raw, &u); err != nil {
		t.Fatal(err)
	}
	if u.PollAnswer == nil {
		t.Fatal("poll_answer not decoded — Update is missing the field")
	}
	if u.PollAnswer.PollID != "p1" || u.PollAnswer.User.ID != 88 {
		t.Fatalf("poll answer = %+v", u.PollAnswer)
	}
	if len(u.PollAnswer.OptionIDs) != 1 || u.PollAnswer.OptionIDs[0] != 2 {
		t.Fatalf("option_ids = %v", u.PollAnswer.OptionIDs)
	}
}

// poll_answer never arrives unless it is in allowed_updates.
func TestAllowedUpdatesIncludePollAnswer(t *testing.T) {
	cap, client := newPollCapture(t)
	if err := client.SetWebhook(context.Background(), "https://x.test/hook", "s"); err != nil {
		t.Fatal(err)
	}
	got, _ := cap.payload["allowed_updates"].([]any)
	found := false
	for _, v := range got {
		if v == "poll_answer" {
			found = true
		}
	}
	if !found {
		t.Fatalf("allowed_updates = %v, want poll_answer", got)
	}
}

func TestSetMessageReaction(t *testing.T) {
	cap, client := newPollCapture(t)
	if err := client.SetMessageReaction(context.Background(), -900, 42, "🎉"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(cap.path, "/setMessageReaction") {
		t.Fatalf("path = %q", cap.path)
	}
	reactions, _ := cap.payload["reaction"].([]any)
	if len(reactions) != 1 {
		t.Fatalf("reaction = %v", cap.payload["reaction"])
	}
}

func TestSendTextWithEffectCarriesEffectID(t *testing.T) {
	cap, client := newPollCapture(t)
	if _, err := client.SendTextWithEffect(
		context.Background(), 900, "Tabriklaymiz", "5046509860389126442", nil,
	); err != nil {
		t.Fatal(err)
	}
	if cap.payload["message_effect_id"] != "5046509860389126442" {
		t.Fatalf("message_effect_id = %v", cap.payload["message_effect_id"])
	}
}

// An empty effect id must not put the key in the payload at all — Telegram
// rejects an empty string.
func TestSendTextWithEffectOmitsEmptyID(t *testing.T) {
	cap, client := newPollCapture(t)
	if _, err := client.SendTextWithEffect(
		context.Background(), 900, "Salom", "", nil,
	); err != nil {
		t.Fatal(err)
	}
	if _, present := cap.payload["message_effect_id"]; present {
		t.Fatal("empty effect id must be omitted")
	}
}
```

- [ ] **Step 2: Testni ishga tushirib, QIZIL bo'lishini tasdiqla**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && \
go test ./internal/bot/... -run 'TestSendPoll|TestPollAnswerDecodes|TestAllowedUpdates|TestSetMessageReaction|TestSendTextWithEffect' -count=1
```

Expected: **FAIL** — `undefined: PollRequest`, `client.SendPoll undefined`.

- [ ] **Step 3: types.go ga poll tiplarini qo'sh**

`backend/internal/bot/types.go` — `Update` structiga maydon qo'shiladi:

```go
type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message,omitempty"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
	MyChatMember  *ChatMemberUpd `json:"my_chat_member,omitempty"`
	PollAnswer    *PollAnswer    `json:"poll_answer,omitempty"`
}
```

Fayl oxiriga qo'shiladi:

```go
// PollAnswer is a vote in a non-anonymous poll the bot sent. Anonymous polls
// deliver no user, which is why every quiz poll sets is_anonymous=false.
type PollAnswer struct {
	PollID    string `json:"poll_id"`
	User      User   `json:"user"`
	OptionIDs []int  `json:"option_ids"`
}

// PollRequest is one quiz poll to send. Telegram's limits are enforced by
// Client.SendPoll before the request leaves the process.
type PollRequest struct {
	Question    string
	Options     []string
	CorrectIdx  int
	Explanation string
	OpenPeriod  int   // seconds, 5..600
	ReplyTo     int64 // photo message this poll belongs to; 0 = none
}
```

- [ ] **Step 4: client.go ga metodlarni qo'sh**

`backend/internal/bot/client.go` fayl oxiriga:

```go
// Telegram Bot API limits for polls. Exceeding any of them is a 400 from
// Telegram, so they are checked here where the caller gets a usable error.
const (
	pollQuestionMaxChars = 300
	pollOptionMaxChars   = 100
	pollExplanationMax   = 200
	pollMinOptions       = 2
	pollMaxOptions       = 10
	pollMinOpenPeriod    = 5
	pollMaxOpenPeriod    = 600
)

// SendPoll sends a quiz-type poll and returns the message id and the poll id.
// The poll id is what inbound poll_answer updates carry — it is the only
// handle back to the question that was asked.
func (c *Client) SendPoll(ctx context.Context, chatID int64, req PollRequest) (int64, string, error) {
	if n := utf8.RuneCountInString(req.Question); n < 1 || n > pollQuestionMaxChars {
		return 0, "", fmt.Errorf("poll question must be 1..%d chars, got %d", pollQuestionMaxChars, n)
	}
	if len(req.Options) < pollMinOptions || len(req.Options) > pollMaxOptions {
		return 0, "", fmt.Errorf("poll needs %d..%d options, got %d", pollMinOptions, pollMaxOptions, len(req.Options))
	}
	for i, opt := range req.Options {
		if n := utf8.RuneCountInString(opt); n < 1 || n > pollOptionMaxChars {
			return 0, "", fmt.Errorf("poll option %d must be 1..%d chars, got %d", i, pollOptionMaxChars, n)
		}
	}
	if req.CorrectIdx < 0 || req.CorrectIdx >= len(req.Options) {
		return 0, "", fmt.Errorf("correct index %d out of range", req.CorrectIdx)
	}
	if req.OpenPeriod < pollMinOpenPeriod || req.OpenPeriod > pollMaxOpenPeriod {
		return 0, "", fmt.Errorf("open_period must be %d..%d, got %d", pollMinOpenPeriod, pollMaxOpenPeriod, req.OpenPeriod)
	}

	payload := map[string]any{
		"chat_id":           chatID,
		"question":          req.Question,
		"options":           req.Options,
		"type":              "quiz",
		"correct_option_id": req.CorrectIdx,
		"is_anonymous":      false,
		"open_period":       req.OpenPeriod,
	}
	if req.Explanation != "" {
		payload["explanation"] = truncateRunes(req.Explanation, pollExplanationMax)
	}
	if req.ReplyTo != 0 {
		payload["reply_to_message_id"] = req.ReplyTo
	}

	var msg struct {
		MessageID int64 `json:"message_id"`
		Poll      struct {
			ID string `json:"id"`
		} `json:"poll"`
	}
	if err := c.call(ctx, "sendPoll", payload, &msg); err != nil {
		return 0, "", err
	}
	return msg.MessageID, msg.Poll.ID, nil
}

// SendTextWithEffect sends text with an optional full-screen message effect.
// Telegram applies message_effect_id in private chats only; callers must not
// pass one for a group.
func (c *Client) SendTextWithEffect(ctx context.Context, chatID int64, text, effectID string, markup *InlineKeyboardMarkup) (int64, error) {
	payload := map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	}
	if effectID != "" {
		payload["message_effect_id"] = effectID
	}
	if markup != nil {
		payload["reply_markup"] = markup
	}
	var msg Message
	if err := c.call(ctx, "sendMessage", payload, &msg); err != nil {
		return 0, err
	}
	return msg.MessageID, nil
}

// SetMessageReaction puts a single emoji reaction on a message — the one
// celebration primitive that works in groups.
func (c *Client) SetMessageReaction(ctx context.Context, chatID, messageID int64, emoji string) error {
	return c.call(ctx, "setMessageReaction", map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
		"reaction":   []map[string]string{{"type": "emoji", "emoji": emoji}},
	}, nil)
}

// SendSticker posts a sticker by file_id. Callers skip it when no file_id is
// configured — an unverified file_id is a runtime error, not a decoration.
func (c *Client) SendSticker(ctx context.Context, chatID int64, fileID string) error {
	if strings.TrimSpace(fileID) == "" {
		return nil
	}
	return c.call(ctx, "sendSticker", map[string]any{
		"chat_id": chatID,
		"sticker": fileID,
	}, nil)
}
```

`client.go` importlariga `"unicode/utf8"` qo'shiladi.

- [ ] **Step 5: allowed_updates ga poll_answer qo'sh**

`client.go` ichida **ikkita** joyda — `GetUpdates` (159-qator atrofida) va
`SetWebhook` (176-qator atrofida) — ro'yxat bir xil bo'lishi kerak:

```go
			"allowed_updates": []string{
				"message", "callback_query", "my_chat_member", "poll_answer",
			},
```

- [ ] **Step 6: Testni ishga tushirib, YASHIL bo'lishini tasdiqla**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && \
go test ./internal/bot/... -run 'TestSendPoll|TestPollAnswerDecodes|TestAllowedUpdates|TestSetMessageReaction|TestSendTextWithEffect' -count=1
```

Expected: **PASS** (7 test).

- [ ] **Step 7: Butun paket + vet**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && go build ./... && go vet ./... && \
TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
go test ./internal/bot/... -count=1
```

Expected: `ok`.

- [ ] **Step 8: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest"
git add backend/internal/bot/types.go backend/internal/bot/client.go \
        backend/internal/bot/client_poll_test.go
git commit -F - <<'EOF'
feat(bot): teach the Telegram client to send quiz polls

Inline keyboards cap an answer at what fits a phone button, which is why the
quiz truncates at 60 runes today. A quiz poll carries 100 characters per
option, counts down on its own, and reports each voter separately — but only
when is_anonymous is false, so that is not left to the caller.

Telegram's size limits are enforced before the request leaves the process.
A 400 from their API tells you a field was wrong; it does not tell you which
question produced it, and the bot picks questions at random.

poll_answer joins allowed_updates in both the webhook and long-poll paths;
missing it in either one silently drops every vote.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
```

---

## Task 3: Sozlamalar va savol tanlash filtri

**Files:**
- Create: `backend/internal/bot/quizconfig.go`
- Create: `backend/internal/bot/quizpoll.go`
- Test: `backend/internal/bot/quizpoll_test.go`

**Interfaces:**
- Consumes: `sqlc.GetLimitConfigValue`, `sqlc.RandomPollableQuestionIDs` (Task 1)
- Produces:
  - `func (s *QuizService) quizSeconds(ctx context.Context) int`
  - `func (s *QuizService) quizQuestionCount(ctx context.Context) int`
  - `func (s *QuizService) winnerStickerID() string`
  - `func (s *QuizService) pickPollableQuestionID(ctx context.Context) (uuid.UUID, error)`
  - `func buildPollRequest(question string, answers []sqlc.ListQuizAnswersRow, explanation string, seconds int, replyTo int64) (PollRequest, error)`
  - `QuizService.WinnerSticker string` maydoni

- [ ] **Step 1: Testni yoz**

`backend/internal/bot/quizpoll_test.go` (yangi fayl):

```go
package bot

import (
	"context"
	"strings"
	"testing"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func answerRow(pos int32, correct bool, text string) sqlc.ListQuizAnswersRow {
	return sqlc.ListQuizAnswersRow{Position: pos, IsCorrect: correct, Text: text}
}

func TestBuildPollRequestMapsCorrectIndex(t *testing.T) {
	req, err := buildPollRequest(
		"Savol matni",
		[]sqlc.ListQuizAnswersRow{
			answerRow(1, false, "Birinchi"),
			answerRow(2, true, "Ikkinchi"),
			answerRow(3, false, "Uchinchi"),
		},
		"Izoh matni", 10, 55,
	)
	if err != nil {
		t.Fatalf("buildPollRequest: %v", err)
	}
	if req.CorrectIdx != 1 {
		t.Fatalf("CorrectIdx = %d, want 1", req.CorrectIdx)
	}
	if len(req.Options) != 3 || req.Options[1] != "Ikkinchi" {
		t.Fatalf("Options = %v", req.Options)
	}
	if req.OpenPeriod != 10 || req.ReplyTo != 55 {
		t.Fatalf("req = %+v", req)
	}
	if req.Explanation != "Izoh matni" {
		t.Fatalf("Explanation = %q", req.Explanation)
	}
}

// A question with no correct answer is corrupt data, not a poll.
func TestBuildPollRequestRejectsNoCorrectAnswer(t *testing.T) {
	_, err := buildPollRequest("Savol", []sqlc.ListQuizAnswersRow{
		answerRow(1, false, "Bir"), answerRow(2, false, "Ikki"),
	}, "", 10, 0)
	if err == nil {
		t.Fatal("want error when no answer is marked correct")
	}
}

// Long text must be rejected here rather than truncated: the corpus filter
// is supposed to have excluded it, so reaching this point means the filter
// leaked and the operator needs to know.
func TestBuildPollRequestRejectsOversizeOption(t *testing.T) {
	_, err := buildPollRequest("Savol", []sqlc.ListQuizAnswersRow{
		answerRow(1, true, strings.Repeat("x", 101)),
		answerRow(2, false, "Ikki"),
	}, "", 10, 0)
	if err == nil {
		t.Fatal("want error for a 101-char option")
	}
}

// The 300-char question limit is enforced too, even though today's corpus
// tops out at 222.
func TestBuildPollRequestRejectsOversizeQuestion(t *testing.T) {
	_, err := buildPollRequest(strings.Repeat("s", 301), []sqlc.ListQuizAnswersRow{
		answerRow(1, true, "Bir"), answerRow(2, false, "Ikki"),
	}, "", 10, 0)
	if err == nil {
		t.Fatal("want error for a 301-char question")
	}
}

// The corpus filter is the whole reason polls are viable — prove it excludes
// a question whose answer is too long, and keeps one whose answers fit.
func TestPickPollableQuestionSkipsLongAnswers(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	longID := seedQuizQuestionWithAnswers(t, pool, false,
		[]string{strings.Repeat("u", 140), "Qisqa"})
	okID := seedQuizQuestionWithAnswers(t, pool, false,
		[]string{"Qisqa bir", "Qisqa ikki"})

	svc := &QuizService{Q: q, Pool: pool}
	seen := map[string]bool{}
	for i := 0; i < 25; i++ {
		id, err := svc.pickPollableQuestionID(ctx)
		if err != nil {
			t.Fatalf("pickPollableQuestionID: %v", err)
		}
		seen[id.String()] = true
	}
	if seen[longID.String()] {
		t.Fatal("picked a question whose answer exceeds the 100-char poll limit")
	}
	if !seen[okID.String()] {
		t.Fatal("never picked the question that fits — filter is too aggressive")
	}
}

func TestQuizSecondsFallsBackWhenKeyMissing(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	svc := &QuizService{Q: sqlc.New(pool), Pool: pool}

	if _, err := pool.Exec(ctx,
		`DELETE FROM limit_config WHERE key = 'tg_quiz_seconds'`); err != nil {
		t.Fatal(err)
	}
	if got := svc.quizSeconds(ctx); got != defaultQuizSeconds {
		t.Fatalf("quizSeconds = %d, want fallback %d", got, defaultQuizSeconds)
	}
}

// An operator typing 2 into /settings/limits must not produce a poll
// Telegram will reject — the value is clamped to the API's 5..600 window.
func TestQuizSecondsClampsOutOfRangeConfig(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	svc := &QuizService{Q: sqlc.New(pool), Pool: pool}

	if _, err := pool.Exec(ctx,
		`INSERT INTO limit_config (key, free_value, vip_value) VALUES ('tg_quiz_seconds', 2, 2)
		 ON CONFLICT (key) DO UPDATE SET free_value = 2`); err != nil {
		t.Fatal(err)
	}
	if got := svc.quizSeconds(ctx); got != pollMinOpenPeriod {
		t.Fatalf("quizSeconds = %d, want clamp to %d", got, pollMinOpenPeriod)
	}

	if _, err := pool.Exec(ctx,
		`UPDATE limit_config SET free_value = 5000 WHERE key = 'tg_quiz_seconds'`); err != nil {
		t.Fatal(err)
	}
	if got := svc.quizSeconds(ctx); got != pollMaxOpenPeriod {
		t.Fatalf("quizSeconds = %d, want clamp to %d", got, pollMaxOpenPeriod)
	}
}

func TestQuizQuestionCountReadsConfig(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	svc := &QuizService{Q: sqlc.New(pool), Pool: pool}

	if _, err := pool.Exec(ctx,
		`INSERT INTO limit_config (key, free_value, vip_value) VALUES ('tg_quiz_questions', 7, 7)
		 ON CONFLICT (key) DO UPDATE SET free_value = 7`); err != nil {
		t.Fatal(err)
	}
	if got := svc.quizQuestionCount(ctx); got != 7 {
		t.Fatalf("quizQuestionCount = %d, want 7", got)
	}
}
```

`seedQuizQuestionWithAnswers` yordamchisi `quiz_test.go` ga qo'shiladi
(mavjud `seedQuizQuestion` yonida):

```go
// seedQuizQuestionWithAnswers seeds a question whose answer texts are given
// explicitly, so tests can exercise the poll length filter.
func seedQuizQuestionWithAnswers(t *testing.T, pool *pgxpool.Pool, withImage bool, texts []string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var catID, qID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO category (code, sort_order) VALUES ($1, 1) RETURNING id`,
		"tg_poll_"+uuid.NewString()[:8],
	).Scan(&catID); err != nil {
		t.Fatal(err)
	}
	var imageID any
	if withImage {
		var img uuid.UUID
		if err := pool.QueryRow(ctx,
			`INSERT INTO image (storage_key, sha256, width, height, mime)
			 VALUES ($1, $2, 100, 100, 'image/png') RETURNING id`,
			"q/"+uuid.NewString()+".png", uuid.NewString(),
		).Scan(&img); err != nil {
			t.Fatal(err)
		}
		imageID = img
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO question (source_ext_id, category_id, content_hash, image_id)
		VALUES ($1, $2, $3, $4) RETURNING id`,
		"tgp-"+uuid.NewString(), catID, uuid.NewString(), imageID,
	).Scan(&qID); err != nil {
		t.Fatal(err)
	}
	var correctID uuid.UUID
	for i, text := range texts {
		var aID uuid.UUID
		if err := pool.QueryRow(ctx,
			`INSERT INTO answer (question_id, position, is_correct) VALUES ($1,$2,$3) RETURNING id`,
			qID, i+1, i == 0,
		).Scan(&aID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO answer_translation (answer_id, locale, text, status)
			 VALUES ($1,'uz-Latn',$2,'verified')`, aID, text); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			correctID = aID
		}
	}
	if _, err := pool.Exec(ctx,
		`UPDATE question SET correct_answer_id=$2 WHERE id=$1`, qID, correctID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO question_translation (question_id, locale, text, status, source)
		 VALUES ($1,'uz-Latn','Poll savoli?','verified','')`, qID); err != nil {
		t.Fatal(err)
	}
	return qID
}
```

- [ ] **Step 2: Testni ishga tushirib, QIZIL bo'lishini tasdiqla**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && \
TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
go test ./internal/bot/... -run 'TestBuildPollRequest|TestPickPollable|TestQuizSeconds|TestQuizQuestionCount' -count=1
```

Expected: **FAIL** — `undefined: buildPollRequest`, `undefined: defaultQuizSeconds`.

- [ ] **Step 3: quizconfig.go ni yoz**

`backend/internal/bot/quizconfig.go`:

```go
package bot

import (
	"context"

	"go.uber.org/zap"
)

// Defaults used when limit_config has no row or the read fails. The bot must
// keep running with a sane game rather than refuse to start a quiz.
const (
	defaultQuizSeconds   = 10
	defaultQuizQuestions = 10

	limitKeyQuizSeconds   = "tg_quiz_seconds"
	limitKeyQuizQuestions = "tg_quiz_questions"

	minQuizQuestions = 1
	maxQuizQuestions = 50
)

func (s *QuizService) limitValue(ctx context.Context, key string, fallback int) int {
	if s == nil || s.Q == nil {
		return fallback
	}
	v, err := s.Q.GetLimitConfigValue(ctx, key)
	if err != nil {
		s.logger().Debug("quiz: limit_config read failed, using default",
			zap.String("key", key), zap.Error(err))
		return fallback
	}
	return int(v)
}

// quizSeconds is the per-question countdown, clamped to what sendPoll accepts
// so a mistyped admin value cannot make every poll fail.
func (s *QuizService) quizSeconds(ctx context.Context) int {
	v := s.limitValue(ctx, limitKeyQuizSeconds, defaultQuizSeconds)
	if v < pollMinOpenPeriod {
		return pollMinOpenPeriod
	}
	if v > pollMaxOpenPeriod {
		return pollMaxOpenPeriod
	}
	return v
}

// quizQuestionCount is how many questions one game asks.
func (s *QuizService) quizQuestionCount(ctx context.Context) int {
	v := s.limitValue(ctx, limitKeyQuizQuestions, defaultQuizQuestions)
	if v < minQuizQuestions {
		return minQuizQuestions
	}
	if v > maxQuizQuestions {
		return maxQuizQuestions
	}
	return v
}

// winnerStickerID is optional decoration. Empty means no sticker is sent —
// an unverified file_id would be a runtime error on every finished game.
func (s *QuizService) winnerStickerID() string {
	if s == nil {
		return ""
	}
	return s.WinnerSticker
}
```

- [ ] **Step 4: quizpoll.go ni yoz**

`backend/internal/bot/quizpoll.go`:

```go
package bot

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/db/sqlc"
)

// buildPollRequest turns a question and its answers into a quiz poll.
// Oversize text is an error, not something to truncate: the corpus filter is
// supposed to have excluded it upstream, so hitting it means the filter has
// a hole worth surfacing.
func buildPollRequest(question string, answers []sqlc.ListQuizAnswersRow, explanation string, seconds int, replyTo int64) (PollRequest, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return PollRequest{}, fmt.Errorf("question text is empty")
	}
	if n := utf8.RuneCountInString(question); n > pollQuestionMaxChars {
		return PollRequest{}, fmt.Errorf("question is %d chars, poll limit is %d", n, pollQuestionMaxChars)
	}

	options := make([]string, 0, len(answers))
	correctIdx := -1
	for i, a := range answers {
		text := strings.TrimSpace(a.Text)
		if text == "" {
			return PollRequest{}, fmt.Errorf("answer %d has no text", i)
		}
		if n := utf8.RuneCountInString(text); n > pollOptionMaxChars {
			return PollRequest{}, fmt.Errorf("answer %d is %d chars, poll limit is %d", i, n, pollOptionMaxChars)
		}
		if a.IsCorrect && correctIdx < 0 {
			correctIdx = i
		}
		options = append(options, text)
	}
	if len(options) < pollMinOptions || len(options) > pollMaxOptions {
		return PollRequest{}, fmt.Errorf("question has %d answers, poll needs %d..%d", len(options), pollMinOptions, pollMaxOptions)
	}
	if correctIdx < 0 {
		return PollRequest{}, fmt.Errorf("no answer marked correct")
	}

	return PollRequest{
		Question:    question,
		Options:     options,
		CorrectIdx:  correctIdx,
		Explanation: strings.TrimSpace(explanation),
		OpenPeriod:  seconds,
		ReplyTo:     replyTo,
	}, nil
}

// pickPollableQuestionID prefers an illustrated question, then falls back to
// a text-only one. Both draws exclude questions carrying an answer longer
// than a poll option allows.
func (s *QuizService) pickPollableQuestionID(ctx context.Context) (uuid.UUID, error) {
	for _, hasImage := range []bool{true, false} {
		ids, err := s.Q.RandomPollableQuestionIDs(ctx, sqlc.RandomPollableQuestionIDsParams{
			HasImage:     hasImage,
			MaxAnswerLen: pollOptionMaxChars,
			LimitCount:   1,
		})
		if err != nil {
			return uuid.Nil, err
		}
		if len(ids) > 0 {
			return ids[0], nil
		}
	}
	return uuid.Nil, nil
}
```

- [ ] **Step 5: QuizService ga WinnerSticker maydonini qo'sh**

`backend/internal/bot/quiz.go`, `QuizService` structi (34-42 qatorlar):

```go
type QuizService struct {
	Q             *sqlc.Queries
	Pool          *pgxpool.Pool
	TG            *Client
	MediaBaseURL  string
	PublicBaseURL string
	WinnerSticker string // optional file_id; empty skips the sticker
	Log           *zap.Logger
}
```

- [ ] **Step 6: Testni ishga tushirib, YASHIL bo'lishini tasdiqla**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && \
TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
go test ./internal/bot/... -run 'TestBuildPollRequest|TestPickPollable|TestQuizSeconds|TestQuizQuestionCount' -count=1
```

Expected: **PASS** (8 test).

- [ ] **Step 7: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest"
git add backend/internal/bot/quizconfig.go backend/internal/bot/quizpoll.go \
        backend/internal/bot/quizpoll_test.go backend/internal/bot/quiz_test.go \
        backend/internal/bot/quiz.go
git commit -F - <<'EOF'
feat(bot): pick only questions a quiz poll can carry

A fifth of the corpus has an answer longer than Telegram's 100-character poll
option, and the fix is to not ask those questions rather than to send half of
one. buildPollRequest treats oversize text as an error for the same reason: a
question reaching it means the filter has a hole, and silently trimming would
hide that.

Question count and per-question seconds move to limit_config, which already
has an admin screen, so tuning the pace does not need a deploy. Both are
clamped — an operator typing 2 seconds would otherwise make every poll fail
at Telegram's five-second floor.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
```

---

## Task 4: Savolni poll sifatida yuborish

**Files:**
- Modify: `backend/internal/bot/quiz.go` (`sendNextQuestion`, `StartOrNext`)
- Test: `backend/internal/bot/quizflow_test.go`

**Interfaces:**
- Consumes: `buildPollRequest`, `pickPollableQuestionID`, `quizSeconds`, `quizQuestionCount` (Task 3); `Client.SendPoll` (Task 2); `CreateQuizPoll`, `SetQuizSessionMode`, `AdvanceQuizSessionQuestion` (Task 1)
- Produces: `func (s *QuizService) sendNextQuestion(ctx context.Context, session sqlc.TelegramQuizSession) error` — rasm + poll yuboradi, `telegram_quiz_poll` ga yozadi

- [ ] **Step 1: Testni yoz**

`backend/internal/bot/quizflow_test.go` (yangi fayl). Bu test `fakeTelegram`
ni poll'ni ham qayd qiladigan qilib kengaytiradi:

```go
package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

type recordedCall struct {
	method  string
	payload map[string]any
}

type recordingTG struct {
	mu    sync.Mutex
	calls []recordedCall
	srv   *httptest.Server
}

func (r *recordingTG) methodCalls(method string) []map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []map[string]any{}
	for _, c := range r.calls {
		if c.method == method {
			out = append(out, c.payload)
		}
	}
	return out
}

func newRecordingTelegram(t *testing.T) (*recordingTG, *Client) {
	t.Helper()
	r := &recordingTG{}
	msgID := int64(500)
	pollSeq := 0
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		parts := strings.Split(req.URL.Path, "/")
		method := parts[len(parts)-1]
		var body map[string]any
		_ = json.NewDecoder(req.Body).Decode(&body)
		r.mu.Lock()
		r.calls = append(r.calls, recordedCall{method: method, payload: body})
		r.mu.Unlock()
		msgID++
		if method == "sendPoll" {
			pollSeq++
			_, _ = fmt.Fprintf(w,
				`{"ok":true,"result":{"message_id":%d,"poll":{"id":"poll-%d"}}}`, msgID, pollSeq)
			return
		}
		_, _ = fmt.Fprintf(w, `{"ok":true,"result":{"message_id":%d}}`, msgID)
	}))
	t.Cleanup(r.srv.Close)
	return r, NewClient(r.srv.URL, "T", r.srv.Client())
}

// An illustrated question becomes two messages: the photo, then a poll that
// replies to it. Telegram cannot attach an image to a poll.
func TestSendNextQuestionSendsPhotoThenPoll(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, true, []string{"To'g'ri", "Xato"})

	rec, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client,
		MediaBaseURL: "http://media.test", PublicBaseURL: "http://app.test"}

	if err := svc.StartOrNext(ctx, -8001, 21); err != nil {
		t.Fatalf("StartOrNext: %v", err)
	}

	photos := rec.methodCalls("sendPhoto")
	polls := rec.methodCalls("sendPoll")
	if len(photos) != 1 {
		t.Fatalf("want 1 sendPhoto, got %d", len(photos))
	}
	if len(polls) != 1 {
		t.Fatalf("want 1 sendPoll, got %d", len(polls))
	}
	if polls[0]["reply_to_message_id"] == nil {
		t.Fatal("poll must reply to the photo so the pair reads as one question")
	}
	if polls[0]["type"] != "quiz" {
		t.Fatalf("poll type = %v", polls[0]["type"])
	}
}

// The poll id Telegram returns must be stored, or the poll_answer that comes
// back cannot be matched to a question.
func TestSendNextQuestionRecordsPollID(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	_, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client,
		MediaBaseURL: "http://media.test", PublicBaseURL: "http://app.test"}

	if err := svc.StartOrNext(ctx, -8002, 22); err != nil {
		t.Fatal(err)
	}
	stored, err := q.GetQuizPoll(ctx, "poll-1")
	if err != nil {
		t.Fatalf("poll id was not recorded: %v", err)
	}
	if stored.QuestionNo != 1 {
		t.Fatalf("QuestionNo = %d, want 1", stored.QuestionNo)
	}
	if stored.CorrectIdx != 0 {
		t.Fatalf("CorrectIdx = %d, want 0", stored.CorrectIdx)
	}
}

// A group session must be marked as such at creation — the final message
// format branches on it, and chat type is the only signal.
func TestStartOrNextMarksGroupMode(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	_, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, -8003, 23, "supergroup"); err != nil {
		t.Fatal(err)
	}
	session, err := q.GetActiveQuizSessionByChat(ctx, -8003)
	if err != nil {
		t.Fatal(err)
	}
	if session.Mode != "group" {
		t.Fatalf("Mode = %q, want group", session.Mode)
	}
	if session.TotalQuestions != defaultQuizQuestions {
		t.Fatalf("TotalQuestions = %d, want %d", session.TotalQuestions, defaultQuizQuestions)
	}
}

func TestStartGameMarksSoloModeInPrivate(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	_, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, 9001, 24, "private"); err != nil {
		t.Fatal(err)
	}
	session, _ := q.GetActiveQuizSessionByChat(ctx, 9001)
	if session.Mode != "solo" {
		t.Fatalf("Mode = %q, want solo", session.Mode)
	}
}
```

- [ ] **Step 2: Testni ishga tushirib, QIZIL bo'lishini tasdiqla**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && \
TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
go test ./internal/bot/... -run 'TestSendNextQuestion|TestStartOrNextMarksGroup|TestStartGameMarksSolo' -count=1
```

Expected: **FAIL** — `svc.StartGame undefined`, va `sendPoll` chaqirilmagani
uchun `want 1 sendPoll, got 0`.

- [ ] **Step 3: quiz.go da sendNextQuestion ni poll'ga o'tkaz**

`backend/internal/bot/quiz.go` — mavjud `sendNextQuestion` (188-244 qatorlar)
**to'liq almashtiriladi**:

```go
func (s *QuizService) sendNextQuestion(ctx context.Context, session sqlc.TelegramQuizSession) error {
	qID, err := s.pickPollableQuestionID(ctx)
	if err != nil {
		return err
	}
	if qID == uuid.Nil {
		_, err := s.TG.SendText(ctx, session.ChatID, "Hozircha mos savol topilmadi. Keyinroq qayta urinib ko'ring.", nil)
		return err
	}

	detail, err := s.Q.GetQuestion(ctx, sqlc.GetQuestionParams{ID: qID, Locale: quizLocale})
	if err != nil {
		return err
	}
	answers, err := s.Q.ListQuizAnswers(ctx, sqlc.ListQuizAnswersParams{
		QuestionID: qID, Locale: quizLocale,
	})
	if err != nil {
		return err
	}

	questionNo, err := s.Q.AdvanceQuizSessionQuestion(ctx, session.ID)
	if err != nil {
		return err
	}

	// The photo goes first and the poll replies to it: Telegram polls cannot
	// carry an image, so one question is two messages.
	var photoMsgID int64
	if detail.ImageKey.Valid {
		if url := s.mediaURL(detail.ImageKey.String); url != "" {
			caption := fmt.Sprintf("Savol %d/%d", questionNo, session.TotalQuestions)
			photoMsgID, err = s.TG.SendPhoto(ctx, session.ChatID, url, caption, nil)
			if err != nil {
				s.logger().Warn("quiz: sendPhoto failed, continuing with poll only",
					zap.Error(err), zap.Int64("chat_id", session.ChatID))
				photoMsgID = 0
			}
		}
	}

	req, err := buildPollRequest(detail.Text, answers, "", s.quizSeconds(ctx), photoMsgID)
	if err != nil {
		// The corpus filter should have excluded this question; say so and
		// move on rather than stranding the chat.
		s.logger().Warn("quiz: question is not pollable, skipping",
			zap.Error(err), zap.String("question_id", qID.String()))
		_, sendErr := s.TG.SendText(ctx, session.ChatID, "Bu savol o'tkazib yuborildi. /next bilan davom eting.", nil)
		return sendErr
	}

	msgID, pollID, err := s.TG.SendPoll(ctx, session.ChatID, req)
	if err != nil {
		return err
	}

	if err := s.Q.CreateQuizPoll(ctx, sqlc.CreateQuizPollParams{
		PollID:     pollID,
		SessionID:  session.ID,
		QuestionID: uuid.NullUUID{UUID: qID, Valid: true},
		QuestionNo: questionNo,
		CorrectIdx: int32(req.CorrectIdx),
	}); err != nil {
		return err
	}

	return s.Q.SetQuizSessionQuestion(ctx, sqlc.SetQuizSessionQuestionParams{
		ID:              session.ID,
		QuestionID:      uuid.NullUUID{UUID: qID, Valid: true},
		AnswerMessageID: msgID,
	})
}
```

- [ ] **Step 4: StartGame ni qo'sh va StartOrNext ni unga bog'la**

`quiz.go` da `ensureActiveSession` dan keyin qo'shiladi:

```go
// StartGame begins a quiz, recording whether the chat is a group so the
// final message can pick its format. Chat type decides the mode — a group
// with one participant is still a group.
func (s *QuizService) StartGame(ctx context.Context, chatID, tgUserID int64, chatType string) error {
	if s == nil || s.TG == nil || s.Q == nil {
		return fmt.Errorf("quiz service not configured")
	}
	enabled, err := flags.Bool(ctx, s.Pool, flags.KeyTelegramQuiz, true)
	if err != nil {
		return err
	}
	if !enabled {
		_, err := s.TG.SendText(ctx, chatID, "Quiz hozircha o'chirilgan. Keyinroq qayta urinib ko'ring.", nil)
		return err
	}

	session, err := s.ensureActiveSession(ctx, chatID, tgUserID)
	if err != nil {
		return err
	}
	if session.QuestionNo == 0 {
		mode := "solo"
		if IsGroupChat(chatType) {
			mode = "group"
		}
		total := s.quizQuestionCount(ctx)
		if err := s.Q.SetQuizSessionMode(ctx, sqlc.SetQuizSessionModeParams{
			ID: session.ID, Mode: mode, TotalQuestions: int32(total),
		}); err != nil {
			return err
		}
		session.Mode = mode
		session.TotalQuestions = int32(total)
		intro := fmt.Sprintf("🚦 Quiz boshlandi!\n%d savol · har biriga %d sekund",
			total, s.quizSeconds(ctx))
		if _, err := s.TG.SendText(ctx, chatID, intro, nil); err != nil {
			return err
		}
	}
	return s.sendNextQuestion(ctx, session)
}
```

`StartOrNext` (68-115 qatorlar) tanasining oxirgi qatori — `return s.sendNextQuestion(ctx, session)` — o'zgarmaydi; `StartOrNext` `/next` uchun qoladi.

`StartOrNext` ichidagi `if session.AwaitingAnswer` bloki o'chiriladi: poll
rejimida bir vaqtda bitta poll ochiq bo'ladi va uni taymer yopadi, javob
kutish bloklovchi holat emas. O'rniga:

```go
	if session.QuestionNo >= session.TotalQuestions {
		return s.finishGame(ctx, session)
	}
```

⚠️ `finishGame` Task 6 da yoziladi. Task 4 da vaqtincha o'rniga:

```go
	if session.QuestionNo >= session.TotalQuestions {
		_, err := s.TG.SendText(ctx, session.ChatID, "O'yin tugadi.", s.ctaMarkup())
		return err
	}
```

- [ ] **Step 5: dispatcher.go da /quiz ni StartGame ga ulash**

`backend/internal/bot/dispatcher.go`, `case "/quiz", "/next":` bloki:

```go
	case "/quiz":
		if b.Quiz == nil {
			return b.TG.SendMessage(ctx, chatID, msgQuizUnavailable)
		}
		if err := b.Quiz.StartGame(ctx, chatID, tgUserID, chatType); err != nil {
			b.logger().Error("bot: quiz start failed", zap.Error(err), zap.Int64("chat_id", chatID))
			return b.TG.SendMessage(ctx, chatID, msgQuizUnavailable)
		}
	case "/next":
		if b.Quiz == nil {
			return b.TG.SendMessage(ctx, chatID, msgQuizUnavailable)
		}
		if err := b.Quiz.StartOrNext(ctx, chatID, tgUserID); err != nil {
			b.logger().Error("bot: quiz next failed", zap.Error(err), zap.Int64("chat_id", chatID))
			return b.TG.SendMessage(ctx, chatID, msgQuizUnavailable)
		}
```

- [ ] **Step 6: Testni ishga tushirib, YASHIL bo'lishini tasdiqla**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && \
TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
go test ./internal/bot/... -run 'TestSendNextQuestion|TestStartOrNextMarksGroup|TestStartGameMarksSolo' -count=1
```

Expected: **PASS** (4 test).

- [ ] **Step 7: Eski testlarni moslashtir**

`quiz_test.go` dagi `TestQuizStartSendsQuestionWithAnswers` va
`TestQuizAnswerGradesAndIsIdempotent` endi inline tugma emas, poll kutadi.
`TestQuizAnswerGradesAndIsIdempotent` — `handleAnswer` Task 5 da
`HandlePollAnswer` bilan almashadi, shuning uchun bu testni **Task 5 da**
qayta yozamiz. Task 4 da faqat `TestQuizStartSendsQuestionWithAnswers` ni
poll'ni kutadigan qilib yangilash kifoya:

```go
func TestQuizStartSendsQuestionWithAnswers(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, true, []string{"To'g'ri", "Xato"})

	rec, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client,
		MediaBaseURL: "http://media.test", PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, -5001, 11, "supergroup"); err != nil {
		t.Fatalf("StartGame: %v", err)
	}
	polls := rec.methodCalls("sendPoll")
	if len(polls) != 1 {
		t.Fatalf("want 1 poll, got %d", len(polls))
	}
	session, err := q.GetActiveQuizSessionByChat(ctx, -5001)
	if err != nil {
		t.Fatal(err)
	}
	if session.QuestionNo != 1 {
		t.Fatalf("QuestionNo = %d, want 1", session.QuestionNo)
	}
}
```

- [ ] **Step 8: To'liq paket**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && go build ./... && go vet ./... && \
TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
go test ./internal/bot/... -count=1
```

Expected: `ok`. (`TestQuizAnswerGradesAndIsIdempotent` hali eski
`handleAnswer` ni sinaydi va o'tishi kerak — u Task 5 da almashadi.)

- [ ] **Step 9: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest"
git add backend/internal/bot/quiz.go backend/internal/bot/dispatcher.go \
        backend/internal/bot/quizflow_test.go backend/internal/bot/quiz_test.go
git commit -F - <<'EOF'
feat(bot): ask quiz questions as polls instead of inline keyboards

The photo now leads and the poll replies to it, because Telegram will not
attach an image to a poll and the pair has to read as one question. The poll
id comes back from sendPoll and is stored immediately — it is the only handle
an inbound poll_answer gives us, and without the row the vote has no question.

/quiz and /next split: /quiz opens a game and records whether the chat is a
group, /next only advances one. Chat type decides the mode, so a group where
one person shows up still gets the group format.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
```

---

## Task 5: poll_answer'ni qabul qilib, ishtirokchi hisobini yuritish

**Files:**
- Create: `backend/internal/bot/quizscore.go`
- Modify: `backend/internal/bot/quiz.go` (eski `handleAnswer` va `HandleCallback` javob shoxi olib tashlanadi)
- Modify: `backend/internal/bot/dispatcher.go` (`poll_answer` yo'naltirish)
- Test: `backend/internal/bot/quizscore_test.go` (Task 1 da yaratilgan — qo'shiladi)

**Interfaces:**
- Consumes: `UpsertQuizParticipant`, `GetQuizPoll`, `CloseQuizPoll` (Task 1); `Update.PollAnswer` (Task 2)
- Produces: `func (s *QuizService) HandlePollAnswer(ctx context.Context, pa PollAnswer) error`

- [ ] **Step 1: Testni yoz**

`backend/internal/bot/quizscore_test.go` **oxiriga** qo'shiladi:

```go
// Two different Telegram users answering the same poll must produce two
// separate rows. Before polls, the first tap answered for the whole chat.
func TestHandlePollAnswerScoresEachUserSeparately(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	_, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, -8100, 31, "supergroup"); err != nil {
		t.Fatal(err)
	}

	// correct index is 0 (seed marks the first answer correct)
	if err := svc.HandlePollAnswer(ctx, PollAnswer{
		PollID: "poll-1", User: User{ID: 401, FirstName: "Aziz"}, OptionIDs: []int{0},
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandlePollAnswer(ctx, PollAnswer{
		PollID: "poll-1", User: User{ID: 402, FirstName: "Malika"}, OptionIDs: []int{1},
	}); err != nil {
		t.Fatal(err)
	}

	session, _ := q.GetActiveQuizSessionByChat(ctx, -8100)
	rows, err := q.ListQuizRanking(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 participants, got %d", len(rows))
	}
	if rows[0].DisplayName != "Aziz" || rows[0].CorrectCount != 1 {
		t.Fatalf("winner row = %+v", rows[0])
	}
	if rows[1].DisplayName != "Malika" || rows[1].CorrectCount != 0 {
		t.Fatalf("second row = %+v", rows[1])
	}
}

// A poll_answer for a poll we never recorded (an old game, another bot run)
// must be ignored rather than error the webhook.
func TestHandlePollAnswerIgnoresUnknownPoll(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client}

	if err := svc.HandlePollAnswer(ctx, PollAnswer{
		PollID: "does-not-exist", User: User{ID: 1}, OptionIDs: []int{0},
	}); err != nil {
		t.Fatalf("unknown poll must be ignored, got %v", err)
	}
}

// Retracting a vote sends option_ids: [] — it must not count as an answer.
func TestHandlePollAnswerIgnoresEmptyVote(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})
	_, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, -8101, 32, "supergroup"); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandlePollAnswer(ctx, PollAnswer{
		PollID: "poll-1", User: User{ID: 501, FirstName: "Bekzod"}, OptionIDs: []int{},
	}); err != nil {
		t.Fatal(err)
	}
	session, _ := q.GetActiveQuizSessionByChat(ctx, -8101)
	rows, _ := q.ListQuizRanking(ctx, session.ID)
	if len(rows) != 0 {
		t.Fatalf("retracted vote created a participant: %+v", rows)
	}
}

// A user with no first name still needs a label in the ranking.
func TestHandlePollAnswerFallsBackToUsername(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})
	_, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, -8102, 33, "supergroup"); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandlePollAnswer(ctx, PollAnswer{
		PollID: "poll-1", User: User{ID: 601, Username: "nodira_u"}, OptionIDs: []int{0},
	}); err != nil {
		t.Fatal(err)
	}
	session, _ := q.GetActiveQuizSessionByChat(ctx, -8102)
	rows, _ := q.ListQuizRanking(ctx, session.ID)
	if len(rows) != 1 || rows[0].DisplayName != "nodira_u" {
		t.Fatalf("display name fallback failed: %+v", rows)
	}
}
```

- [ ] **Step 2: Testni ishga tushirib, QIZIL bo'lishini tasdiqla**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && \
TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
go test ./internal/bot/... -run 'TestHandlePollAnswer' -count=1
```

Expected: **FAIL** — `svc.HandlePollAnswer undefined`.

- [ ] **Step 3: quizscore.go ni yoz**

`backend/internal/bot/quizscore.go`:

```go
package bot

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"avtotest.uz/backend/internal/db/sqlc"
)

// displayName picks the best label Telegram gave us for the ranking.
func displayName(u User) string {
	if n := strings.TrimSpace(u.FirstName); n != "" {
		return n
	}
	if n := strings.TrimSpace(u.Username); n != "" {
		return n
	}
	return "Ishtirokchi"
}

// HandlePollAnswer records one vote. Unlike the old callback path it knows
// who voted, so every participant keeps their own score instead of the first
// tap answering for the whole chat.
func (s *QuizService) HandlePollAnswer(ctx context.Context, pa PollAnswer) error {
	if s == nil || s.Q == nil {
		return nil
	}
	// A retracted vote arrives with an empty option list.
	if len(pa.OptionIDs) == 0 {
		return nil
	}
	poll, err := s.Q.GetQuizPoll(ctx, pa.PollID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// An old game or another process's poll — not our business.
			return nil
		}
		return err
	}

	correctDelta := int32(0)
	if pa.OptionIDs[0] == int(poll.CorrectIdx) {
		correctDelta = 1
	}

	elapsed := time.Since(poll.OpenedAt).Milliseconds()
	if elapsed < 0 {
		elapsed = 0
	}

	return s.Q.UpsertQuizParticipant(ctx, sqlc.UpsertQuizParticipantParams{
		SessionID:    poll.SessionID,
		TgUserID:     pa.User.ID,
		DisplayName:  displayName(pa.User),
		CorrectDelta: correctDelta,
		ElapsedMs:    elapsed,
	})
}
```

- [ ] **Step 4: quiz.go dan eski javob yo'lini olib tashla**

`HandleCallback` (139-164 qatorlar) dagi `cbAnswerPrefix` shoxi va
`handleAnswer` funksiyasi (266-341 qatorlar), hamda `answerMarkup`
(365-379) va `parseAnswerIndex` (381-394) **o'chiriladi** — javoblar endi
poll orqali keladi. `cbAnswerPrefix` konstantasi ham o'chiriladi.

`HandleCallback` qoladigan holati:

```go
// HandleCallback processes the next / stop taps that follow a finished game.
func (s *QuizService) HandleCallback(ctx context.Context, cq CallbackQuery) error {
	if s == nil || s.TG == nil {
		return nil
	}
	_ = s.TG.AnswerCallbackQuery(ctx, cq.ID, "", false)
	if cq.Message == nil {
		return nil
	}
	chatID := cq.Message.Chat.ID
	switch strings.TrimSpace(cq.Data) {
	case cbNext:
		return s.StartOrNext(ctx, chatID, cq.From.ID)
	case cbStop:
		return s.Stop(ctx, chatID)
	default:
		return nil
	}
}
```

`quiz_test.go` dagi `TestQuizAnswerGradesAndIsIdempotent` **o'chiriladi** —
u sinaydigan `handleAnswer` endi mavjud emas; uning o'rnini Task 5 ning
`TestHandlePollAnswer*` testlari egallaydi.

- [ ] **Step 5: dispatcher.go da poll_answer ni yo'naltir**

`HandleUpdate` boshiga, `MyChatMember` tekshiruvidan keyin:

```go
	if u.PollAnswer != nil {
		if b.Quiz == nil {
			return nil
		}
		if err := b.Quiz.HandlePollAnswer(ctx, *u.PollAnswer); err != nil {
			b.logger().Error("bot: poll answer failed", zap.Error(err))
		}
		return nil
	}
```

- [ ] **Step 6: Testni ishga tushirib, YASHIL bo'lishini tasdiqla**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && \
TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
go test ./internal/bot/... -run 'TestHandlePollAnswer' -count=1
```

Expected: **PASS** (4 test).

- [ ] **Step 7: To'liq paket**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && go build ./... && go vet ./... && \
TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
go test ./internal/bot/... -count=1
```

Expected: `ok`.

- [ ] **Step 8: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest"
git add backend/internal/bot/quizscore.go backend/internal/bot/quizscore_test.go \
        backend/internal/bot/quiz.go backend/internal/bot/quiz_test.go \
        backend/internal/bot/dispatcher.go
git commit -F - <<'EOF'
fix(bot): score every player in a group quiz, not just the first tap

HandleCallback had the answering user in cq.From and threw it away, passing
only the chat id down. In a group that meant whoever tapped first answered on
everyone's behalf and the keyboard was then cleared, so nobody else could
play at all. poll_answer carries the voter, so each one now gets their own
row and their own score.

Two inbound shapes are ignored on purpose: a poll we have no record of
(an old game, a restarted process) and an empty option list, which is what
Telegram sends when a vote is retracted.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
```

---

## Task 6: Taymer, keyingi savolga o'tish va o'yin yakuni

**Files:**
- Modify: `backend/internal/bot/quiz.go` (`finishGame`, `scheduleNext`, `Stop`)
- Modify: `backend/internal/bot/quizscore.go` (`rankingText`)
- Test: `backend/internal/bot/quizfinish_test.go`

**Interfaces:**
- Consumes: `ListQuizRanking` (Task 1), `quizSeconds` (Task 3)
- Produces:
  - `func (s *QuizService) finishGame(ctx context.Context, session sqlc.TelegramQuizSession) error`
  - `func rankingText(rows []sqlc.ListQuizRankingRow, mode string, total int32) string`

- [ ] **Step 1: Testni yoz**

`backend/internal/bot/quizfinish_test.go` (yangi fayl):

```go
package bot

import (
	"context"
	"strings"
	"testing"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func TestRankingTextListsEveryParticipant(t *testing.T) {
	rows := []sqlc.ListQuizRankingRow{
		{DisplayName: "Aziz", CorrectCount: 9, AnsweredCount: 10, TotalMs: 42000},
		{DisplayName: "Malika", CorrectCount: 8, AnsweredCount: 10, TotalMs: 61000},
		{DisplayName: "Bekzod", CorrectCount: 8, AnsweredCount: 10, TotalMs: 97000},
		{DisplayName: "Nodira", CorrectCount: 6, AnsweredCount: 10, TotalMs: 74000},
	}
	out := rankingText(rows, "group", 10)
	for _, name := range []string{"Aziz", "Malika", "Bekzod", "Nodira"} {
		if !strings.Contains(out, name) {
			t.Fatalf("ranking is missing %s:\n%s", name, out)
		}
	}
	if !strings.Contains(out, "9/10") {
		t.Fatalf("ranking lost the winner's score:\n%s", out)
	}
	// The winner must be named, not just listed first.
	if !strings.Contains(out, "Tabriklaymiz") {
		t.Fatalf("ranking does not congratulate anyone:\n%s", out)
	}
	if strings.Index(out, "Aziz") > strings.Index(out, "Malika") {
		t.Fatalf("winner is not first:\n%s", out)
	}
}

// Solo games get a plain result, not a one-line leaderboard.
func TestRankingTextSoloFormat(t *testing.T) {
	rows := []sqlc.ListQuizRankingRow{
		{DisplayName: "Aziz", CorrectCount: 8, AnsweredCount: 10, TotalMs: 54000},
	}
	out := rankingText(rows, "solo", 10)
	if !strings.Contains(out, "8/10") {
		t.Fatalf("solo result missing score:\n%s", out)
	}
	if strings.Contains(out, "🥇") {
		t.Fatalf("solo result should not render a podium:\n%s", out)
	}
}

func TestRankingTextHandlesNobodyPlaying(t *testing.T) {
	out := rankingText(nil, "group", 10)
	if !strings.Contains(out, "Hech kim") {
		t.Fatalf("empty ranking must say nobody played:\n%s", out)
	}
	if strings.Contains(out, "Tabriklaymiz") {
		t.Fatalf("must not congratulate nobody:\n%s", out)
	}
}

// The whole point of the timer: after the configured number of questions the
// game ends and the ranking is sent.
func TestFinishGameDeactivatesAndSendsRanking(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	rec, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, -8200, 41, "supergroup"); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandlePollAnswer(ctx, PollAnswer{
		PollID: "poll-1", User: User{ID: 701, FirstName: "Aziz"}, OptionIDs: []int{0},
	}); err != nil {
		t.Fatal(err)
	}
	session, _ := q.GetActiveQuizSessionByChat(ctx, -8200)
	if err := svc.finishGame(ctx, session); err != nil {
		t.Fatal(err)
	}

	if _, err := q.GetActiveQuizSessionByChat(ctx, -8200); err == nil {
		t.Fatal("session is still active after finishGame")
	}
	var found bool
	for _, p := range rec.methodCalls("sendMessage") {
		if text, _ := p["text"].(string); strings.Contains(text, "Aziz") {
			found = true
		}
	}
	if !found {
		t.Fatal("final message did not include the ranking")
	}
}

// /stop mid-game must still report what people scored so far.
func TestStopSendsRankingSoFar(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	rec, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, -8201, 42, "supergroup"); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandlePollAnswer(ctx, PollAnswer{
		PollID: "poll-1", User: User{ID: 801, FirstName: "Malika"}, OptionIDs: []int{0},
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Stop(ctx, -8201); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, p := range rec.methodCalls("sendMessage") {
		if text, _ := p["text"].(string); strings.Contains(text, "Malika") {
			found = true
		}
	}
	if !found {
		t.Fatal("/stop did not report the scores collected so far")
	}
}
```

- [ ] **Step 2: Testni ishga tushirib, QIZIL bo'lishini tasdiqla**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && \
TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
go test ./internal/bot/... -run 'TestRankingText|TestFinishGame|TestStopSendsRanking' -count=1
```

Expected: **FAIL** — `undefined: rankingText`, `svc.finishGame undefined`.

- [ ] **Step 3: rankingText ni quizscore.go ga qo'sh**

`backend/internal/bot/quizscore.go` oxiriga:

```go
var podium = []string{"🥇", "🥈", "🥉"}

// avgSeconds is the mean answer time, used only for display and tie-breaks.
func avgSeconds(totalMs int64, answered int32) float64 {
	if answered <= 0 {
		return 0
	}
	return float64(totalMs) / float64(answered) / 1000.0
}

// rankingText renders the end-of-game message. Group games get a podium and
// name the winner; a solo game gets a plain score, since a leaderboard of one
// is not a leaderboard.
func rankingText(rows []sqlc.ListQuizRankingRow, mode string, total int32) string {
	if len(rows) == 0 {
		return "O'yin tugadi.\n\nHech kim qatnashmadi."
	}

	var b strings.Builder
	if mode != "group" {
		r := rows[0]
		fmt.Fprintf(&b, "✅ Natijangiz: %d/%d · o'rtacha %.1fs",
			r.CorrectCount, total, avgSeconds(r.TotalMs, r.AnsweredCount))
		return b.String()
	}

	b.WriteString("🏆 O'yin tugadi!\n\n")
	for i, r := range rows {
		marker := fmt.Sprintf("%2d.", i+1)
		if i < len(podium) {
			marker = podium[i]
		}
		fmt.Fprintf(&b, "%s %s — %d/%d · %.1fs\n",
			marker, r.DisplayName, r.CorrectCount, total,
			avgSeconds(r.TotalMs, r.AnsweredCount))
	}
	fmt.Fprintf(&b, "\n👥 %d ishtirokchi · %d savol\n", len(rows), total)
	fmt.Fprintf(&b, "\n🎉 Tabriklaymiz, %s!", rows[0].DisplayName)
	return b.String()
}
```

`quizscore.go` importlariga `"fmt"` qo'shiladi.

- [ ] **Step 4: finishGame ni quiz.go ga qo'sh**

`backend/internal/bot/quiz.go` — `Stop` funksiyasidan keyin:

```go
// finishGame closes the session and reports the ranking. It is the only
// place a game ends, so /stop and the last question share one code path.
func (s *QuizService) finishGame(ctx context.Context, session sqlc.TelegramQuizSession) error {
	rows, err := s.Q.ListQuizRanking(ctx, session.ID)
	if err != nil {
		return err
	}
	if err := s.Q.DeactivateQuizSession(ctx, session.ID); err != nil {
		return err
	}

	body := rankingText(rows, session.Mode, session.TotalQuestions)
	body += "\n\nDriver Go — rasmiy formatda bepul mashq"

	msgID, err := s.sendFinalMessage(ctx, session, body)
	if err != nil {
		return err
	}
	if len(rows) > 0 {
		s.celebrate(ctx, session, msgID)
	}
	return nil
}
```

`Stop` funksiyasi (118-136 qatorlar) `finishGame` ga o'tkaziladi:

```go
// Stop ends the active quiz session for the chat and reports scores so far.
func (s *QuizService) Stop(ctx context.Context, chatID int64) error {
	session, err := s.Q.GetActiveQuizSessionByChat(ctx, chatID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			_, sendErr := s.TG.SendText(ctx, chatID, "Faol quiz yo'q. Boshlash: /quiz", nil)
			return sendErr
		}
		return err
	}
	return s.finishGame(ctx, session)
}
```

⚠️ `sendFinalMessage` va `celebrate` Task 7 da yoziladi. Task 6 da
vaqtinchalik oddiy versiya:

```go
func (s *QuizService) sendFinalMessage(ctx context.Context, session sqlc.TelegramQuizSession, body string) (int64, error) {
	return s.TG.SendText(ctx, session.ChatID, body, s.ctaMarkup())
}

func (s *QuizService) celebrate(_ context.Context, _ sqlc.TelegramQuizSession, _ int64) {}
```

- [ ] **Step 5: StartOrNext dagi vaqtinchalik yakunni finishGame ga almashtir**

Task 4 Step 4 da qo'yilgan vaqtinchalik blok:

```go
	if session.QuestionNo >= session.TotalQuestions {
		return s.finishGame(ctx, session)
	}
```

- [ ] **Step 6: Taymerni qo'sh — savol yopilgach keyingisiga o'tish**

`quiz.go` da `sendNextQuestion` ning oxiriga, `SetQuizSessionQuestion` dan
keyin qo'shiladi:

```go
	s.scheduleAdvance(session.ChatID, s.quizSeconds(ctx))
	return nil
```

va yangi funksiya:

```go
// scheduleAdvance moves the game on after the poll closes. Telegram closes
// the poll itself; this only decides when to ask the next question. A lost
// timer (deploy, restart) is recoverable with /next — it is deliberately not
// durable state.
func (s *QuizService) scheduleAdvance(chatID int64, seconds int) {
	if s.Advance == nil {
		return
	}
	s.Advance(chatID, time.Duration(seconds+1)*time.Second)
}
```

`QuizService` structiga maydon qo'shiladi:

```go
	// Advance schedules the next question. Injected so tests drive the clock
	// instead of sleeping through a real countdown.
	Advance func(chatID int64, after time.Duration)
```

Ishlab chiqarishda `cmd/api` da ulanadi (Task 8).

- [ ] **Step 7: Testni ishga tushirib, YASHIL bo'lishini tasdiqla**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && \
TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
go test ./internal/bot/... -run 'TestRankingText|TestFinishGame|TestStopSendsRanking' -count=1
```

Expected: **PASS** (5 test).

- [ ] **Step 8: To'liq paket**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && go build ./... && go vet ./... && \
TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
go test ./internal/bot/... -count=1
```

Expected: `ok`.

- [ ] **Step 9: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest"
git add backend/internal/bot/quiz.go backend/internal/bot/quizscore.go \
        backend/internal/bot/quizfinish_test.go
git commit -F - <<'EOF'
feat(bot): end the quiz with a ranking instead of a running tally

A group game now closes on its own after the configured number of questions
and reports who scored what, ordered by correct answers and broken on average
speed. /stop routes through the same path, so quitting early still tells the
room where they stood rather than discarding it.

The advance timer is injected rather than durable: losing it to a restart
costs a /next, and persisting a scheduler for a chat game would be more
machinery than the problem is worth.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
```

---

## Task 7: Vizual effektlar

**Files:**
- Modify: `backend/internal/bot/quiz.go` (`sendFinalMessage`, `celebrate`)
- Test: `backend/internal/bot/quizeffect_test.go`

**Interfaces:**
- Consumes: `Client.SendTextWithEffect`, `SetMessageReaction`, `SendSticker` (Task 2); `winnerStickerID` (Task 3)
- Produces: yo'q (ichki)

- [ ] **Step 1: Testni yoz**

`backend/internal/bot/quizeffect_test.go` (yangi fayl):

```go
package bot

import (
	"context"
	"testing"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

// Telegram applies message_effect_id in private chats only. Sending one to a
// group is an API error, so the group path must never carry it.
func TestGroupFinalMessageCarriesNoEffect(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	rec, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, -8300, 51, "supergroup"); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandlePollAnswer(ctx, PollAnswer{
		PollID: "poll-1", User: User{ID: 901, FirstName: "Aziz"}, OptionIDs: []int{0},
	}); err != nil {
		t.Fatal(err)
	}
	session, _ := q.GetActiveQuizSessionByChat(ctx, -8300)
	if err := svc.finishGame(ctx, session); err != nil {
		t.Fatal(err)
	}

	for _, p := range rec.methodCalls("sendMessage") {
		if _, present := p["message_effect_id"]; present {
			t.Fatal("a group message must not carry message_effect_id")
		}
	}
	// The guaranteed group celebration is a reaction.
	if len(rec.methodCalls("setMessageReaction")) == 0 {
		t.Fatal("group finish did not react to the result message")
	}
}

func TestPrivateFinalMessageCarriesEffect(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	rec, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, 9500, 52, "private"); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandlePollAnswer(ctx, PollAnswer{
		PollID: "poll-1", User: User{ID: 902, FirstName: "Aziz"}, OptionIDs: []int{0},
	}); err != nil {
		t.Fatal(err)
	}
	session, _ := q.GetActiveQuizSessionByChat(ctx, 9500)
	if err := svc.finishGame(ctx, session); err != nil {
		t.Fatal(err)
	}

	var effectSeen bool
	for _, p := range rec.methodCalls("sendMessage") {
		if p["message_effect_id"] == effectCelebrate {
			effectSeen = true
		}
	}
	if !effectSeen {
		t.Fatal("private finish did not carry the celebration effect")
	}
}

// An unset sticker id must not produce a sendSticker call at all.
func TestNoStickerWhenNotConfigured(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	rec, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, -8301, 53, "supergroup"); err != nil {
		t.Fatal(err)
	}
	if err := svc.HandlePollAnswer(ctx, PollAnswer{
		PollID: "poll-1", User: User{ID: 903, FirstName: "Aziz"}, OptionIDs: []int{0},
	}); err != nil {
		t.Fatal(err)
	}
	session, _ := q.GetActiveQuizSessionByChat(ctx, -8301)
	if err := svc.finishGame(ctx, session); err != nil {
		t.Fatal(err)
	}
	if len(rec.methodCalls("sendSticker")) != 0 {
		t.Fatal("sent a sticker with no file_id configured")
	}
}

// Nobody played means nothing to celebrate.
func TestNoCelebrationWhenNobodyPlayed(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	rec, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, -8302, 54, "supergroup"); err != nil {
		t.Fatal(err)
	}
	session, _ := q.GetActiveQuizSessionByChat(ctx, -8302)
	if err := svc.finishGame(ctx, session); err != nil {
		t.Fatal(err)
	}
	if len(rec.methodCalls("setMessageReaction")) != 0 {
		t.Fatal("celebrated an empty game")
	}
}
```

- [ ] **Step 2: Testni ishga tushirib, QIZIL bo'lishini tasdiqla**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && \
TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
go test ./internal/bot/... -run 'TestGroupFinalMessage|TestPrivateFinalMessage|TestNoSticker|TestNoCelebration' -count=1
```

Expected: **FAIL** — `undefined: effectCelebrate`, va guruh javobida
`setMessageReaction` chaqirilmagani uchun.

- [ ] **Step 3: sendFinalMessage va celebrate ni to'liq yoz**

`backend/internal/bot/quiz.go` — Task 6 dagi vaqtinchalik ikkita funksiya
almashtiriladi:

```go
// Telegram's 🎉 message effect. Private chats only — passing it to a group
// is an API error, which is why sendFinalMessage branches on mode.
const effectCelebrate = "5046509860389126442"

func (s *QuizService) sendFinalMessage(ctx context.Context, session sqlc.TelegramQuizSession, body string) (int64, error) {
	if session.Mode == "group" {
		return s.TG.SendText(ctx, session.ChatID, body, s.ctaMarkup())
	}
	return s.TG.SendTextWithEffect(ctx, session.ChatID, body, effectCelebrate, s.ctaMarkup())
}

// celebrate adds the decoration that survives a group chat. A reaction always
// works; the sticker is optional because an unverified file_id would fail on
// every finished game. Neither failure is worth aborting the result message.
func (s *QuizService) celebrate(ctx context.Context, session sqlc.TelegramQuizSession, msgID int64) {
	if session.Mode != "group" || msgID == 0 {
		return
	}
	if err := s.TG.SetMessageReaction(ctx, session.ChatID, msgID, "🎉"); err != nil {
		s.logger().Debug("quiz: reaction failed", zap.Error(err))
	}
	if sticker := s.winnerStickerID(); sticker != "" {
		if err := s.TG.SendSticker(ctx, session.ChatID, sticker); err != nil {
			s.logger().Debug("quiz: sticker failed", zap.Error(err))
		}
	}
}
```

- [ ] **Step 4: Testni ishga tushirib, YASHIL bo'lishini tasdiqla**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && \
TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
go test ./internal/bot/... -run 'TestGroupFinalMessage|TestPrivateFinalMessage|TestNoSticker|TestNoCelebration' -count=1
```

Expected: **PASS** (4 test).

- [ ] **Step 5: To'liq paket**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && go build ./... && go vet ./... && \
TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
go test ./internal/bot/... -count=1
```

Expected: `ok`.

- [ ] **Step 6: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest"
git add backend/internal/bot/quiz.go backend/internal/bot/quizeffect_test.go
git commit -F - <<'EOF'
feat(bot): celebrate a finished quiz where Telegram lets us

Private chats get the full-screen 🎉 effect. Groups cannot — Telegram applies
message_effect_id to private chats only and errors otherwise — so a group
gets a reaction on the result message instead, which always works.

The winner sticker stays optional and unset. A hardcoded file_id we have not
verified would fail on every finished game, and a decoration is not worth
that; the reaction carries the moment on its own until a real sticker is
chosen.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
```

---

## Task 8: Ishlab chiqarish ulanishi va yakuniy tekshiruv

**Files:**
- Modify: `backend/internal/server/server.go` (`QuizService` ga `Advance` va `WinnerSticker`)
- Modify: `backend/internal/config/config.go` (`TelegramQuizWinnerSticker`)
- Test: `backend/internal/bot/quizadvance_test.go`

**Interfaces:**
- Consumes: `QuizService.Advance` (Task 6)
- Produces: yo'q (yakuniy sim)

- [ ] **Step 1: Testni yoz**

`backend/internal/bot/quizadvance_test.go` (yangi fayl):

```go
package bot

import (
	"context"
	"sync"
	"testing"
	"time"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

// Sending a question must schedule the next one, or a group game stalls
// after question 1 with no way forward but /next.
func TestSendNextQuestionSchedulesAdvance(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	_, client := newRecordingTelegram(t)

	var mu sync.Mutex
	var scheduled []time.Duration
	svc := &QuizService{
		Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test",
		Advance: func(_ int64, after time.Duration) {
			mu.Lock()
			scheduled = append(scheduled, after)
			mu.Unlock()
		},
	}

	if err := svc.StartGame(ctx, -8400, 61, "supergroup"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(scheduled) != 1 {
		t.Fatalf("want 1 scheduled advance, got %d", len(scheduled))
	}
	// One second of slack past the poll's own countdown.
	want := time.Duration(defaultQuizSeconds+1) * time.Second
	if scheduled[0] != want {
		t.Fatalf("scheduled after %v, want %v", scheduled[0], want)
	}
}

// A nil Advance must not panic — long-poll dev runs wire it, tests may not.
func TestSendNextQuestionWithoutSchedulerDoesNotPanic(t *testing.T) {
	pool := testdb.New(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	_ = seedQuizQuestionWithAnswers(t, pool, false, []string{"To'g'ri", "Xato"})

	_, client := newRecordingTelegram(t)
	svc := &QuizService{Q: q, Pool: pool, TG: client, PublicBaseURL: "http://app.test"}

	if err := svc.StartGame(ctx, -8401, 62, "supergroup"); err != nil {
		t.Fatalf("nil Advance must be tolerated: %v", err)
	}
}
```

- [ ] **Step 2: Testni ishga tushirib, QIZIL bo'lishini tasdiqla**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && \
TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
go test ./internal/bot/... -run 'TestSendNextQuestionSchedules|TestSendNextQuestionWithoutScheduler' -count=1
```

Expected: birinchi test **FAIL** bo'ladi agar Task 6 Step 6 to'liq
bajarilmagan bo'lsa; bajarilgan bo'lsa ikkalasi ham o'tadi — bu holda
testning haqiqiyligini isbotlash uchun `scheduleAdvance` chaqiruvini
vaqtincha izohga ol, qizil bo'lishini ko'r, keyin qaytar.

- [ ] **Step 3: Umumiy scheduler yordamchisini yoz**

`QuizService` **ikkita** joyda quriladi (tekshirilgan):
`internal/server/server.go:237` (webhook rejimi) va `cmd/api/main.go:82`
(longpoll rejimi). Ikkalasiga ham bir xil scheduler kerak — birini
unutish o'sha rejimda o'yinni 1-savolda muzlatib qo'yadi.

Takrorlamaslik uchun yordamchi `internal/bot/quizconfig.go` oxiriga
qo'shiladi:

```go
// NewAdvanceScheduler returns a QuizService.Advance implementation. It runs
// detached because the caller is a webhook handler or a long-poll loop —
// both must return promptly — and the next question is due only after the
// poll's own countdown has run out.
func NewAdvanceScheduler(svc *QuizService, log *zap.Logger) func(int64, time.Duration) {
	if log == nil {
		log = zap.NewNop()
	}
	return func(chatID int64, after time.Duration) {
		go func() {
			timer := time.NewTimer(after)
			defer timer.Stop()
			<-timer.C
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := svc.StartOrNext(ctx, chatID, 0); err != nil {
				log.Warn("quiz: scheduled advance failed",
					zap.Int64("chat_id", chatID), zap.Error(err))
			}
		}()
	}
}
```

`quizconfig.go` importlariga `"time"` qo'shiladi.

- [ ] **Step 4: Ikkala qurish joyiga ham ulash**

`backend/internal/server/server.go`, 237-244 qatorlar — literalga ikkita
maydon qo'shiladi va undan keyin scheduler ulanadi:

```go
					quizSvc := &bot.QuizService{
						Q:             deps.Queries,
						Pool:          deps.Pool,
						TG:            tgClient,
						MediaBaseURL:  cfg.MediaBaseURL,
						PublicBaseURL: cfg.PublicBaseURL,
						WinnerSticker: cfg.TelegramQuizWinnerSticker,
						Log:           log,
					}
					quizSvc.Advance = bot.NewAdvanceScheduler(quizSvc, log)
```

`backend/cmd/api/main.go`, 82-89 qatorlar — xuddi shunday:

```go
		quizSvc := &bot.QuizService{
			Q:             q,
			Pool:          pool,
			TG:            tgClient,
			MediaBaseURL:  cfg.MediaBaseURL,
			PublicBaseURL: cfg.PublicBaseURL,
			WinnerSticker: cfg.TelegramQuizWinnerSticker,
			Log:           logger,
		}
		quizSvc.Advance = bot.NewAdvanceScheduler(quizSvc, logger)
```

- [ ] **Step 5: config.go ga stiker sozlamasini qo'sh**

`backend/internal/config/config.go` — `Config` structida, 52-qatordagi
`TelegramWebhookSecret` dan keyin:

```go
	TelegramWebhookSecret string
	// Optional file_id for the group winner sticker. Empty skips the sticker
	// entirely — an unverified file_id would fail on every finished game.
	TelegramQuizWinnerSticker string
```

O'qish joyi, 123-qatordagi `TelegramWebhookSecret` satridan keyin (fayl
`getenv` yordamchisidan foydalanadi — 238-qator):

```go
		TelegramWebhookSecret:     getenv("TELEGRAM_WEBHOOK_SECRET", ""),
		TelegramQuizWinnerSticker: getenv("TELEGRAM_QUIZ_WINNER_STICKER", ""),
```

⚠️ `gofmt` maydon tekislashni o'zgartiradi — qo'shni qatorlar ham
qayta tekislanadi, bu kutilgan.

- [ ] **Step 6: Testni YASHIL qil va to'liq tekshiruv**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && \
go build ./... && go vet ./... && \
TEST_DATABASE_URL="postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable" \
go test ./... -p 1 -count=1 2>&1 | tail -30
```

Expected: barcha paketlar `ok` yoki `no test files`. Bitta ham `FAIL` yo'q.

- [ ] **Step 7: Linter**

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd "/home/sher/Рабочий стол/avtotest/backend" && golangci-lint run
```

Expected: 0 muammo.

- [ ] **Step 8: Commit**

```bash
cd "/home/sher/Рабочий стол/avtotest"
git add backend/internal/server/server.go backend/cmd/api/main.go \
        backend/internal/config/config.go backend/internal/bot/quizconfig.go \
        backend/internal/bot/quizadvance_test.go
git commit -F - <<'EOF'
feat(bot): drive the quiz forward on its own timer

The scheduler runs detached so a webhook still returns promptly, and fires a
second past the poll's countdown rather than racing it. A nil scheduler stays
valid — tests and one-shot runs should not need one, and a panic here would
take down update handling for every chat.

It is built once and wired in both places a QuizService is constructed. The
webhook and long-poll paths each build their own, and wiring only one would
leave that mode frozen on question one with no error to explain why.

The winner sticker is read from the environment and defaults to unset.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
```

---

## Task 9: Qo'lda tekshirish va PR

**Files:** yo'q (tekshiruv)

- [ ] **Step 1: Bot uchun webhook'ni qayta ro'yxatdan o'tkazish kerakligini yozib qo'y**

`allowed_updates` o'zgardi. Ishlab chiqarishda `setWebhook` qayta
chaqirilmasa, `poll_answer` **kelmaydi** va butun guruh hisobi jim ishlamaydi.

Deploy'dan keyin bir marta bajariladigan qadam (PR tavsifiga kiritiladi):

```bash
curl -sS -X POST "https://api.telegram.org/bot<TOKEN>/setWebhook" \
  -H 'Content-Type: application/json' \
  -d '{"url":"<WEBHOOK_URL>","secret_token":"<SECRET>",
       "allowed_updates":["message","callback_query","my_chat_member","poll_answer"]}'
```

Tekshirish:

```bash
curl -sS "https://api.telegram.org/bot<TOKEN>/getWebhookInfo"
```

Javobda `allowed_updates` ichida `poll_answer` bo'lishi shart.

- [ ] **Step 2: Lokal qo'lda sinov (ixtiyoriy, lekin tavsiya etiladi)**

Test bot tokeni bilan long-poll rejimida ishga tushirib, shaxsiy chatda va
test guruhida `/quiz` ni oxirigacha o'yna. Tekshiriladigan ro'yxat:

- [ ] Rasm chiqdi, ostida so'rovnoma unga reply bo'lib turibdi
- [ ] So'rovnomada sanoq ko'rinyapti va 10 sekunddan keyin yopilyapti
- [ ] To'g'ri javobda Telegram konfetti ko'rsatyapti
- [ ] Guruhda ikkinchi odam ham javob bera olyapti (birinchisi bloklamaydi)
- [ ] 10 savoldan keyin reyting chiqdi, hamma ishtirokchi ro'yxatda
- [ ] G'olib to'g'ri (ko'p to'g'ri javob; tenglikda tezrog'i)
- [ ] Guruhda yakuniy xabarga 🎉 reaksiya qo'yildi
- [ ] Shaxsiy chatda salyut effekti ishladi
- [ ] `/stop` o'rtada reytingni chiqardi

- [ ] **Step 3: PR yarat**

```bash
cd "/home/sher/Рабочий стол/avtotest"
git push -u origin feat/m4-07-telegram-group-quiz
gh pr create --title "feat(bot): Telegram guruh quizi — ko'p kishilik, vaqt chegarali" --body "$(cat <<'EOF'
## Nima o'zgardi

`/quiz` bir kishilik savol-javobdan 10 savollik, vaqt chegarali guruh
musobaqasiga aylandi.

Spec: `docs/superpowers/specs/2026-07-29-telegram-group-quiz-design.md`
Reja: `docs/superpowers/plans/2026-07-29-telegram-group-quiz.md`

## Tuzatilgan to'rtta nuqson

1. **Uzun javoblar kesilardi** (`quizButtonMaxRunes = 60`) — endi poll
   varianti 100 belgi, va sig'maydigan savollar umuman tanlanmaydi
2. **Guruhda birinchi bosgan hamma uchun javob berardi** — `cq.From.ID`
   uzatilmasdi; endi `poll_answer` har bir ovoz beruvchini alohida beradi
3. **Kim nechta bilgani saqlanmasdi** — `telegram_quiz_participant` jadvali
4. **Vaqt chegarasi yo'q edi** — poll `open_period` (sozlanadigan, default 10s)

## Migratsiya

`0046_telegram_quiz_multiplayer` — **faqat qo'shimcha**: ikkita yangi jadval,
`telegram_quiz_session` ga uchta DEFAULT'li ustun, `limit_config` ga ikkita
sozlama qatori. Mavjud ma'lumot o'zgarmaydi.

## ⚠️ Deploy'dan keyin majburiy qadam

`allowed_updates` ga `poll_answer` qo'shildi. Webhook qayta ro'yxatdan
o'tkazilmasa ovozlar **kelmaydi**:

\`\`\`bash
curl -sS -X POST "https://api.telegram.org/bot<TOKEN>/setWebhook" \\
  -H 'Content-Type: application/json' \\
  -d '{"url":"<WEBHOOK_URL>","secret_token":"<SECRET>",
       "allowed_updates":["message","callback_query","my_chat_member","poll_answer"]}'
\`\`\`

## Tekshiruv

- \`go build ./...\` ✅
- \`go vet ./...\` ✅
- \`go test ./... -p 1\` ✅
- \`golangci-lint run\` ✅

## Tegilmagan

Ilova ichidagi mashq/bilet/imtihon, to'lov, premium, referral, GRAND MOCK,
umumiy CSS tokenlar.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## Self-review natijasi

**Spec qamrovi** — har bir bo'lim uchun task bor:

| Spec bo'limi | Task |
|---|---|
| §4 Ma'lumotlar modeli (0046) | Task 1 |
| §4 Sozlamalar (`limit_config`) | Task 1 (qator), Task 3 (o'qish) |
| §3 Bot API cheklovlari | Task 2 (majburlash), Task 3 (`buildPollRequest`) |
| §5.1 Boshlanishi | Task 4 (`StartGame`) |
| §5.2 Har bir savol — ikki xabar | Task 4 |
| §5.3 Javob qabul qilish | Task 5 |
| §5.4 Savoldan savolga o'tish | Task 6 (taymer), Task 8 (ishlab chiqarish simi) |
| §5.5 Yakun (guruh + solo) | Task 6 (`rankingText`, `finishGame`) |
| §6 Vizual effektlar | Task 7 |
| §7 Chetki holatlar | Task 5 (noma'lum poll, bo'sh ovoz), Task 6 (hech kim o'ynamadi, `/stop`), Task 4 (savol topilmadi) |
| §8 Testlash | Har bir taskda, avval sindirib |
| D5 korpus filtri | Task 1 (so'rov), Task 3 (test) |

**Aniqlangan va tuzatilgan nomuvofiqliklar:**
- `quizButtonMaxRunes` va `answerMarkup` endi ishlatilmaydi — Task 5 Step 4 da
  o'chiriladi, aks holda `golangci-lint` "unused" beradi
- `sendFinalMessage`/`celebrate` Task 6 da vaqtinchalik, Task 7 da to'liq —
  ikkala taskda ham aniq ko'rsatilgan
- `Advance` maydoni Task 6 da e'lon qilinadi, Task 8 da ulanadi

**Task 8 dagi taxminlar kodga qarshi tekshirildi (2026-07-29):**
- `QuizService` ikkita joyda quriladi — `internal/server/server.go:237`
  (webhook) va `cmd/api/main.go:82` (longpoll). Reja ikkalasini ham qamraydi;
  faqat bittasini ulash o'sha rejimni 1-savolda muzlatadi.
- `config.go` `getenv(key, def)` yordamchisidan foydalanadi (238-qator) —
  reja `os.Getenv` emas, o'shani ishlatadi.
- Ikkala literal ham `Log` maydonini beradi, lekin o'zgaruvchi nomlari
  har xil (`log` va `logger`) — reja har biriga to'g'ri nomni yozadi.
