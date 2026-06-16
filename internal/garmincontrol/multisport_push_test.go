package garmincontrol_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/trainingplan"
	"github.com/vinzenzs/kazper/internal/workouts"
	wt "github.com/vinzenzs/kazper/internal/workouttemplates"
)

// plannedMultisportWorkout is a materialized multisport brick row: sport
// "multisport", a multisport_template_id, no single-sport template_id.
func plannedMultisportWorkout() *workouts.Workout {
	msID := uuid.New()
	name := "Brick"
	return &workouts.Workout{
		ID: uuid.New(), Status: workouts.StatusPlanned, Sport: workouts.SportMultisport,
		Name:                 &name,
		MultisportTemplateID: &msID,
		StartedAt:            time.Date(2026, 6, 1, 6, 0, 0, 0, time.UTC),
		EndedAt:              time.Date(2026, 6, 1, 7, 0, 0, 0, time.UTC),
	}
}

// brickSegments is a bike→T2→run effective program with already-resolved targets.
func brickSegments() []trainingplan.ProgramSegment {
	lo, hi := 230, 268
	return []trainingplan.ProgramSegment{
		{Sport: wt.SportBike, Steps: []wt.Step{{Type: wt.NodeStep, Intent: wt.IntentActive,
			Duration: &wt.Duration{Kind: wt.DurationTime, Seconds: ptr(1800)},
			Target:   &wt.Target{Kind: wt.TargetPowerW, Low: &lo, High: &hi, Origin: "Z4"}}}},
		{Sport: "transition", Duration: &wt.Duration{Kind: wt.DurationTime, Seconds: ptr(300)}},
		{Sport: wt.SportRun, Steps: []wt.Step{{Type: wt.NodeStep, Intent: wt.IntentActive,
			Duration: &wt.Duration{Kind: wt.DurationTime, Seconds: ptr(1200)},
			Target:   &wt.Target{Kind: wt.TargetNone}}}},
	}
}

// TestScheduleWorkout_MultisportSendsSegmentForm asserts a materialized
// multisport workout pushes the multi-segment bridge form (segments in order),
// not the single-sport steps form — reusing Phase 1's multi-segment compile.
func TestScheduleWorkout_MultisportSendsSegmentForm(t *testing.T) {
	bridge := newBridgeStub(t)
	fw := newFakeWorkouts()
	w := plannedMultisportWorkout()
	fw.rows[w.ID] = w
	r := newEngine(bridge.server.URL, fw, &fakeTemplates{}, &fakePlan{segments: brickSegments()})

	rec := req(t, r, http.MethodPost, "/garmin/schedule/workout", `{"workout_id":"`+w.ID.String()+`"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// One create (the multisport form) + one schedule call.
	assert.Equal(t, 1, bridge.createCalls)
	assert.Equal(t, 1, bridge.schedCalls)
	// The bridge body is the multi-segment form: a segments array carrying each
	// leg's sport in order (bike, transition, run), not a flat single-sport step
	// list with a top-level "sport".
	assert.Contains(t, bridge.createBody, `"segments"`)
	assert.Contains(t, bridge.createBody, `"bike"`)
	assert.Contains(t, bridge.createBody, `"transition"`)
	assert.Contains(t, bridge.createBody, `"run"`)
	assert.NotContains(t, bridge.createBody, `"sport":"multisport"`)
	// The resolved bike target rides along verbatim.
	assert.Contains(t, bridge.createBody, `"power_w"`)

	// Garmin ids stored back on the workout.
	got, err := fw.GetByID(context.Background(), w.ID)
	require.NoError(t, err)
	require.NotNil(t, got.GarminWorkoutID)
	assert.Equal(t, "gw-1", *got.GarminWorkoutID)
}
