package avatar

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func encodePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func encodeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestNormalizeMakesSquareJPEG(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
		ct   string
	}{
		{"landscape png", encodePNG(t, 640, 480), "image/png"},
		{"small jpeg upscales", encodeJPEG(t, 160, 160), "image/jpeg"},
		{"unlabelled jpeg", encodeJPEG(t, 320, 400), "application/octet-stream"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := normalize(tc.data, tc.ct)
			if err != nil {
				t.Fatalf("normalize: %v", err)
			}
			cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
			if err != nil {
				t.Fatal(err)
			}
			if format != "jpeg" || cfg.Width != OutputPx || cfg.Height != OutputPx {
				t.Fatalf("got %s %dx%d, want jpeg %dx%d", format, cfg.Width, cfg.Height, OutputPx, OutputPx)
			}
		})
	}
}

// Re-encoding is what strips metadata: an APP1/Exif segment in the source
// (GPS, camera) must not survive into the stored file.
func TestNormalizeDropsMetadata(t *testing.T) {
	src := encodeJPEG(t, 300, 300)
	exif := append([]byte{0xFF, 0xE1, 0x00, 0x10}, []byte("Exif\x00\x00GPSDATA!!")...)
	withExif := append(append(append([]byte{}, src[:2]...), exif...), src[2:]...)
	if _, err := jpeg.Decode(bytes.NewReader(withExif)); err != nil {
		t.Fatalf("fixture is not a valid jpeg: %v", err)
	}
	out, err := normalize(withExif, "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("Exif")) || bytes.Contains(out, []byte("GPSDATA")) {
		t.Fatal("metadata survived re-encoding")
	}
}

func TestNormalizeRejects(t *testing.T) {
	// A PNG header claiming 20000x20000 would need 1.6 GB to decode.
	bomb := encodePNG(t, 1, 1)
	bomb[16], bomb[17], bomb[18], bomb[19] = 0, 0, 0x4E, 0x20 // width 20000
	bomb[20], bomb[21], bomb[22], bomb[23] = 0, 0, 0x4E, 0x20 // height 20000
	binary.BigEndian.PutUint32(bomb[29:33], crc32.ChecksumIEEE(bomb[12:29]))
	if cfg, err := png.DecodeConfig(bytes.NewReader(bomb)); err != nil || cfg.Width != 20000 {
		t.Fatalf("bomb fixture header unreadable: %v %+v", err, cfg)
	}
	for _, tc := range []struct {
		name string
		data []byte
		ct   string
	}{
		{"html labelled jpeg", []byte("<!doctype html><html></html>"), "image/jpeg"},
		{"gif", []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"), "image/gif"},
		{"webp", append([]byte("RIFF\x24\x00\x00\x00WEBPVP8 "), make([]byte, 24)...), "image/webp"},
		{"truncated jpeg", encodeJPEG(t, 64, 64)[:40], "image/jpeg"},
		{"dimension bomb", bomb, "image/png"},
		{"empty", nil, "image/jpeg"},
		{"jpeg bytes labelled text", encodeJPEG(t, 64, 64), "text/plain"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := normalize(tc.data, tc.ct); !errors.Is(err, ErrUnusablePhoto) {
				t.Fatalf("err=%v, want ErrUnusablePhoto", err)
			}
		})
	}
}
