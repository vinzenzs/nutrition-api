package mcpserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Version is baked into the User-Agent header. Exported so the version
// subcommand can surface it.
const Version = "0.1.0"

// apiClient is a thin HTTP client targeting the REST API. Each method
// returns the response status, body bytes, and a transport-level error
// (non-nil only if the call could not complete an HTTP exchange).
type apiClient struct {
	baseURL   *url.URL
	token     string
	userAgent string
	http      *http.Client
}

func newAPIClient(baseURL *url.URL, token string, requestTimeout time.Duration) *apiClient {
	return &apiClient{
		baseURL:   baseURL,
		token:     token,
		userAgent: "nutrition-mcp/" + Version,
		http:      &http.Client{Timeout: requestTimeout},
	}
}

// Get executes GET path with the given query params.
func (c *apiClient) Get(ctx context.Context, path string, query url.Values) (int, []byte, error) {
	return c.do(ctx, http.MethodGet, path, query, nil, "")
}

// Post executes POST path with a JSON body. idempotencyKey is forwarded if non-empty.
func (c *apiClient) Post(ctx context.Context, path string, query url.Values, body []byte, idempotencyKey string) (int, []byte, error) {
	return c.do(ctx, http.MethodPost, path, query, body, idempotencyKey)
}

// Patch executes PATCH path with a JSON body.
func (c *apiClient) Patch(ctx context.Context, path string, body []byte, idempotencyKey string) (int, []byte, error) {
	return c.do(ctx, http.MethodPatch, path, nil, body, idempotencyKey)
}

// Put executes PUT path with a JSON body.
func (c *apiClient) Put(ctx context.Context, path string, body []byte, idempotencyKey string) (int, []byte, error) {
	return c.do(ctx, http.MethodPut, path, nil, body, idempotencyKey)
}

// Delete executes DELETE path.
func (c *apiClient) Delete(ctx context.Context, path string, idempotencyKey string) (int, []byte, error) {
	return c.do(ctx, http.MethodDelete, path, nil, nil, idempotencyKey)
}

// Healthz hits /healthz with a short timeout and returns nil on 200.
func (c *apiClient) Healthz(ctx context.Context) error {
	hc := &http.Client{Timeout: 2 * time.Second}
	endpoint := *c.baseURL
	endpoint.Path = "/healthz"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz: %d", resp.StatusCode)
	}
	return nil
}

func (c *apiClient) do(ctx context.Context, method, path string, query url.Values, body []byte, idempotencyKey string) (int, []byte, error) {
	endpoint := *c.baseURL
	endpoint.Path = path
	if len(query) > 0 {
		endpoint.RawQuery = query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bodyReader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, transportErr(err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, transportErr(err)
	}
	return resp.StatusCode, respBody, nil
}

// transportErr wraps a low-level HTTP error so callers can distinguish
// transport failures from valid-HTTP-with-non-2xx-status responses.
type transportError struct{ inner error }

func (e *transportError) Error() string { return e.inner.Error() }
func (e *transportError) Unwrap() error { return e.inner }

func transportErr(err error) error {
	var t *transportError
	if errors.As(err, &t) {
		return err
	}
	return &transportError{inner: err}
}

// IsTransportError reports whether err originates from the HTTP transport
// (DNS failure, connection refused, timeout) rather than a non-2xx response.
func IsTransportError(err error) bool {
	if err == nil {
		return false
	}
	var t *transportError
	return errors.As(err, &t)
}
