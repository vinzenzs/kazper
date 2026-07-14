package activitystreams

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seg appends `secs` samples of value `w` (with no noise — the detector smooths,
// and deterministic fixtures make boundaries checkable).
func seg(dst []float64, w float64, secs int) []float64 {
	for i := 0; i < secs; i++ {
		dst = append(dst, w)
	}
	return dst
}

// A clean 5×4-min session at 300 W with 2-min recoveries at 120 W.
func fiveByFour() []float64 {
	var p []float64
	p = seg(p, 120, 120) // lead-in easy
	for k := 0; k < 5; k++ {
		p = seg(p, 300, 240) // 4-min effort
		if k < 4 {
			p = seg(p, 120, 120) // 2-min recovery
		}
	}
	p = seg(p, 120, 120) // lead-out
	return p
}

func TestDetectIntervals_FiveByFour(t *testing.T) {
	res := detectIntervals("w1", fiveByFour())

	require.Len(t, res.Intervals, 5, "should find five efforts")
	require.NotNil(t, res.ThresholdW)
	assert.Greater(t, *res.ThresholdW, 120.0)
	assert.Less(t, *res.ThresholdW, 300.0)
	assert.Empty(t, res.Reason)

	for i, iv := range res.Intervals {
		assert.Equal(t, i+1, iv.N)
		// Smoothing softens the edges, so durations land near 240 s, not exact.
		assert.InDelta(t, 240, iv.DurationS, 40, "effort %d duration", i+1)
		assert.InDelta(t, 300, iv.AvgW, 15, "effort %d avg", i+1)
		assert.InDelta(t, 300, iv.MaxW, 1)
		assert.Greater(t, iv.KJ, 0.0)
	}
	// Four recoveries between five efforts.
	require.Len(t, res.Rests, 4)
	assert.Equal(t, 1, res.Rests[0].AfterN)
	assert.InDelta(t, 120, res.Rests[0].AvgW, 15)

	assert.Equal(t, 5, res.Summary.Count)
	assert.InDelta(t, 240, res.Summary.MeanEffortS, 40)
	assert.InDelta(t, 300, res.Summary.MeanEffortW, 15)
}

func TestDetectIntervals_DipDoesNotSplit(t *testing.T) {
	// One effort with a 25-second sub-threshold dip in the middle (≤ 30 s merge).
	var p []float64
	p = seg(p, 120, 120)
	p = seg(p, 300, 100)
	p = seg(p, 120, 25) // brief lull
	p = seg(p, 300, 100)
	p = seg(p, 120, 120)

	res := detectIntervals("w1", p)
	require.Len(t, res.Intervals, 1, "the dip must not split the effort")
	assert.GreaterOrEqual(t, res.Intervals[0].DurationS, 200)
}

func TestDetectIntervals_SubMinuteBurstDiscarded(t *testing.T) {
	// A single 40-second spike surrounded by easy riding: below the 60 s minimum.
	var p []float64
	p = seg(p, 130, 300)
	p = seg(p, 320, 40)
	p = seg(p, 130, 300)

	res := detectIntervals("w1", p)
	assert.Empty(t, res.Intervals, "a sub-minute burst is not an interval")
}

func TestDetectIntervals_SteadyRideNoEfforts(t *testing.T) {
	// A steady endurance ride: 200 W with deterministic bounded noise (an LCG,
	// not a periodic wave — a sine is intrinsically bimodal). One power mode.
	var p []float64
	seed := uint64(12345)
	for i := 0; i < 3600; i++ {
		seed = seed*6364136223846793005 + 1442695040888963407
		noise := float64(int64(seed>>40)%40) - 20 // ~[-20, 20]
		p = append(p, 200+noise)
	}
	res := detectIntervals("w1", p)
	assert.Empty(t, res.Intervals)
	assert.Nil(t, res.ThresholdW)
	assert.Equal(t, "no_distinct_efforts", res.Reason)
}

func TestDetectIntervals_Empty(t *testing.T) {
	res := detectIntervals("w1", nil)
	assert.Empty(t, res.Intervals)
	assert.Nil(t, res.ThresholdW)
	assert.Equal(t, "no_distinct_efforts", res.Reason)
}

// A trimodal ride (endurance / tempo / VO₂ blocks): Otsu picks one split; the
// documented v1 behavior is that it still returns efforts above that split, with
// their own avg_w separating the tiers for the reader.
func TestDetectIntervals_TrimodalLumps(t *testing.T) {
	var p []float64
	p = seg(p, 150, 600) // endurance
	p = seg(p, 230, 300) // tempo
	p = seg(p, 150, 300) // easy
	p = seg(p, 330, 300) // VO2
	p = seg(p, 150, 300) // easy

	res := detectIntervals("w1", p)
	// At least the VO2 block is detected; threshold reported so the split is auditable.
	require.NotEmpty(t, res.Intervals)
	require.NotNil(t, res.ThresholdW)
	// Every detected effort's avg is above the threshold.
	for _, iv := range res.Intervals {
		assert.Greater(t, iv.AvgW, *res.ThresholdW)
	}
}
