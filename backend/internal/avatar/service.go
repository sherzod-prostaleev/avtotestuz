// Package avatar keeps a learner's Telegram profile photo as their avatar.
//
// Only a phone-verified Telegram link earns a photo (see migration 0077): the
// legacy /start <token> link can bind a stranger's Telegram to a profile.
// All Telegram work runs in the background, after the caller's transaction
// has committed — sign-in, linking and GET /me never wait on Telegram, and a
// Telegram failure only ever means "the initial letter stays".
package avatar

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"avtotest.uz/backend/internal/bot"
	"avtotest.uz/backend/internal/db/sqlc"
)

const (
	// KeyPrefix sits inside images/, the only prefix of the media bucket
	// MinIO serves anonymously (deploy/docker-compose.prod.yml).
	KeyPrefix = "images/avatars/"
	// MaxDownloadBytes caps what is read from Telegram.
	MaxDownloadBytes = 2 << 20
	// RefreshAfter: a stored photo (or a "no photo" answer) is re-checked
	// at most this often, lazily, when the learner loads GET /me.
	RefreshAfter = 7 * 24 * time.Hour
	// targetPx: the rendition asked for (Telegram has 160/320/640).
	targetPx = 320
	// retryAfter keeps a failing or just-started refresh from being asked
	// again on every page load.
	retryAfter = time.Hour
	// jobTimeout bounds one profile's whole fetch-and-store.
	jobTimeout = 30 * time.Second
	// maxParallel caps concurrent Telegram fetches (Telegram rate-limits
	// bots, and the first deploy has every verified learner stale at once).
	maxParallel = 4
	// learnerKind is the only profile kind with an avatar; kiosk stations
	// ('station') never get one and never cause a Telegram call.
	learnerKind = "user"
)

// PhotoSource fetches a Telegram user's current profile photo. (nil, "", nil)
// means there is no photo the bot may see.
type PhotoSource interface {
	ProfilePhoto(ctx context.Context, userID int64, targetPx int, maxBytes int64) ([]byte, string, error)
}

// ObjectStore is the part of the media bucket this package writes.
type ObjectStore interface {
	Put(ctx context.Context, key, contentType string, data []byte) error
	Delete(ctx context.Context, key string) error
}

type jobKind int

const (
	jobNone      jobKind = iota
	jobReconcile         // drop the photo if the verified link is gone
	jobFetch             // (re)fetch from Telegram if the link is verified, else drop
)

type Service struct {
	q            *sqlc.Queries
	store        ObjectStore
	photos       PhotoSource // nil: Telegram not configured, never fetch
	mediaBaseURL string
	log          *zap.Logger
	now          func() time.Time

	mu       sync.Mutex
	pending  map[uuid.UUID]jobKind   // queued or running profiles
	notUntil map[uuid.UUID]time.Time // no lazy refresh before this
	sem      chan struct{}
	wg       sync.WaitGroup
}

// New returns a Service; photos may be nil (no bot token), in which case
// photos are only ever cleared.
func New(q *sqlc.Queries, store ObjectStore, photos PhotoSource, mediaBaseURL string, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	return &Service{
		q:            q,
		store:        store,
		photos:       photos,
		mediaBaseURL: strings.TrimRight(mediaBaseURL, "/"),
		log:          log,
		now:          time.Now,
		pending:      map[uuid.UUID]jobKind{},
		notUntil:     map[uuid.UUID]time.Time{},
		sem:          make(chan struct{}, maxParallel),
	}
}

// TelegramLinked: profileID just got a phone-verified link (new, moved here,
// or re-pointed). Fetches the photo in the background. Call after commit.
func (s *Service) TelegramLinked(profileID uuid.UUID) {
	if s == nil {
		return
	}
	s.enqueue(profileID, jobFetch)
}

// TelegramUnlinked: profileID may have lost its verified link. Drops the
// photo in the background if it has. Call after commit.
func (s *Service) TelegramUnlinked(profileID uuid.UUID) {
	if s == nil {
		return
	}
	s.enqueue(profileID, jobReconcile)
}

