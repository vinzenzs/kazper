package effortanalytics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pt builds a fit point whose power lies exactly on P(t) = cp + w'/t for the
// given cp (W) and wPrime (J) — i.e. work = cp·t + w'.
func onLine(durS int, cp, wPrimeJ float64) CPPoint {
	watts := cp + wPrimeJ/float64(durS)
	return CPPoint{DurationS: durS, Watts: watts}
}

// A set of points exactly on a known CP/W′ line is recovered exactly.
func TestFitCPModel_ExactLine(t *testing.T) {
	cp, wPrimeJ := 250.0, 20000.0 // 250 W, 20 kJ
	pts := []CPPoint{
		onLine(300, cp, wPrimeJ),
		onLine(600, cp, wPrimeJ),
		onLine(1200, cp, wPrimeJ),
		onLine(1800, cp, wPrimeJ),
	}
	m := fitCPModel(pts)
	assert.InDelta(t, 250.0, m.CPWatts, 1e-6)
	assert.InDelta(t, 20.0, m.WPrimeKJ, 1e-6)
	assert.InDelta(t, 1.0, m.RSquared, 1e-9)
	assert.InDelta(t, 0.0, m.RMSEW, 1e-6)
}

// A perturbed point lowers R² below 1 and gives a positive power-space RMSE,
// while CP/W′ stay in a sane neighbourhood.
func TestFitCPModel_Noisy(t *testing.T) {
	cp, wPrimeJ := 250.0, 20000.0
	pts := []CPPoint{
		onLine(300, cp, wPrimeJ),
		onLine(600, cp, wPrimeJ),
		{DurationS: 1200, Watts: onLine(1200, cp, wPrimeJ).Watts + 8}, // 8 W high
		onLine(1800, cp, wPrimeJ),
	}
	m := fitCPModel(pts)
	assert.Less(t, m.RSquared, 1.0)
	assert.Greater(t, m.RSquared, 0.5)
	assert.Greater(t, m.RMSEW, 0.0)
	assert.InDelta(t, 250.0, m.CPWatts, 15.0)
}

// --- gates ---------------------------------------------------------------

func TestGateCP_InsufficientPoints(t *testing.T) {
	pts := []CPPoint{{DurationS: 300, Watts: 300}, {DurationS: 1200, Watts: 260}}
	assert.Equal(t, "insufficient_points", gateCP(pts))
}

func TestGateCP_SpanTooNarrow(t *testing.T) {
	// Three distinct durations but max (500) < 3× min (300).
	pts := []CPPoint{
		{DurationS: 300, Watts: 300},
		{DurationS: 400, Watts: 290},
		{DurationS: 500, Watts: 285},
	}
	assert.Equal(t, "span_too_narrow", gateCP(pts))
}

func TestGateCP_Ok(t *testing.T) {
	// 300..1800 spans 6× — passes.
	pts := []CPPoint{
		{DurationS: 300, Watts: 316},
		{DurationS: 600, Watts: 283},
		{DurationS: 1800, Watts: 261},
	}
	assert.Equal(t, "", gateCP(pts))
	// Exactly 3× (600..1800) is allowed (gate rejects only < 3×).
	assert.Equal(t, "", gateCP([]CPPoint{
		{DurationS: 600, Watts: 283}, {DurationS: 1200, Watts: 266}, {DurationS: 1800, Watts: 261},
	}))
}

// --- band selection ------------------------------------------------------

func TestSelectInBand_BoundariesAndExclusions(t *testing.T) {
	curve := []CurvePoint{
		{DurationS: 5, Value: 900, WorkoutID: "w", Date: "2026-06-01"},
		{DurationS: 60, Value: 500},              // 1m — out (< 120)
		{DurationS: 120, Value: 400, WorkoutID: "a", Date: "2026-06-02"}, // in (boundary)
		{DurationS: 300, Value: 320},             // in
		{DurationS: 1800, Value: 261},            // in (boundary)
		{DurationS: 3600, Value: 250},            // 60m — out (> 1800)
	}
	got := selectInBand(curve)
	var durs []int
	for _, p := range got {
		durs = append(durs, p.DurationS)
	}
	assert.Equal(t, []int{120, 300, 1800}, durs)
	// Projection carries watts + provenance.
	require.Len(t, got, 3)
	assert.Equal(t, 400.0, got[0].Watts)
	assert.Equal(t, "a", got[0].WorkoutID)
	assert.Equal(t, "2026-06-02", got[0].Date)
}
