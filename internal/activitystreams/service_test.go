package activitystreams

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// constSeries builds n samples all at v.
func constSeries(n int, v float64) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = v
	}
	return s
}

// A perfectly steady ride: NP == mean power, so VI == 1.00. HR steady → EF and
// decoupling defined; decoupling ≈ 0 because both halves are identical.
func TestExecutionMetrics_SteadyRide(t *testing.T) {
	power := constSeries(3600, 200)
	hr := constSeries(3600, 150)
	m := executionMetrics(power, nil, hr)

	require.NotNil(t, m.VariabilityIndex)
	assert.InDelta(t, 1.00, *m.VariabilityIndex, 0.001)
	require.NotNil(t, m.EfficiencyFactor)
	assert.InDelta(t, 200.0/150.0, *m.EfficiencyFactor, 0.001)
	require.NotNil(t, m.DecouplingPct)
	assert.InDelta(t, 0.0, *m.DecouplingPct, 0.001)
}

// Surging power in blocks longer than the 30s smoothing window lifts NP above
// mean power, so VI > 1. (High-frequency 1s alternation would be smoothed away —
// that is the point of the rolling average.)
func TestExecutionMetrics_VariableRideHasVIAboveOne(t *testing.T) {
	power := make([]float64, 3600)
	for i := range power {
		if (i/300)%2 == 0 { // 5-minute hard/easy blocks
			power[i] = 300
		} else {
			power[i] = 100
		}
	}
	m := executionMetrics(power, nil, nil)
	require.NotNil(t, m.VariabilityIndex)
	assert.Greater(t, *m.VariabilityIndex, 1.0)
}

// A run with no power: VI is nil (needs power), EF falls back to speed/HR.
func TestExecutionMetrics_RunWithoutPower(t *testing.T) {
	speed := constSeries(2400, 3.5) // m/s
	hr := constSeries(2400, 160)
	m := executionMetrics(nil, speed, hr)

	assert.Nil(t, m.VariabilityIndex)
	require.NotNil(t, m.EfficiencyFactor)
	assert.InDelta(t, 3.5/160.0, *m.EfficiencyFactor, 0.0005)
	require.NotNil(t, m.DecouplingPct)
	assert.InDelta(t, 0.0, *m.DecouplingPct, 0.001)
}

// HR that drops out (mostly zeros) fails the coverage gate: no EF, no decoupling.
func TestExecutionMetrics_DropoutHeavyHRGivesNoHRMetrics(t *testing.T) {
	power := constSeries(3600, 200)
	hr := make([]float64, 3600)
	for i := range hr {
		if i%10 == 0 { // only 10% coverage
			hr[i] = 150
		}
	}
	m := executionMetrics(power, nil, hr)

	require.NotNil(t, m.VariabilityIndex) // power-only metric still fine
	assert.Nil(t, m.EfficiencyFactor)
	assert.Nil(t, m.DecouplingPct)
}

// An activity shorter than the 20-minute floor yields no metrics at all.
func TestExecutionMetrics_TooShortYieldsNothing(t *testing.T) {
	power := constSeries(600, 200) // 10 min
	hr := constSeries(600, 150)
	m := executionMetrics(power, nil, hr)

	assert.Nil(t, m.VariabilityIndex)
	assert.Nil(t, m.EfficiencyFactor)
	assert.Nil(t, m.DecouplingPct)
}

// Second half more efficient (same power, lower HR) → negative decoupling.
func TestExecutionMetrics_NegativeDecoupling(t *testing.T) {
	power := constSeries(3600, 200)
	hr := make([]float64, 3600)
	for i := range hr {
		if i < 1800 {
			hr[i] = 160
		} else {
			hr[i] = 140 // more output per beat later
		}
	}
	m := executionMetrics(power, nil, hr)
	require.NotNil(t, m.DecouplingPct)
	assert.Less(t, *m.DecouplingPct, 0.0)
	// r1 = 200/160 = 1.25, r2 = 200/140 ≈ 1.4286; (1.25-1.4286)/1.25 ≈ -14.3%
	assert.InDelta(t, -14.3, *m.DecouplingPct, 0.2)
}

// Positive decoupling: power fades in the second half at steady HR (fatigue).
func TestExecutionMetrics_PositiveDecoupling(t *testing.T) {
	power := make([]float64, 3600)
	for i := range power {
		if i < 1800 {
			power[i] = 220
		} else {
			power[i] = 190
		}
	}
	hr := constSeries(3600, 150)
	m := executionMetrics(power, nil, hr)
	require.NotNil(t, m.DecouplingPct)
	assert.Greater(t, *m.DecouplingPct, 0.0)
}

func TestNormalizedPower_SteadyEqualsMean(t *testing.T) {
	np, ok := normalizedPower(constSeries(3600, 210))
	require.True(t, ok)
	assert.InDelta(t, 210.0, np, 0.001)
}

func TestNormalizedPower_TooShort(t *testing.T) {
	_, ok := normalizedPower(constSeries(10, 200))
	assert.False(t, ok)
}

func TestBucketMean(t *testing.T) {
	// 100 samples → 10 buckets, each the mean of its 10-sample span.
	in := make([]float64, 100)
	for i := range in {
		in[i] = float64(i)
	}
	out := bucketMean(in, 10)
	require.Len(t, out, 10)
	assert.InDelta(t, 4.5, out[0], 0.001)  // mean(0..9)
	assert.InDelta(t, 94.5, out[9], 0.001) // mean(90..99)
}

func TestBucketMean_NoUpsample(t *testing.T) {
	in := constSeries(5, 1)
	out := bucketMean(in, 50)
	assert.Len(t, out, 5) // target >= n returns input untouched
}

func TestNonZeroMean(t *testing.T) {
	m, cov := nonZeroMean([]float64{0, 100, 0, 200})
	assert.InDelta(t, 150.0, m, 0.001)
	assert.InDelta(t, 0.5, cov, 0.001)
}

func TestNonZeroMean_AllZero(t *testing.T) {
	m, cov := nonZeroMean([]float64{0, 0, 0})
	assert.Equal(t, 0.0, m)
	assert.Equal(t, 0.0, cov)
}

// Guards against NaN leaking from empty inputs.
func TestExecutionMetrics_EmptyInputs(t *testing.T) {
	m := executionMetrics(nil, nil, nil)
	assert.Nil(t, m.VariabilityIndex)
	assert.Nil(t, m.EfficiencyFactor)
	assert.Nil(t, m.DecouplingPct)
	assert.False(t, math.IsNaN(mean(nil)))
}