// AvatarURL is the public URL of the learner's photo, or "" for none. It
// never fails the caller (GET /me): a database error just means no photo
// this time. Side effects are background only — a missing or week-old
// photo is refreshed, and a photo whose verified link is gone is removed.
func (s *Service) AvatarURL(ctx context.Context, profileID uuid.UUID, kind string) string {
	if s == nil || kind != learnerKind {
		return ""
	}
	st, err := s.q.GetProfileAvatarState(ctx, profileID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) && ctx.Err() == nil {
			s.log.Warn("avatar.state_query_failed", zap.String("profile_id", profileID.String()), zap.Error(err))
		}
		return ""
	}
	if st.Kind != learnerKind || !st.VerifiedTgUserID.Valid {
		if st.AvatarKey.Valid || st.AvatarUpdatedAt.Valid {
			s.enqueue(profileID, jobReconcile)
		}
		return ""
	}
	if !st.AvatarUpdatedAt.Valid || s.now().Sub(st.AvatarUpdatedAt.Time) >= RefreshAfter {
		s.lazyRefresh(profileID)
	}
	if !st.AvatarKey.Valid || !validKey(st.AvatarKey.String) {
		return ""
	}
	return s.mediaBaseURL + "/" + st.AvatarKey.String
}

// Wait blocks until every queued job has finished (tests, shutdown).
func (s *Service) Wait() {
	if s == nil {
		return
	}
	s.wg.Wait()
}

func (s *Service) lazyRefresh(profileID uuid.UUID) {
	now := s.now()
	s.mu.Lock()
	if now.Before(s.notUntil[profileID]) {
		s.mu.Unlock()
		return
	}
	if len(s.notUntil) > 4096 {
		for id, t := range s.notUntil {
			if now.After(t) {
				delete(s.notUntil, id)
			}
		}
	}
	// Set before the fetch, not after: concurrent page loads that read the
	// same stale row must not each start one.
	s.notUntil[profileID] = now.Add(retryAfter)
	s.mu.Unlock()
	s.enqueue(profileID, jobFetch)
}

// enqueue coalesces work per profile: one goroutine per profile at a time,
// and a request that arrives while it runs is folded into one more pass
// (fetch wins over reconcile). Each pass re-reads the database, so the last
// pass always acts on the latest link state.
func (s *Service) enqueue(profileID uuid.UUID, kind jobKind) {
	s.mu.Lock()
	if cur, ok := s.pending[profileID]; ok {
		if kind > cur {
			s.pending[profileID] = kind
		}
		s.mu.Unlock()
		return
	}
	s.pending[profileID] = kind
	s.wg.Add(1)
	s.mu.Unlock()
	go s.run(profileID)
}

func (s *Service) run(profileID uuid.UUID) {
	defer s.wg.Done()
	for {
		s.mu.Lock()
		kind := s.pending[profileID]
		if kind == jobNone {
			delete(s.pending, profileID)
			s.mu.Unlock()
			return
		}
		s.pending[profileID] = jobNone
		s.mu.Unlock()

		s.sem <- struct{}{}
		ctx, cancel := context.WithTimeout(context.Background(), jobTimeout)
		s.sync(ctx, profileID, kind)
		cancel()
		<-s.sem
	}
}

func (s *Service) sync(ctx context.Context, profileID uuid.UUID, kind jobKind) {
	st, err := s.q.GetProfileAvatarState(ctx, profileID)
	if errors.Is(err, pgx.ErrNoRows) {
		return
	}
	if err != nil {
		s.log.Warn("avatar.state_query_failed", zap.String("profile_id", profileID.String()), zap.Error(err))
		return
	}
	if st.Kind != learnerKind || !st.VerifiedTgUserID.Valid {
		s.clear(ctx, profileID)
		return
	}
	if kind != jobFetch || s.photos == nil {
		return
	}
	s.fetch(ctx, profileID, st.VerifiedTgUserID.Int64)
}

