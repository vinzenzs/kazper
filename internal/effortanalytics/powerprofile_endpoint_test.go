package effortanalytics_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/effortanalytics"
)

// seedFullProfile stores a constant-power ride long enough to populate every
// ladder duration (so all four anchors rank).
func seedFullProfile(t *testing.T, f *fixture, watts float64) {
	t.Helper()
	w := seedWorkout(t, f.repo, time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC))
	_, err := f.svc.ComputeAndReplace(context.Background(), w, constSlice(3600, watts), nil)
	require.NoError(t, err)
}

func getProfile(t *testing.T, f *fixture, query string) (*httptest.ResponseRecorder, effortanalytics.PowerProfileResult) {
	t.Helper()
	rec := get(t, f.r, "/workouts/power-profile?"+query)
	var res effortanalytics.PowerProfileResult
	if rec.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	}
	return rec, res
}

func TestPowerProfile_FullFourAnchorRank(t *testing.T) {
	f := setup(t)
	seedFullProfile(t, f, 250)

	rec, res := getProfile(t, f, "from=2026-03-10&to=2026-03-10&tz=UTC&weight_kg=70&sex=male")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	require.Len(t, res.Anchors, 4)
	assert.Empty(t, res.MissingAnchors)
	assert.Equal(t, "male", res.Sex)
	assert.InDelta(t, 70.0, res.WeightKg, 0.001)
	assert.Equal(t, "param", res.WeightSource)

	labels := map[string]effortanalytics.PowerProfileAnchor{}
	for _, a := range res.Anchors {
		labels[a.Label] = a
		assert.InDelta(t, 250.0, a.Watts, 0.1)
		assert.InDelta(t, 250.0/70.0, a.WPerKg, 0.05)
		assert.NotEmpty(t, a.Category)
		assert.GreaterOrEqual(t, a.Percentile, 0.0)
		assert.LessOrEqual(t, a.Percentile, 100.0)
	}
	for _, want := range []string{"neuromuscular", "anaerobic", "vo2max", "threshold"} {
		_, ok := labels[want]
		assert.True(t, ok, "anchor %q present", want)
	}
	// All four present → phenotype is computed (not null).
	require.NotNil(t, res.Phenotype)
}

func TestPowerProfile_MissingAnchorsOmittedAndListed(t *testing.T) {
	f := setup(t)
	// A 200-second ride only reaches ladder durations ≤ 200 → 5 s and 60 s
	// anchors present; 300 s (vo2max) and 1200 s (threshold) missing.
	w := seedWorkout(t, f.repo, time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC))
	_, err := f.svc.ComputeAndReplace(context.Background(), w, constSlice(200, 400), nil)
	require.NoError(t, err)

	rec, res := getProfile(t, f, "from=2026-03-10&to=2026-03-10&tz=UTC&weight_kg=70")
	require.Equal(t, http.StatusOK, rec.Code)

	present := map[string]bool{}
	for _, a := range res.Anchors {
		present[a.Label] = true
	}
	assert.True(t, present["neuromuscular"])
	assert.True(t, present["anaerobic"])
	assert.ElementsMatch(t, []string{"vo2max", "threshold"}, res.MissingAnchors)
	// Incomplete profile → no phenotype.
	assert.Nil(t, res.Phenotype)
}

func TestPowerProfile_WeightResolution(t *testing.T) {
	f := setup(t)
	seedFullProfile(t, f, 250)

	// 1) Explicit param wins even when a stored weight exists.
	f.weight.kg, f.weight.found = 80, true
	rec, res := getProfile(t, f, "from=2026-03-10&to=2026-03-10&tz=UTC&weight_kg=65")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.InDelta(t, 65.0, res.WeightKg, 0.001)
	assert.Equal(t, "param", res.WeightSource)

	// 2) No param → stored fallback.
	rec, res = getProfile(t, f, "from=2026-03-10&to=2026-03-10&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.InDelta(t, 80.0, res.WeightKg, 0.001)
	assert.Equal(t, "stored", res.WeightSource)

	// 3) No param and no stored weight → 400 weight_data_missing.
	f.weight.found = false
	rec, _ = getProfile(t, f, "from=2026-03-10&to=2026-03-10&tz=UTC")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "weight_data_missing", body["error"])
}

func TestPowerProfile_ParamValidation(t *testing.T) {
	f := setup(t)
	seedFullProfile(t, f, 250)
	cases := []struct{ name, query, wantErr string }{
		{"bad sex", "from=2026-03-10&to=2026-03-10&weight_kg=70&sex=other", "sex_invalid"},
		{"zero weight", "from=2026-03-10&to=2026-03-10&weight_kg=0", "weight_kg_invalid"},
		{"negative weight", "from=2026-03-10&to=2026-03-10&weight_kg=-5", "weight_kg_invalid"},
		{"nonnumeric weight", "from=2026-03-10&to=2026-03-10&weight_kg=heavy", "weight_kg_invalid"},
		{"missing range", "weight_kg=70", "range_required"},
		{"bad date", "from=03/10/2026&to=2026-03-12&weight_kg=70", "date_invalid"},
		{"reversed", "from=2026-03-12&to=2026-03-10&weight_kg=70", "range_invalid"},
		{"too large", "from=2025-01-01&to=2026-12-31&weight_kg=70", "range_too_large"},
		{"bad tz", "from=2026-03-10&to=2026-03-10&weight_kg=70&tz=Nowhere", "tz_invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, _ := getProfile(t, f, tc.query)
			require.Equal(t, http.StatusBadRequest, rec.Code)
			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tc.wantErr, body["error"])
		})
	}
}

func TestPowerProfile_SexDefaultsMale(t *testing.T) {
	f := setup(t)
	seedFullProfile(t, f, 250)
	_, res := getProfile(t, f, "from=2026-03-10&to=2026-03-10&tz=UTC&weight_kg=70")
	assert.Equal(t, "male", res.Sex)
}

func TestPowerProfile_ReadIsIdempotentNoMutation(t *testing.T) {
	f := setup(t)
	seedFullProfile(t, f, 250)

	first := get(t, f.r, "/workouts/power-profile?from=2026-03-10&to=2026-03-10&tz=UTC&weight_kg=70")
	second := get(t, f.r, "/workouts/power-profile?from=2026-03-10&to=2026-03-10&tz=UTC&weight_kg=70")
	require.Equal(t, http.StatusOK, first.Code)
	require.Equal(t, http.StatusOK, second.Code)
	assert.JSONEq(t, first.Body.String(), second.Body.String())
}
