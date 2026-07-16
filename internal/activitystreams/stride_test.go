package activitystreams

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syntheticRun builds paired speed/cadence samples whose composition is known by
// construction: over the speed range, cadence scales as speed^cadExp and step
// length takes the rest (speed^(1-cadExp)) — so the true contribution split is
// exactly cadExp : 1-cadExp.
//
//	speed   = v
//	cadence = k · v^cadExp
//	step    = v / (cadence/60) = (60/k) · v^(1-cadExp)
func syntheticRun(t *testing.T, lo, hi float64, perSpeed int, cadExp float64) (speed, cadence []float64) {
	t.Helper()
	// k scales the fixture to a PHYSICAL runner: ~170 spm and ~1.2 m steps at
	// 3.5 m/s. The split is scale-invariant, so k doesn't affect what's under
	// test — but a fixture implying 97 spm at 5 m/s would be a bad model to
	// reason from.
	const k = 116.0
	steps := 24
	for i := 0; i <= steps; i++ {
		v := lo + (hi-lo)*float64(i)/float64(steps)
		c := k * math.Pow(v, cadExp)
		for j := 0; j < perSpeed; j++ {
			speed = append(speed, v)
			cadence = append(cadence, c)
		}
	}
	return speed, cadence
}

// ============================================================================

// A run whose speed gain is 30% turnover / 70% step length must be recovered as
// such — this is the whole claim of the endpoint.
func TestStrideAnalysis_RecoversAKnownSplit(t *testing.T) {
	speed, cadence := syntheticRun(t, 2.5, 5.0, 8, 0.3)

	got := strideAnalysis(speed, cadence, 0)

	require.NotNil(t, got.Contribution, "a run with real speed range must yield a split")
	assert.Nil(t, got.Reason)
	assert.InDelta(t, 30, got.Contribution.CadencePct, 1.0)
	assert.InDelta(t, 70, got.Contribution.StepPct, 1.0)
	assert.InDelta(t, 100, got.Contribution.CadencePct+got.Contribution.StepPct, 0.001,
		"the split is a partition — it must sum to 100")
}

func TestStrideAnalysis_RecoversTheOppositeSplit(t *testing.T) {
	// A turnover-dominant runner: 80% of the speed gain from cadence.
	speed, cadence := syntheticRun(t, 2.5, 5.0, 8, 0.8)

	got := strideAnalysis(speed, cadence, 0)

	require.NotNil(t, got.Contribution)
	assert.InDelta(t, 80, got.Contribution.CadencePct, 1.0)
	assert.InDelta(t, 20, got.Contribution.StepPct, 1.0)
}

// The limiter question: a runner whose step length plateaus gets there by
// turnover alone at the top end — and the bins must show it rather than the
// split asserting it.
func TestStrideAnalysis_PlateauIsVisibleInTheBins(t *testing.T) {
	// Step length fixed at 1.2 m: all speed gain is turnover (cadExp = 1).
	speed, cadence := syntheticRun(t, 2.5, 5.0, 8, 1.0)

	got := strideAnalysis(speed, cadence, 0)

	require.NotNil(t, got.Contribution)
	assert.InDelta(t, 100, got.Contribution.CadencePct, 1.0)
	assert.InDelta(t, 0, got.Contribution.StepPct, 1.0)

	// And the bins carry a flat step length the reader can see for themselves.
	require.Greater(t, len(got.Bins), 3)
	first, last := got.Bins[0], got.Bins[len(got.Bins)-1]
	assert.InDelta(t, first.StepLengthM, last.StepLengthM, 0.02, "step length is flat")
	assert.Greater(t, last.CadenceSPM, first.CadenceSPM, "cadence carries the speed")
}

// A steady-state run holds no answer — and must say so rather than fit noise.
func TestStrideAnalysis_SteadyRunRefusesToNameALimiter(t *testing.T) {
	// 3.0–3.2 m/s: 0.2 m/s of spread, under the 0.5 gate.
	speed, cadence := syntheticRun(t, 3.0, 3.2, 20, 0.3)

	got := strideAnalysis(speed, cadence, 0)

	assert.Nil(t, got.Contribution, "no confident nonsense from a steady run")
	require.NotNil(t, got.Reason)
	assert.Equal(t, ReasonInsufficientSpeedRange, *got.Reason)
	assert.NotEmpty(t, got.Bins, "the bins still return — the reader sees what data there was")
}

func TestStrideAnalysis_SpeedRangeGateBoundary(t *testing.T) {
	// The gate measures the span of BIN MIDPOINTS, so build runs either side.
	under := strideAnalysis(mk(t, 3.0, 3.3, 0.3))
	assert.Nil(t, under.Contribution, "0.3 m/s of spread is not an answer")

	over := strideAnalysis(mk(t, 3.0, 4.0, 0.3))
	assert.NotNil(t, over.Contribution, "1.0 m/s of spread is")
}

// mk is a small adapter so the boundary test reads as a table.
func mk(t *testing.T, lo, hi, cadExp float64) ([]float64, []float64, float64) {
	t.Helper()
	s, c := syntheticRun(t, lo, hi, 8, cadExp)
	return s, c, 0
}

