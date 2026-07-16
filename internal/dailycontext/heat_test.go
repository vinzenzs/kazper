package dailycontext_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/heat"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// fakeHeat returns a canned report per workout id.
type fakeHeat struct {
	reports map[uuid.UUID]*heat.Report
	err     error
	calls   int
}

func (f *fakeHeat) ReportFor(_ context.Context, id uuid.UUID) (*heat.Report, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.reports[id], nil
}

// planOn materializes a planned session on `date`.
func planOn(t *testing.T, f *fix, date time.Time) uuid.UUID {
	t.Helper()
	start := time.Date(date.Year(), date.Month(), date.Day(), 9, 0, 0, 0, time.UTC)
	w := &workouts.Workout{
		Source:    workouts.SourceManual,
		Sport:     workouts.SportBike,
		Status:    workouts.StatusPlanned,
		StartedAt: start,
		EndedAt:   start.Add(2 * time.Hour),
	}
	_, err := f.workouts.Upsert(context.Background(), w)
	require.NoError(t, err)
	return w.ID
}

// hotReport is a computable heat answer.
func hotReport(id uuid.UUID, date string, loadC float64) *heat.Report {
	return &heat.Report{
		WorkoutID:  id,
		Date:       date,
		Location:   &heat.Location{Name: "Mallorca", Source: "travel", Lat: 39.57, Lon: 2.65},
		Load:       &heat.Load{HeatLoadC: loadC},
		Acclim:     &heat.AcclimEvidence{Level: heat.AcclimGood, Count: 6},
		Adjustment: &heat.Adjustment{ReductionPct: 6.5},
	}
}

// ============================================================================

func TestBuildFor_HeatBlock_TodayAndTomorrow(t *testing.T) {
	f := setup(t)
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	todayID := planOn(t, f, date)
	tomorrowID := planOn(t, f, date.AddDate(0, 0, 1))

	f.svc.SetHeatProvider(&fakeHeat{reports: map[uuid.UUID]*heat.Report{
		todayID:    hotReport(todayID, "2026-07-15", 26.0),
		tomorrowID: hotReport(tomorrowID, "2026-07-16", 33.0),
	}})

	out, err := f.svc.BuildFor(context.Background(), date, time.UTC)
	require.NoError(t, err)

	require.NotNil(t, out.Heat)
	require.NotNil(t, out.Heat.Today)
	assert.Equal(t, todayID, out.Heat.Today.WorkoutID)
	assert.InDelta(t, 26.0, out.Heat.Today.HeatLoadC, 0.01)

	require.NotNil(t, out.Heat.Tomorrow)
	assert.Equal(t, tomorrowID, out.Heat.Tomorrow.WorkoutID)
	assert.InDelta(t, 33.0, out.Heat.Tomorrow.HeatLoadC, 0.01)
	assert.Equal(t, "2026-07-16", out.Heat.Tomorrow.Date)
	assert.Equal(t, "Mallorca", out.Heat.Tomorrow.LocationName, "the resolved location is visible")
	assert.Equal(t, "good", out.Heat.Tomorrow.Acclimatization)
	assert.InDelta(t, 6.5, out.Heat.Tomorrow.ReductionPct, 0.01)
}

func TestBuildFor_HeatBlock_OmittedWithoutProvider(t *testing.T) {
	f := setup(t)
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	planOn(t, f, date)

	out, err := f.svc.BuildFor(context.Background(), date, time.UTC)
	require.NoError(t, err)

	assert.Nil(t, out.Heat)
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), `"heat"`)
}

// Nothing planned → nothing to say → no block (and no wasted heat call).
func TestBuildFor_HeatBlock_NoPlannedSessionsOmitsBlock(t *testing.T) {
	f := setup(t)
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	fake := &fakeHeat{reports: map[uuid.UUID]*heat.Report{}}
	f.svc.SetHeatProvider(fake)

	out, err := f.svc.BuildFor(context.Background(), date, time.UTC)
	require.NoError(t, err)

	assert.Nil(t, out.Heat)
	assert.Zero(t, fake.calls, "no planned session, nothing to ask about")
}

// An indoor session has no heat question: not_applicable must not become a
// block entry with zeroed numbers.
func TestBuildFor_HeatBlock_IndoorSessionOmitted(t *testing.T) {
	f := setup(t)
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	id := planOn(t, f, date)
	f.svc.SetHeatProvider(&fakeHeat{reports: map[uuid.UUID]*heat.Report{
		id: {WorkoutID: id, Date: "2026-07-15", NotApplicable: true},
	}})

	out, err := f.svc.BuildFor(context.Background(), date, time.UTC)
	require.NoError(t, err)
	assert.Nil(t, out.Heat, "an indoor day says nothing worth a key")
}

// A degraded report (no location, no forecast) is not an answer either.
func TestBuildFor_HeatBlock_DegradedReportOmitted(t *testing.T) {
	for _, reason := range []string{"location_unconfigured", "weather_unavailable"} {
		t.Run(reason, func(t *testing.T) {
			f := setup(t)
			date := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
			id := planOn(t, f, date)
			r := reason
			f.svc.SetHeatProvider(&fakeHeat{reports: map[uuid.UUID]*heat.Report{
				id: {WorkoutID: id, Date: "2026-07-15", Reason: &r},
			}})

			out, err := f.svc.BuildFor(context.Background(), date, time.UTC)
			require.NoError(t, err)
			assert.Nil(t, out.Heat)
		})
	}
}

// Heat is supplementary: its failure must not fail the check-in bundle.
func TestBuildFor_HeatBlock_ErrorOmitsBlockAndKeepsPayload(t *testing.T) {
	f := setup(t)
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	planOn(t, f, date)
	f.svc.SetHeatProvider(&fakeHeat{err: errors.New("forecast exploded")})

	out, err := f.svc.BuildFor(context.Background(), date, time.UTC)

	require.NoError(t, err, "a heat failure is not a check-in failure")
	assert.Nil(t, out.Heat)
	assert.Equal(t, "2026-07-15", out.Date)
	assert.NotNil(t, out.Workouts)
}

// One computable day is enough for the block; the other side stays absent.
func TestBuildFor_HeatBlock_PartialDaysKeepTheBlock(t *testing.T) {
	f := setup(t)
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	tomorrowID := planOn(t, f, date.AddDate(0, 0, 1))
	f.svc.SetHeatProvider(&fakeHeat{reports: map[uuid.UUID]*heat.Report{
		tomorrowID: hotReport(tomorrowID, "2026-07-16", 34.0),
	}})

	out, err := f.svc.BuildFor(context.Background(), date, time.UTC)
	require.NoError(t, err)

	require.NotNil(t, out.Heat)
	assert.Nil(t, out.Heat.Today, "nothing planned today")
	require.NotNil(t, out.Heat.Tomorrow)
	assert.InDelta(t, 34.0, out.Heat.Tomorrow.HeatLoadC, 0.01)
}

// assumed_outdoor must survive into the block — a suggestion resting on an
// assumption should carry it.
func TestBuildFor_HeatBlock_AssumedOutdoorSurvives(t *testing.T) {
	f := setup(t)
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	id := planOn(t, f, date)
	rep := hotReport(id, "2026-07-15", 31.0)
	rep.AssumedOutdoor = true
	f.svc.SetHeatProvider(&fakeHeat{reports: map[uuid.UUID]*heat.Report{id: rep}})

	out, err := f.svc.BuildFor(context.Background(), date, time.UTC)
	require.NoError(t, err)

	require.NotNil(t, out.Heat)
	require.NotNil(t, out.Heat.Today)
	assert.True(t, out.Heat.Today.AssumedOutdoor)
}
