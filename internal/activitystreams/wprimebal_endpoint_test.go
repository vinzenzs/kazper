package activitystreams_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/activitystreams"
)

// seedPowerWorkout seeds a completed workout with a constant-power stream and
// returns its id. 300 W over a CP of 250 W drains W′.
func seedPowerWorkout(t *testing.T, f *fixture, durS, watts int) string {
	t.Helper()
	id := seedWorkout(t, f.repo).String()
	require.Equal(t, http.StatusOK,
		post(t, f.r, "/workouts/"+id+"/streams", fmt.Sprintf(`{"power":%s}`, arr(durS, float64(watts)))).Code)
	return id
}

func getWP(t *testing.T, f *fixture, id, query string) (activitystreams.WPrimeBalanceResult, int) {
	t.Helper()
	rec := get(t, f.r, "/workouts/"+id+"/w-prime-balance?"+query)
	if rec.Code != http.StatusOK {
		return activitystreams.WPrimeBalanceResult{}, rec.Code
	}
	var res activitystreams.WPrimeBalanceResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	return res, rec.Code
}

func TestWPrimeEndpoint_HappyPath(t *testing.T) {
	f := setup(t)
	id := seedPowerWorkout(t, f, 1800, 300) // 30 min @ 300 W

	res, code := getWP(t, f, id, "cp_watts=250&w_prime_kj=20")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, 250.0, res.Params.CPWatts)
	assert.Equal(t, 20.0, res.Params.WPrimeKJ)
	assert.Equal(t, 1800, res.DurationS)
	require.Len(t, res.Series, 1800) // full-resolution when downsample omitted
	// 50 W over CP drains W′ (20 kJ) in 400 s → deeply negative by 30 min.
	assert.Less(t, res.Summary.MinWPrimeKJ, 0.0)
	assert.Greater(t, res.Summary.MaxDepletionPct, 100.0)
	assert.Equal(t, 1799, res.Summary.MinAtS) // deepest at the end
	assert.Greater(t, res.Summary.TimeBelow25PctS, 0)
}

func TestWPrimeEndpoint_SummaryOnly(t *testing.T) {
	f := setup(t)
	id := seedPowerWorkout(t, f, 1800, 300)
	res, code := getWP(t, f, id, "cp_watts=250&w_prime_kj=20&summary_only=true")
	require.Equal(t, http.StatusOK, code)
	assert.Empty(t, res.Series)
	assert.Nil(t, res.Downsample)
	assert.Less(t, res.Summary.MinWPrimeKJ, 20.0) // summary still computed
}

func TestWPrimeEndpoint_Downsample(t *testing.T) {
	f := setup(t)
	id := seedPowerWorkout(t, f, 1800, 300)
	res, code := getWP(t, f, id, "cp_watts=250&w_prime_kj=20&downsample=100")
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, res.Downsample)
	assert.Equal(t, 100, *res.Downsample)
	assert.Len(t, res.Series, 100)
	assert.Equal(t, 1800, res.DurationS) // full-resolution length preserved

	// Out of bounds → 400 downsample_invalid.
	for _, ds := range []string{"5", "5001", "abc"} {
		rec := get(t, f.r, "/workouts/"+id+"/w-prime-balance?cp_watts=250&w_prime_kj=20&downsample="+ds)
		require.Equal(t, http.StatusBadRequest, rec.Code, ds)
		assert.Contains(t, rec.Body.String(), "downsample_invalid")
	}
}

func TestWPrimeEndpoint_ParamValidation(t *testing.T) {
	f := setup(t)
	id := seedPowerWorkout(t, f, 600, 300)
	cases := []struct{ name, query, wantErr string }{
		{"missing cp", "w_prime_kj=20", "cp_invalid"},
		{"cp zero", "cp_watts=0&w_prime_kj=20", "cp_invalid"},
		{"cp non-numeric", "cp_watts=abc&w_prime_kj=20", "cp_invalid"},
		{"missing w_prime", "cp_watts=250", "w_prime_invalid"},
		{"w_prime negative", "cp_watts=250&w_prime_kj=-5", "w_prime_invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := get(t, f.r, "/workouts/"+id+"/w-prime-balance?"+tc.query)
			require.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.wantErr)
		})
	}
}

func TestWPrimeEndpoint_NotFoundDistinctions(t *testing.T) {
	f := setup(t)
	// Unknown workout → workout_not_found.
	rec := get(t, f.r, "/workouts/"+seedWorkoutMissing()+"/w-prime-balance?cp_watts=250&w_prime_kj=20")
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "workout_not_found")

	// Workout with no stored streams → streams_not_found.
	bare := seedWorkout(t, f.repo).String()
	rec = get(t, f.r, "/workouts/"+bare+"/w-prime-balance?cp_watts=250&w_prime_kj=20")
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "streams_not_found")

	// Workout with streams but NO power (HR only) → power_stream_missing.
	hrOnly := seedWorkout(t, f.repo).String()
	require.Equal(t, http.StatusOK,
		post(t, f.r, "/workouts/"+hrOnly+"/streams", fmt.Sprintf(`{"heart_rate":%s}`, arr(1800, 150))).Code)
	rec = get(t, f.r, "/workouts/"+hrOnly+"/w-prime-balance?cp_watts=250&w_prime_kj=20")
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "power_stream_missing")
}

func TestWPrimeEndpoint_ReadIsIdempotent(t *testing.T) {
	f := setup(t)
	id := seedPowerWorkout(t, f, 1200, 280)
	a := get(t, f.r, "/workouts/"+id+"/w-prime-balance?cp_watts=250&w_prime_kj=20")
	b := get(t, f.r, "/workouts/"+id+"/w-prime-balance?cp_watts=250&w_prime_kj=20")
	assert.Equal(t, a.Body.String(), b.Body.String())
	// Unit isolation: no nutrition fields on this workout-analytics read.
	assert.NotContains(t, a.Body.String(), "kcal")
	assert.NotContains(t, a.Body.String(), "protein")
}

func seedWorkoutMissing() string {
	return "00000000-0000-0000-0000-000000000000"
}
