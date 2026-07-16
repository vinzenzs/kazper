package heat_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/heat"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// warmingMorningJSON is the shape that produced the live bug: cool before dawn,
// hot by mid-morning. Hours are UTC; the athlete is in Vienna (+2 in July).
func warmingMorningJSON() string {
	// local hour → °C (Vienna). 05:00 local ≈ 16 °C, 10:00 local ≈ 30 °C.
	byLocalHour := map[int]float64{
		0: 15, 1: 15, 2: 15, 3: 14, 4: 14, 5: 16, 6: 18, 7: 21, 8: 24,
		9: 27, 10: 30, 11: 32, 12: 33, 13: 34, 14: 34, 15: 33, 16: 32,
		17: 30, 18: 28, 19: 25, 20: 22, 21: 20, 22: 18, 23: 16,
	}
	var times, temps, hums, winds, clouds []string
	for utcH := 0; utcH <= 23; utcH++ {
		localH := (utcH + 2) % 24
		times = append(times, fmt.Sprintf(`"2026-07-20T%02d:00"`, utcH))
		temps = append(temps, fmt.Sprintf("%v", byLocalHour[localH]))
		hums = append(hums, "55")
		winds = append(winds, "1")
		clouds = append(clouds, "0")
	}
	return fmt.Sprintf(`{"hourly":{"time":[%s],"temperature_2m":[%s],"relative_humidity_2m":[%s],"wind_speed_10m":[%s],"cloud_cover":[%s]}}`,
		strings.Join(times, ","), strings.Join(temps, ","), strings.Join(hums, ","),
		strings.Join(winds, ","), strings.Join(clouds, ","))
}

func viennaLoc(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Vienna")
	require.NoError(t, err)
	return loc
}

// scheduleByDate reproduces the ad-hoc scheduling path: `time.Parse("2006-01-02")`
// → UTC midnight, no time of day.
func scheduleByDate(t *testing.T, f *fixture, mins float64) uuid.UUID {
	t.Helper()
	date, err := time.Parse("2006-01-02", "2026-07-20")
	require.NoError(t, err)
	require.Equal(t, time.UTC, date.Location(), "the scheduling path yields UTC midnight")

	w := &workouts.Workout{
		Source:      workouts.SourceManual,
		Sport:       workouts.SportBike,
		Status:      workouts.StatusPlanned,
		StartedAt:   date,
		EndedAt:     date.Add(time.Duration(mins) * time.Minute),
		Environment: outdoor(),
	}
	_, err = f.workoutsRepo.Upsert(context.Background(), w)
	require.NoError(t, err)
	return w.ID
}

func getHeatAt(t *testing.T, f *fixture, id uuid.UUID, query string) heat.Report {
	t.Helper()
	path := "/workouts/" + id.String() + "/heat"
	if query != "" {
		path += "?" + query
	}
	rec := doGet(t, f.r, path)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out heat.Report
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out
}

// setupVienna mounts the heat endpoint for a Vienna athlete with a 06:00
// habitual start.
func setupVienna(t *testing.T, body string) *fixture {
	t.Helper()
	f := setup(t, body)
	f.svc.SetTrainingStart(viennaLoc(t), 6, 0)
	return f
}

// ============================================================================

// The live finding, reproduced: a date-only scheduled session must NOT be
// scored at the pre-dawn hours its stored midnight implies.
func TestAnchoring_MidnightScheduledSessionScoresTheHabitualStart(t *testing.T) {
	f := setupVienna(t, warmingMorningJSON())
	id := scheduleByDate(t, f, 150)

	out := getHeatAt(t, f, id, "")

	assert.Equal(t, heat.StartAssumed, out.StartSource)
	assert.Equal(t, "06:00", out.AssumedStart,
		"the assumed hour itself must be visible, so a wrong default is self-evident")
	require.NotNil(t, out.Window)
	assert.Contains(t, out.Window.From, "T06:00", "scored from the habitual 06:00 local")

	// 06:00–08:30 local ≈ (18+21+24)/3 = 21 °C, not the ~15 °C of 02:00–04:30.
	require.NotNil(t, out.Conditions)
	assert.InDelta(t, 21, out.Conditions.TemperatureC, 1.5)
	assert.Greater(t, out.Conditions.TemperatureC, 17.0,
		"the pre-dawn under-read is the bug this fixes")
}

// The coach's actual question: early vs late. Two calls, two answers.
func TestAnchoring_StartParamAnswersTheEarlyVsLateQuestion(t *testing.T) {
	f := setupVienna(t, warmingMorningJSON())
	id := scheduleByDate(t, f, 150)

	early := getHeatAt(t, f, id, "start=06:00")
	late := getHeatAt(t, f, id, "start=10:00")

	assert.Equal(t, heat.StartFromParam, early.StartSource)
	assert.Empty(t, early.AssumedStart, "an explicit ask is not an assumption")
	assert.Equal(t, heat.StartFromParam, late.StartSource)

	// 10:00–12:30 local ≈ (30+32+33)/3 ≈ 31.7 °C vs 06:00's ~21 °C.
	require.NotNil(t, late.Conditions)
	assert.InDelta(t, 31.7, late.Conditions.TemperatureC, 1.5)
	assert.Greater(t, late.Load.HeatLoadC, early.Load.HeatLoadC+8,
		"a late start is materially hotter — the whole point of the param")
	assert.Greater(t, late.Adjustment.ReductionPct, early.Adjustment.ReductionPct,
		"and it should cost more of the target")

	require.NotNil(t, late.Window)
	assert.Contains(t, late.Window.From, "T10:00")
}

