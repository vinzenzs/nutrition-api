package mcpserver

import (
	"context"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fromPhotoRecord captures the multipart request the wrapper sends, so the
// tests can assert the form fields and the image part separately.
type fromPhotoRecord struct {
	method      string
	path        string
	contentType string
	idemKey     string
	imageBytes  []byte
	formFields  map[string]string
}

func newFromPhotoRecorder(t *testing.T, status int, respBody string) (*apiClient, *[]fromPhotoRecord) {
	t.Helper()
	var (
		mu      sync.Mutex
		records []fromPhotoRecord
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := fromPhotoRecord{
			method:      r.Method,
			path:        r.URL.Path,
			contentType: r.Header.Get("Content-Type"),
			idemKey:     r.Header.Get("Idempotency-Key"),
			formFields:  map[string]string{},
		}
		// Parse multipart so the tests can assert the inner fields.
		mediaType, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if strings.HasPrefix(mediaType, "multipart/") {
			mr := multipart.NewReader(r.Body, params["boundary"])
			for {
				part, err := mr.NextPart()
				if err != nil {
					break
				}
				data, _ := io.ReadAll(part)
				if part.FormName() == "image" {
					rec.imageBytes = data
				} else {
					rec.formFields[part.FormName()] = string(data)
				}
			}
		}
		mu.Lock()
		records = append(records, rec)
		mu.Unlock()
		w.WriteHeader(status)
		_, _ = io.WriteString(w, respBody)
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c := &apiClient{
		baseURL:   u,
		token:     "t",
		userAgent: "ua",
		http:      &http.Client{Timeout: 5 * time.Second},
	}
	return c, &records
}

func TestLogMealFromPhoto_BuildsMultipartWithBase64DecodedImage(t *testing.T) {
	c, recs := newFromPhotoRecorder(t, 201, `{"meal":{"id":"m1"},"inference":{"model":"x"}}`)
	originalBytes := []byte{0xff, 0xd8, 0xff, 0xe0, 0xde, 0xad, 0xbe, 0xef}
	qty := 250.0
	args := LogMealFromPhotoArgs{
		ImageBase64: base64.StdEncoding.EncodeToString(originalBytes),
		QuantityG:   &qty,
		MealType:    "lunch",
		Note:        "café plate",
	}
	r := handleLogMealFromPhoto(context.Background(), c, args)
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/meals/from_photo", rec.path)
	assert.Contains(t, rec.contentType, "multipart/form-data")
	assert.Equal(t, originalBytes, rec.imageBytes, "image bytes survive the base64 round-trip")
	assert.Equal(t, "250", rec.formFields["quantity_g"])
	assert.Equal(t, "lunch", rec.formFields["meal_type"])
	assert.Equal(t, "café plate", rec.formFields["note"])
	assert.NotEmpty(t, rec.idemKey, "derived idempotency-key is forwarded")
}

func TestLogMealFromPhoto_OmitsUnsetMetadataFields(t *testing.T) {
	c, recs := newFromPhotoRecorder(t, 201, `{}`)
	args := LogMealFromPhotoArgs{
		ImageBase64: base64.StdEncoding.EncodeToString([]byte{0xff}),
	}
	_ = handleLogMealFromPhoto(context.Background(), c, args)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	_, hasQty := rec.formFields["quantity_g"]
	_, hasType := rec.formFields["meal_type"]
	_, hasNote := rec.formFields["note"]
	_, hasLogged := rec.formFields["logged_at"]
	assert.False(t, hasQty)
	assert.False(t, hasType)
	assert.False(t, hasNote)
	assert.False(t, hasLogged)
}

func TestLogMealFromPhoto_ExplicitIdempotencyKeyForwarded(t *testing.T) {
	c, recs := newFromPhotoRecorder(t, 201, `{}`)
	args := LogMealFromPhotoArgs{
		ImageBase64:    base64.StdEncoding.EncodeToString([]byte{0x01}),
		IdempotencyKey: "explicit-photo-key",
	}
	_ = handleLogMealFromPhoto(context.Background(), c, args)
	require.Len(t, *recs, 1)
	assert.Equal(t, "explicit-photo-key", (*recs)[0].idemKey)
}

func TestLogMealFromPhoto_VisionUnavailable503Forwarded(t *testing.T) {
	c, _ := newFromPhotoRecorder(t, 503, `{"error":"vision_unavailable","reason":"ANTHROPIC_API_KEY not configured"}`)
	args := LogMealFromPhotoArgs{
		ImageBase64: base64.StdEncoding.EncodeToString([]byte{0x01}),
	}
	r := handleLogMealFromPhoto(context.Background(), c, args)
	assert.True(t, r.IsError, "503 maps to isError on the tool result")
}

func TestLogMealFromPhoto_RateLimitedForwarded(t *testing.T) {
	c, _ := newFromPhotoRecorder(t, 429,
		`{"error":"vision_rate_limited","retry_after_seconds":42}`)
	args := LogMealFromPhotoArgs{
		ImageBase64: base64.StdEncoding.EncodeToString([]byte{0x02}),
	}
	r := handleLogMealFromPhoto(context.Background(), c, args)
	assert.True(t, r.IsError)
}

func TestLogMealFromPhoto_BadBase64IsErrorResult(t *testing.T) {
	c, _ := newFromPhotoRecorder(t, 201, `{}`)
	args := LogMealFromPhotoArgs{
		ImageBase64: "this is not valid base64 !!!@",
	}
	r := handleLogMealFromPhoto(context.Background(), c, args)
	assert.True(t, r.IsError, "decode failure surfaces as isError before any HTTP call")
}
