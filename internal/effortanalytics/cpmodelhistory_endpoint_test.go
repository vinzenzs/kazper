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

func getCPHistory(t *testing.T, f *fixture, query string) (effortanalytics.CPModelHistoryResult, int) {
	t.Helper()
	rec := get(t, f.r, "/workouts/cp-model/history?"+query)
	if rec.Code != http.StatusOK {
		return effortanalytics.CPModelHistoryResult{}, rec.Code
	}
	var res effortanalytics.CPModelHistoryResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	return res, rec.Code
}

// seedFitRide stores a ride whose best-efforts span the CP band (so the fit
// succeeds) on `day`.
func seedFitRide(t *testing.T, f *fixture, day time.Time) {
	t.Helper()
	w := seedWorkout(t, f.repo, day)
	// A 30-min effort with a descending profile gives distinct in-band bests.
	power := append(constSlice(300, 360), constSlice(300, 320)...)
	power = append(power, constSlice(600, 300)...)
	power = append(power, constSlice(600, 285)...)
	_, err := f.svc.ComputeAndReplace(context.Background(), w, power, nil)
	require.NoError(t, err)
}

func TestCPModelHistory_SeasonSeriesAndAnchors(t *testing.T) {
	f := setup(t)
	// Rides across two months so several Monday anchors have data in-window.
	for _, d := range []string{"2026-05-04", "2026-05-18", "2026-06-01", "2026-06-15"} {
		day, _ := time.Parse("2006-01-02", d)
		seedFitRide(t, f, day.Add(9*time.Hour))
	}

	res, code := getCPHistory(t, f, "from=2026-05-01&to=2026-06-30&tz=UTC")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, 90, res.WindowDays)
	require.NotEmpty(t, res.Anchors)
	// Every anchor date is a Monday, ascending.
	for _, a := range res.Anchors {
		d, err := time.Parse("2006-01-02", a.Date)
		require.NoError(t, err)
		assert.Equal(t, time.Monday, d.Weekday(), "anchor %s must be a Monday", a.Date)
	}
	// At least one anchor produced a fitted model.
	fitted := 0
	for _, a := range res.Anchors {
		if a.Model != nil {
			fitted++
			assert.Greater(t, a.Model.CPWatts, 0.0)
		}
	}
	assert.Greater(t, fitted, 0)
}

func TestCPModelHistory_GappedAnchorsRetained(t *testing.T) {
	f := setup(t)
	// No rides at all → every anchor is a gate reason, none dropped.
	res, code := getCPHistory(t, f, "from=2026-05-01&to=2026-05-31&tz=UTC")
	require.Equal(t, http.StatusOK, code)
	require.NotEmpty(t, res.Anchors)
	for _, a := range res.Anchors {
		assert.Nil(t, a.Model)
		assert.NotEmpty(t, a.Reason, "a gapped anchor keeps its gate reason")
	}
}

func TestCPModelHistory_WindowDaysMatrix(t *testing.T) {
	f := setup(t)
	// Valid custom window.
	res, code := getCPHistory(t, f, "from=2026-05-01&to=2026-05-31&tz=UTC&window_days=45")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, 45, res.WindowDays)

	// Invalid windows.
	for _, wd := range []string{"29", "366", "abc"} {
		rec := get(t, f.r, "/workouts/cp-model/history?from=2026-05-01&to=2026-05-31&tz=UTC&window_days="+wd)
		require.Equal(t, http.StatusBadRequest, rec.Code, wd)
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "window_days_invalid", body["error"], wd)
	}
}

func TestCPModelHistory_RangeContract(t *testing.T) {
	f := setup(t)
	cases := []struct{ name, path, wantErr string }{
		{"missing range", "/workouts/cp-model/history", "range_required"},
		{"reversed", "/workouts/cp-model/history?from=2026-06-02&to=2026-06-01", "range_invalid"},
		{"too large", "/workouts/cp-model/history?from=2025-01-01&to=2026-12-31", "range_too_large"},
		{"bad tz", "/workouts/cp-model/history?from=2026-05-01&to=2026-05-31&tz=Nowhere", "tz_invalid"},
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

func TestCPModelHistory_ReadOnly(t *testing.T) {
	f := setup(t)
	seedFitRide(t, f, time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC))
	a := get(t, f.r, "/workouts/cp-model/history?from=2026-05-01&to=2026-06-30&tz=UTC")
	b := get(t, f.r, "/workouts/cp-model/history?from=2026-05-01&to=2026-06-30&tz=UTC")
	assert.Equal(t, a.Body.String(), b.Body.String())
}
