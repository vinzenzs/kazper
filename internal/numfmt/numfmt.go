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

// Round2 rounds f to two decimal places. Used where a value is stored at 2dp
// (e.g. a workout's intensity_factor, a NUMERIC(4,2) column).
func Round2(f float64) float64 {
	return math.Round(f*100) / 100
}

// Round2Ptr is the nil-passthrough form for *float64 fields. Returns nil when p
// is nil; otherwise returns a fresh pointer to the 2dp-rounded value.
func Round2Ptr(p *float64) *float64 {
	if p == nil {
		return nil
	}
	r := Round2(*p)
	return &r
}

// Round3 rounds f to three decimal places (e.g. a workout's efficiency_factor,
// a NUMERIC(6,3) column).
func Round3(f float64) float64 {
	return math.Round(f*1000) / 1000
}
