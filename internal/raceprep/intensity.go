package raceprep

import "math"

// IntensityFromTSS derives a 1-5 zone from a workout's TSS + duration via the
// Coggan intensity factor:
//
//   IF = sqrt(tss / (durationHr * 100))
//
// Mapping (closed-low intervals, matching the design):
//
//   IF < 0.65         → Zone 1 (recovery / very easy)
//   IF ∈ [0.65, 0.75) → Zone 2 (easy aerobic / endurance)
//   IF ∈ [0.75, 0.85) → Zone 3 (tempo)
//   IF ∈ [0.85, 0.92) → Zone 4 (threshold)
//   IF ≥ 0.92         → Zone 5 (VO2max / sprints)
//
// Returns (2, true) when tss is nil OR durationMin ≤ 0 — Z2 is a defensible
// neutral default and the caller surfaces the "intensity defaulted" disclosure
// via the recommend notes builder. The bool is the trigger for that note.
func IntensityFromTSS(tss *float64, durationMin int) (int, bool) {
	if tss == nil || durationMin <= 0 {
		return 2, true
	}
	durationHr := float64(durationMin) / 60.0
	if durationHr == 0 {
		return 2, true
	}
	if_ := math.Sqrt(*tss / (durationHr * 100))
	switch {
	case if_ < 0.65:
		return 1, false
	case if_ < 0.75:
		return 2, false
	case if_ < 0.85:
		return 3, false
	case if_ < 0.92:
		return 4, false
	default:
		return 5, false
	}
}
