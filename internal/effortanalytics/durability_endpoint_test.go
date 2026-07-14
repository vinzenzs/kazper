package effortanalytics_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/effortanalytics"
)

func getDurability(t *testing.T, f *fixture, query string) (effortanalytics.DurabilityResult, int) {
	t.Helper()
	rec := get(t, f.r, "/workouts/durability?"+query)
	if rec.Code != http.StatusOK {
		return effortanalytics.DurabilityResult{}, rec.Code
	}
	var res effortanalytics.DurabilityResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	return res, rec.Code
}

// A long declining-power ride: high fresh bests, lower tiered bests → fade.
func seedDecliningRide(t *testing.T, f *fixture, day time.Time) {
	t.Helper()
	w := seedWorkout(t, f.repo, day)
	// 4000 s @ 300 W then 4000 s @ 180 W → reaches the 500/1000/1500 kJ tiers.
	power := append(constSlice(4000, 300), constSlice(4000, 180)...)
	_, err := f.svc.ComputeAndReplace(context.Background(), w, power, nil)
	require.NoError(t, err)
}

func TestDurability_FadeHappyPath(t *testing.T) {
	f := setup(t)
	seedDecliningRide(t, f, time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC))

	res, code := getDurability(t, f, "from=2026-03-10&to=2026-03-10&tz=UTC")
	require.Equal(t, http.StatusOK, code)
	assert.Empty(t, res.Reason)
	require.NotEmpty(t, res.Durations)

	// Find the 5-min column.
	var five *effortanalytics.DurabilityDuration
	for i := range res.Durations {
		if res.Durations[i].DurationS == 300 {
			five = &res.Durations[i]
		}
	}
	require.NotNil(t, five)
	require.NotNil(t, five.Fresh)
	assert.InDelta(t, 300, five.Fresh.Watts, 5) // fresh 5-min is up in the 300 W block
	require.NotEmpty(t, five.Tiers)
	// The deepest tier faded well below fresh.
	last := five.Tiers[len(five.Tiers)-1]
	assert.Less(t, last.Watts, five.Fresh.Watts)
	assert.Greater(t, last.FadePct, 0.0)
}

func TestDurability_TierLeakRegression(t *testing.T) {
	f := setup(t)
	seedDecliningRide(t, f, time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC))

	// The FRESH power curve must reflect tier-0 only: its 5-min best is the high
	// (300 W) fresh value, never the faded tier value the durability read exposes.
	rec := get(t, f.r, "/workouts/power-curve?from=2026-03-10&to=2026-03-10&sport=bike&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	var curve effortanalytics.Curve
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &curve))
	var curve300 float64
	for _, p := range curve.Points {
		if p.DurationS == 300 {
			curve300 = p.Value
		}
	}
	assert.InDelta(t, 300, curve300, 5, "fresh curve must not be dragged down by tiered rows")
}

func TestDurability_NoTieredData(t *testing.T) {
	f := setup(t)
	// A short ride reaches no tier → fresh rows only.
	w := seedWorkout(t, f.repo, time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC))
	_, err := f.svc.ComputeAndReplace(context.Background(), w, constSlice(600, 250), nil)
	require.NoError(t, err)

	res, code := getDurability(t, f, "from=2026-03-10&to=2026-03-10&tz=UTC")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "no_tiered_data", res.Reason)
	// Fresh columns present (60/300 fit in 600 s), each with empty Tiers.
	for _, d := range res.Durations {
		assert.Empty(t, d.Tiers)
		assert.NotNil(t, d.Fresh)
	}
}

func TestDurability_EmptyWindow(t *testing.T) {
	f := setup(t)
	res, code := getDurability(t, f, "from=2026-03-10&to=2026-03-12&tz=UTC")
	require.Equal(t, http.StatusOK, code)
	assert.Empty(t, res.Durations)
	assert.Equal(t, "no_tiered_data", res.Reason)
}

func TestDurability_RePostReplacesTiers(t *testing.T) {
	f := setup(t)
	day := time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC)
	w := seedWorkout(t, f.repo, day)
	// First a long ride (has tiers), then re-post a short ride (no tiers).
	_, err := f.svc.ComputeAndReplace(context.Background(), w, append(constSlice(4000, 300), constSlice(4000, 180)...), nil)
	require.NoError(t, err)
	_, err = f.svc.ComputeAndReplace(context.Background(), w, constSlice(600, 250), nil)
	require.NoError(t, err)

	res, code := getDurability(t, f, "from=2026-03-10&to=2026-03-10&tz=UTC")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "no_tiered_data", res.Reason, "the re-post cleared the prior tiered rows")
}

func TestDurability_ErrorContract(t *testing.T) {
	f := setup(t)
	cases := []struct{ name, path, wantErr string }{
		{"missing range", "/workouts/durability", "range_required"},
		{"bad date", "/workouts/durability?from=03/10/2026&to=2026-03-12", "date_invalid"},
		{"reversed", "/workouts/durability?from=2026-03-12&to=2026-03-10", "range_invalid"},
		{"too large", "/workouts/durability?from=2025-01-01&to=2026-12-31", "range_too_large"},
		{"bad tz", "/workouts/durability?from=2026-03-10&to=2026-03-12&tz=Nowhere", "tz_invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := get(t, f.r, tc.path)
			require.Equal(t, http.StatusBadRequest, rec.Code)
			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tc.wantErr, body["error"])
		})
	}
}

func TestDurability_ReadOnly(t *testing.T) {
	f := setup(t)
	seedDecliningRide(t, f, time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC))
	a := get(t, f.r, "/workouts/durability?from=2026-03-10&to=2026-03-10&tz=UTC")
	b := get(t, f.r, "/workouts/durability?from=2026-03-10&to=2026-03-10&tz=UTC")
	assert.Equal(t, a.Body.String(), b.Body.String())
}
