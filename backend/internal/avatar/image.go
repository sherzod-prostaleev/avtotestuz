package avatar

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png" // registers the PNG decoder for image.Decode
	"net/http"
)

// OutputPx is the stored avatar's side. The largest spot that shows it is
// 44 CSS px, so 256 covers a 3x phone screen with room to spare.
const OutputPx = 256

// maxSourceSide bounds the decoded source. Telegram's renditions are at most
// 640 (the 320 one is asked for); a 2 MiB file announcing a far larger canvas
// is a decompression bomb, refused from its header before any pixel is read.
const maxSourceSide = 2048

// ErrUnusablePhoto: the file is not a photo this server can decode safely.
var ErrUnusablePhoto = errors.New("unusable profile photo")

// errUndecodableImage: a genuine image in a format this server cannot read
// (WebP). Unlike ErrUnusablePhoto it is not evidence that the learner has no
// usable photo, so the caller keeps whatever avatar is stored.
var errUndecodableImage = fmt.Errorf("%w: image format not decodable here", ErrUnusablePhoto)

// normalize turns a downloaded profile photo into the stored form: a
// centre-cropped, OutputPx square JPEG. Decoding and re-encoding (rather than
// storing the bytes as received) is the point — only pixels survive, so EXIF
// GPS, camera data and any polyglot payload riding in the file are gone, and
// what /media serves is always a plain JPEG.
//
// Only JPEG and PNG are decoded: WebP needs golang.org/x/image, which is not
// a dependency. Telegram serves profile photos as JPEG.
func normalize(data []byte, contentType string) ([]byte, error) {
	switch contentType {
	case "image/jpeg", "image/png", "image/webp", "application/octet-stream":
	default:
		return nil, fmt.Errorf("%w: content type %q", ErrUnusablePhoto, contentType)
	}
	// The label is the file host's claim; the bytes decide.
	switch http.DetectContentType(data) {
	case "image/jpeg", "image/png":
	case "image/webp":
		return nil, errUndecodableImage
	default:
		return nil, fmt.Errorf("%w: not a jpeg or png", ErrUnusablePhoto)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnusablePhoto, err)
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxSourceSide || cfg.Height > maxSourceSide {
		return nil, fmt.Errorf("%w: %dx%d", ErrUnusablePhoto, cfg.Width, cfg.Height)
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnusablePhoto, err)
	}

	// Centre square, flattened onto white: JPEG has no alpha, and a
	// transparent PNG would otherwise come out black.
	b := src.Bounds()
	side := min(b.Dx(), b.Dy())
	origin := image.Pt(b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2)
	square := image.NewRGBA(image.Rect(0, 0, side, side))
	draw.Draw(square, square.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(square, square.Bounds(), src, origin, draw.Over)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, scaleBox(square, OutputPx), &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// scaleBox resizes a square RGBA image to n×n by averaging the source pixels
// each output pixel covers (nearest pixel when enlarging). Plenty for a
// face-sized thumbnail, and needs nothing outside the standard library.
func scaleBox(src *image.RGBA, n int) *image.RGBA {
	side := src.Bounds().Dx()
	dst := image.NewRGBA(image.Rect(0, 0, n, n))
	span := func(i int) (int, int) {
		lo := i * side / n
		hi := max((i+1)*side/n, lo+1)
		return lo, hi
	}
	for dy := range n {
		y0, y1 := span(dy)
		for dx := range n {
			x0, x1 := span(dx)
			var r, g, bl, count int
			for y := y0; y < y1; y++ {
				row := src.Pix[y*src.Stride:]
				for x := x0; x < x1; x++ {
					p := row[x*4 : x*4+4]
					r += int(p[0])
					g += int(p[1])
					bl += int(p[2])
					count++
				}
			}
			o := dst.PixOffset(dx, dy)
			dst.Pix[o] = uint8(r / count)
			dst.Pix[o+1] = uint8(g / count)
			dst.Pix[o+2] = uint8(bl / count)
			dst.Pix[o+3] = 0xff
		}
	}
	return dst
}
