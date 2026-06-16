package coachcontext

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/vinzenzs/kazper/internal/workouts"
)

func wkt(sport workouts.Sport, durMin int, msTemplate *uuid.UUID) *workouts.Workout {
	start := time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC)
	return &workouts.Workout{
		ID:                   uuid.New(),
		Sport:                sport,
		Status:               workouts.StatusCompleted,
		StartedAt:            start,
		EndedAt:              start.Add(time.Duration(durMin) * time.Minute),
		MultisportTemplateID: msTemplate,
	}
}

func TestSummarize_MultisportDecomposesBySport(t *testing.T) {
	tid := uuid.New()
	resolve := func(id string) ([]string, bool) {
		if id == tid.String() {
			return []string{"swim", "bike", "run"}, true
		}
		return nil, false
	}
	ws := []*workouts.Workout{
		wkt(workouts.SportRun, 60, nil),       // plain run
		wkt(workouts.SportMultisport, 120, &tid), // brick → swim+bike+run
	}
	s := summarize(ws, resolve)

	assert.Equal(t, 2, s.Count, "two sessions")
	assert.Equal(t, 2, s.BySport["run"], "1 plain run + 1 from the brick")
	assert.Equal(t, 1, s.BySport["swim"])
	assert.Equal(t, 1, s.BySport["bike"])
	assert.Zero(t, s.BySport["multisport"], "the brick is decomposed, not bucketed")
	assert.Zero(t, s.BySport["transition"], "transitions never appear")
	assert.InDelta(t, 180.0, s.TotalDurationMin, 0.001, "duration still sums both sessions")
}

func TestSummarize_MultisportFallsBackWhenUnresolved(t *testing.T) {
	tid := uuid.New()
	resolveMiss := func(string) ([]string, bool) { return nil, false } // template gone / repo unset
	ws := []*workouts.Workout{wkt(workouts.SportMultisport, 90, &tid)}
	s := summarize(ws, resolveMiss)

	assert.Equal(t, 1, s.Count)
	assert.Equal(t, 1, s.BySport["multisport"], "unresolved brick falls back to the multisport bucket")
}

func TestSummarize_NilResolverAndSingleSportUnaffected(t *testing.T) {
	ws := []*workouts.Workout{
		wkt(workouts.SportRun, 45, nil),
		wkt(workouts.SportBike, 90, nil),
	}
	s := summarize(ws, nil) // nil resolver must be safe
	assert.Equal(t, 1, s.BySport["run"])
	assert.Equal(t, 1, s.BySport["bike"])
	assert.Equal(t, 2, s.Count)
}

func TestSummarize_MultisportWithoutTemplateIDStaysBucketed(t *testing.T) {
	// A multisport row with no template id can't be decomposed → multisport bucket.
	resolve := func(string) ([]string, bool) { return []string{"swim"}, true }
	ws := []*workouts.Workout{wkt(workouts.SportMultisport, 60, nil)}
	s := summarize(ws, resolve)
	assert.Equal(t, 1, s.BySport["multisport"])
}
