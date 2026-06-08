package off

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
)

// envelope is the top-level shape of OFF's v2 product response. Only the
// fields we care about are typed; everything else is preserved via the
// raw payload kept on the Product.
type envelope struct {
	Status         int             `json:"status"`
	StatusVerbose  string          `json:"status_verbose"`
	Code           string          `json:"code"`
	Product        json.RawMessage `json:"product"`
}

type productPayload struct {
	ProductName    string          `json:"product_name"`
	Brands         string          `json:"brands"`
	ServingSize    string          `json:"serving_size"`
	Nutriments     json.RawMessage `json:"nutriments"`
}

// kJ → kcal conversion factor (4.184 kJ per kcal).
const kjPerKcal = 4.184

// parseResponse parses the raw OFF response body into a Product. It returns
// ErrProductNotFound when the response has status:0.
func parseResponse(body []byte, barcode string, logger *slog.Logger) (*Product, error) {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decode off envelope: %w", err)
	}
	if env.Status == 0 {
		return nil, ErrProductNotFound
	}

	var pp productPayload
	if len(env.Product) > 0 {
		if err := json.Unmarshal(env.Product, &pp); err != nil {
			return nil, fmt.Errorf("decode off product: %w", err)
		}
	}

	p := &Product{
		Barcode:    barcode,
		Name:       pp.ProductName,
		Brand:      pp.Brands,
		RawPayload: body,
	}
	if g, ok := parseServingSize(pp.ServingSize); ok {
		p.ServingSizeG = &g
	} else if pp.ServingSize != "" {
		logger.Warn("off: unparseable serving_size",
			"barcode", barcode, "value", pp.ServingSize)
	}
	p.Nutriments = parseNutriments(pp.Nutriments)
	return p, nil
}

// parseNutriments pulls the per-100g values from OFF's nutriments object.
// Missing fields are kept nil. kcal is derived from kJ when only energy_100g
// is present.
func parseNutriments(raw json.RawMessage) Nutriments {
	if len(raw) == 0 {
		return Nutriments{}
	}
	// OFF returns numeric-or-string values inconsistently across products,
	// so decode into a map[string]any and re-parse each field.
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return Nutriments{}
	}

	get := func(keys ...string) *float64 {
		for _, k := range keys {
			if v, ok := m[k]; ok {
				if f, ok := toFloat(v); ok {
					return &f
				}
			}
		}
		return nil
	}

	n := Nutriments{
		ProteinGPer100g: get("proteins_100g"),
		CarbsGPer100g:   get("carbohydrates_100g"),
		FatGPer100g:     get("fat_100g"),
		FiberGPer100g:   get("fiber_100g"),
		SugarGPer100g:   get("sugars_100g"),
		SaltGPer100g:    get("salt_100g"),

		IronMgPer100g:        get("iron_100g"),
		CalciumMgPer100g:     get("calcium_100g"),
		VitaminDMcgPer100g:   get("vitamin-d_100g"),
		VitaminB12McgPer100g: get("vitamin-b12_100g"),
		VitaminCMgPer100g:    get("vitamin-c_100g"),
		MagnesiumMgPer100g:   get("magnesium_100g"),
		PotassiumMgPer100g:   get("potassium_100g"),
		ZincMgPer100g:        get("zinc_100g"),
	}

	// kcal preference: explicit kcal first; otherwise derive from kJ.
	if v := get("energy-kcal_100g"); v != nil {
		n.KcalPer100g = v
	} else if v := get("energy_100g"); v != nil {
		kcal := round1(*v / kjPerKcal)
		n.KcalPer100g = &kcal
	}
	return n
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(x, 64)
		return f, err == nil
	}
	return 0, false
}

var servingSizeRegex = regexp.MustCompile(`^\s*([0-9]+(?:[.,][0-9]+)?)\s*(g|grams?)?\s*$`)

// parseServingSize extracts a numeric gram value from OFF's free-text
// serving_size field (e.g. "30g", "125 g", "  30 grams"). Returns false for
// strings the regex cannot tolerate.
func parseServingSize(s string) (float64, bool) {
	m := servingSizeRegex.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	num := strings.ReplaceAll(m[1], ",", ".")
	f, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func round1(f float64) float64 {
	return float64(int(f*10+sign(f)*0.5)) / 10
}

func sign(f float64) float64 {
	if f < 0 {
		return -1
	}
	return 1
}
