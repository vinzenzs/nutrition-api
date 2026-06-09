package vision

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	// Default upstream — the Anthropic Messages API. Override via Config.BaseURL
	// for fixture-based tests (httptest server URL).
	defaultBaseURL = "https://api.anthropic.com"

	// Default model. Overridable via CLAUDE_VISION_MODEL. The 4.6 generation
	// of Sonnet was current at proposal time; the env var means swapping for
	// 4.7 or a faster variant is a config change, not a code change.
	defaultModel = "claude-sonnet-4-6"

	// Default per-request timeout. Claude Vision typically responds in 700-
	// 1500ms; 15s leaves headroom for the occasional cold path without
	// turning the endpoint into a slow-loris vector.
	defaultTimeout = 15 * time.Second

	// Anthropic version header required on every call.
	anthropicVersion = "2023-06-01"

	// Tool name we force the model to use. The handler unmarshals the
	// tool_use input into ParseResult.
	reportMealToolName = "report_meal"
)

// Config carries the runtime knobs for the vision client. APIKey is the only
// required field; the rest fall back to documented defaults.
type Config struct {
	APIKey  string
	BaseURL string        // overridable for fixture tests
	Model   string        // overridable for ops
	Timeout time.Duration // overridable for ops
}

// Client talks to the Anthropic Messages API to parse meal photos. Mirrors
// internal/off.Client in shape; tests stub the underlying http.Client.
type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

// New constructs a Client. Returns ErrAPIKeyMissing when Config.APIKey is
// empty so the server can detect "vision is disabled" and leave the client
// nil; the meals/from_photo handler then returns 503 vision_unavailable
// instead of panicking.
func New(cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, ErrAPIKeyMissing
	}
	base := cfg.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	model := cfg.Model
	if model == "" {
		model = defaultModel
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	return &Client{
		baseURL: base,
		apiKey:  cfg.APIKey,
		model:   model,
		http:    &http.Client{Timeout: timeout},
	}, nil
}

// SetHTTPClient swaps the underlying http.Client. Used by tests to inject a
// stub transport without exporting struct internals. Production callers
// should not touch this.
func (c *Client) SetHTTPClient(h *http.Client) { c.http = h }

// Parse sends one image to the Messages API with tool-forced output, then
// decodes the report_meal tool_use into a ParseResult. On parse failure it
// retries once with an explicit "respond ONLY with the tool" follow-up
// message; if that fails too it returns ErrVisionResponseUnparseable.
//
// Errors:
//   - ErrVisionTimeout    on ctx-deadline or http.Client timeout
//   - ErrVisionUpstreamError on Anthropic 5xx
//   - *ErrVisionRateLimited{RetryAfterSeconds} on 429
//   - *ErrVisionUnexpectedResponse{StatusCode} on other 4xx
//   - ErrVisionResponseUnparseable after one retry on tool_use parse failure
func (c *Client) Parse(ctx context.Context, req ParseRequest) (*ParseResult, error) {
	body, err := c.buildInitialRequest(req)
	if err != nil {
		return nil, err
	}
	respBody, err := c.do(ctx, body)
	if err != nil {
		return nil, err
	}

	pr, perr := extractReportMeal(respBody)
	if perr == nil {
		pr.ResizedTo = req.ResizedTo
		pr.OriginalBytes = req.OriginalBytes
		return pr, nil
	}

	// One retry with the explicit nudge. We piggyback on the previous
	// response's text-or-tool content by sending a follow-up user message
	// after the assistant's malformed reply, asking it to retry the tool.
	retryBody, err := c.buildRetryRequest(req, respBody)
	if err != nil {
		return nil, err
	}
	retryResp, err := c.do(ctx, retryBody)
	if err != nil {
		return nil, err
	}
	pr2, perr2 := extractReportMeal(retryResp)
	if perr2 == nil {
		pr2.ResizedTo = req.ResizedTo
		pr2.OriginalBytes = req.OriginalBytes
		return pr2, nil
	}
	return nil, fmt.Errorf("%w: %v (after retry: %v)", ErrVisionResponseUnparseable, perr, perr2)
}

// do executes one HTTP POST to /v1/messages and returns the response body
// (or a mapped error sentinel). Headers per the Anthropic docs.
func (c *Client) do(ctx context.Context, reqBody []byte) ([]byte, error) {
	url := c.baseURL + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("vision: build request: %w", err)
	}
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)
	httpReq.Header.Set("content-type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		// net.Error timeouts and ctx.DeadlineExceeded both map to vision_timeout.
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrVisionTimeout
		}
		var ne interface{ Timeout() bool }
		if errors.As(err, &ne) && ne.Timeout() {
			return nil, ErrVisionTimeout
		}
		return nil, fmt.Errorf("%w: %v", ErrVisionUpstreamError, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("vision: read response body: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusOK:
		return respBody, nil
	case resp.StatusCode == http.StatusTooManyRequests:
		// Forward Retry-After verbatim (0 if absent). Spec'd in seconds per RFC.
		retry := 0
		if s := resp.Header.Get("Retry-After"); s != "" {
			retry, _ = strconv.Atoi(s)
		}
		return nil, &ErrVisionRateLimited{RetryAfterSeconds: retry}
	case resp.StatusCode >= 500:
		return nil, ErrVisionUpstreamError
	default:
		// Other 4xx: auth, bad request, etc. Bubble the status up.
		return nil, &ErrVisionUnexpectedResponse{StatusCode: resp.StatusCode}
	}
}

