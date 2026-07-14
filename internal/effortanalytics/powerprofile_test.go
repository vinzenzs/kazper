package effortanalytics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Column indices into a cogganRow (mirrors the anchor mapping).
const (
	colNM  = 0
	colAN  = 1
	colVO2 = 2
	colFT  = 3
)

// A W/kg exactly at a published category anchor lands in that category. The
// spot-checks cover several cells per sex so a transcription slip is caught.
func TestRankAnchor_PublishedAnchors(t *testing.T) {
	cases := []struct {
		name   string
		wPerKg float64
		col    int
		sex    string
		cat    string
	}{
		{"male FT world class", 6.40, colFT, sexMale, "World class"},
		{"male 5min very good", 6.32, colVO2, sexMale, "Very good"},
		{"male 5s good", 17.23, colNM, sexMale, "Good"},
		{"male 1min untrained", 6.11, colAN, sexMale, "Untrained"},
		{"female FT world class", 5.69, colFT, sexFemale, "World class"},
		{"female 5min good", 5.01, colVO2, sexFemale, "Good"},
		{"female 5s moderate", 12.75, colNM, sexFemale, "Moderate"},
	}
	for _, c := range cases {
		cat, _ := rankAnchor(c.wPerKg, c.col, c.sex)
		assert.Equal(t, c.cat, cat, c.name)
	}
}

// The value falling between two anchors takes the LOWER band's category and a
// percentile strictly between the two anchors' levels.
func TestRankAnchor_BetweenAnchors(t *testing.T) {
	// Male FT: Excellent 5.60 .. Very good 5.20. Midpoint 5.40 → "Very good".
	cat, pct := rankAnchor(5.40, colFT, sexMale)
	assert.Equal(t, "Very good", cat)
	// 8 rows → levels 0..7 → percentiles 0,14.3,...,100. Very good is row index 3
	// (level 4 = 57.1), Excellent row 2 (level 5 = 71.4). Midpoint ≈ 64.3.
	assert.InDelta(t, 64.3, pct, 0.5)
}

// Percentile is monotonic in W/kg and clamped to [0,100] at the ends.
func TestRankAnchor_MonotonicAndClamped(t *testing.T) {
	_, pctTop := rankAnchor(99.0, colNM, sexMale)
	assert.Equal(t, 100.0, pctTop)
	_, pctBottom := rankAnchor(0.5, colNM, sexMale)
	assert.Equal(t, 0.0, pctBottom)

	// Strictly increasing across a sweep.
	var prev float64 = -1
	for w := 3.0; w <= 7.0; w += 0.25 {
		_, pct := rankAnchor(w, colFT, sexMale)
		assert.GreaterOrEqual(t, pct, prev, "percentile must be non-decreasing at w=%v", w)
		prev = pct
	}
}

// The exact top and bottom category anchors clamp to 100 / 0.
func TestRankAnchor_EndpointsExact(t *testing.T) {
	cat, pct := rankAnchor(24.04, colNM, sexMale)
	assert.Equal(t, "World class", cat)
	assert.Equal(t, 100.0, pct)
	cat, pct = rankAnchor(3.61, colFT, sexMale)
	assert.Equal(t, "Untrained", cat)
	assert.Equal(t, 0.0, pct)
}

// anchor is a test helper building a PowerProfileAnchor with just a label +
// percentile (the only fields phenotype reads).
func anchorPct(label string, pct float64) PowerProfileAnchor {
	return PowerProfileAnchor{Label: label, Percentile: pct}
}

func TestPhenotype_Branches(t *testing.T) {
	cases := []struct {
		name                 string
		nm, an, vo2, ft      float64
		want                 string
	}{
		// Sprint >> endurance.
		{"sprinter", 90, 88, 40, 42, "sprinter"},
		// Endurance >> sprint, 5min peak → climber.
		{"climber", 40, 42, 90, 80, "climber"},
		// Endurance >> sprint, FT peak → time_trialist.
		{"time_trialist", 40, 42, 78, 88, "time_trialist"},
		// Balanced.
		{"all_rounder", 60, 62, 58, 61, "all_rounder"},
	}
	for _, c := range cases {
		got := phenotype([]PowerProfileAnchor{
			anchorPct("neuromuscular", c.nm),
			anchorPct("anaerobic", c.an),
			anchorPct("vo2max", c.vo2),
			anchorPct("threshold", c.ft),
		})
		require.NotNil(t, got, c.name)
		assert.Equal(t, c.want, *got, c.name)
	}
}

// Phenotype is nil unless all four anchors are present.
func TestPhenotype_NilOnIncomplete(t *testing.T) {
	got := phenotype([]PowerProfileAnchor{
		anchorPct("neuromuscular", 90),
		anchorPct("anaerobic", 88),
		anchorPct("vo2max", 40),
		// threshold missing
	})
	assert.Nil(t, got)
}

// The threshold anchor ranks against the FT column with no 0.95 haircut: a raw
// 20-min W/kg equal to a published FT anchor gets that FT category directly.
func TestThresholdAnchor_NoHaircut(t *testing.T) {
	// Male FT "Good" = 4.81. A 20-min best of exactly 4.81 W/kg → "Good", not a
	// haircut-adjusted lower band.
	cat, _ := rankAnchor(4.81, colFT, sexMale)
	assert.Equal(t, "Good", cat)
}
