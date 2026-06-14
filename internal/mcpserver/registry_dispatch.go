package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vinzenzs/nutrition-api/internal/agenttools"
)

// registerSharedTools registers every MCP-exposed tool in the shared
// agenttools registry through one generic handler (DD3). It replaces the
// hand-written registerXxxTools functions one domain at a time: a tool is
// either ported (an MCP-exposed registry entry, registered here) or not yet
// ported (its bespoke registerXxxTools still runs in Run). The two never
// register the same name, so they coexist cleanly during the migration.
//
// The multipart photo tool is the one documented exception (DD5): it stays a
// registry entry for discovery but is registered bespoke because HTTPCall is
// JSON-shaped and cannot express a multipart upload.
func registerSharedTools(server *mcp.Server, c *apiClient) {
	for _, s := range agenttools.MCPRegistry() {
		if s.Name == multipartPhotoTool {
			continue
		}
		spec := s // capture per iteration for the closure
		server.AddTool(&mcp.Tool{
			Name:        spec.Name,
			Description: spec.Description,
			InputSchema: mustReflectSchema(spec),
		}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return dispatchMCP(ctx, c, spec, req.Params.Arguments), nil
		})
	}
}

// multipartPhotoTool is registered bespoke (DD5); the generic loop skips it.
const multipartPhotoTool = "log_meal_from_photo"

// dispatchMCP executes one registry tool: build its single HTTPCall from the
// raw input, attach a shared-derivation idempotency key for write tiers (DD4),
// dispatch through the API client, and map the response through the existing
// tool-result/error mapping. This is exactly what each bespoke handler did,
// written once.
func dispatchMCP(ctx context.Context, c *apiClient, s agenttools.Spec, raw json.RawMessage) *mcp.CallToolResult {
	call, err := s.Build(raw)
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	var key string
	if s.Tier.IsWrite() {
		key = agenttools.EffectiveIdempotencyKey(agenttools.ExplicitIdempotencyKey(raw), s.Name, raw)
	}
	status, body, err := c.do(ctx, call.Method, call.Path, call.Query, call.Body, key)
	return toToolResult(status, body, err)
}

// mustReflectSchema reflects a registry entry's typed arg struct into the JSON
// Schema the MCP server announces, via the same jsonschema library and call the
// SDK uses internally (server.go setSchema → jsonschema.ForType). The SDK
// announces this exact (unresolved) schema, so the output is byte-identical to
// the prior generic mcp.AddTool registration — proven per-tool by the golden
// test. Panics on reflection failure: a tool that cannot announce its schema is
// a programming error that must fail at startup, not at first call.
func mustReflectSchema(s agenttools.Spec) *jsonschema.Schema {
	if s.SchemaType == nil {
		panic(fmt.Sprintf("agenttools tool %q: MCP-exposed tool has no SchemaType", s.Name))
	}
	schema, err := jsonschema.ForType(reflect.TypeOf(s.SchemaType), &jsonschema.ForOptions{})
	if err != nil {
		panic(fmt.Sprintf("agenttools tool %q: schema reflection failed: %v", s.Name, err))
	}
	return schema
}
