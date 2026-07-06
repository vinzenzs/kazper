package effortanalytics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// byDuration indexes a record slice for assertions.
func byDuration(recs []Record) map[int]float64 {
	m := map[int]float64{}
	for _, r := range recs {
		m[r.DurationS] = r.Value
	}
	return m
}

func TestMeanMaximal_ConstantSeries(t *testing.T) {
	// A full hour at a flat 250 W: every ladder duration's best mean is 250.
	samples := make([]float64, 3600)
	for i := range samples {
		samples[i] = 250
	}
	recs := meanMaximal(samples, MetricPower)
	require.Len(t, recs, len(Ladder))
	for _, r := range recs {
		assert.Equal(t, MetricPower, r.Metric)
		assert.InDelta(t, 250.0, r.Value, 0.001)
	}
}

func TestMeanMaximal_SkipsDurationsLongerThanActivity(t *testing.T) {
	// 10 samples: only the 5 s ladder step fits (15 s and up are skipped).
	samples := make([]float64, 10)
	for i := range samples {
		samples[i] = 100
	}
	recs := meanMaximal(samples, MetricPower)
	require.Len(t, recs, 1)
	assert.Equal(t, 5, recs[0].DurationS)
}

func TestMeanMaximal_FindsBestWindow(t *testing.T) {
	// First 5 s at 500 W, then 55 s at 100 W.
	samples := make([]float64, 60)
	for i := range samples {
		if i < 5 {
			samples[i] = 500
		} else {
			samples[i] = 100
		}
	}
	got := byDuration(meanMaximal(samples, MetricPower))
	// Best 5 s window is the 500 W opener.
	assert.InDelta(t, 500.0, got[5], 0.001)
	// Best 60 s window is the whole activity: (5*500 + 55*100)/60 = 133.3.
	assert.InDelta(t, 133.3, got[60], 0.05)
}

func TestMeanMaximal_EmptySeries(t *testing.T) {
	assert.Nil(t, meanMaximal(nil, MetricSpeed))
}
