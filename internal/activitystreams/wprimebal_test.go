package activitystreams

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func constPower(n int, w float64) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = w
	}
	return s
}

// Constant supra-CP power drains W′ linearly at (P − CP) J/s.
func TestWPrimeBalance_ConstantSupraCPLinearDepletion(t *testing.T) {
	cp, wp := 250.0, 20000.0
	bal := wPrimeBalance(constPower(100, 350), cp, wp) // 100 W over CP
	require.Len(t, bal, 100)
	assert.InDelta(t, 19900, bal[0], 1e-9)  // after 1 s: 20000 − 100
	assert.InDelta(t, 10000, bal[99], 1e-9) // after 100 s: 20000 − 100·100
	// Strictly linear −100 J/s.
	for i := 1; i < len(bal); i++ {
		assert.InDelta(t, -100, bal[i]-bal[i-1], 1e-9)
	}
}

// Depleting then riding below CP recharges toward W′ asymptotically, never over.
func TestWPrimeBalance_DepletionThenRecovery(t *testing.T) {
	cp, wp := 250.0, 20000.0
	power := append(constPower(100, 350), constPower(600, 200)...) // deplete, then easy
	bal := wPrimeBalance(power, cp, wp)
	depleted := bal[99]
	end := bal[len(bal)-1]
	assert.InDelta(t, 10000, depleted, 1e-9)
	assert.Greater(t, end, depleted)   // recovered
	assert.Less(t, end, wp)            // but never fully back to W′ in finite time
	for _, b := range bal {
		assert.LessOrEqual(t, b, wp+1e-9) // never exceeds W′
	}
}

// A ride demanding more than the supplied W′ drives the balance negative and
// the depletion past 100% — the diagnostic that params are stale (no clamp).
func TestWPrimeBalance_NegativeFloorOver100Pct(t *testing.T) {
	cp, wp := 250.0, 20000.0
	bal := wPrimeBalance(constPower(200, 400), cp, wp) // 150 J/s for 200 s → −10 kJ
	sum := wPrimeSummarize(bal, wp)
	assert.Less(t, sum.MinWPrimeKJ, 0.0)
	assert.Greater(t, sum.MaxDepletionPct, 100.0)
	assert.Equal(t, 199, sum.MinAtS) // deepest at the last second
}

// An entirely sub-CP ride that starts full stays flat at W′ (0% depletion).
func TestWPrimeBalance_AllSubCPFlat(t *testing.T) {
	cp, wp := 250.0, 20000.0
	bal := wPrimeBalance(constPower(300, 180), cp, wp)
	sum := wPrimeSummarize(bal, wp)
	for _, b := range bal {
		assert.InDelta(t, wp, b, 1e-9)
	}
	assert.InDelta(t, 20.0, sum.MinWPrimeKJ, 1e-9)
	assert.InDelta(t, 0.0, sum.MaxDepletionPct, 1e-9)
	assert.Equal(t, 0, sum.TimeBelow25PctS)
}

// time_below_25 counts seconds under 25% of W′ (5 kJ here).
func TestWPrimeSummarize_TimeBelow25(t *testing.T) {
	cp, wp := 250.0, 20000.0
	// 150 J/s: crosses 5 kJ (25%) at 20000−5000=15000 J drained → 100 s in.
	bal := wPrimeBalance(constPower(160, 400), cp, wp)
	sum := wPrimeSummarize(bal, wp)
	assert.Equal(t, 60, sum.TimeBelow25PctS) // seconds 101..160 are below 5 kJ
}

func TestWPrimeSummarize_Empty(t *testing.T) {
	sum := wPrimeSummarize(nil, 20000)
	assert.Equal(t, WPrimeSummary{}, sum)
}
