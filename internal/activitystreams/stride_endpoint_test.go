package activitystreams_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/activitystreams"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// seedRunWorkout seeds a completed RUN (the package's shared seedWorkout is a
// bike, which the stride endpoint rightly refuses).
func seedRunWorkout(t *testing.T, f *fixture) uuid.UUID {
	t.Helper()
	start := time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC)
	w := &workouts.Workout{
		Source:    workouts.SourceManual,
		Sport:     workouts.SportRun,
		Status:    workouts.StatusCompleted,
		StartedAt: start,
		EndedAt:   start.Add(time.Hour),
	}
	_, err := f.repo.Upsert(context.Background(), w)
	require.NoError(t, err)
	return w.ID
}

// strideFixtureK scales the synthetic cadence so the fixture is a PHYSICAL
// runner: ~170 spm and ~1.2 m steps at 3.5 m/s, staying in human range across
// 2.5–5.0 m/s. The split itself is scale-invariant, but a fixture that implies
// 97 spm at 5 m/s would let an unphysical result pass the plausibility checks.
const strideFixtureK = 116.0

// runStreams renders a progression run: speed sweeps lo→hi, cadence composed so
// the split is known (cadence = k·v^cadExp → cadence contributes cadExp).
func runStreams(lo, hi, cadExp float64, perSpeed int) (string, string) {
	var speeds, cads []string
	for i := 0; i <= 24; i++ {
		v := lo + (hi-lo)*float64(i)/24
		c := strideFixtureK * math.Pow(v, cadExp)
		for j := 0; j < perSpeed; j++ {
			speeds = append(speeds, fmt.Sprintf("%g", v))
			cads = append(cads, fmt.Sprintf("%g", c))
		}
	}
	return "[" + strings.Join(speeds, ",") + "]", "[" + strings.Join(cads, ",") + "]"
}

// seedRunWithStreams posts paired speed+cadence onto a fresh run.
func seedRunWithStreams(t *testing.T, f *fixture, speed, cadence string) string {
	t.Helper()
	id := seedRunWorkout(t, f).String()
	require.Equal(t, http.StatusOK,
		post(t, f.r, "/workouts/"+id+"/streams",
			fmt.Sprintf(`{"speed":%s,"cadence":%s}`, speed, cadence)).Code)
	return id
}

func getStride(t *testing.T, f *fixture, id, query string) (activitystreams.StrideResult, int) {
	t.Helper()
	rec := get(t, f.r, "/workouts/"+id+"/stride?"+query)
	if rec.Code != http.StatusOK {
		return activitystreams.StrideResult{}, rec.Code
	}
	var res activitystreams.StrideResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	return res, rec.Code
}

// ============================================================================

func TestStrideEndpoint_HappyPath(t *testing.T) {
	f := setup(t)
	speed, cadence := runStreams(2.5, 5.0, 0.3, 8) // 30% turnover / 70% step
	id := seedRunWithStreams(t, f, speed, cadence)

	res, code := getStride(t, f, id, "")
	require.Equal(t, http.StatusOK, code)

	assert.Equal(t, id, res.WorkoutID.String())
	require.NotNil(t, res.Contribution)
	assert.Nil(t, res.Reason)
	assert.InDelta(t, 30, res.Contribution.CadencePct, 1.5)
	assert.InDelta(t, 70, res.Contribution.StepPct, 1.5)
	assert.InDelta(t, 100, res.Contribution.CadencePct+res.Contribution.StepPct, 0.11,
		"the split is a partition (1dp rounding tolerance)")

	require.NotEmpty(t, res.Bins)
	assert.Positive(t, res.AnalyzedS)
	assert.Zero(t, res.ExcludedS)
	assert.NotEmpty(t, res.Scatter, "the scatter ships unless summary_only")
	assert.Nil(t, res.MinSpeedMps, "no cutoff was applied")

	// Plausible running numbers: ~1 m steps, cadence in the human range.
	for _, b := range res.Bins {
		assert.Greater(t, b.StepLengthM, 0.5)
		assert.Less(t, b.StepLengthM, 2.5)
		assert.Greater(t, b.CadenceSPM, 60.0)
		assert.Less(t, b.CadenceSPM, 250.0)
	}
}

