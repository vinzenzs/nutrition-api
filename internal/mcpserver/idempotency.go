package mcpserver

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// deriveIdempotencyKey returns a deterministic key for a tool call when the
// agent did not supply one explicitly. It hashes the tool name plus the
// canonical JSON of args, with the idempotency_key field stripped if present
// (otherwise the key would depend on itself).
func deriveIdempotencyKey(toolName string, args any) string {
	canonical, _ := canonicalJSON(args)
	h := sha256.New()
	h.Write([]byte(toolName))
	h.Write([]byte("|"))
	h.Write(canonical)
	return hex.EncodeToString(h.Sum(nil))
}

// effectiveIdempotencyKey returns the explicit key if non-empty, otherwise a
// derived key. The explicit key always wins.
func effectiveIdempotencyKey(explicit, toolName string, args any) string {
	if explicit != "" {
		return explicit
	}
	return deriveIdempotencyKey(toolName, args)
}

// canonicalJSON marshals v to JSON with object keys sorted recursively and
// the field named "idempotency_key" stripped from any object it appears in.
// This lets the derived key be stable across reorderings of struct fields
// and across the presence/absence of an explicit idempotency_key.
func canonicalJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var generic any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&generic); err != nil {
		return nil, err
	}
	cleaned := stripIdempotencyKey(generic)
	return marshalCanonical(cleaned)
}

func stripIdempotencyKey(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			if k == "idempotency_key" {
				continue
			}
			out[k] = stripIdempotencyKey(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = stripIdempotencyKey(val)
		}
		return out
	default:
		return x
	}
}

// marshalCanonical writes v with sorted object keys, no insignificant
// whitespace. It rejects non-JSON types.
func marshalCanonical(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeCanonical(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeCanonical(buf *bytes.Buffer, v any) error {
	switch x := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		if x {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case string:
		s, err := json.Marshal(x)
		if err != nil {
			return err
		}
		buf.Write(s)
	case json.Number:
		buf.WriteString(string(x))
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			ks, err := json.Marshal(k)
			if err != nil {
				return err
			}
			buf.Write(ks)
			buf.WriteByte(':')
			if err := writeCanonical(buf, x[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	case []any:
		buf.WriteByte('[')
		for i, val := range x {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonical(buf, val); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	default:
		// Anything else gets marshalled via encoding/json as a safety net.
		s, err := json.Marshal(x)
		if err != nil {
			return err
		}
		buf.Write(s)
	}
	return nil
}