// Standing and dropouts are excluded and counted — never diluted into a bin.
func TestStrideAnalysis_NonPositiveSamplesExcludedAndCounted(t *testing.T) {
	speed, cadence := syntheticRun(t, 2.5, 5.0, 4, 0.3)
	analyzed := len(speed)

	// 10 s standing (speed 0) + 5 s of cadence dropout.
	for i := 0; i < 10; i++ {
		speed = append(speed, 0)
		cadence = append(cadence, 0)
	}
	for i := 0; i < 5; i++ {
		speed = append(speed, 3.0)
		cadence = append(cadence, 0)
	}

	got := strideAnalysis(speed, cadence, 0)

	assert.Equal(t, analyzed, got.AnalyzedS)
	assert.Equal(t, 15, got.ExcludedS)
	// A zero-cadence sample would divide by zero / produce an infinite step —
	// none may reach a bin.
	for _, b := range got.Bins {
		assert.False(t, math.IsInf(b.StepLengthM, 0))
		assert.Greater(t, b.CadenceSPM, 0.0)
	}
}

func TestStrideAnalysis_MinSpeedFilterExcludesAndCounts(t *testing.T) {
	// A run with a walk break at 1.2 m/s.
	speed, cadence := syntheticRun(t, 2.5, 5.0, 4, 0.3)
	running := len(speed)
	for i := 0; i < 30; i++ {
		speed = append(speed, 1.2)
		cadence = append(cadence, 70)
	}

	// Without the cutoff the walk is analyzed.
	off := strideAnalysis(speed, cadence, 0)
	assert.Equal(t, running+30, off.AnalyzedS)
	assert.Zero(t, off.ExcludedS)

	// With it, the walk is excluded and counted.
	on := strideAnalysis(speed, cadence, 1.8)
	assert.Equal(t, running, on.AnalyzedS)
	assert.Equal(t, 30, on.ExcludedS)
	for _, b := range on.Bins {
		assert.GreaterOrEqual(t, b.SpeedHighMps, 1.8)
	}
}

func TestStrideAnalysis_StepLengthIsMetresPerStep(t *testing.T) {
	// 3 m/s at 180 spm → 3 / (180/60) = 1.0 m per step. The plausible number.
	got := strideAnalysis([]float64{3.0, 3.0}, []float64{180, 180}, 0)

	require.Len(t, got.Bins, 1)
	assert.InDelta(t, 1.0, got.Bins[0].StepLengthM, 0.001)
	assert.InDelta(t, 180, got.Bins[0].CadenceSPM, 0.001)
	assert.Equal(t, 2, got.Bins[0].Seconds)
}

func TestStrideAnalysis_BinsAreAscendingAndBounded(t *testing.T) {
	speed, cadence := syntheticRun(t, 2.5, 5.0, 4, 0.3)

	got := strideAnalysis(speed, cadence, 0)

	require.NotEmpty(t, got.Bins)
	for i, b := range got.Bins {
		assert.InDelta(t, speedBinWidthMps, b.SpeedHighMps-b.SpeedLowMps, 0.0001)
		if i > 0 {
			assert.Greater(t, b.SpeedLowMps, got.Bins[i-1].SpeedLowMps, "ascending")
		}
	}
	assert.LessOrEqual(t, got.Bins[0].SpeedLowMps, 2.5)
	assert.GreaterOrEqual(t, got.Bins[len(got.Bins)-1].SpeedHighMps, 5.0)
}

func TestStrideAnalysis_ScatterCappedAtThousand(t *testing.T) {
	speed, cadence := syntheticRun(t, 2.5, 5.0, 200, 0.3) // ~5000 samples

	got := strideAnalysis(speed, cadence, 0)

	assert.Greater(t, got.AnalyzedS, 1000)
	assert.LessOrEqual(t, len(got.Scatter), strideScatterMax)
	assert.NotEmpty(t, got.Scatter)
	// The thinning keeps shape: the scatter still spans the run's range.
	assert.Less(t, got.Scatter[0].SpeedMps, 3.0)
	assert.Greater(t, got.Scatter[len(got.Scatter)-1].SpeedMps, 4.0)
}

func TestStrideAnalysis_EmptyAndAllExcluded(t *testing.T) {
	empty := strideAnalysis(nil, nil, 0)
	assert.Zero(t, empty.AnalyzedS)
	assert.NotNil(t, empty.Bins, "serializes as [] not null")
	require.NotNil(t, empty.Reason)
	assert.Equal(t, ReasonInsufficientSpeedRange, *empty.Reason)

	standing := strideAnalysis([]float64{0, 0, 0}, []float64{0, 0, 0}, 0)
	assert.Zero(t, standing.AnalyzedS)
	assert.Equal(t, 3, standing.ExcludedS)
	assert.Nil(t, standing.Contribution)
}

// Ragged arrays must not panic or silently pair mismatched samples.
func TestStrideAnalysis_RaggedArrays(t *testing.T) {
	got := strideAnalysis([]float64{3.0, 3.1, 3.2}, []float64{180}, 0)

	// Only index 0 has both; the rest have a zero cadence → excluded.
	assert.Equal(t, 1, got.AnalyzedS)
	assert.Equal(t, 2, got.ExcludedS)
}

// A single-bin run can't be fitted: one point defines no slope.
func TestStrideAnalysis_SingleBinYieldsNoSplit(t *testing.T) {
	got := strideAnalysis([]float64{3.0, 3.0, 3.05}, []float64{180, 180, 181}, 0)

	require.Len(t, got.Bins, 1)
	assert.Nil(t, got.Contribution)
	require.NotNil(t, got.Reason)
	assert.Equal(t, ReasonInsufficientSpeedRange, *got.Reason)
}
