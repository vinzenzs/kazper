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

// seedEffort seeds a completed bike workout on `day` and computes its best-effort
// ladder from a constant `watts` power stream of `durS` seconds (so it
// contributes `watts` at every ladder duration ≤ durS).
func seedEffort(t *testing.T, f *fixture, day time.Time, watts, durS int) {
	t.Helper()
	w := seedWorkout(t, f.repo, day)
	_, err := f.svc.ComputeAndReplace(context.Background(), w, constSlice(durS, float64(watts)), nil)
	require.NoError(t, err)
}

func getCP(t *testing.T, f *fixture, query string) (effortanalytics.CPModelResult, int) {
	t.Helper()
	rec := get(t, f.r, "/workouts/cp-model?"+query)
	if rec.Code != http.StatusOK {
		return effortanalytics.CPModelResult{}, rec.Code
	}
	var res effortanalytics.CPModelResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	return res, rec.Code
}

func durationsOf(pts []effortanalytics.CPPoint) []int {
	out := make([]int, len(pts))
	for i, p := range pts {
		out[i] = p.DurationS
	}
	return out
}

// A window with a short high effort + a long lower effort fits a model over the
// in-band durations only.
func TestCPModel_FitsOverInBandEfforts(t *testing.T) {
	f := setup(t)
	// 400 W for 5 min (contributes at ≤300s) and 280 W for 30 min (≤1800s).
	seedEffort(t, f, time.Date(2026, 3, 5, 8, 0, 0, 0, time.UTC), 400, 300)
	seedEffort(t, f, time.Date(2026, 3, 12, 8, 0, 0, 0, time.UTC), 280, 1800)

	res, code := getCP(t, f, "from=2026-03-01&to=2026-03-31&tz=UTC")
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, res.Model)
	assert.Empty(t, res.Reason)
	// In-band ladder durations only: 5m/10m/20m/30m.
	assert.Equal(t, []int{300, 600, 1200, 1800}, durationsOf(res.Points))
	assert.Greater(t, res.Model.CPWatts, 0.0)
	assert.GreaterOrEqual(t, res.Model.WPrimeKJ, 0.0)
	// The 5-min point (400 W) sits above the ~280 W long-effort line, so CP lands
	// between the two and W′ is positive.
	assert.Less(t, res.Model.CPWatts, 400.0)
	assert.Greater(t, res.Model.CPWatts, 260.0)
	assert.Greater(t, res.Model.WPrimeKJ, 0.0)
}

// Sprint and 60-minute efforts never enter the fit points.
func TestCPModel_ExcludesOutOfBandDurations(t *testing.T) {
	f := setup(t)
	// A single 60-min effort contributes at every ladder duration incl. 60s + 60m.
	seedEffort(t, f, time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC), 250, 3600)

	res, code := getCP(t, f, "from=2026-03-01&to=2026-03-31&tz=UTC")
	require.Equal(t, http.StatusOK, code)
	for _, d := range durationsOf(res.Points) {
		assert.GreaterOrEqual(t, d, 120)
		assert.LessOrEqual(t, d, 1800)
	}
	// 5s/1m/60m are excluded; 5m/10m/20m/30m remain.
	assert.Equal(t, []int{300, 600, 1200, 1800}, durationsOf(res.Points))
}

// Fewer than three in-band durations degrades to a null model with a reason.
func TestCPModel_InsufficientPoints(t *testing.T) {
	f := setup(t)
	// A 5-min effort reaches only the 300s in-band duration.
	seedEffort(t, f, time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC), 400, 300)

	res, code := getCP(t, f, "from=2026-03-01&to=2026-03-31&tz=UTC")
	require.Equal(t, http.StatusOK, code)
	assert.Nil(t, res.Model)
	assert.Equal(t, "insufficient_points", res.Reason)
	assert.Equal(t, []int{300}, durationsOf(res.Points)) // the found point is still returned
}

// An empty window is a 200 null model, not a 404/5xx.
func TestCPModel_EmptyWindow(t *testing.T) {
	f := setup(t)
	res, code := getCP(t, f, "from=2026-03-01&to=2026-03-31&tz=UTC")
	require.Equal(t, http.StatusOK, code)
	assert.Nil(t, res.Model)
	assert.Equal(t, "insufficient_points", res.Reason)
	assert.Empty(t, res.Points)
}

// Compute-on-read: the same window twice yields identical output (no mutation).
func TestCPModel_ReadIsIdempotent(t *testing.T) {
	f := setup(t)
	seedEffort(t, f, time.Date(2026, 3, 5, 8, 0, 0, 0, time.UTC), 400, 300)
	seedEffort(t, f, time.Date(2026, 3, 12, 8, 0, 0, 0, time.UTC), 280, 1800)
	a := get(t, f.r, "/workouts/cp-model?from=2026-03-01&to=2026-03-31&tz=UTC")
	b := get(t, f.r, "/workouts/cp-model?from=2026-03-01&to=2026-03-31&tz=UTC")
	assert.Equal(t, a.Body.String(), b.Body.String())
	// Unit isolation: no nutrition/hydration fields on this workout-analytics read.
	assert.NotContains(t, a.Body.String(), "kcal")
	assert.NotContains(t, a.Body.String(), "total_ml")
	assert.NotContains(t, a.Body.String(), "protein")
}

func TestCPModel_RangeErrorContract(t *testing.T) {
	f := setup(t)
	cases := []struct{ name, query, wantErr string }{
		{"missing range", "", "range_required"},
		{"bad date", "from=03/01/2026&to=2026-03-31", "date_invalid"},
		{"from after to", "from=2026-03-31&to=2026-03-01", "range_invalid"},
		{"range too large", "from=2025-01-01&to=2026-12-31", "range_too_large"},
		{"bad tz", "from=2026-03-01&to=2026-03-31&tz=NowhereLand", "tz_invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := get(t, f.r, "/workouts/cp-model?"+tc.query)
			require.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.wantErr)
		})
	}
}
