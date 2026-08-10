package broadcast

import (
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

const (
	AudienceAllActive = "all_active"
	AudienceVIP       = "vip"
	AudienceNonVIP    = "non_vip"

	ChannelsInapp = "inapp"
	ChannelsBoth  = "both"

	maxTitleLen = 120
	maxBodyLen  = 2000
)

var allowedAudiences = map[string]struct{}{
	AudienceAllActive: {},
	AudienceVIP:       {},
	AudienceNonVIP:    {},
}

// NormalizeChannels maps API labels onto stored CHECK values.
// "webpush" is accepted as an alias of "both" (in-app is always written).
func NormalizeChannels(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", ChannelsInapp:
		return ChannelsInapp, nil
	case ChannelsBoth, "webpush":
		return ChannelsBoth, nil
	default:
		return "", fmt.Errorf("channels must be inapp or both")
	}
}

func NormalizeAudience(raw string) (string, error) {
	a := strings.ToLower(strings.TrimSpace(raw))
	if a == "" {
		a = AudienceAllActive
	}
	if _, ok := allowedAudiences[a]; !ok {
		return "", fmt.Errorf("audience must be all_active, vip, or non_vip")
	}
	return a, nil
}

func SanitizeActionURL(raw string) (string, error) {
	href := strings.TrimSpace(raw)
	if href == "" {
		return "", nil
	}
	if !strings.HasPrefix(href, "/") || strings.HasPrefix(href, "//") || strings.Contains(href, "://") {
		return "", fmt.Errorf("action_url must be a relative path starting with /")
	}
	if strings.ContainsAny(href, " \t\n\r") {
		return "", fmt.Errorf("action_url must not contain whitespace")
	}
	return href, nil
}

func SanitizeImageURL(raw string, allowedHosts []string) (string, error) {
	img := strings.TrimSpace(raw)
	if img == "" {
		return "", nil
	}
	u, err := url.Parse(img)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return "", fmt.Errorf("image_url must be an https URL")
	}
	hosts := make([]string, 0, len(allowedHosts))
	for _, h := range allowedHosts {
		h = strings.ToLower(strings.TrimSpace(h))
		if h != "" {
			hosts = append(hosts, h)
		}
	}
	// Production rule: non-empty image_url requires an explicit host allowlist.
	if len(hosts) == 0 {
		return "", fmt.Errorf("image_url rejected: BROADCAST_IMAGE_HOSTS allowlist is empty")
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range hosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return img, nil
		}
	}
	return "", fmt.Errorf("image_url host is not allowed")
}

func ValidateContent(title, body string) (string, string, error) {
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	if title == "" {
		title = "Driver Go"
	}
	if body == "" {
		return "", "", fmt.Errorf("body is required")
	}
	if utf8.RuneCountInString(title) > maxTitleLen {
		return "", "", fmt.Errorf("title too long")
	}
	if utf8.RuneCountInString(body) > maxBodyLen {
		return "", "", fmt.Errorf("body too long")
	}
	return title, body, nil
}
