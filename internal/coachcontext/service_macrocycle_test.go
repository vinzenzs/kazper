package coachcontext_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/athleteconfig"
	"github.com/vinzenzs/kazper/internal/bodyweight"
	"github.com/vinzenzs/kazper/internal/coachcontext"
	"github.com/vinzenzs/kazper/internal/fitnessmetrics"
	"github.com/vinzenzs/kazper/internal/macrocycle"
	"github.com/vinzenzs/kazper/internal/recoverymetrics"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/trainingphases"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// macroFix wires a coach-context service with the macrocycle repo cross-injected,
// plus the phases + macrocycle repos used to seed the season + its member phases.
type macroFix struct {
	svc    *coachcontext.Service
	pool   *pgxpool.Pool
	phases *trainingphases.PhasesRepo
	macros *macrocycle.Repo
}

func setupMacro(t *testing.T) *macroFix {
	t.Helper()
	pool := storetest.NewPool(t)
	ph := trainingphases.NewPhasesRepo(pool)
	mr := macrocycle.NewRepo(pool)
	svc := coachcontext.NewService(
		workouts.NewRepo(pool), fitnessmetrics.NewRepo(pool), recoverymetrics.NewRepo(pool),
		ph, athleteconfig.NewRepo(pool), bodyweight.NewRepo(pool),
	)
	svc.SetMacrocycleRepo(mr)
	return &macroFix{svc: svc, pool: pool, phases: ph, macros: mr}
}

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// seedRace inserts a race row and returns its id.
func (f *macroFix) seedRace(t *testing.T, raceDate time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := f.pool.Exec(context.Background(),
		`INSERT INTO races (id, name, race_date) VALUES ($1, 'A-race', $2)`, id, raceDate)
	require.NoError(t, err)
	return id
}

// seedSeason inserts a macrocycle covering [start,end] with an optional race anchor.
func (f *macroFix) seedSeason(t *testing.T, start, end time.Time, raceID *uuid.UUID) uuid.UUID {
	t.Helper()
	m := &macrocycle.Macrocycle{Name: "season", StartDate: start, EndDate: end, RaceID: raceID}
	require.NoError(t, f.macros.Insert(context.Background(), m))
	return m.ID
}

// seedPhase inserts a phase optionally linked to a season at an ordinal.
func (f *macroFix) seedPhase(t *testing.T, name string, start, end time.Time, macroID *uuid.UUID, ordinal *int) {
	t.Helper()
	p := &trainingphases.Phase{
		Name: name, Type: trainingphases.PhaseTypeBuild,
		StartDate: start, EndDate: end,
		MacrocycleID: macroID, MacrocycleOrdinal: ordinal,
	}
	require.NoError(t, f.phases.Insert(context.Background(), p))
}

func intp(v int) *int { return &v }

func TestMacrocycle_CoveringWithRaceAnchorAndOrdinal(t *testing.T) {
	f := setupMacro(t)
	anchor := date(2026, 3, 15)
	raceID := f.seedRace(t, date(2026, 9, 27))
	seasonID := f.seedSeason(t, date(2026, 1, 5), date(2026, 12, 31), &raceID)
	// Covering phase is ordinal 2; five more members make total_periods = 6.
	f.seedPhase(t, "build", date(2026, 3, 1), date(2026, 3, 28), &seasonID, intp(2))
	for _, o := range []int{1, 3, 4, 5, 6} {
		f.seedPhase(t, "p", date(2025, 12, 1), date(2025, 12, 2), &seasonID, intp(o))
	}

	out, err := f.svc.BuildTraining(context.Background(), anchor, time.UTC, 0, 0)
	require.NoError(t, err)
	require.NotNil(t, out.Macrocycle)
	assert.Equal(t, seasonID, out.Macrocycle.ID)
	require.NotNil(t, out.Macrocycle.RaceName)
	assert.Equal(t, "A-race", *out.Macrocycle.RaceName)
	require.NotNil(t, out.Macrocycle.DaysToRace)
	assert.Equal(t, 196, *out.Macrocycle.DaysToRace)
	require.NotNil(t, out.Macrocycle.CurrentPhaseOrdinal)
	assert.Equal(t, 2, *out.Macrocycle.CurrentPhaseOrdinal)
	assert.Equal(t, 6, out.Macrocycle.TotalPeriods)
}

func TestMacrocycle_UnanchoredSeasonNullsRaceFields(t *testing.T) {
	f := setupMacro(t)
	anchor := date(2026, 3, 15)
	seasonID := f.seedSeason(t, date(2026, 1, 5), date(2026, 12, 31), nil)
	f.seedPhase(t, "build", date(2026, 3, 1), date(2026, 3, 28), &seasonID, intp(1))

	out, err := f.svc.BuildTraining(context.Background(), anchor, time.UTC, 0, 0)
	require.NoError(t, err)
	require.NotNil(t, out.Macrocycle)
	assert.Nil(t, out.Macrocycle.RaceID)
	assert.Nil(t, out.Macrocycle.RaceName)
	assert.Nil(t, out.Macrocycle.RaceDate)
	assert.Nil(t, out.Macrocycle.DaysToRace)
	require.NotNil(t, out.Macrocycle.CurrentPhaseOrdinal)
	assert.Equal(t, 1, *out.Macrocycle.CurrentPhaseOrdinal)
}

func TestMacrocycle_NoCoveringSeasonIsNull(t *testing.T) {
	f := setupMacro(t)
	// Season is in 2025; the anchor is 2026.
	f.seedSeason(t, date(2025, 1, 1), date(2025, 12, 31), nil)
	out, err := f.svc.BuildTraining(context.Background(), date(2026, 3, 15), time.UTC, 0, 0)
	require.NoError(t, err)
	assert.Nil(t, out.Macrocycle)
}

func TestMacrocycle_OverlappingSeasonsMostRecentlyUpdatedWins(t *testing.T) {
	f := setupMacro(t)
	anchor := date(2026, 3, 15)
	// Two seasons both cover the anchor; B is inserted second → later updated_at.
	f.seedSeason(t, date(2026, 1, 1), date(2026, 12, 31), nil)
	seasonB := f.seedSeason(t, date(2026, 2, 1), date(2026, 11, 30), nil)

	out, err := f.svc.BuildTraining(context.Background(), anchor, time.UTC, 0, 0)
	require.NoError(t, err)
	require.NotNil(t, out.Macrocycle)
	assert.Equal(t, seasonB, out.Macrocycle.ID)
}

func TestMacrocycle_CoveringPhaseOutsideSeasonLeavesOrdinalNull(t *testing.T) {
	f := setupMacro(t)
	anchor := date(2026, 3, 15)
	seasonID := f.seedSeason(t, date(2026, 1, 5), date(2026, 12, 31), nil)
	// Two member phases that do NOT cover the anchor date.
	f.seedPhase(t, "jan", date(2026, 1, 5), date(2026, 1, 31), &seasonID, intp(1))
	f.seedPhase(t, "feb", date(2026, 2, 1), date(2026, 2, 28), &seasonID, intp(2))
	// An unlinked phase that DOES cover the anchor — PhaseFor returns this one.
	f.seedPhase(t, "loose", date(2026, 3, 1), date(2026, 3, 28), nil, nil)

	out, err := f.svc.BuildTraining(context.Background(), anchor, time.UTC, 0, 0)
	require.NoError(t, err)
	require.NotNil(t, out.Macrocycle)
	assert.Nil(t, out.Macrocycle.CurrentPhaseOrdinal)
	assert.Equal(t, 2, out.Macrocycle.TotalPeriods)
}
