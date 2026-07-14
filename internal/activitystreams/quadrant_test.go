package activitystreams

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A hand-checkable reference: at the reference cadence/power the AEPF/CPV of a
// sample equal the reference point exactly.
func TestQuadrant_ReferencePointMath(t *testing.T) {
	// crank 172.5 mm, cadence 90 rpm → cpv_ref.
	crankM := 172.5 / 1000.0
	wantCPV := 90.0 * crankM * 2 * math.Pi / 60
	wantAEPF := 250.0 / wantCPV // cp 250 W at 90 rpm

	res := quadrantAnalysis(constF(10, 250), constF(10, 90), 250, 90, 172.5)
	assert.InDelta(t, wantCPV, res.Summary.CPVRefMps, 1e-9)
	assert.InDelta(t, wantAEPF, res.Summary.AEPFRefN, 1e-9)
	// Every sample sits exactly on the reference → high-force & high-velocity
	// (≥ boundaries) → quadrant I.
	assert.Equal(t, 10, res.Summary.PedalingS)
	assert.InDelta(t, 100.0, res.Summary.Q1Pct, 0.01)
}

// Grinding: high power at low cadence → high force, low velocity → quadrant II.
func TestQuadrant_GrindingIsQ2(t *testing.T) {
	// Reference 250 W @ 90 rpm. Samples: 300 W @ 60 rpm (more force, less speed).
	res := quadrantAnalysis(constF(30, 300), constF(30, 60), 250, 90, 172.5)
	assert.InDelta(t, 100.0, res.Summary.Q2Pct, 0.01, "grinding is high-force/low-velocity")
	assert.Equal(t, 30, res.Summary.PedalingS)
}

// Spinning: modest power at high cadence → low force, high velocity → quadrant IV.
func TestQuadrant_SpinningIsQ4(t *testing.T) {
	res := quadrantAnalysis(constF(30, 180), constF(30, 110), 250, 90, 172.5)
	assert.InDelta(t, 100.0, res.Summary.Q4Pct, 0.01, "spinning is low-force/high-velocity")
}

// Coasting/dropout samples (power ≤ 0 or cadence ≤ 0) are excluded, not diluting.
func TestQuadrant_CoastingExcluded(t *testing.T) {
	power := append(constF(20, 300), constF(10, 0)...)   // 10 s coasting
	cadence := append(constF(20, 60), constF(10, 0)...)
	res := quadrantAnalysis(power, cadence, 250, 90, 172.5)
	assert.Equal(t, 20, res.Summary.PedalingS)
	assert.Equal(t, 10, res.Summary.ExcludedS)
	// Shares computed over pedaling only.
	assert.InDelta(t, 100.0, res.Summary.Q2Pct, 0.01)
}

// Crank length shifts AEPF (a shorter crank → higher force for the same power).
func TestQuadrant_CrankSensitivity(t *testing.T) {
	long := quadrantAnalysis(constF(5, 250), constF(5, 90), 250, 90, 175)
	short := quadrantAnalysis(constF(5, 250), constF(5, 90), 250, 90, 165)
	assert.Greater(t, short.Summary.AEPFRefN, long.Summary.AEPFRefN,
		"a shorter crank yields higher effective pedal force")
}

// The scatter is systematically downsampled to ≤ 1000 points.
func TestQuadrant_ScatterDownsampled(t *testing.T) {
	res := quadrantAnalysis(constF(5000, 250), constF(5000, 90), 250, 90, 172.5)
	assert.LessOrEqual(t, len(res.Scatter), quadrantScatterMax)
	assert.NotEmpty(t, res.Scatter)
}

func TestQuadrant_AllCoastingIsEmpty(t *testing.T) {
	res := quadrantAnalysis(constF(10, 0), constF(10, 0), 250, 90, 172.5)
	assert.Equal(t, 0, res.Summary.PedalingS)
	assert.Equal(t, 10, res.Summary.ExcludedS)
	assert.Equal(t, 0.0, res.Summary.Q1Pct)
}

func constF(n int, v float64) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = v
	}
	return s
}
