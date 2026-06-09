package vision

import (
	"errors"
	"fmt"
)

// ErrAPIKeyMissing is returned by New() when Config.APIKey is empty. The
// httpserver wiring uses this signal to leave the vision client nil; the
// meals/from_photo handler then returns 503 vision_unavailable.
var ErrAPIKeyMissing = errors.New("vision: anthropic api key not configured")

// ErrVisionTimeout fires when the upstream call exceeds the configured
// timeout (default 15s). Mapped to 504 vision_timeout.
var ErrVisionTimeout = errors.New("vision: upstream timeout")

// ErrVisionUpstreamError is the catch-all for 5xx from Anthropic. Mapped to
// 504 vision_upstream_error.
var ErrVisionUpstreamError = errors.New("vision: upstream 5xx error")

// ErrVisionResponseUnparseable fires after one retry on missing/invalid
// tool_use. Mapped to 502 vision_response_unparseable.
var ErrVisionResponseUnparseable = errors.New("vision: response unparseable after retry")

// ErrUnsupportedMediaType is returned by Resize for HEIC/unknown formats in
// v1. Mapped to 415 unsupported_media_type by the handler. JPEG and PNG are
// supported; HEIC requires libheif/CGo (out of scope for v1; phones can save
// JPEG via image_picker's `jpegQuality` flag).
var ErrUnsupportedMediaType = errors.New("vision: unsupported media type (v1 accepts jpeg/png only)")

// ErrVisionRateLimited captures the rate-limit signal with the upstream
// Retry-After hint (0 if unset). Mapped to 429 vision_rate_limited with the
// hint echoed in the response body so the client can back off correctly.
type ErrVisionRateLimited struct {
	RetryAfterSeconds int
}

func (e *ErrVisionRateLimited) Error() string {
	return fmt.Sprintf("vision: rate limited (retry after %ds)", e.RetryAfterSeconds)
}

// ErrVisionUnexpectedResponse is the bucket for non-rate-limit 4xx from
// Anthropic (auth, request shape, etc.). The status is surfaced so the
// handler can log it. Mapped to 502 vision_unexpected_response.
type ErrVisionUnexpectedResponse struct {
	StatusCode int
}

func (e *ErrVisionUnexpectedResponse) Error() string {
	return fmt.Sprintf("vision: unexpected response (status %d)", e.StatusCode)
}

// IsVisionError reports whether err is one of the package's documented
// sentinels or typed errors. Useful for handlers that want a single guard
// before mapping; the handler still type-switches to map specific codes.
func IsVisionError(err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, ErrAPIKeyMissing),
		errors.Is(err, ErrVisionTimeout),
		errors.Is(err, ErrVisionUpstreamError),
		errors.Is(err, ErrVisionResponseUnparseable),
		errors.Is(err, ErrUnsupportedMediaType):
		return true
	}
	var rl *ErrVisionRateLimited
	if errors.As(err, &rl) {
		return true
	}
	var ur *ErrVisionUnexpectedResponse
	if errors.As(err, &ur) {
		return true
	}
	return false
}
