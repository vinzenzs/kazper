package workouts_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/workouts"
)

// buildSlotChain creates a plan→week→slot→template chain WITHOUT materializing,
// returning the slot and template ids. (seedPlannedFromSlot materializes; here
// the caller materializes via ReconcileSlotOrUpsertPlanned to exercise adoption.)
func buildSlotChain(t *testing.T, f *fixture, sport string) (planSlotID, templateID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	var planID, weekID uuid.UUID
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO workout_templates (sport, name, steps) VALUES ($1, 'T', '[{"kind":"work"}]'::jsonb) RETURNING id`,
		sport).Scan(&templateID))
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO training_plans (name, start_date) VALUES ('P', '2026-06-01') RETURNING id`).Scan(&planID))
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO plan_weeks (plan_id, ordinal) VALUES ($1, 1) RETURNING id`, planID).Scan(&weekID))
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO plan_slots (plan_week_id, weekday, template_id) VALUES ($1, 0, $2) RETURNING id`,
		weekID, templateID).Scan(&planSlotID))
	return planSlotID, templateID
}

// reconcileSlot materializes a single-sport slot via the reverse-reconciling path.
func reconcileSlot(t *testing.T, f *fixture, sport string, start time.Time, slotID, templateID uuid.UUID) *workouts.Workout {
	t.Helper()
	w, err := f.repo.ReconcileSlotOrUpsertPlanned(context.Background(), f.pool, workouts.PlannedSlotInput{
		PlanSlotID: slotID,
		TemplateID: &templateID,
		Sport:      sport,
		StartedAt:  start,
		EndedAt:    start.Add(time.Hour),
	}, time.UTC)
	require.NoError(t, err)
	return w
}

// ----- 1.3 forward ±1-day tolerance -----

func TestIngest_CrossDayByOneFulfillsViaTolerance(t *testing.T) {
	f := setup(t)
	// Planned run on reconDay+1; the activity lands one day earlier with no
	// same-day candidate → the adjacent-day planned is fulfilled.
	planned, slotID, _ := seedPlannedFromSlot(t, f, "run", at(24+18))
	merged := ingestGarmin(t, f, "garmin:xday", "run", at(7), http.StatusOK)
	assert.Equal(t, planned.ID, merged.ID, "adjacent-day planned fulfilled via ±1 tolerance")
	require.NotNil(t, merged.PlanSlotID)
	assert.Equal(t, slotID, *merged.PlanSlotID)
	assert.Equal(t, workouts.StatusCompleted, merged.Status)
}

func TestIngest_PrefersSameDayOverAdjacent(t *testing.T) {
	f := setup(t)
	sameDay := postPlanned(t, f, "run", at(18))   // reconDay
	adjacent := postPlanned(t, f, "run", at(24+18)) // reconDay+1
	merged := ingestGarmin(t, f, "garmin:pref", "run", at(7), http.StatusOK)
	assert.Equal(t, sameDay.ID, merged.ID, "same-day candidate preferred, not treated as ambiguous")
	// The adjacent planned is untouched.
	got, code := getWorkout(t, f, adjacent.ID)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, workouts.StatusPlanned, got.Status)
	assert.Nil(t, got.ExternalID)
}

// ----- 3.x reverse reconciliation at materialize -----

func TestReverse_MaterializeAdoptsExistingImport(t *testing.T) {
	f := setup(t)
	// A completed garmin run imported before any plan → standalone.
	imp := ingestGarmin(t, f, "garmin:rev1", "run", at(7), http.StatusCreated)
	require.Nil(t, imp.PlanSlotID)
	require.Equal(t, 1, countDay(t, f, "completed"))

	// Materialize a matching run slot the same day → adopts the import.
	slotID, tmplID := buildSlotChain(t, f, "run")
	w := reconcileSlot(t, f, "run", at(18), slotID, tmplID)
	assert.Equal(t, imp.ID, w.ID, "adopted the existing import, not a new row")
	assert.Equal(t, workouts.StatusCompleted, w.Status)
	require.NotNil(t, w.PlanSlotID)
	assert.Equal(t, slotID, *w.PlanSlotID)
	require.NotNil(t, w.TemplateID)
	assert.Equal(t, tmplID, *w.TemplateID)
	assert.False(t, w.NeedsLink)
	assert.Equal(t, 1, countDay(t, f, ""), "no duplicate row created")
	assert.Equal(t, 0, countDay(t, f, "planned"))
}

