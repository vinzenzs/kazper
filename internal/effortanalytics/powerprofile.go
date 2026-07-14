package effortanalytics

import "github.com/vinzenzs/kazper/internal/numfmt"

// Power-profile ranking against the Coggan power-profile tables (Allen & Coggan,
// "Training and Racing with a Power Meter"). W/kg at four benchmark durations is
// mapped onto named ability categories (untrained → world class) per sex, and the
// relative standing across the four anchors implies a rider phenotype. Pure: no
// I/O, no athlete-config — a fixed reference, not a personal calibration.

// The four Coggan anchors. The threshold anchor is the 20-minute best used as a
// functional-threshold proxy (no 0.95 haircut — advisory, transparency over a
// hidden fudge). 5 s / 60 s / 300 s are exact ladder durations.
const (
	anchorNeuromuscularS = 5
	anchorAnaerobicS     = 60
	anchorVO2MaxS        = 300
	anchorThresholdS     = 1200
)

// cogganAnchor is one benchmark duration's metadata.
type cogganAnchor struct {
	label     string // response label
	durationS int
	col       int // column index into the table rows (0=NM, 1=AN, 2=VO2, 3=FT)
}

var cogganAnchors = []cogganAnchor{
	{"neuromuscular", anchorNeuromuscularS, 0},
	{"anaerobic", anchorAnaerobicS, 1},
	{"vo2max", anchorVO2MaxS, 2},
	{"threshold", anchorThresholdS, 3},
}

// cogganRow is one named category and its W/kg threshold for each of the four
// columns [neuromuscular(5s), anaerobic(1min), vo2max(5min), threshold(FT)].
// Rows are ordered top (world class) → bottom (untrained). The value is the
// LOWER bound of the band: a W/kg at or above it, and below the next row up,
// falls in this category.
type cogganRow struct {
	category string
	wPerKg   [4]float64
}

// Coggan power-profile tables — the eight named category anchors per sex, from
// the published chart. Percentiles interpolate between adjacent rows (see
// rankAnchor); intermediate levels the chart draws between these anchors are
// reproduced by that interpolation.
var cogganTableMale = []cogganRow{
	{"World class", [4]float64{24.04, 11.50, 7.60, 6.40}},
	{"Exceptional", [4]float64{22.34, 10.73, 7.17, 6.00}},
	{"Excellent", [4]float64{20.63, 9.96, 6.75, 5.60}},
	{"Very good", [4]float64{18.93, 9.19, 6.32, 5.20}},
	{"Good", [4]float64{17.23, 8.42, 5.89, 4.81}},
	{"Moderate", [4]float64{15.53, 7.65, 5.47, 4.41}},
	{"Fair", [4]float64{13.83, 6.88, 5.04, 4.01}},
	{"Untrained", [4]float64{12.13, 6.11, 4.61, 3.61}},
}

var cogganTableFemale = []cogganRow{
	{"World class", [4]float64{19.42, 9.29, 6.61, 5.69}},
	{"Exceptional", [4]float64{18.09, 8.68, 6.21, 5.33}},
	{"Excellent", [4]float64{16.75, 8.06, 5.81, 4.97}},
	{"Very good", [4]float64{15.42, 7.44, 5.41, 4.61}},
	{"Good", [4]float64{14.08, 6.83, 5.01, 4.24}},
	{"Moderate", [4]float64{12.75, 6.21, 4.61, 3.88}},
	{"Fair", [4]float64{11.42, 5.59, 4.20, 3.52}},
	{"Untrained", [4]float64{10.08, 4.97, 3.80, 3.16}},
}

func cogganTable(sex string) []cogganRow {
	if sex == sexFemale {
		return cogganTableFemale
	}
	return cogganTableMale
}

// rankAnchor maps a W/kg at one column to its Coggan category band and an
// interpolated percentile [0,100]. The category is the highest band the value
// meets. The percentile assigns each of the N rows a level position — bottom row
// (untrained) → 0, top row (world class) → 100 — and linearly interpolates
// between adjacent rows, clamped at both ends. Category is the authoritative
// output; percentile is a smooth position-within-table estimate.
func rankAnchor(wPerKg float64, col int, sex string) (string, float64) {
	rows := cogganTable(sex)
	n := len(rows)
	top := rows[0].wPerKg[col]       // world class
	bottom := rows[n-1].wPerKg[col]  // untrained

	// Clamp beyond the table ends.
	if wPerKg >= top {
		return rows[0].category, 100
	}
	if wPerKg <= bottom {
		return rows[n-1].category, 0
	}

	// Find the adjacent rows bracketing the value. Rows descend, so walk from the
	// top until the value is at/above a row's threshold.
	for i := 0; i < n-1; i++ {
		hi := rows[i].wPerKg[col]   // higher category bound
		lo := rows[i+1].wPerKg[col] // lower category bound
		if wPerKg < hi && wPerKg >= lo {
			// Level positions: row i+1 (lower) sits at level (n-1)-(i+1),
			// row i (higher) at level (n-1)-i, mapped to percentile via /(n-1).
			pctLo := float64((n-1)-(i+1)) / float64(n-1) * 100
			pctHi := float64((n-1)-i) / float64(n-1) * 100
			frac := (wPerKg - lo) / (hi - lo)
			pct := pctLo + frac*(pctHi-pctLo)
			return rows[i+1].category, numfmt.Round1(pct)
		}
	}
	// Unreachable given the clamps above, but return the floor defensively.
	return rows[n-1].category, 0
}

// phenotype classifies the rider from the relative percentile standing of the
// four anchors: sprint strength (neuromuscular + anaerobic) vs endurance
// strength (vo2max + threshold). Returns nil unless all four anchors are present
// (a full profile is required to name a type). Advisory; never persisted.
func phenotype(anchors []PowerProfileAnchor) *string {
	pct := map[string]float64{}
	for _, a := range anchors {
		pct[a.Label] = a.Percentile
	}
	for _, need := range []string{"neuromuscular", "anaerobic", "vo2max", "threshold"} {
		if _, ok := pct[need]; !ok {
			return nil
		}
	}
	sprint := (pct["neuromuscular"] + pct["anaerobic"]) / 2
	endur := (pct["vo2max"] + pct["threshold"]) / 2
	diff := sprint - endur

	var t string
	switch {
	case diff >= phenotypeSpread:
		t = "sprinter"
	case diff <= -phenotypeSpread:
		// Endurance-leaning: a climber peaks at 5 min (repeated hard efforts),
		// a time-trialist sustains threshold.
		if pct["vo2max"] > pct["threshold"] {
			t = "climber"
		} else {
			t = "time_trialist"
		}
	default:
		t = "all_rounder"
	}
	return &t
}

// phenotypeSpread is the percentile gap between sprint and endurance strength
// that tips the classification off "all_rounder".
const phenotypeSpread = 15.0
