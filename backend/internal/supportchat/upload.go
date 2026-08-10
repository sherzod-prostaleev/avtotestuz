package supportchat

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const maxUploadBytes = 12 << 20 // 12 MiB

var allowedMIME = map[string]string{
	"image/jpeg":         ".jpg",
	"image/png":          ".png",
	"image/webp":         ".webp",
	"image/gif":          ".gif",
	"application/pdf":    ".pdf",
	"text/plain":         ".txt",
	"application/msword": ".doc",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": ".docx",
	"application/vnd.ms-excel": ".xls",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": ".xlsx",
	"application/zip": ".zip",
}

// UploadedFile is a stored attachment ready to attach to a message.
type UploadedFile struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Mime string `json:"mime"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
}

func (s *Service) storeUpload(r *http.Request, conversationID uuid.UUID) (UploadedFile, error) {
	if s.Blobs == nil {
		return UploadedFile{}, fmt.Errorf("uploads unavailable")
	}
	r.Body = http.MaxBytesReader(nil, r.Body, maxUploadBytes+512)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		return UploadedFile{}, fmt.Errorf("file too large or invalid multipart")
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		return UploadedFile{}, fmt.Errorf("file required")
	}
	defer func() { _ = file.Close() }()

	name := strings.TrimSpace(hdr.Filename)
	if name == "" {
		name = "file"
	}
	if utf8.RuneCountInString(name) > maxAttachName {
		name = string([]rune(name)[:maxAttachName])
	}

	ct := hdr.Header.Get("Content-Type")
	if ct == "" || ct == "application/octet-stream" {
		ext := strings.ToLower(filepath.Ext(name))
		ct = mime.TypeByExtension(ext)
	}
	ct = strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
	ext, ok := allowedMIME[ct]
	if !ok {
		return UploadedFile{}, fmt.Errorf("unsupported file type")
	}

	data, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		return UploadedFile{}, fmt.Errorf("read failed")
	}
	if int64(len(data)) > maxUploadBytes {
		return UploadedFile{}, fmt.Errorf("file too large")
	}
	if len(data) == 0 {
		return UploadedFile{}, fmt.Errorf("empty file")
	}
	if err := validateMagic(ct, data); err != nil {
		return UploadedFile{}, err
	}

	prefix := "support"
	if conversationID != uuid.Nil {
		prefix = "support/" + conversationID.String()
	}
	key := fmt.Sprintf("%s/%s%s", prefix, uuid.NewString(), ext)
	if err := s.Blobs.Put(r.Context(), key, ct, data); err != nil {
		return UploadedFile{}, fmt.Errorf("store failed")
	}
	// URL is filled after the message row exists (authenticated download path).
	return UploadedFile{
		Key:  key,
		Name: name,
		Mime: ct,
		Size: int64(len(data)),
	}, nil
}

// validateMagic rejects MIME spoofing for types with stable signatures.
// Office/zip-like formats share PK headers; text/plain is content-sniffed lightly.
func validateMagic(ct string, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty file")
	}
	switch ct {
	case "image/jpeg":
		if len(data) < 3 || data[0] != 0xFF || data[1] != 0xD8 || data[2] != 0xFF {
			return fmt.Errorf("file content does not match image/jpeg")
		}
	case "image/png":
		sig := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
		if len(data) < len(sig) || string(data[:len(sig)]) != string(sig) {
			return fmt.Errorf("file content does not match image/png")
		}
	case "image/gif":
		if len(data) < 6 || (string(data[:6]) != "GIF87a" && string(data[:6]) != "GIF89a") {
			return fmt.Errorf("file content does not match image/gif")
		}
	case "image/webp":
		if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
			return fmt.Errorf("file content does not match image/webp")
		}
	case "application/pdf":
		if len(data) < 5 || string(data[:5]) != "%PDF-" {
			return fmt.Errorf("file content does not match application/pdf")
		}
	case "application/zip",
		"application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		// OLE (.doc/.xls) or ZIP-based OOXML / zip.
		ole := len(data) >= 8 && data[0] == 0xD0 && data[1] == 0xCF && data[2] == 0x11 && data[3] == 0xE0
		zip := len(data) >= 4 && data[0] == 'P' && data[1] == 'K' && (data[2] == 3 || data[2] == 5 || data[2] == 7) &&
			(data[3] == 4 || data[3] == 6 || data[3] == 8)
		if !ole && !zip {
			return fmt.Errorf("file content does not match office/zip container")
		}
	case "text/plain":
		// Reject obvious binary payloads claimed as text.
		n := len(data)
		if n > 512 {
			n = 512
		}
		for i := 0; i < n; i++ {
			if data[i] == 0 {
				return fmt.Errorf("file content does not match text/plain")
			}
		}
	}
	return nil
}