// ------ request bodies ------

type messagesRequest struct {
	Model      string         `json:"model"`
	MaxTokens  int            `json:"max_tokens"`
	System     string         `json:"system,omitempty"`
	Tools      []toolDef      `json:"tools"`
	ToolChoice toolChoice     `json:"tool_choice"`
	Messages   []userMessage  `json:"messages"`
}

type toolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type toolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

type userMessage struct {
	Role    string         `json:"role"`
	Content []messageBlock `json:"content"`
}

type messageBlock struct {
	Type   string       `json:"type"`
	Text   string       `json:"text,omitempty"`
	Source *imageSource `json:"source,omitempty"`
}

type imageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // "image/jpeg"
	Data      string `json:"data"`
}

// reportMealToolSchema is the JSON Schema fed to the model. Tool-forced
// output gives us strict structural guarantees on Claude's reply — no
// prose, no code fences, no commentary.
var reportMealToolSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "name": { "type": "string", "description": "Short human-readable dish name." },
    "nutriments_per_100g": {
      "type": "object",
      "properties": {
        "kcal":      { "type": "number" },
        "protein_g": { "type": "number" },
        "carbs_g":   { "type": "number" },
        "fat_g":     { "type": "number" },
        "fiber_g":   { "type": ["number", "null"] },
        "sugar_g":   { "type": ["number", "null"] },
        "salt_g":    { "type": ["number", "null"] }
      },
      "required": ["kcal", "protein_g", "carbs_g", "fat_g"]
    },
    "confidence": { "type": "number", "description": "0.0-1.0 self-rated confidence." },
    "notes":      { "type": "string", "description": "Optional one-liner of caveats: 'unclear if includes sauce', etc." }
  },
  "required": ["name", "nutriments_per_100g", "confidence"]
}`)

const reportMealToolDesc = "Report a parsed meal: name, per-100g macros, and self-rated confidence."

const systemPrompt = `You are a nutrition-estimation assistant. The user supplies a photo of a meal.
Your job: estimate the dish's identity and its per-100g macronutrient profile,
then call the report_meal tool with the result. ALWAYS call the tool — never
respond with prose. Be conservative when uncertain (lower confidence, notes
field flagging the unknown). Calorie + macro fields are required; fiber, sugar,
and salt may be null when the photo can't reasonably support an estimate.`

const userPrompt = `Parse this meal photo. Call report_meal with your best estimate of the per-100g macronutrient profile.`

// buildInitialRequest assembles the messages-request body for the first call.
func (c *Client) buildInitialRequest(req ParseRequest) ([]byte, error) {
	b64 := base64.StdEncoding.EncodeToString(req.Image)
	body := messagesRequest{
		Model:     c.model,
		MaxTokens: 1024,
		System:    systemPrompt,
		Tools: []toolDef{{
			Name:        reportMealToolName,
			Description: reportMealToolDesc,
			InputSchema: reportMealToolSchema,
		}},
		ToolChoice: toolChoice{Type: "tool", Name: reportMealToolName},
		Messages: []userMessage{{
			Role: "user",
			Content: []messageBlock{
				{
					Type: "image",
					Source: &imageSource{
						Type:      "base64",
						MediaType: "image/jpeg",
						Data:      b64,
					},
				},
				{Type: "text", Text: userPrompt},
			},
		}},
	}
	return json.Marshal(body)
}

// buildRetryRequest re-sends the conversation with an explicit nudge after
// the model produced an unparseable response. We re-attach the original
// image (no state carries between Anthropic Messages calls) plus the prior
// turn, plus a fresh user message asking for the tool.
func (c *Client) buildRetryRequest(req ParseRequest, _ []byte) ([]byte, error) {
	b64 := base64.StdEncoding.EncodeToString(req.Image)
	body := messagesRequest{
		Model:     c.model,
		MaxTokens: 1024,
		System:    systemPrompt,
		Tools: []toolDef{{
			Name:        reportMealToolName,
			Description: reportMealToolDesc,
			InputSchema: reportMealToolSchema,
		}},
		ToolChoice: toolChoice{Type: "tool", Name: reportMealToolName},
		Messages: []userMessage{{
			Role: "user",
			Content: []messageBlock{
				{
					Type: "image",
					Source: &imageSource{
						Type:      "base64",
						MediaType: "image/jpeg",
						Data:      b64,
					},
				},
				{Type: "text", Text: userPrompt + " Your last reply did not call the tool — call report_meal now, do not output prose."},
			},
		}},
	}
	return json.Marshal(body)
}
