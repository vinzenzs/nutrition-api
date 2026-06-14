package cookidoo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Version is baked into the User-Agent header. Bump when the fetch behaviour
// changes observably to upstream operators.
const Version = "0.1.0"

// defaultMaxBytes caps the page body we read. Cookidoo recipe pages are ~100KB;
// 4 MiB is generous headroom while bounding memory on a hostile response.
const defaultMaxBytes = 4 << 20

// Config controls the Cookidoo client's HTTP behaviour.
type Config struct {
	// Timeout per request. Zero means 15 seconds.
	Timeout time.Duration
	// HTTPClient overrides the underlying client. Tests inject a stub here. When
	// nil, a client is built with Timeout and a redirect guard that refuses to
	// follow redirects off a Cookidoo host (SSRF defence).
	HTTPClient *http.Client
}

// Client fetches and parses Cookidoo recipe pages.
type Client struct {
	userAgent  string
	httpClient *http.Client
}

// New returns a configured Cookidoo client.
func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{
			Timeout:       timeout,
			CheckRedirect: refuseOffHostRedirect,
		}
	}
	return &Client{
		userAgent:  fmt.Sprintf("nutrition-cookidoo/%s (+https://github.com/vinzenzs/kazper)", Version),
		httpClient: hc,
	}
}

// refuseOffHostRedirect rejects any redirect whose target is not a Cookidoo
// host, so a recipe URL cannot bounce the fetch to an arbitrary internal
// address. Also caps the redirect chain length.
func refuseOffHostRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("too many redirects")
	}
	if !hostPattern.MatchString(req.URL.Hostname()) {
		return fmt.Errorf("refusing redirect to non-cookidoo host %q", req.URL.Hostname())
	}
	return nil
}

// Fetch validates the URL, fetches the page, and parses its Recipe JSON-LD.
// Errors: ErrNotCookidooURL (before any network call), *ErrFetchFailed
// (transport or non-2xx), ErrNoRecipeJSONLD (page had no parseable recipe).
func (c *Client) Fetch(ctx context.Context, rawURL string) (*Recipe, error) {
	if err := ValidateRecipeURL(rawURL); err != nil {
		return nil, err
	}
	return c.fetchAndParse(ctx, rawURL)
}

// fetchAndParse performs the GET and parse without URL validation. Split out so
// package tests can exercise fetch/parse/status handling against an httptest
// server whose host wouldn't pass the recipe-URL allowlist.
func (c *Client) fetchAndParse(ctx context.Context, rawURL string) (*Recipe, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, &ErrFetchFailed{Err: fmt.Errorf("build request: %w", err)}
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/html")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &ErrFetchFailed{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &ErrFetchFailed{StatusCode: resp.StatusCode}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, defaultMaxBytes))
	if err != nil {
		return nil, &ErrFetchFailed{Err: fmt.Errorf("read body: %w", err)}
	}
	return ParseRecipe(body)
}
