// Package vision integrates with the Anthropic Claude Vision API to parse
// a meal photo into a freeform-meal payload. Mirrors `internal/off/` in
// shape: small focused client, fixture-based tests, no SDK dependency.
//
// The endpoint contract is documented in
// openspec/changes/add-meal-from-photo/specs/vision-integration/ — see the
// design.md for why we use Anthropic tool-use to force structured output.
package vision

// Nutriments is the per-100g shape Claude reports for a parsed meal. Macros
// are required (non-pointer) per the tool input schema; the trace nutriments
// (fiber / sugar / salt) are nullable because they are sometimes genuinely
// unknown from a photo and forcing zeros there would corrupt downstream
// averages.
type Nutriments struct {
	Kcal     float64  `json:"kcal"`
	ProteinG float64  `json:"protein_g"`
	CarbsG   float64  `json:"carbs_g"`
	FatG     float64  `json:"fat_g"`
	FiberG   *float64 `json:"fiber_g,omitempty"`
	SugarG   *float64 `json:"sugar_g,omitempty"`
	SaltG    *float64 `json:"salt_g,omitempty"`
}

// ParseRequest is the input to Client.Parse. The image bytes are expected to
// already be resized (Resize is exposed separately so the handler does it
// before idempotency-key derivation).
type ParseRequest struct {
	// Image holds the resized JPEG-encoded bytes that get base64-encoded into
	// the Anthropic image content block.
	Image []byte

	// ResizedTo is the [width, height] that Image ended up at, surfaced back
	// in ParseResult so the response can report what the caller actually
	// sent to Claude.
	ResizedTo [2]int

	// OriginalBytes is the original image size before resize, surfaced for
	// the inference block (clients can decide to surface "you uploaded 4MB"
	// to the user).
	OriginalBytes int
}

// ParseResult is the structured output the model returns via tool_use. The
// caller maps this into the existing freeform-meal create path.
type ParseResult struct {
	Name              string      `json:"name"`
	NutrimentsPer100g Nutriments  `json:"nutriments_per_100g"`
	Confidence        float64     `json:"confidence"`
	Notes             string      `json:"notes,omitempty"`

	// Echo-back / accounting fields populated by the client (not from the
	// model). Surfaced in the inference block on the HTTP response so the
	// caller can log cost / latency context.
	InputTokens   int    `json:"input_tokens"`
	OutputTokens  int    `json:"output_tokens"`
	Model         string `json:"model"`
	ResizedTo     [2]int `json:"resized_to"`
	OriginalBytes int    `json:"original_bytes"`
}
