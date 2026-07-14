package activitystreams_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/activitystreams"
)

// segArr builds a JSON power array from {watts, seconds} segments.
func segArr(segs ...[2]int) string {
	var b strings.Builder
	b.WriteByte('[')
	first := true
	for _, s := range segs {
		for i := 0; i < s[1]; i++ {
			if !first {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, "%d", s[0])
			first = false
		}
	}
	b.WriteByte(']')
	return b.String()
}

func seedStructuredPower(t *testing.T, f *fixture, arrJSON string) string {
	t.Helper()
	id := seedWorkout(t, f.repo).String()
	require.Equal(t, http.StatusOK,
		post(t, f.r, "/workouts/"+id+"/streams", fmt.Sprintf(`{"power":%s}`, arrJSON)).Code)
	return id
}

func getIntervals(t *testing.T, f *fixture, id string) (activitystreams.IntervalsResult, int) {
	t.Helper()
	rec := get(t, f.r, "/workouts/"+id+"/intervals")
	if rec.Code != http.StatusOK {
		return activitystreams.IntervalsResult{}, rec.Code
	}
	var res activitystreams.IntervalsResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	return res, rec.Code
}

func TestIntervalsEndpoint_StructuredRide(t *testing.T) {
	f := setup(t)
	// 5×4-min @ 300 W, 2-min recoveries @ 120 W, easy lead-in/out.
	segs := [][2]int{{120, 120}}
	for k := 0; k < 5; k++ {
		segs = append(segs, [2]int{300, 240})
		if k < 4 {
			segs = append(segs, [2]int{120, 120})
		}
	}
	segs = append(segs, [2]int{120, 120})
	id := seedStructuredPower(t, f, segArr(segs...))

	res, code := getIntervals(t, f, id)
	require.Equal(t, http.StatusOK, code)
	require.Len(t, res.Intervals, 5)
	require.NotNil(t, res.ThresholdW)
	assert.Empty(t, res.Reason)
	assert.Equal(t, 5, res.Summary.Count)
	assert.Len(t, res.Rests, 4)
	// Rounded watts at the boundary (whole numbers).
	assert.Equal(t, res.Intervals[0].AvgW, float64(int(res.Intervals[0].AvgW)))
}

func TestIntervalsEndpoint_SteadyRide(t *testing.T) {
	f := setup(t)
	// A flat endurance hour at one power — no work/rest split to find.
	id := seedStructuredPower(t, f, segArr([2]int{200, 3600}))

	res, code := getIntervals(t, f, id)
	require.Equal(t, http.StatusOK, code)
	assert.Empty(t, res.Intervals)
	assert.Nil(t, res.ThresholdW)
	assert.Equal(t, "no_distinct_efforts", res.Reason)
}

func TestIntervalsEndpoint_NotFoundDistinctions(t *testing.T) {
	f := setup(t)
	// Unknown workout → workout_not_found.
	rec := get(t, f.r, "/workouts/"+seedWorkoutMissing()+"/intervals")
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "workout_not_found")

	// Known workout, no stored streams → streams_not_found.
	bare := seedWorkout(t, f.repo).String()
	rec = get(t, f.r, "/workouts/"+bare+"/intervals")
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "streams_not_found")

	// Streams present but no power (HR only) → power_stream_missing.
	hrOnly := seedWorkout(t, f.repo).String()
	require.Equal(t, http.StatusOK,
		post(t, f.r, "/workouts/"+hrOnly+"/streams", fmt.Sprintf(`{"heart_rate":%s}`, arr(1800, 150))).Code)
	rec = get(t, f.r, "/workouts/"+hrOnly+"/intervals")
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "power_stream_missing")
}

func TestIntervalsEndpoint_ReadIsIdempotentNoMutation(t *testing.T) {
	f := setup(t)
	id := seedStructuredPower(t, f,
		segArr([2]int{120, 120}, [2]int{300, 240}, [2]int{120, 120}, [2]int{300, 240}, [2]int{120, 120}))
	a := get(t, f.r, "/workouts/"+id+"/intervals")
	b := get(t, f.r, "/workouts/"+id+"/intervals")
	assert.Equal(t, a.Body.String(), b.Body.String())
	// Unit isolation: no nutrition fields on this workout-analytics read.
	assert.NotContains(t, a.Body.String(), "kcal")
	assert.NotContains(t, a.Body.String(), "protein")
}