// The limiter question, end to end: a step-length plateau shows in the bins.
func TestStrideEndpoint_PlateauShowsInBins(t *testing.T) {
	f := setup(t)
	speed, cadence := runStreams(2.5, 5.0, 1.0, 8) // all turnover
	id := seedRunWithStreams(t, f, speed, cadence)

	res, code := getStride(t, f, id, "")
	require.Equal(t, http.StatusOK, code)

	require.NotNil(t, res.Contribution)
	assert.InDelta(t, 100, res.Contribution.CadencePct, 1.5)
	first, last := res.Bins[0], res.Bins[len(res.Bins)-1]
	assert.InDelta(t, first.StepLengthM, last.StepLengthM, 0.03, "step length plateaus")
	assert.Greater(t, last.CadenceSPM, first.CadenceSPM)
}

func TestStrideEndpoint_SteadyRunReturnsBinsWithoutASplit(t *testing.T) {
	f := setup(t)
	speed, cadence := runStreams(3.0, 3.2, 0.3, 20) // 0.2 m/s of spread
	id := seedRunWithStreams(t, f, speed, cadence)

	res, code := getStride(t, f, id, "")
	require.Equal(t, http.StatusOK, code, "a steady run is not an error")

	assert.Nil(t, res.Contribution)
	require.NotNil(t, res.Reason)
	assert.Equal(t, "insufficient_speed_range", *res.Reason)
	assert.NotEmpty(t, res.Bins, "the data that existed is still shown")
}

func TestStrideEndpoint_RideIsRejected(t *testing.T) {
	f := setup(t)
	// The shared seedWorkout is a bike; give it the same paired streams.
	id := seedWorkout(t, f.repo).String()
	speed, cadence := runStreams(2.5, 5.0, 0.3, 4)
	require.Equal(t, http.StatusOK,
		post(t, f.r, "/workouts/"+id+"/streams",
			fmt.Sprintf(`{"speed":%s,"cadence":%s}`, speed, cadence)).Code)

	rec := get(t, f.r, "/workouts/"+id+"/stride")
	require.Equal(t, http.StatusConflict, rec.Code,
		"a ride's cadence × step length is nonsense — 409, not a number")
	assert.JSONEq(t, `{"error":"sport_unsupported"}`, rec.Body.String())
}

func TestStrideEndpoint_MissingStreamSentinels(t *testing.T) {
	f := setup(t)

	t.Run("workout_not_found", func(t *testing.T) {
		rec := get(t, f.r, "/workouts/"+uuid.New().String()+"/stride")
		require.Equal(t, http.StatusNotFound, rec.Code)
		assert.JSONEq(t, `{"error":"workout_not_found"}`, rec.Body.String())
	})

	t.Run("streams_not_found", func(t *testing.T) {
		id := seedRunWorkout(t, f).String() // no streams posted
		rec := get(t, f.r, "/workouts/"+id+"/stride")
		require.Equal(t, http.StatusNotFound, rec.Code)
		assert.JSONEq(t, `{"error":"streams_not_found"}`, rec.Body.String())
	})

	t.Run("cadence_stream_missing", func(t *testing.T) {
		// A run synced before the bridge learned directDoubleCadence: it has
		// streams, just not cadence. Its own sentinel, so "not synced yet"
		// never reads as "no streams at all".
		id := seedRunWorkout(t, f).String()
		require.Equal(t, http.StatusOK,
			post(t, f.r, "/workouts/"+id+"/streams", `{"speed":[3.0,3.1,3.2]}`).Code)
		rec := get(t, f.r, "/workouts/"+id+"/stride")
		require.Equal(t, http.StatusNotFound, rec.Code)
		assert.JSONEq(t, `{"error":"cadence_stream_missing"}`, rec.Body.String())
	})

	t.Run("speed_stream_missing", func(t *testing.T) {
		id := seedRunWorkout(t, f).String()
		require.Equal(t, http.StatusOK,
			post(t, f.r, "/workouts/"+id+"/streams", `{"cadence":[170,172,174]}`).Code)
		rec := get(t, f.r, "/workouts/"+id+"/stride")
		require.Equal(t, http.StatusNotFound, rec.Code)
		assert.JSONEq(t, `{"error":"speed_stream_missing"}`, rec.Body.String())
	})
}

func TestStrideEndpoint_MinSpeedParam(t *testing.T) {
	f := setup(t)
	speed, cadence := runStreams(2.5, 5.0, 0.3, 4)
	// Append a walk break well below the cutoff.
	speed = strings.TrimSuffix(speed, "]") + strings.Repeat(",1.2", 30) + "]"
	cadence = strings.TrimSuffix(cadence, "]") + strings.Repeat(",70", 30) + "]"
	id := seedRunWithStreams(t, f, speed, cadence)

	off, code := getStride(t, f, id, "")
	require.Equal(t, http.StatusOK, code)

	on, code := getStride(t, f, id, "min_speed_mps=1.8")
	require.Equal(t, http.StatusOK, code)

	assert.Equal(t, off.AnalyzedS-30, on.AnalyzedS, "the walk is excluded")
	assert.Equal(t, 30, on.ExcludedS, "and counted")
	require.NotNil(t, on.MinSpeedMps, "the applied cutoff is echoed")
	assert.InDelta(t, 1.8, *on.MinSpeedMps, 0.001)
}

