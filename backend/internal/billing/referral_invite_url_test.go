package billing

import "testing"

// The learner's invite opens the Mini App with the code as start_param;
// without a (valid) bot username the website /r/<CODE> link stays.
func TestReferralInviteURL(t *testing.T) {
	s := Service{PublicBaseURL: "https://drivergo.uz/", TelegramBotUsername: "DriverGouzBot"}
	if got := s.ReferralInviteURL("REF-AB23CD"); got != "https://t.me/DriverGouzBot?startapp=ref_REF-AB23CD" {
		t.Fatalf("got %q", got)
	}
	for _, bot := range []string{"", "bad bot", "@x"} {
		s.TelegramBotUsername = bot
		if got := s.ReferralInviteURL("REF-AB23CD"); got != "https://drivergo.uz/r/REF-AB23CD" {
			t.Fatalf("bot %q: got %q", bot, got)
		}
	}
	// A code outside start_param's charset never goes into a t.me link.
	s.TelegramBotUsername = "DriverGouzBot"
	if got := s.ReferralInviteURL("REF AB"); got != "https://drivergo.uz/r/REF AB" {
		t.Fatalf("got %q", got)
	}
	if got := s.WebReferralInviteURL("REF-AB23CD"); got != "https://drivergo.uz/r/REF-AB23CD" {
		t.Fatalf("web got %q", got)
	}
}
