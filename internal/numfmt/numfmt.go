// Package numfmt holds shared response-side rounding for nutrient floats.
//
// Storage and computation use full precision. These helpers are applied at the
// HTTP-response boundary so JSON consumers never see float artefacts like
// 70.44969999999999. See openspec/specs/nutrition-goals/spec.md "Nutrient
// values in responses are rounded to one decimal place".
package numfmt

import "math"

// Round1 rounds f to one decimal place.
func Round1(f float64) float64 {
	return math.Round(f*10) / 10
}

// Round1Ptr is the nil-passthrough form for *float64 fields. Returns nil
// when p is nil; otherwise returns a fresh pointer to the rounded value.
func Round1Ptr(p *float64) *float64 {
	if p == nil {
		return nil
	}
	r := Round1(*p)
	return &r
}
