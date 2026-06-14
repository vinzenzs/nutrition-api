package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/vinzenzs/kazper/internal/agenttools"
)

// dispatcher executes a tool's REST call in-process against the server's own
// HTTP handler (the Gin engine). Going through ServeHTTP traverses the full
// middleware stack — auth, idempotency, request logging — exactly as a network
// call would, so the chat loop has no privileged path to repos and every tool
// action is audited. The bearer token of the originating /chat request is
// forwarded so the sub-request authenticates as the same caller.
type dispatcher struct {
	handler http.Handler
	specs   map[string]agenttools.Spec
}

func newDispatcher(handler http.Handler) *dispatcher {
	return &dispatcher{handler: handler, specs: agenttools.ByName(agenttools.ChatRegistry())}
}

// toolResult is the outcome of one tool execution.
type toolResult struct {
	status int
	body   []byte
	// ok is true for 2xx.
	ok bool
	// err is set when the tool input could not be built into a call (the tool
	// never reached the REST layer).
	err error
}

// execute builds the call for toolName(input) and runs it via loopback. For
// write tools it attaches a deterministic Idempotency-Key derived from the tool
// name + canonical input, so an identical replayed turn replays rather than
// duplicates.
func (d *dispatcher) execute(ctx context.Context, toolName string, input json.RawMessage, bearer string) toolResult {
	spec, ok := d.specs[toolName]
	if !ok {
		return toolResult{err: errUnknownTool(toolName)}
	}
	call, err := spec.Build(input)
	if err != nil {
		return toolResult{err: err}
	}

	target := call.Path
	if len(call.Query) > 0 {
		target += "?" + call.Query.Encode()
	}
	var bodyReader *bytes.Reader
	if len(call.Body) > 0 {
		bodyReader = bytes.NewReader(call.Body)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, call.Method, target, bodyReader)
	if err != nil {
		return toolResult{err: err}
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	// Write tools carry a derived Idempotency-Key so a replayed turn replays
	// rather than duplicates — except PUT (full-replace), which the idempotency
	// middleware rejects with 400 idempotency_unsupported_for_put.
	if spec.Tier.IsWrite() && call.Method != http.MethodPut {
		req.Header.Set("Idempotency-Key", agenttools.DeriveIdempotencyKey(toolName, input))
	}

	rec := httptest.NewRecorder()
	d.handler.ServeHTTP(rec, req)

	body := rec.Body.Bytes()
	return toolResult{
		status: rec.Code,
		body:   body,
		ok:     rec.Code >= 200 && rec.Code < 300,
	}
}

type unknownToolError struct{ name string }

func (e unknownToolError) Error() string { return "unknown tool: " + e.name }

func errUnknownTool(name string) error { return unknownToolError{name: name} }
