package auth

import "strings"

// DescribeDevice turns a User-Agent into the coarse "Chrome · Android" the bot
// shows in a Telegram login prompt, so the learner can tell their own browser
// from someone else's. It only ever returns words from the fixed lists below
// (or ""), so nothing a client puts in its User-Agent reaches the bot message.
// It is a hint, not proof: an attacker chooses their own User-Agent.
func DescribeDevice(ua string) string {
	browser := browserFamily(ua)
	os := osFamily(ua)
	switch {
	case browser != "" && os != "":
		return browser + " · " + os
	case browser != "":
		return browser
	default:
		return os
	}
}

// Order matters: Chromium forks also say "Chrome", iOS browsers also say
// "Safari", so the specific tokens are checked first.
var browserTokens = []struct{ token, name string }{
	{"Edg/", "Edge"},
	{"EdgA/", "Edge"},
	{"EdgiOS/", "Edge"},
	{"OPR/", "Opera"},
	{"YaBrowser/", "Yandex"},
	{"SamsungBrowser/", "Samsung Internet"},
	{"Firefox/", "Firefox"},
	{"FxiOS/", "Firefox"},
	{"CriOS/", "Chrome"},
	{"Chrome/", "Chrome"},
	{"Safari/", "Safari"},
}

func browserFamily(ua string) string {
	for _, b := range browserTokens {
		if strings.Contains(ua, b.token) {
			if b.name == "Safari" && !strings.Contains(ua, "Version/") {
				return ""
			}
			return b.name
		}
	}
	return ""
}

func osFamily(ua string) string {
	switch {
	case strings.Contains(ua, "iPhone"):
		return "iPhone"
	case strings.Contains(ua, "iPad"):
		return "iPad"
	case strings.Contains(ua, "Android"):
		return "Android"
	case strings.Contains(ua, "Windows"):
		return "Windows"
	case strings.Contains(ua, "CrOS"):
		return "ChromeOS"
	case strings.Contains(ua, "Macintosh"), strings.Contains(ua, "Mac OS X"):
		return "macOS"
	case strings.Contains(ua, "Linux"):
		return "Linux"
	}
	return ""
}