func TestReverse_AdjacentDayAdoptedAndSameDayPreferred(t *testing.T) {
	t.Run("adjacent-day adoption when no same-day", func(t *testing.T) {
		f := setup(t)
		imp := ingestGarmin(t, f, "garmin:adj", "bike", at(7), http.StatusCreated) // reconDay
		slotID, tmplID := buildSlotChain(t, f, "bike")
		w := reconcileSlot(t, f, "bike", at(24+9), slotID, tmplID) // slot on reconDay+1
		assert.Equal(t, imp.ID, w.ID, "adjacent-day import adopted via ±1 tolerance")
		require.NotNil(t, w.PlanSlotID)
	})

	t.Run("same-day import preferred over adjacent", func(t *testing.T) {
		f := setup(t)
		adjacent := ingestGarmin(t, f, "garmin:msd-adj", "run", at(24+7), http.StatusCreated) // reconDay+1
		sameDay := ingestGarmin(t, f, "garmin:msd-same", "run", at(7), http.StatusCreated)    // reconDay
		slotID, tmplID := buildSlotChain(t, f, "run")
		w := reconcileSlot(t, f, "run", at(18), slotID, tmplID) // slot on reconDay
		assert.Equal(t, sameDay.ID, w.ID, "same-day import adopted, not the adjacent one")
		// The adjacent import stays standalone.
		got, code := getWorkout(t, f, adjacent.ID)
		require.Equal(t, http.StatusOK, code)
		assert.Nil(t, got.PlanSlotID)
	})
}

func TestReverse_DeclinesOnMoreThanOneCandidate(t *testing.T) {
	f := setup(t)
	// Two same-day completed imports → ambiguous → reverse declines.
	ingestGarmin(t, f, "garmin:two-a", "run", at(7), http.StatusCreated)
	ingestGarmin(t, f, "garmin:two-b", "run", at(9), http.StatusCreated)
	slotID, tmplID := buildSlotChain(t, f, "run")
	w := reconcileSlot(t, f, "run", at(18), slotID, tmplID)

	assert.Equal(t, workouts.StatusPlanned, w.Status, "a planned row is created, not an adoption")
	require.NotNil(t, w.PlanSlotID)
	assert.Equal(t, slotID, *w.PlanSlotID)
	assert.Equal(t, 2, countDay(t, f, "completed"), "both imports left standalone")
	assert.Equal(t, 1, countDay(t, f, "planned"))
}

func TestReverse_ReMaterializeIsIdempotent(t *testing.T) {
	f := setup(t)
	imp := ingestGarmin(t, f, "garmin:idem", "run", at(7), http.StatusCreated)
	slotID, tmplID := buildSlotChain(t, f, "run")

	first := reconcileSlot(t, f, "run", at(18), slotID, tmplID)
	assert.Equal(t, imp.ID, first.ID)
	// Re-materialize: the slot already owns the adopted completed row → guarded
	// path skips it; no duplicate, completed row unchanged.
	second := reconcileSlot(t, f, "run", at(18), slotID, tmplID)
	assert.Equal(t, imp.ID, second.ID)
	assert.Equal(t, workouts.StatusCompleted, second.Status)
	assert.Equal(t, 1, countDay(t, f, ""), "still one row")
}

func TestReverse_MultisportSlotNeverAdopts(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	// A completed garmin activity exists, but a multisport slot must not adopt it.
	ingestGarmin(t, f, "garmin:ms", "run", at(7), http.StatusCreated)

	var planID, weekID, slotID, msID uuid.UUID
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO multisport_templates (name, segments) VALUES ('brick', '[{"sport":"run","steps":[{"type":"step","intent":"active","duration":{"kind":"time","seconds":600},"target":{"kind":"none"}}]},{"sport":"bike","steps":[{"type":"step","intent":"active","duration":{"kind":"time","seconds":600},"target":{"kind":"none"}}]}]'::jsonb) RETURNING id`).Scan(&msID))
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO training_plans (name, start_date) VALUES ('P', '2026-06-01') RETURNING id`).Scan(&planID))
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO plan_weeks (plan_id, ordinal) VALUES ($1, 1) RETURNING id`, planID).Scan(&weekID))
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO plan_slots (plan_week_id, weekday, multisport_template_id) VALUES ($1, 0, $2) RETURNING id`,
		weekID, msID).Scan(&slotID))

	name := "brick"
	w, err := f.repo.ReconcileSlotOrUpsertPlanned(ctx, f.pool, workouts.PlannedSlotInput{
		PlanSlotID:           slotID,
		MultisportTemplateID: &msID,
		Sport:                string(workouts.SportMultisport),
		Name:                 &name,
		StartedAt:            at(18),
		EndedAt:              at(20),
	}, time.UTC)
	require.NoError(t, err)
	assert.Equal(t, workouts.StatusPlanned, w.Status, "multisport slot materializes a planned row")
	assert.Equal(t, workouts.SportMultisport, w.Sport)
	require.NotNil(t, w.MultisportTemplateID)
	assert.Equal(t, 1, countDay(t, f, "completed"), "the run import is untouched")
}
