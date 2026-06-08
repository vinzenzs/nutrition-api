package off

// Product is the parsed view of an Open Food Facts product, normalized to the
// fields we persist. Pointer types denote nullability — OFF data is patchy and
// missing fields are stored as null rather than zero.
type Product struct {
	Barcode       string
	Name          string
	Brand         string
	ServingSizeG  *float64
	Nutriments    Nutriments
	// RawPayload is the original OFF response body, preserved for future
	// re-extraction of fields we don't yet parse.
	RawPayload []byte
}

// Nutriments holds the per-100g values we care about. Every field is nullable.
// Micros (iron, calcium, vit D, vit B12, vit C, magnesium, potassium, zinc)
// are pulled from OFF using the units OFF reports them in: milligrams for
// iron/calcium/vit C/magnesium/potassium/zinc; micrograms for vit D and vit B12.
type Nutriments struct {
	KcalPer100g     *float64
	ProteinGPer100g *float64
	CarbsGPer100g   *float64
	FatGPer100g     *float64
	FiberGPer100g   *float64
	SugarGPer100g   *float64
	SaltGPer100g    *float64

	IronMgPer100g        *float64
	CalciumMgPer100g     *float64
	VitaminDMcgPer100g   *float64
	VitaminB12McgPer100g *float64
	VitaminCMgPer100g    *float64
	MagnesiumMgPer100g   *float64
	PotassiumMgPer100g   *float64
	ZincMgPer100g        *float64
}
