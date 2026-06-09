package vision_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/vision"
)

// makeSolidJPEG returns an in-memory JPEG of the supplied dimensions, filled
// with a single colour. Useful for asserting resize behaviour without
// shipping binary fixtures.
func makeSolidJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 150, B: 100, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}))
	return buf.Bytes()
}

func makeSolidPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 80, G: 80, B: 80, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func decodedSize(t *testing.T, raw []byte) (int, int) {
	t.Helper()
	img, _, err := image.Decode(bytes.NewReader(raw))
	require.NoError(t, err)
	b := img.Bounds()
	return b.Dx(), b.Dy()
}

func TestResize_LargeImageScaledToMaxEdge(t *testing.T) {
	raw := makeSolidJPEG(t, 3000, 2000)
	out, dims, err := vision.Resize(raw)
	require.NoError(t, err)
	assert.Equal(t, vision.MaxEdgePx, dims[0], "longer edge clamped to MaxEdgePx")
	// Aspect ratio preserved: 3000:2000 = 3:2 → 1568 wide → 1045 tall.
	assert.Equal(t, 1045, dims[1])
	w, h := decodedSize(t, out)
	assert.Equal(t, 1568, w)
	assert.Equal(t, 1045, h)
}

func TestResize_PortraitOrientationHandled(t *testing.T) {
	raw := makeSolidJPEG(t, 1200, 3000) // tall portrait
	_, dims, err := vision.Resize(raw)
	require.NoError(t, err)
	assert.Equal(t, vision.MaxEdgePx, dims[1], "longer edge (height) clamped")
	assert.Equal(t, 627, dims[0]) // 1200 * 1568 / 3000 = 627.2 → 627
}

func TestResize_AlreadyUnderMaxIsNotUpscaled(t *testing.T) {
	raw := makeSolidJPEG(t, 800, 600)
	_, dims, err := vision.Resize(raw)
	require.NoError(t, err)
	assert.Equal(t, [2]int{800, 600}, dims)
}

func TestResize_PNGConvertedToJPEG(t *testing.T) {
	raw := makeSolidPNG(t, 1000, 1000)
	out, _, err := vision.Resize(raw)
	require.NoError(t, err)
	// Output content-type sniffs as JPEG even though input was PNG.
	assert.True(t, bytes.HasPrefix(out, []byte{0xFF, 0xD8, 0xFF}), "JPEG magic bytes")
}

func TestResize_HEICReturnsUnsupportedMediaType(t *testing.T) {
	// HEIC magic bytes are an ftyp box with a heic brand. http.DetectContentType
	// returns "application/octet-stream" for HEIC; we still want the typed
	// sentinel back so the handler can map to 415.
	heicHeader := []byte{
		0x00, 0x00, 0x00, 0x18, // box size
		'f', 't', 'y', 'p',
		'h', 'e', 'i', 'c',
		0x00, 0x00, 0x00, 0x00,
		'h', 'e', 'i', 'c',
		'm', 'i', 'f', '1',
	}
	_, _, err := vision.Resize(heicHeader)
	assert.ErrorIs(t, err, vision.ErrUnsupportedMediaType)
}

func TestResize_GibberishReturnsUnsupportedMediaType(t *testing.T) {
	_, _, err := vision.Resize([]byte("this is not an image"))
	assert.ErrorIs(t, err, vision.ErrUnsupportedMediaType)
}