// A session that carries a real time is scored at that time, no assumption.
func TestAnchoring_RealStartTimeIsHonoured(t *testing.T) {
	f := setupVienna(t, warmingMorningJSON())
	loc := viennaLoc(t)
	start := time.Date(2026, 7, 20, 12, 0, 0, 0, loc) // a real midday session
	w := &workouts.Workout{
		Source:      workouts.SourceManual,
		Sport:       workouts.SportBike,
		Status:      workouts.StatusPlanned,
		StartedAt:   start,
		EndedAt:     start.Add(2 * time.Hour),
		Environment: outdoor(),
	}
	_, err := f.workoutsRepo.Upsert(context.Background(), w)
	require.NoError(t, err)

	out := getHeatAt(t, f, w.ID, "")

	assert.Equal(t, heat.StartFromWorkout, out.StartSource)
	assert.Empty(t, out.AssumedStart, "a stated time assumes nothing")
	require.NotNil(t, out.Conditions)
	// 12:00–14:00 local ≈ (33+34+34)/3 ≈ 33.7 °C.
	assert.InDelta(t, 33.7, out.Conditions.TemperatureC, 1.5)
}

func TestAnchoring_StartParamValidation(t *testing.T) {
	f := setupVienna(t, warmingMorningJSON())
	id := scheduleByDate(t, f, 150)

	for _, bad := range []string{"25:00", "10:70", "10", "abc", "10:00:00"} {
		rec := doGet(t, f.r, "/workouts/"+id.String()+"/heat?start="+bad)
		require.Equal(t, http.StatusBadRequest, rec.Code, "start=%s", bad)
		assert.Contains(t, rec.Body.String(), "start_invalid")
	}

	// A valid param still works after the rejections.
	ok := getHeatAt(t, f, id, "start=07:15")
	assert.Equal(t, heat.StartFromParam, ok.StartSource)
	assert.Contains(t, ok.Window.From, "T07:15")
}

// The duration moves with the anchor: a 2.5 h session stays 2.5 h.
func TestAnchoring_DurationSurvivesReanchoring(t *testing.T) {
	f := setupVienna(t, warmingMorningJSON())
	id := scheduleByDate(t, f, 150)

	out := getHeatAt(t, f, id, "start=09:00")

	assert.InDelta(t, 150, out.DurationMin, 0.1)
	assert.Contains(t, out.Window.From, "T09:00")
	assert.Contains(t, out.Window.To, "T11:30")
}

// The prior degradations and the 409 must be untouched by the anchoring change.
func TestAnchoring_PriorBehaviourUnchanged(t *testing.T) {
	t.Run("indoor still not_applicable", func(t *testing.T) {
		f := setupVienna(t, warmingMorningJSON())
		id := planSession(t, f, workouts.SportBike, 120, indoor())
		out := getHeatAt(t, f, id, "")
		assert.True(t, out.NotApplicable)
		assert.Zero(t, *f.calls, "still no forecast for an indoor session")
	})

	t.Run("completed still 409", func(t *testing.T) {
		f := setupVienna(t, warmingMorningJSON())
		id := completedHot(t, f, 1, 120, 30, outdoor())
		rec := doGet(t, f.r, "/workouts/"+id.String()+"/heat")
		assert.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("weather_unavailable still degrades", func(t *testing.T) {
		f := setupVienna(t, "")
		id := scheduleByDate(t, f, 150)
		out := getHeatAt(t, f, id, "")
		require.NotNil(t, out.Reason)
		assert.Equal(t, "weather_unavailable", *out.Reason)
		// The window it tried to score is still reported.
		assert.NotNil(t, out.Window)
		assert.Equal(t, heat.StartAssumed, out.StartSource)
	})
}

// An unwired service (no SetTrainingStart) must still behave sanely rather than
// anchoring at hour zero.
func TestAnchoring_UnconfiguredDefaultsToSixUTC(t *testing.T) {
	f := setup(t, warmingMorningJSON()) // no SetTrainingStart
	id := scheduleByDate(t, f, 120)

	out := getHeatAt(t, f, id, "")

	assert.Equal(t, heat.StartAssumed, out.StartSource)
	require.NotNil(t, out.Window)
	assert.Contains(t, out.Window.From, "T06:00", "falls back to 06:00 in the default zone")
}

// The echo names the default that applied, so a wrong config is self-evident
// rather than hidden behind a number.
func TestAnchoring_AssumedStartEchoesTheConfiguredDefault(t *testing.T) {
	f := setup(t, warmingMorningJSON())
	f.svc.SetTrainingStart(viennaLoc(t), 17, 30) // an evening athlete
	id := scheduleByDate(t, f, 60)

	out := getHeatAt(t, f, id, "")

	assert.Equal(t, "17:30", out.AssumedStart)
	require.NotNil(t, out.Window)
	assert.Contains(t, out.Window.From, "T17:30")
	// 17:30–18:30 local ≈ 30/28 °C — an evening read, not a dawn one.
	assert.Greater(t, out.Conditions.TemperatureC, 25.0)
}