func TestStrideEndpoint_MinSpeedValidation(t *testing.T) {
	f := setup(t)
	speed, cadence := runStreams(2.5, 5.0, 0.3, 4)
	id := seedRunWithStreams(t, f, speed, cadence)

	for _, bad := range []string{"0.4", "5.1", "-1", "0", "abc", "NaN"} {
		rec := get(t, f.r, "/workouts/"+id+"/stride?min_speed_mps="+bad)
		require.Equal(t, http.StatusBadRequest, rec.Code, "min_speed_mps=%s", bad)
		assert.JSONEq(t, `{"error":"min_speed_invalid"}`, rec.Body.String())
	}

	// The bounds themselves are valid.
	for _, ok := range []string{"0.5", "5.0"} {
		_, code := getStride(t, f, id, "min_speed_mps="+ok)
		assert.Equal(t, http.StatusOK, code, "min_speed_mps=%s", ok)
	}
}

func TestStrideEndpoint_SummaryOnlyOmitsScatter(t *testing.T) {
	f := setup(t)
	speed, cadence := runStreams(2.5, 5.0, 0.3, 8)
	id := seedRunWithStreams(t, f, speed, cadence)

	full, code := getStride(t, f, id, "")
	require.Equal(t, http.StatusOK, code)
	require.NotEmpty(t, full.Scatter)

	rec := get(t, f.r, "/workouts/"+id+"/stride?summary_only=true")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), "scatter", "the chart data stays out")

	var summary activitystreams.StrideResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &summary))
	assert.Empty(t, summary.Scatter)
	// The reasoning data survives.
	assert.NotEmpty(t, summary.Bins)
	require.NotNil(t, summary.Contribution)
	assert.InDelta(t, full.Contribution.CadencePct, summary.Contribution.CadencePct, 0.001)
}

func TestStrideEndpoint_RoundsAtTheBoundary(t *testing.T) {
	f := setup(t)
	// 3.333 m/s at 171.7 spm → step 1.1648…; cadence 1dp, step 2dp.
	speed := "[" + strings.Repeat("3.333,", 40) + "3.333]"
	cadence := "[" + strings.Repeat("171.66,", 40) + "171.66]"
	id := seedRunWithStreams(t, f, speed, cadence)

	res, code := getStride(t, f, id, "")
	require.Equal(t, http.StatusOK, code)
	require.NotEmpty(t, res.Bins)

	b := res.Bins[0]
	assert.Equal(t, roundTo(b.CadenceSPM, 1), b.CadenceSPM, "cadence is 1dp")
	assert.Equal(t, roundTo(b.StepLengthM, 2), b.StepLengthM, "step length is 2dp")
	assert.InDelta(t, 171.7, b.CadenceSPM, 0.001)
	assert.InDelta(t, 1.16, b.StepLengthM, 0.001)
}

// roundTo states the property (already rounded → rounding again is a no-op).
func roundTo(v float64, dp int) float64 {
	p := math.Pow(10, float64(dp))
	return math.Round(v*p) / p
}

// The read must not mutate the workout or its streams.
func TestStrideEndpoint_ReadOnly(t *testing.T) {
	f := setup(t)
	speed, cadence := runStreams(2.5, 5.0, 0.3, 4)
	id := seedRunWithStreams(t, f, speed, cadence)

	before := get(t, f.r, "/workouts/"+id+"/streams")
	require.Equal(t, http.StatusOK, before.Code)

	_, code := getStride(t, f, id, "")
	require.Equal(t, http.StatusOK, code)

	after := get(t, f.r, "/workouts/"+id+"/streams")
	require.Equal(t, http.StatusOK, after.Code)
	assert.JSONEq(t, before.Body.String(), after.Body.String(), "streams untouched")

	w, err := f.repo.GetByID(context.Background(), uuid.MustParse(id))
	require.NoError(t, err)
	assert.Equal(t, workouts.SportRun, w.Sport)
	assert.Nil(t, w.TSS, "no derived value was persisted")
}