func (s *Service) clear(ctx context.Context, profileID uuid.UUID) {
	prev, err := s.q.ClearProfileAvatarUnlessVerified(ctx, profileID)
	if errors.Is(err, pgx.ErrNoRows) {
		return // nothing stored, or the link is verified again
	}
	if err != nil {
		s.log.Warn("avatar.clear_failed", zap.String("profile_id", profileID.String()), zap.Error(err))
		return
	}
	s.deleteObject(ctx, prev)
}

func (s *Service) fetch(ctx context.Context, profileID uuid.UUID, tgUserID int64) {
	data, contentType, err := s.photos.ProfilePhoto(ctx, tgUserID, targetPx, MaxDownloadBytes)
	switch {
	case errors.Is(err, bot.ErrPhotoTooLarge) || errors.Is(err, bot.ErrPhotoNotImage):
		s.log.Info("avatar.photo_rejected", zap.String("profile_id", profileID.String()), zap.Error(err))
		s.record(ctx, profileID, tgUserID, "")
		return
	case err != nil:
		// Transient or ours to fix (token, network): keep what the learner
		// has; lazyRefresh's back-off stops a retry on every page load.
		s.backOff(profileID)
		s.log.Warn("avatar.telegram_fetch_failed", zap.String("profile_id", profileID.String()), zap.Error(err))
		return
	case data == nil:
		// No photo, or hidden from bots: back to the initial letter.
		s.record(ctx, profileID, tgUserID, "")
		return
	}

	jpg, err := normalize(data, contentType)
	if err != nil {
		s.log.Info("avatar.photo_rejected", zap.String("profile_id", profileID.String()), zap.Error(err))
		s.record(ctx, profileID, tgUserID, "")
		return
	}
	key, err := newKey()
	if err != nil {
		s.log.Warn("avatar.key_failed", zap.Error(err))
		return
	}
	if err := s.store.Put(ctx, key, "image/jpeg", jpg); err != nil {
		s.backOff(profileID)
		s.log.Warn("avatar.store_failed", zap.String("profile_id", profileID.String()), zap.Error(err))
		return
	}
	if !s.record(ctx, profileID, tgUserID, key) {
		s.deleteObject(ctx, pgtype.Text{String: key, Valid: true})
	}
}

// record writes a finished check (key "" = no usable photo) and deletes the
// photo it replaced. False when nothing was written — the link changed while
// fetching, or the write failed — so the caller drops its fresh upload.
func (s *Service) record(ctx context.Context, profileID uuid.UUID, tgUserID int64, key string) bool {
	prev, err := s.q.SetProfileTelegramAvatar(ctx, sqlc.SetProfileTelegramAvatarParams{
		ProfileID: profileID,
		TgUserID:  tgUserID,
		AvatarKey: pgtype.Text{String: key, Valid: key != ""},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}
	if err != nil {
		s.log.Warn("avatar.record_failed", zap.String("profile_id", profileID.String()), zap.Error(err))
		return false
	}
	if prev.String != key {
		s.deleteObject(ctx, prev)
	}
	return true
}

func (s *Service) backOff(profileID uuid.UUID) {
	s.mu.Lock()
	s.notUntil[profileID] = s.now().Add(retryAfter)
	s.mu.Unlock()
}

// deleteObject is best-effort: a leftover object is unreferenced and its key
// unguessable. Only keys this package minted are ever deleted, whatever the
// column holds.
func (s *Service) deleteObject(ctx context.Context, key pgtype.Text) {
	if !key.Valid || !validKey(key.String) {
		return
	}
	if err := s.store.Delete(ctx, key.String); err != nil {
		s.log.Warn("avatar.delete_failed", zap.Error(err))
	}
}

var keyEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// newKey is 128 random bits: an avatar URL is public, so it must be
// unguessable and must not reveal (or be derivable from) whose it is.
func newKey() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return KeyPrefix + strings.ToLower(keyEncoding.EncodeToString(b[:])) + ".jpg", nil
}

// validKey accepts exactly what newKey produces.
func validKey(key string) bool {
	name, ok := strings.CutPrefix(key, KeyPrefix)
	if !ok {
		return false
	}
	name, ok = strings.CutSuffix(name, ".jpg")
	if !ok || len(name) != 26 {
		return false
	}
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < '2' || r > '7') {
			return false
		}
	}
	return true
}
