package cookidoo

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// jsonLDBlock matches the body of every <script type="application/ld+json">
// element. The (?is) flags make it case-insensitive and dot-match-newline so
// multi-line JSON-LD blocks are captured whole.
var jsonLDBlock = regexp.MustCompile(`(?is)<script[^>]*type=["']application/ld\+json["'][^>]*>(.*?)</script>`)

// ParseRecipe extracts the Schema.org Recipe from a fetched Cookidoo HTML page.
// It scans every JSON-LD block (recipes may sit alongside other types, or
// nested under @graph) and returns the first Recipe found. Returns
// ErrNoRecipeJSONLD when the page carries no parseable Recipe block.
func ParseRecipe(htmlBytes []byte) (*Recipe, error) {
	matches := jsonLDBlock.FindAllSubmatch(htmlBytes, -1)
	for _, m := range matches {
		raw := strings.TrimSpace(string(m[1]))
		if raw == "" {
			continue
		}
		// A block is either a single object or an array of objects; @graph may
		// nest further. Decode loosely and walk for a Recipe.
		var decoded any
		if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
			continue // skip malformed blocks rather than failing the whole page
		}
		if node := findRecipeNode(decoded); node != nil {
			return recipeFromNode(node), nil
		}
	}
	return nil, ErrNoRecipeJSONLD
}

// findRecipeNode walks a decoded JSON-LD value (object, array, or @graph
// container) and returns the first map whose @type is or includes "Recipe".
func findRecipeNode(v any) map[string]any {
	switch t := v.(type) {
	case map[string]any:
		if typeIncludesRecipe(t["@type"]) {
			return t
		}
		if graph, ok := t["@graph"]; ok {
			return findRecipeNode(graph)
		}
	case []any:
		for _, item := range t {
			if node := findRecipeNode(item); node != nil {
				return node
			}
		}
	}
	return nil
}

func typeIncludesRecipe(v any) bool {
	switch t := v.(type) {
	case string:
		return strings.EqualFold(t, "Recipe")
	case []any:
		for _, item := range t {
			if s, ok := item.(string); ok && strings.EqualFold(s, "Recipe") {
				return true
			}
		}
	}
	return false
}

func recipeFromNode(node map[string]any) *Recipe {
	r := &Recipe{
		Name:        stringField(node["name"]),
		Ingredients: stringSliceField(node["recipeIngredient"]),
	}
	r.ServingsYield = parseYield(node["recipeYield"])
	r.TotalTimeMin = parseISODurationMinutes(stringField(node["totalTime"]))
	r.NutritionPerServing = parseNutrition(node["nutrition"])
	return r
}

// stringField coerces a JSON-LD value that should be a string. JSON-LD allows a
// field to be an array of strings; we take the first.
func stringField(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		if len(t) > 0 {
			if s, ok := t[0].(string); ok {
				return s
			}
		}
	}
	return ""
}

// stringSliceField coerces recipeIngredient, which is normally an array of
// strings but may degrade to a single string. Entries are preserved verbatim
// (no trimming) so what we store matches the page exactly.
func stringSliceField(v any) []string {
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	}
	return nil
}

var leadingIntPattern = regexp.MustCompile(`\d+`)

// parseYield extracts the leading integer from recipeYield. Handles "6
// Portionen", a bare number, or an array whose first element carries the count.
func parseYield(v any) *int {
	switch t := v.(type) {
	case float64:
		n := int(t)
		return &n
	case string:
		if m := leadingIntPattern.FindString(t); m != "" {
			if n, err := strconv.Atoi(m); err == nil {
				return &n
			}
		}
	case []any:
		if len(t) > 0 {
			return parseYield(t[0])
		}
	}
	return nil
}

var isoDurationPattern = regexp.MustCompile(`(?i)^P(?:(\d+)D)?T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$`)

// parseISODurationMinutes parses an ISO-8601 duration like "PT1H30M" into whole
// minutes. Returns nil when the value is empty or not a recognised duration.
func parseISODurationMinutes(s string) *int {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	m := isoDurationPattern.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	atoi := func(x string) int {
		if x == "" {
			return 0
		}
		n, _ := strconv.Atoi(x)
		return n
	}
	total := atoi(m[1])*24*60 + atoi(m[2])*60 + atoi(m[3]) + atoi(m[4])/60
	if total == 0 {
		return nil
	}
	return &total
}

// parseNutrition maps a Schema.org NutritionInformation object onto our macro
// fields. Cookidoo reports values like "589 kcal" / "45 g" / German comma
// decimals; each is parsed leniently and missing/unparseable fields stay nil.
func parseNutrition(v any) NutritionPerServing {
	var n NutritionPerServing
	node, ok := v.(map[string]any)
	if !ok {
		return n
	}
	n.Kcal = parseNumberWithUnit(node["calories"])
	n.ProteinG = parseNumberWithUnit(node["proteinContent"])
	n.CarbsG = parseNumberWithUnit(node["carbohydrateContent"])
	n.FatG = parseNumberWithUnit(node["fatContent"])
	n.FiberG = parseNumberWithUnit(node["fiberContent"])
	n.SugarG = parseNumberWithUnit(node["sugarContent"])
	// Schema.org carries sodiumContent; we store salt. Without a reliable
	// sodium→salt factor on a per-serving string we leave salt nil rather than
	// guess — honest-over-approximate, matching the project's nutriment stance.
	return n
}

var numberPattern = regexp.MustCompile(`-?\d+(?:[.,]\d+)?`)

// parseNumberWithUnit extracts the leading number from a Schema.org nutrition
// string ("589 kcal", "1,5 g", "45 g") tolerating comma or dot decimals and a
// trailing unit. Returns nil when no number is present. Numeric JSON values are
// also accepted.
func parseNumberWithUnit(v any) *float64 {
	switch t := v.(type) {
	case float64:
		return &t
	case string:
		m := numberPattern.FindString(t)
		if m == "" {
			return nil
		}
		m = strings.Replace(m, ",", ".", 1)
		f, err := strconv.ParseFloat(m, 64)
		if err != nil {
			return nil
		}
		return &f
	}
	return nil
}
