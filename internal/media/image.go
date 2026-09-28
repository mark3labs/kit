// Package media normalises binary media (currently images) so that it can be
// attached to an LLM message without exhausting the context window or
// breaching a provider payload limit.
//
// The entry point is Normalize. It decodes an image, scales it down until it
// fits a pixel and byte budget, and re-encodes it. Encoding may change format
// (a large PNG screenshot usually becomes a JPEG) because a lossless format
// often cannot meet a byte budget at a useful resolution.
package media

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

// Media type constants for the image formats this package understands.
const (
	MediaTypePNG  = "image/png"
	MediaTypeJPEG = "image/jpeg"
	MediaTypeGIF  = "image/gif"
	MediaTypeWebP = "image/webp"
)

// Default limits. These match the behaviour of comparable agents, but the
// maximum edge is lower than a naive 2000 px: providers downscale an image
// whose long edge is above roughly 1568 px anyway, so extra pixels only cost
// bytes, never detail.
const (
	// DefaultMaxEdge is the largest allowed width or height, in pixels.
	DefaultMaxEdge = 1568
	// DefaultMaxEncodedBytes is the largest allowed encoded payload, in
	// bytes, measured before base64 expansion.
	DefaultMaxEncodedBytes = 3_500_000
	// IngestLimitBytes is a hard ceiling on the on-disk size of a file
	// this package will even attempt to decode. It guards against a
	// decompression bomb and against loading a huge file into memory.
	IngestLimitBytes = 20 * 1024 * 1024
	// MaxDecodePixels caps the pixel count of an image this package will
	// decode. The byte limit alone is not enough: a flat image deflates to
	// almost nothing, so a file far below IngestLimitBytes can declare a
	// canvas that needs gigabytes of pixel storage. 50 million pixels is
	// about 200 MB as RGBA, and still holds an 8K screenshot.
	MaxDecodePixels = 50_000_000
)

// maxScaleAttempts bounds the downscale loop. Each attempt multiplies the
// edge by scaleStep, so the loop reaches a single pixel long before it ends.
const (
	maxScaleAttempts = 32
	scaleStep        = 0.75
)

// jpegQualities are tried in order once PNG fails the byte budget. The first
// value is the preferred quality; later values trade detail for size.
var jpegQualities = []int{80, 70, 55, 40}

// Errors returned by this package. Callers should compare with errors.Is.
var (
	// ErrNotImage reports that the data is not a supported image format.
	ErrNotImage = errors.New("not a supported image")
	// ErrTooLarge reports that the file is above the ingest limit and was
	// never decoded.
	ErrTooLarge = errors.New("media exceeds ingestion limit")
	// ErrDecode reports that the data could not be decoded as an image.
	ErrDecode = errors.New("image could not be decoded")
	// ErrIrreducible reports that the image could not be made to fit the
	// configured budget.
	ErrIrreducible = errors.New("image could not be resized below the size limit")
)

// Limits holds the budget applied to an image. A zero field means "use the
// default", so the zero Limits value is the default policy.
type Limits struct {
	// MaxEdge is the largest allowed width or height, in pixels.
	MaxEdge int
	// MaxEncodedBytes is the largest allowed encoded payload, in bytes.
	MaxEncodedBytes int
	// NoResize rejects an oversized image instead of scaling it down.
	NoResize bool
}

func (l Limits) maxEdge() int {
	if l.MaxEdge > 0 {
		return l.MaxEdge
	}
	return DefaultMaxEdge
}

func (l Limits) maxBytes() int {
	if l.MaxEncodedBytes > 0 {
		return l.MaxEncodedBytes
	}
	return DefaultMaxEncodedBytes
}

// Result is a normalised image ready to attach to a message.
type Result struct {
	// Data is the encoded image payload.
	Data []byte
	// MediaType is the MIME type of Data. It can differ from the input
	// media type when re-encoding changed the format.
	MediaType string
	// Width and Height are the pixel dimensions of Data.
	Width, Height int
	// Resized reports whether the image was scaled down.
	Resized bool
	// OriginalWidth and OriginalHeight are the dimensions before scaling.
	OriginalWidth, OriginalHeight int
	// OriginalBytes is the length of the input payload.
	OriginalBytes int
}

