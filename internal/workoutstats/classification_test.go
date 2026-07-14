package workoutstats

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func label(secs [5]int64) string {
	if s := classify(secs); s != nil {
		return *s
	}
	return "null"
}

func TestClassify_Labels(t *testing.T) {
	cases := []struct {
		name string
		secs [5]int64
		want string
	}{
		// moderate (Z3) exactly 20.0% → threshold (the big-Z3 middle).
		{"threshold at the 20.0 boundary", [5]int64{400, 200, 200, 100, 100}, "threshold"},
		// low ≥ 75 and high > moderate → polarized.
		{"polarized", [5]int64{500, 300, 50, 100, 50}, "polarized"},
		// low ≥ 75 but high ≤ moderate → pyramidal.
		{"pyramidal", [5]int64{500, 300, 120, 50, 30}, "pyramidal"},
		// low exactly 75.0, high < moderate → pyramidal.
		{"low at the 75.0 floor", [5]int64{400, 350, 150, 60, 40}, "pyramidal"},
		// high == moderate (not strictly greater) → pyramidal, not polarized.
		{"high equals moderate splits to pyramidal", [5]int64{500, 275, 100, 60, 40}, "pyramidal"},
		// low < 75, moderate < 20 → mixed.
		{"mixed", [5]int64{300, 300, 150, 150, 100}, "mixed"},
		// no zone time → null.
		{"zero zone time", [5]int64{0, 0, 0, 0, 0}, "null"},
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, label(c.secs), c.name)
	}
}

func TestBands_Collapse(t *testing.T) {
	b := bands([5]int64{400, 200, 200, 100, 100}) // low 60, mod 20, high 20
	assert.Equal(t, 60.0, b.LowPct)
	assert.Equal(t, 20.0, b.ModeratePct)
	assert.Equal(t, 20.0, b.HighPct)

	assert.Equal(t, Bands{}, bands([5]int64{0, 0, 0, 0, 0}), "no zone time → zero bands")
}

func TestAggregate_SharesSumTo100(t *testing.T) {
	// A deliberately non-round split to exercise rounding at the boundary.
	a := &zoneAccum{secs: [5]int64{333, 333, 167, 100, 67}, counted: 3}
	agg := aggregate(a)
	require.Equal(t, 1000, agg.TotalZoneSecs)
	var sum float64
	for _, z := range agg.Zones {
		require.NotNil(t, z.SharePct, "share present when there is zone time")
		sum += *z.SharePct
	}
	assert.InDelta(t, 100.0, sum, 0.2, "rounded shares sum to ~100")
	assert.Equal(t, 1, agg.Zones[0].Zone)
	assert.Equal(t, 5, agg.Zones[4].Zone)
}

func TestAggregate_ZeroZoneTimeOmitsShare(t *testing.T) {
	agg := aggregate(&zoneAccum{})
	assert.Equal(t, 0, agg.TotalZoneSecs)
	for _, z := range agg.Zones {
		assert.Nil(t, z.SharePct, "0% share is a lie, not a measurement — omitted")
	}
}
