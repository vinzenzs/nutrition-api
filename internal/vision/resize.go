package vision

import (
	"bytes"
	"image"
	_ "image/jpeg" // register decoders for image.Decode
	"image/jpeg"
	_ "image/png"
	"net/http"

	"golang.org/x/image/draw"
)

// MaxEdgePx is the Claude Vision recommended ceiling on the longer image
// edge. Above this, the model gets no benefit from extra detail and we just
// pay bandwidth + inference latency.
const MaxEdgePx = 1568

// JPEGQuality is the re-encode quality. 85 is the sweet spot in the JPEG
// quality/size curve — visually indistinguishable from 95 at this scale,
// but ~2x smaller.
const JPEGQuality = 85

// Resize decodes raw, downscales it to fit within MaxEdgePx on the longer
// edge while preserving aspect ratio, and re-encodes as JPEG at JPEGQuality.
// Images already within the limit are still re-encoded (canonical wire shape
// for the Anthropic call: always JPEG, always the same encoder).
//
// Returns (resized JPEG bytes, [width, height], error). The error is
// ErrUnsupportedMediaType when the input is HEIC, AVIF, or any other format
// Go's image.Decode doesn't recognise — that lets the HTTP handler return
// 415 cleanly. JPEG and PNG are supported.
func Resize(raw []byte) ([]byte, [2]int, error) {
	// Sniff the format first so we can give a clean error for HEIC/AVIF.
	ct := http.DetectContentType(raw)
	switch ct {
	case "image/jpeg", "image/png":
		// supported
	default:
		return nil, [2]int{}, ErrUnsupportedMediaType
	}

	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		// Decoder rejected it even though sniff said JPEG/PNG — treat as
		// unsupported rather than crashing the request.
		return nil, [2]int{}, ErrUnsupportedMediaType
	}

	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	targetW, targetH := fitWithinMaxEdge(w, h, MaxEdgePx)

	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	// CatmullRom = highest-quality general-purpose Go scaler. Cost is
	// negligible at our size budget (<2MP after resize).
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: JPEGQuality}); err != nil {
		return nil, [2]int{}, err
	}
	return buf.Bytes(), [2]int{targetW, targetH}, nil
}

// fitWithinMaxEdge computes the largest (w', h') that:
//  1. Preserves the source aspect ratio,
//  2. Has max(w', h') <= maxEdge,
//  3. Equals (w, h) when the source already fits.
func fitWithinMaxEdge(w, h, maxEdge int) (int, int) {
	if w <= maxEdge && h <= maxEdge {
		return w, h
	}
	if w >= h {
		ratio := float64(maxEdge) / float64(w)
		return maxEdge, int(float64(h)*ratio + 0.5)
	}
	ratio := float64(maxEdge) / float64(h)
	return int(float64(w)*ratio + 0.5), maxEdge
}
