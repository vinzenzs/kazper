package workouts_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/workouts"
)

// adhPlan builds a training_plans → plan_weeks → workout_templates chain and
// returns the plan id plus a mkSlot func that creates a fresh plan_slot under
// it (each on-plan workout needs its own slot — workouts.plan_slot_id is
// partial-unique).
func adhPlan(t *testing.T, f *fixture) (planID uuid.UUID, mkSlot func(weekday int) uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	var weekID, templateID uuid.UUID
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO workout_templates (sport, name, steps) VALUES ('run', 'T', '[{"kind":"work"}]'::jsonb) RETURNING id`).Scan(&templateID))
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO training_plans (name, start_date) VALUES ('P', '2026-06-01') RETURNING id`).Scan(&planID))
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO plan_weeks (plan_id, ordinal) VALUES ($1, 1) RETURNING id`, planID).Scan(&weekID))
	mkSlot = func(weekday int) uuid.UUID {
		var slotID uuid.UUID
		require.NoError(t, f.pool.QueryRow(ctx,
			`INSERT INTO plan_slots (plan_week_id, weekday, ordinal, template_id) VALUES ($1, $2, $3, $4) RETURNING id`,
			weekID, weekday, weekday, templateID).Scan(&slotID))
		return slotID
	}
	return planID, mkSlot
}

// adhWorkout inserts a workout row directly so the test controls status,
// timing, slot link, and tss precisely.
func adhWorkout(t *testing.T, f *fixture, sport, status string, slot *uuid.UUID, start time.Time, durMin int, tss *float64) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := f.pool.Exec(context.Background(),
		`INSERT INTO workouts (id, source, sport, status, started_at, ended_at, plan_slot_id, tss)
		 VALUES ($1, 'manual', $2, $3, $4, $5, $6, $7)`,
		id, sport, status, start, start.Add(time.Duration(durMin)*time.Minute), slot, tss)
	require.NoError(t, err)
	return id
}

// ----- 1.2 repo: windowed candidates + plan scoping -----

func TestAdherenceCandidates_WindowAndPlanScope(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	planID, mkSlot := adhPlan(t, f)
	slot := mkSlot(0)

	base := time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)
	// On-plan completed inside the window.
	adhWorkout(t, f, "run", "completed", &slot, base.Add(2*time.Hour), 60, nil)
	// Off-plan completed inside the window (no slot).
	adhWorkout(t, f, "bike", "completed", nil, base.Add(5*time.Hour), 60, nil)
	// Outside the window (the day before) — must be excluded by both queries.
	adhWorkout(t, f, "swim", "completed", nil, base.Add(-3*time.Hour), 60, nil)

	from := base
	to := base.Add(24 * time.Hour) // half-open: excludes the prior-day row

	// Unscoped: both in-window rows, off-plan included; each carries its id and
	// no plan-week provenance (unscoped read has no join).
	all, err := f.repo.AdherenceCandidates(ctx, from, to, nil)
	require.NoError(t, err)
	assert.Len(t, all, 2, "both in-window rows; the prior-day row is outside [from, to)")
	for _, r := range all {
		assert.NotEqual(t, uuid.Nil, r.ID, "id populated on every row")
		assert.Nil(t, r.PlanWeekOrdinal, "unscoped rows carry no plan-week provenance")
		assert.Nil(t, r.PlanStartDate)
	}

	// Plan-scoped: only the on-plan row; off-plan (no-slot) excluded by the join,
	// and the row carries ordinal + plan start_date (phase is null — adhPlan sets
	// no phase_id).
	scoped, err := f.repo.AdherenceCandidates(ctx, from, to, &planID)
	require.NoError(t, err)
	require.Len(t, scoped, 1, "off-plan and out-of-window rows excluded")
	require.NotNil(t, scoped[0].PlanSlotID)
	assert.Equal(t, slot, *scoped[0].PlanSlotID)
	assert.NotEqual(t, uuid.Nil, scoped[0].ID)
	require.NotNil(t, scoped[0].PlanWeekOrdinal)
	assert.Equal(t, 1, *scoped[0].PlanWeekOrdinal)
	require.NotNil(t, scoped[0].PlanStartDate)
	assert.Equal(t, "2026-06-01", scoped[0].PlanStartDate.Format("2006-01-02"))
	assert.Nil(t, scoped[0].PlanPhase, "no phase_id on the plan week → null phase")

	// A foreign plan id matches nothing.
	other, err := f.repo.AdherenceCandidates(ctx, from, to, ptrUUID(uuid.New()))
	require.NoError(t, err)
	assert.Empty(t, other)
}

func ptrUUID(u uuid.UUID) *uuid.UUID { return &u }

// ----- 3.2 endpoint: counts / rate / scoping / validation -----

func seedAdherenceWindow(t *testing.T, f *fixture) (planID uuid.UUID) {
	t.Helper()
	planID, mkSlot := adhPlan(t, f)
	now := time.Now().UTC()
	// completed on-plan (past), missed on-plan (past, still planned),
	// upcoming on-plan (future), unplanned off-plan (past, completed, no slot).
	c := mkSlot(0)
	m := mkSlot(1)
	u := mkSlot(2)
	adhWorkout(t, f, "run", "completed", &c, now.Add(-3*time.Hour), 60, f64p(50))
	adhWorkout(t, f, "bike", "planned", &m, now.Add(-2*time.Hour), 60, f64p(40))
	adhWorkout(t, f, "swim", "planned", &u, now.Add(24*time.Hour), 60, nil)
	adhWorkout(t, f, "run", "completed", nil, now.Add(-1*time.Hour), 45, nil)
	return planID
}

func f64p(v float64) *float64 { return &v }

func adherenceWindowQuery() string {
	now := time.Now().UTC()
	from := now.AddDate(0, 0, -2).Format("2006-01-02")
	to := now.AddDate(0, 0, 2).Format("2006-01-02")
	return fmt.Sprintf("from=%s&to=%s&tz=UTC", from, to)
}

func TestAdherenceEndpoint_CountsAndRate(t *testing.T) {
	f := setup(t)
	seedAdherenceWindow(t, f)

	rec := doReq(t, f.r, http.MethodGet, "/workouts/adherence?"+adherenceWindowQuery(), "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var s workouts.AdherenceSummary
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &s))

	assert.Equal(t, 1, s.Completed)
	assert.Equal(t, 1, s.Missed)
	assert.Equal(t, 1, s.Upcoming)
	assert.Equal(t, 1, s.Unplanned)
	require.NotNil(t, s.AdherenceRate)
	assert.InDelta(t, 0.5, *s.AdherenceRate, 0.0001, "1/(1+1)")
	require.NotNil(t, s.PlannedDurationMin)
	assert.InDelta(t, 120.0, *s.PlannedDurationMin, 0.001, "completed(60) + missed(60)")
	require.NotNil(t, s.CompletedDurationMin)
	assert.InDelta(t, 60.0, *s.CompletedDurationMin, 0.001)
}

func TestAdherenceEndpoint_PlanScopeExcludesUnplanned(t *testing.T) {
	f := setup(t)
	planID := seedAdherenceWindow(t, f)

	rec := doReq(t, f.r, http.MethodGet,
		"/workouts/adherence?"+adherenceWindowQuery()+"&plan_id="+planID.String(), "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var s workouts.AdherenceSummary
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &s))

	assert.Equal(t, 1, s.Completed)
	assert.Equal(t, 1, s.Missed)
	assert.Equal(t, 1, s.Upcoming)
	assert.Equal(t, 0, s.Unplanned, "off-plan work excluded when plan_id is set")
}

func TestAdherenceEndpoint_MissedListAndCalendarTrend(t *testing.T) {
	f := setup(t)
	seedAdherenceWindow(t, f)

	rec := doReq(t, f.r, http.MethodGet, "/workouts/adherence?"+adherenceWindowQuery(), "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var s workouts.AdherenceSummary
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &s))

	// The single missed (bike) session is named; the fulfilled/upcoming/unplanned
	// rows are not, and the small list is not truncated.
	require.Len(t, s.MissedSessions, 1)
	assert.Equal(t, workouts.SportBike, s.MissedSessions[0].Sport)
	assert.InDelta(t, 60.0, s.MissedSessions[0].PlannedDurationMin, 0.001)
	assert.False(t, s.MissedSessionsTruncated)

	// Unscoped → calendar-week buckets: ordinal/phase null, trend reconciles with
	// the top-line completed/missed counts.
	require.NotEmpty(t, s.Weekly)
	var wc, wm int
	for _, w := range s.Weekly {
		assert.Nil(t, w.Ordinal, "calendar mode → null ordinal")
		assert.Nil(t, w.Phase)
		wc += w.Completed
		wm += w.Missed
	}
	assert.Equal(t, s.Completed, wc)
	assert.Equal(t, s.Missed, wm)
}

func TestAdherenceEndpoint_PlanWeekTrend(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	planID := seedAdherenceWindow(t, f)

	// Attach a phase to the plan's single week so it surfaces in the trend
	// (training_phases has no id default — supply one).
	phaseID := uuid.New()
	_, err := f.pool.Exec(ctx,
		`INSERT INTO training_phases (id, name, type, start_date, end_date)
		 VALUES ($1, 'Base', 'base', '2026-06-01', '2026-06-30')`, phaseID)
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx, `UPDATE plan_weeks SET phase_id = $1 WHERE plan_id = $2`, phaseID, planID)
	require.NoError(t, err)

	rec := doReq(t, f.r, http.MethodGet,
		"/workouts/adherence?"+adherenceWindowQuery()+"&plan_id="+planID.String(), "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var s workouts.AdherenceSummary
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &s))

	// One plan week (ordinal 1) → one bucket, keyed by plan week with phase +
	// week_start derived from the plan's start_date (not the workouts' dates).
	require.Len(t, s.Weekly, 1)
	require.NotNil(t, s.Weekly[0].Ordinal)
	assert.Equal(t, 1, *s.Weekly[0].Ordinal)
	require.NotNil(t, s.Weekly[0].Phase)
	assert.Equal(t, "Base", *s.Weekly[0].Phase)
	assert.Equal(t, "2026-06-01", s.Weekly[0].WeekStart)
	assert.Equal(t, 1, s.Weekly[0].Completed)
	assert.Equal(t, 1, s.Weekly[0].Missed)
	// plan-scoped excludes off-plan work, so the missed list is still just the bike.
	require.Len(t, s.MissedSessions, 1)
	assert.Equal(t, workouts.SportBike, s.MissedSessions[0].Sport)
}

func TestAdherenceEndpoint_Validation(t *testing.T) {
	f := setup(t)
	cases := []struct {
		name, query, wantErr string
	}{
		{"missing window", "", "window_required"},
		{"bad date", "from=nope&to=2026-06-16", "window_invalid"},
		{"reversed window", "from=2026-06-16&to=2026-06-10", "window_invalid"},
		{"bad tz", "from=2026-06-10&to=2026-06-16&tz=Mars/Phobos", "tz_invalid"},
		{"bad plan_id", "from=2026-06-10&to=2026-06-16&plan_id=not-a-uuid", "plan_id_invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doReq(t, f.r, http.MethodGet, "/workouts/adherence?"+tc.query, "")
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tc.wantErr, body["error"])
		})
	}
}
