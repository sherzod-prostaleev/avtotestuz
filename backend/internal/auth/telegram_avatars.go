package auth

import (
	"context"

	"github.com/google/uuid"
)

// TelegramAvatars is told about every committed change to a profile's
// phone-verified Telegram link, so the learner's Telegram photo follows it
// (avatar.Service). Both methods must return at once and do their work in
// the background: they run on the sign-in path, which never waits on
// Telegram. Call them only after the transaction commits — the service
// re-reads the link from the database.
type TelegramAvatars interface {
	// TelegramLinked: profileID now has a phone-verified link (new, moved
	// here, upgraded or re-pointed); fetch its photo.
	TelegramLinked(profileID uuid.UUID)
	// TelegramUnlinked: profileID may have lost its verified link; drop its
	// photo if so.
	TelegramUnlinked(profileID uuid.UUID)
}

// telegramLinkChange is what linkTelegramInTx changed, held until the
// caller's commit and then handed to Avatars by afterTelegramLink.
type telegramLinkChange struct {
	linked    bool
	profileID uuid.UUID
	movedFrom uuid.UUID // profile the Telegram account was taken off, or Nil
	// webAppUser is the Mini App user that linked (nil for bot-side links,
	// whose chat already registers them with the bot).
	webAppUser *WebAppUser
}

func (s *Service) afterTelegramLink(ctx context.Context, c telegramLinkChange) {
	if !c.linked {
		return
	}
	if c.webAppUser != nil {
		s.joinBotAudience(ctx, *c.webAppUser)
	}
	if s.Avatars == nil {
		return
	}
	if c.movedFrom != uuid.Nil {
		s.Avatars.TelegramUnlinked(c.movedFrom)
	}
	s.Avatars.TelegramLinked(c.profileID)
}