// DetectMediaType returns the image media type of data, using the file
// signature first and the file name extension as a fallback. It returns an
// empty string when the data is not a supported image.
func DetectMediaType(data []byte, name string) string {
	switch sniffed := http.DetectContentType(data); sniffed {
	case MediaTypePNG, MediaTypeJPEG, MediaTypeGIF, MediaTypeWebP:
		return sniffed
	}
	// http.DetectContentType does not know every WebP variant, so check the
	// RIFF container directly.
	if len(data) >= 12 && bytes.Equal(data[0:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		return MediaTypeWebP
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		return MediaTypePNG
	case ".jpg", ".jpeg":
		return MediaTypeJPEG
	case ".gif":
		return MediaTypeGIF
	case ".webp":
		return MediaTypeWebP
	}
	return ""
}

// IsImageExt reports whether name has an extension this package can decode.
// Use it for a cheap check before a file is read.
func IsImageExt(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return true
	}
	return false
}

// Normalize decodes data, scales it down if it is above the budget in limits,
// and returns an encoded image that fits. The name is used only to help
// detect the media type and in error messages.
//
// Normalize returns the original bytes untouched when the image already fits,
// so a small image costs one decode and no re-encode.
func Normalize(data []byte, name string, limits Limits) (Result, error) {
	if len(data) > IngestLimitBytes {
		return Result{}, fmt.Errorf("%w: %s is %d bytes, limit is %d",
			ErrTooLarge, name, len(data), IngestLimitBytes)
	}
	mediaType := DetectMediaType(data, name)
	if mediaType == "" {
		return Result{}, fmt.Errorf("%w: %s", ErrNotImage, name)
	}

	// Read the dimensions from the header before any pixel storage is
	// allocated, so that a decompression bomb is rejected cheaply.
	cfg, err := decodeConfig(data, mediaType)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %s: %v", ErrDecode, name, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return Result{}, fmt.Errorf("%w: %s: invalid dimensions %dx%d",
			ErrDecode, name, cfg.Width, cfg.Height)
	}
	// Divide instead of multiply, so the comparison cannot overflow.
	if cfg.Width > MaxDecodePixels/cfg.Height {
		return Result{}, fmt.Errorf("%w: %s is %dx%d pixels, limit is %d pixels",
			ErrTooLarge, name, cfg.Width, cfg.Height, MaxDecodePixels)
	}

	img, err := decode(data, mediaType)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %s: %v", ErrDecode, name, err)
	}

	bounds := img.Bounds()
	origW, origH := bounds.Dx(), bounds.Dy()
	res := Result{
		MediaType:      mediaType,
		Width:          origW,
		Height:         origH,
		OriginalWidth:  origW,
		OriginalHeight: origH,
		OriginalBytes:  len(data),
	}
	if origW == 0 || origH == 0 {
		return Result{}, fmt.Errorf("%w: %s: zero dimension", ErrDecode, name)
	}

	maxEdge, maxBytes := limits.maxEdge(), limits.maxBytes()
	fits := origW <= maxEdge && origH <= maxEdge && len(data) <= maxBytes

	// An animated GIF loses its animation on re-encode, and a static GIF is
	// better delivered as PNG. Either way, only pass it through unchanged
	// when it already fits.
	if fits {
		res.Data = data
		return res, nil
	}
	if limits.NoResize {
		return Result{}, fmt.Errorf("%w: %s is %dx%d, %d bytes; limits are %dx%d, %d bytes",
			ErrIrreducible, name, origW, origH, len(data), maxEdge, maxEdge, maxBytes)
	}

	// Start from the largest size that respects the pixel budget, then step
	// down until the encoded payload respects the byte budget too.
	w, h := fitWithin(origW, origH, maxEdge)
	for range maxScaleAttempts {
		if w < 1 || h < 1 {
			break
		}
		scaled := scale(img, w, h)
		encoded, encType, err := encodeWithin(scaled, maxBytes)
		if err == nil {
			res.Data = encoded
			res.MediaType = encType
			res.Width, res.Height = w, h
			res.Resized = true
			return res, nil
		}
		w = int(float64(w) * scaleStep)
		h = int(float64(h) * scaleStep)
	}
	return Result{}, fmt.Errorf("%w: %s is %dx%d, %d bytes; limits are %dx%d, %d bytes",
		ErrIrreducible, name, origW, origH, len(data), maxEdge, maxEdge, maxBytes)
}

