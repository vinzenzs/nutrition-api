package off

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// Version is baked into the User-Agent header. Bump when the API changes
// observably to upstream operators.
const Version = "0.1.0"

// Config controls the OFF client's HTTP behavior.
type Config struct {
	// BaseURL defaults to https://world.openfoodfacts.org.
	BaseURL string
	// Timeout per request. Zero means 5 seconds.
	Timeout time.Duration
	// Contact is interpolated into the User-Agent (e.g. "+https://..." or "+email@...").
	Contact string
	// HTTPClient overrides the underlying client. Tests inject a stub here.
	HTTPClient *http.Client
}

// Client fetches products from Open Food Facts.
type Client struct {
	baseURL    *url.URL
	userAgent  string
	httpClient *http.Client
	logger     *slog.Logger
}

// New returns a configured OFF client. Logger may be nil; a discard logger is
// substituted in that case.
func New(cfg Config, logger *slog.Logger) (*Client, error) {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	base := cfg.BaseURL
	if base == "" {
		base = "https://world.openfoodfacts.org"
	}
	u, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("parse off base url: %w", err)
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: timeout}
	}
	contact := cfg.Contact
	if contact == "" {
		contact = "+https://github.com/vinzenzs/kazper"
	}
	return &Client{
		baseURL:    u,
		userAgent:  fmt.Sprintf("kazper/%s (%s)", Version, contact),
		httpClient: hc,
		logger:     logger,
	}, nil
}

// Fetch retrieves and parses the product for the given barcode. Possible errors:
// ErrProductNotFound, ErrUpstreamTimeout, ErrUpstreamServerError,
// *UnexpectedStatusError.
func (c *Client) Fetch(ctx context.Context, barcode string) (*Product, error) {
	endpoint := *c.baseURL
	endpoint.Path = fmt.Sprintf("/api/v2/product/%s.json", url.PathEscape(barcode))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build off request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || isTimeoutError(err) {
			return nil, ErrUpstreamTimeout
		}
		return nil, fmt.Errorf("off http call: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read off body: %w", err)
	}

	switch {
	case resp.StatusCode >= 500:
		return nil, ErrUpstreamServerError
	case resp.StatusCode == http.StatusNotFound:
		// OFF returns 404 for unknown barcodes via {status:0}, but some
		// gateways may return a real 404. Treat it as not-found.
		return nil, ErrProductNotFound
	case resp.StatusCode >= 400:
		return nil, &UnexpectedStatusError{StatusCode: resp.StatusCode}
	}

	return parseResponse(body, barcode, c.logger)
}

func isTimeoutError(err error) bool {
	var t interface{ Timeout() bool }
	return errors.As(err, &t) && t.Timeout()
}
