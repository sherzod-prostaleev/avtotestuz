package importer

import "testing"

func TestImageMimeDetectsByContent(t *testing.T) {
	cases := []struct {
		name     string
		data     []byte
		rel      string
		wantMime string
		wantExt  string
	}{
		// The bug this guards: source files saved with a wrong extension.
		{"gif bytes named png", []byte("GIF89a\x00\x00"), "images/1.1.png", "image/gif", ".gif"},
		{"jpeg bytes named png", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00}, "images/4.6.1.png", "image/jpeg", ".jpg"},
		{"png bytes named png", []byte{0x89, 'P', 'N', 'G', 0x0D}, "images/1.32.png", "image/png", ".png"},
		{"svg text named png", []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), "images/x.png", "image/svg+xml", ".svg"},
		{"svg with xml prolog", []byte(`<?xml version="1.0"?><svg></svg>`), "images/x.svg", "image/svg+xml", ".svg"},
		{"webp bytes", append([]byte("RIFF\x00\x00\x00\x00WEBP"), 0x00), "images/x.png", "image/webp", ".webp"},
		// Inconclusive content falls back to the extension.
		{"unknown falls back to jpg ext", []byte{0x00, 0x01, 0x02, 0x03}, "images/x.jpeg", "image/jpeg", ".jpg"},
		{"unknown falls back to png default", []byte{0x00, 0x01, 0x02, 0x03}, "images/x.bin", "image/png", ".png"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mime, ext := imageMime(c.data, c.rel)
			if mime != c.wantMime || ext != c.wantExt {
				t.Errorf("imageMime(%q) = %q,%q; want %q,%q", c.rel, mime, ext, c.wantMime, c.wantExt)
			}
		})
	}
}