// decode turns encoded image bytes into an image.Image.
func decode(data []byte, mediaType string) (image.Image, error) {
	r := bytes.NewReader(data)
	switch mediaType {
	case MediaTypePNG:
		return png.Decode(r)
	case MediaTypeJPEG:
		return jpeg.Decode(r)
	case MediaTypeGIF:
		return gif.Decode(r)
	case MediaTypeWebP:
		return webp.Decode(r)
	}
	img, _, err := image.Decode(r)
	return img, err
}

// decodeConfig reads the color model and dimensions of encoded image bytes
// without decoding the pixels.
func decodeConfig(data []byte, mediaType string) (image.Config, error) {
	r := bytes.NewReader(data)
	switch mediaType {
	case MediaTypePNG:
		return png.DecodeConfig(r)
	case MediaTypeJPEG:
		return jpeg.DecodeConfig(r)
	case MediaTypeGIF:
		return gif.DecodeConfig(r)
	case MediaTypeWebP:
		return webp.DecodeConfig(r)
	}
	cfg, _, err := image.DecodeConfig(r)
	return cfg, err
}

// fitWithin returns the largest w,h with the aspect ratio of origW,origH such
// that neither edge is above maxEdge. It never scales an image up.
func fitWithin(origW, origH, maxEdge int) (int, int) {
	if origW <= maxEdge && origH <= maxEdge {
		return origW, origH
	}
	if origW >= origH {
		h := int(float64(origH) * float64(maxEdge) / float64(origW))
		return maxEdge, max(h, 1)
	}
	w := int(float64(origW) * float64(maxEdge) / float64(origH))
	return max(w, 1), maxEdge
}

// scale resamples img to w by h using Catmull-Rom, which keeps edges crisp on
// the screenshots and diagrams that dominate this workload.
func scale(img image.Image, w, h int) image.Image {
	if img.Bounds().Dx() == w && img.Bounds().Dy() == h {
		return img
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	return dst
}

// encodeWithin encodes img and returns the first payload that is at or below
// maxBytes. It prefers lossless PNG and falls back to progressively lower
// JPEG quality. It returns an error when no encoding fits.
func encodeWithin(img image.Image, maxBytes int) ([]byte, string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err == nil && buf.Len() <= maxBytes {
		return buf.Bytes(), MediaTypePNG, nil
	}
	opaque := flattenOnWhite(img)
	for _, q := range jpegQualities {
		buf.Reset()
		if err := jpeg.Encode(&buf, opaque, &jpeg.Options{Quality: q}); err != nil {
			continue
		}
		if buf.Len() <= maxBytes {
			out := make([]byte, buf.Len())
			copy(out, buf.Bytes())
			return out, MediaTypeJPEG, nil
		}
	}
	return nil, "", ErrIrreducible
}

// flattenOnWhite composites img over an opaque white canvas. JPEG has no
// alpha channel and its encoder reads premultiplied RGB, so without this a
// transparent pixel encodes as black. An image that is already opaque is
// returned as is, to avoid a copy.
func flattenOnWhite(img image.Image) image.Image {
	if o, ok := img.(interface{ Opaque() bool }); ok && o.Opaque() {
		return img
	}
	b := img.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(dst, b, img, b.Min, draw.Over)
	return dst
}
