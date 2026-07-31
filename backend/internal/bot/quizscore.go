package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"avtotest.uz/backend/internal/db/sqlc"
)

// anonymousVoterUserID is Telegram's @Channel_Bot service account. It stands
// in for the sender whenever a vote is cast on behalf of a chat, so every
// such voter arrives carrying this one id — the real one is in voter_chat.
const anonymousVoterUserID int64 = 136817688

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

// voterIdentity resolves who a vote belongs to. A vote cast on behalf of a
// chat (a channel playing in its own discussion group) reports the shared
// placeholder account, so keying on the user id would fold every such voter
// into one row labelled "Channel". The voting chat's own id is negative and
// cannot collide with a user id, which makes it a safe key.
func voterIdentity(pa PollAnswer) (int64, string) {
	if pa.VoterChat != nil && pa.VoterChat.ID != 0 {
		name := strings.TrimSpace(pa.VoterChat.Title)
		if name == "" {
			name = "Kanal"
		}
		return pa.VoterChat.ID, name
	}
	return pa.User.ID, displayName(pa.User)
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

	elapsed := time.Since(poll.OpenedAt.Time).Milliseconds()
	if elapsed < 0 {
		elapsed = 0
	}

	voterID, name := voterIdentity(pa)
	return s.Q.UpsertQuizParticipant(ctx, sqlc.UpsertQuizParticipantParams{
		SessionID:    poll.SessionID,
		TgUserID:     voterID,
		DisplayName:  name,
		CorrectDelta: correctDelta,
		ElapsedMs:    elapsed,
	})
}

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
