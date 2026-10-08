package bot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	neturl "net/url"
	"strings"
)

var (
	// ErrPhotoTooLarge: the file is over the caller's byte cap (announced by
	// getFile or found while reading).
	ErrPhotoTooLarge = errors.New("telegram profile photo too large")
	// ErrPhotoNotImage: the file host answered with a non-image Content-Type.
	ErrPhotoNotImage = errors.New("telegram profile photo is not an image")
)

// PhotoSize is one rendition of a Telegram photo.
type PhotoSize struct {
	FileID   string `json:"file_id"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	FileSize int64  `json:"file_size"`
}

type userProfilePhotos struct {
	TotalCount int           `json:"total_count"`
	Photos     [][]PhotoSize `json:"photos"`
}

type telegramFile struct {
	FileID   string `json:"file_id"`
	FilePath string `json:"file_path"`
	FileSize int64  `json:"file_size"`
}

// photoContentTypes is what the file host may label a photo with.
// octet-stream is accepted because a file server may not label it at all; the
// caller decodes the bytes, which is the check that actually counts.
var photoContentTypes = map[string]bool{
	"image/jpeg":               true,
	"image/png":                true,
	"image/webp":               true,
	"application/octet-stream": true,
}

// ProfilePhoto downloads the user's current profile photo in the size
// closest to targetPx, capped at maxBytes. (nil, "", nil) means there is no
// photo the bot may see — none at all, or hidden from bots by the user's
// privacy settings — which is an answer, not a failure.
//
// The download URL embeds the bot token (/file/bot<token>/<path>), so it is
// never put in an error: failures name the step and the HTTP status only.
func (c *Client) ProfilePhoto(ctx context.Context, userID int64, targetPx int, maxBytes int64) ([]byte, string, error) {
	var photos userProfilePhotos
	if err := c.call(ctx, "getUserProfilePhotos", map[string]any{"user_id": userID, "limit": 1}, &photos); err != nil {
		return nil, "", err
	}
	if len(photos.Photos) == 0 || len(photos.Photos[0]) == 0 {
		return nil, "", nil
	}
	size := closestPhotoSize(photos.Photos[0], targetPx)
	if size.FileSize > maxBytes {
		return nil, "", ErrPhotoTooLarge
	}

	var file telegramFile
	if err := c.call(ctx, "getFile", map[string]any{"file_id": size.FileID}, &file); err != nil {
		return nil, "", err
	}
	if file.FileSize > maxBytes {
		return nil, "", ErrPhotoTooLarge
	}
	if !safeFilePath(file.FilePath) {
		return nil, "", errors.New("telegram getFile: unexpected file_path")
	}
	return c.downloadFile(ctx, file.FilePath, maxBytes)
}

func (c *Client) downloadFile(ctx context.Context, filePath string, maxBytes int64) ([]byte, string, error) {
	url := fmt.Sprintf("%s/file/bot%s/%s", c.BaseURL, c.Token, filePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", c.redactToken(err)
	}
	resp, err := c.HC.Do(req)
	if err != nil {
		// *url.Error quotes the whole URL; keep only the cause. The file
		// path is useless without the token, but there is no reason to log
		// a token-shaped URL at all.
		var ue *neturl.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, "", fmt.Errorf("telegram file download: %w", c.redactToken(err))
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("telegram file download: HTTP %d", resp.StatusCode)
	}
	ct := "application/octet-stream"
	if raw := resp.Header.Get("Content-Type"); raw != "" {
		mt, _, err := mime.ParseMediaType(raw)
		if err != nil {
			return nil, "", ErrPhotoNotImage
		}
		ct = strings.ToLower(mt)
	}
	if !photoContentTypes[ct] {
		return nil, "", ErrPhotoNotImage
	}
	if resp.ContentLength > maxBytes {
		return nil, "", ErrPhotoTooLarge
	}
	// One byte past the cap tells "exactly maxBytes" from "more".
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("telegram file download: %w", c.redactToken(err))
	}
	if int64(len(data)) > maxBytes {
		return nil, "", ErrPhotoTooLarge
	}
	return data, ct, nil
}

// closestPhotoSize picks the rendition whose longer side is nearest to
// targetPx, preferring the larger on a tie (it downscales cleanly).
func closestPhotoSize(sizes []PhotoSize, targetPx int) PhotoSize {
	best := sizes[0]
	bestDist := -1
	for _, s := range sizes {
		side := max(s.Width, s.Height)
		dist := side - targetPx
		if dist < 0 {
			dist = -dist
		}
		if bestDist < 0 || dist < bestDist || (dist == bestDist && side > max(best.Width, best.Height)) {
			best, bestDist = s, dist
		}
	}
	return best
}

// safeFilePath accepts the relative paths Telegram hands out
// ("photos/file_7.jpg"). The path is appended to a URL holding the bot
// token, so anything that could climb out of /file/bot<token>/ or change the
// request's meaning is refused rather than escaped.
func safeFilePath(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") || strings.ContainsAny(p, "?#\\%") {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}
