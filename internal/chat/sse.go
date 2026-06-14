package chat

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/vinzenzs/nutrition-api/internal/agenttools"
)

// sseWriter emits the four chat event types over an HTTP response as Server-Sent
// Events, flushing after each so the mobile client renders incrementally.
type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func newSSEWriter(w http.ResponseWriter) (*sseWriter, bool) {
	fl, ok := w.(http.Flusher)
	if !ok {
		return nil, false
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no") // disable proxy buffering
	w.WriteHeader(http.StatusOK)
	fl.Flush()
	return &sseWriter{w: w, flusher: fl}, true
}

func (s *sseWriter) emit(event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, data)
	s.flusher.Flush()
}

// textEvent carries one assistant text delta.
func (s *sseWriter) text(delta string) {
	s.emit("text", map[string]string{"text": delta})
}

// toolEvent reports a tool's lifecycle: id is the upstream tool_use id (shared
// by a call's started and terminal events so a client can coalesce them), status
// is started|ok|error, summary is a short human string — never raw request/
// response bodies.
func (s *sseWriter) tool(id, name, status, summary string) {
	s.emit("tool", map[string]string{"id": id, "name": name, "status": status, "summary": summary})
}

// proposalEvent surfaces the pending write-confirm calls of a paused turn so the
// client can render an approve/reject card. turnID labels the paused turn;
// calls each carry a server-composed human preview (never a raw body). It is
// followed by a done event with stop_reason "awaiting_confirmation".
func (s *sseWriter) proposal(turnID string, calls []agenttools.ProposalCall) {
	s.emit("proposal", map[string]any{"turn_id": turnID, "calls": calls})
}

// doneEvent terminates a successful stream with the full final message.
func (s *sseWriter) done(message, stopReason string, usage Usage) {
	s.emit("done", map[string]any{
		"message":     message,
		"stop_reason": stopReason,
		"usage":       usage,
	})
}

// errorEvent terminates the stream with a typed code.
func (s *sseWriter) error(code, message string) {
	s.emit("error", map[string]string{"code": code, "message": message})
}
