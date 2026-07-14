package effortanalytics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// constPow builds a constant-power series of n seconds.
func constPow(n int, w float64) []float64 {
	p := make([]float64, n)
	for i := range p {
		p[i] = w
	}
	return p
}

// tiersByKJ groups tiered records by kJ tier → set of durations present.
func tiersByKJ(recs []Record) map[int][]int {
	out := map[int][]int{}
	for _, r := range recs {
		out[r.KJTier] = append(out[r.KJTier], r.DurationS)
	}
	return out
}

// A 200 W ride of 8000 s accumulates 1600 kJ: tiers 500/1000/1500 are reached,
// 2000 is not. The 20-min (1200 s) window fits after tiers 500 and 1000 but not
// after 1500 (7500 + 1200 > 8000), so tier 1500 gets only 1m/5m.
func TestTieredEfforts_ReachesTiersWithWindowFit(t *testing.T) {
	recs := tieredEfforts(constPow(8000, 200))
	byKJ := tiersByKJ(recs)

	assert.ElementsMatch(t, []int{60, 300, 1200}, byKJ[500])
	assert.ElementsMatch(t, []int{60, 300, 1200}, byKJ[1000])
	assert.ElementsMatch(t, []int{60, 300}, byKJ[1500], "no 20-min window fits after 1500 kJ")
	_, has2000 := byKJ[2000]
	assert.False(t, has2000, "1600 kJ never reaches the 2000 kJ tier")

	for _, r := range recs {
		assert.Equal(t, MetricPower, r.Metric)
		assert.InDelta(t, 200, r.Value, 0.1)
	}
}

// A short ride never reaches the first (500 kJ) tier → no tiered rows.
func TestTieredEfforts_ShortRideNoTiers(t *testing.T) {
	// 600 s @ 200 W = 120 kJ, well under 500.
	assert.Empty(t, tieredEfforts(constPow(600, 200)))
}

// Fatigue resistance: with power declining through the ride, a tier's best (which
// must start deeper into the ride) is lower than the fresh best at the same
// duration — the durability signal.
func TestTieredEfforts_DecliningPowerFades(t *testing.T) {
	// 4000 s @ 300 W (1200 kJ by the end), then 4000 s @ 180 W.
	p := append(constPow(4000, 300), constPow(4000, 180)...)

	fresh := meanMaximal(p, MetricPower) // tier 0
	tiered := tieredEfforts(p)

	// Fresh 5-min best is up in the 300 W block.
	var fresh300 float64
	for _, r := range fresh {
		if r.DurationS == 300 {
			fresh300 = r.Value
		}
	}
	require.NotZero(t, fresh300)

	// The 1500 kJ tier starts at 5000 s (300·4000 = 1200 kJ, then 180 W: 1500 kJ
	// at 5000 s) — deep in the 180 W block, so its 5-min best is ~180 < fresh.
	var tier1500_300 float64
	for _, r := range tiered {
		if r.KJTier == 1500 && r.DurationS == 300 {
			tier1500_300 = r.Value
		}
	}
	require.NotZero(t, tier1500_300)
	assert.Less(t, tier1500_300, fresh300, "5-min power fades after 1500 kJ")
}

func TestTieredEfforts_Empty(t *testing.T) {
	assert.Empty(t, tieredEfforts(nil))
}
