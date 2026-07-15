package dailycontext_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/bodyweight"
	"github.com/vinzenzs/kazper/internal/coachmemory"
	"github.com/vinzenzs/kazper/internal/dailycontext"
	"github.com/vinzenzs/kazper/internal/fitnessmetrics"
	"github.com/vinzenzs/kazper/internal/goals"
	"github.com/vinzenzs/kazper/internal/hydration"
	"github.com/vinzenzs/kazper/internal/hydrationbalance"
	"github.com/vinzenzs/kazper/internal/meals"
	"github.com/vinzenzs/kazper/internal/recoverymetrics"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/summary"
	"github.com/vinzenzs/kazper/internal/supplements"
	"github.com/vinzenzs/kazper/internal/trainingphases"
	"github.com/vinzenzs/kazper/internal/wellness"
	"github.com/vinzenzs/kazper/internal/workoutfuel"
	"github.com/vinzenzs/kazper/internal/workouts"
)

func ptr[T any](v T) *T { return &v }

type fix struct {
	svc              *dailycontext.Service
	meals            *meals.Repo
	hydration        *hydration.Repo
	workouts         *workouts.Repo
	workoutFuel      *workoutfuel.Repo
	bodyWeight       *bodyweight.Repo
	goalsDefault     *goals.Repo
	goalsOverrides   *goals.OverridesRepo
	templates        *trainingphases.TemplatesRepo
	phases           *trainingphases.PhasesRepo
	recovery         *recoverymetrics.Repo
	fitness          *fitnessmetrics.Repo
	hydrationBalance *hydrationbalance.Repo
	coachMemory      *coachmemory.Repo
	wellness         *wellness.Repo
	supplements      *supplements.Repo
}

func setup(t *testing.T) *fix {
	t.Helper()
	pool := storetest.NewPool(t)
	mealsRepo := meals.NewRepo(pool)
	hydrationRepo := hydration.NewRepo(pool)
	workoutsRepo := workouts.NewRepo(pool)
	workoutFuelRepo := workoutfuel.NewRepo(pool)
	bodyWeightRepo := bodyweight.NewRepo(pool)
	goalsRepo := goals.NewRepo(pool)
	overridesRepo := goals.NewOverridesRepo(pool)
	tplRepo := trainingphases.NewTemplatesRepo(pool)
	phRepo := trainingphases.NewPhasesRepo(pool)
	resolver := goals.NewResolver(
		goalsRepo, overridesRepo,
		trainingphases.NewPhaseLookupAdapter(phRepo),
		trainingphases.NewTemplateLookupAdapter(tplRepo),
	)
	summarySvc := summary.NewService(pool, mealsRepo, resolver)
	recoveryRepo := recoverymetrics.NewRepo(pool)
	fitnessRepo := fitnessmetrics.NewRepo(pool)
	hydrationBalRepo := hydrationbalance.NewRepo(pool)
	coachMemoryRepo := coachmemory.NewRepo(pool)
	wellnessRepo := wellness.NewRepo(pool)
	supplementsRepo := supplements.NewRepo(pool)
	svc := dailycontext.NewService(
		summarySvc, hydrationRepo, workoutsRepo, workoutFuelRepo,
		bodyWeightRepo, overridesRepo, phRepo,
		recoveryRepo, fitnessRepo, hydrationBalRepo,
		coachMemoryRepo, wellnessRepo, supplementsRepo,
	)
	return &fix{
		svc:              svc,
		recovery:         recoveryRepo,
		fitness:          fitnessRepo,
		hydrationBalance: hydrationBalRepo,
		meals:            mealsRepo,
		hydration:        hydrationRepo,
		workouts:         workoutsRepo,
		workoutFuel:      workoutFuelRepo,
		bodyWeight:       bodyWeightRepo,
		goalsDefault:     goalsRepo,
		goalsOverrides:   overridesRepo,
		templates:        tplRepo,
		phases:           phRepo,
		coachMemory:      coachMemoryRepo,
		wellness:         wellnessRepo,
		supplements:      supplementsRepo,
	}
}

// TestBuildFor_HappyPath_AllSlicesPopulated is the "shape integrity" guard
// the design called out: seed every slice and assert every nested field.
func TestBuildFor_HappyPath_AllSlicesPopulated(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, loc)
	dayMid := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)

	// 2 meals via freeform Insert (snapshot nutriments).
	for _, kcal := range []float64{300, 500} {
		_, err := f.meals.Insert(ctx, meals.InsertParams{
			LoggedAt:     dayMid,
			QuantityG:    150,
			SnapshotName: ptr("Test meal"),
			SnapshotNutriments: meals.Nutriments{
				KcalPer100g:     ptr(kcal),
				ProteinGPer100g: ptr(20.0),
			},
		})
		require.NoError(t, err)
	}

	// 3 hydration entries totalling 1500ml.
	for _, ml := range []float64{500, 500, 500} {
		require.NoError(t, f.hydration.Insert(ctx, &hydration.Entry{
			LoggedAt:   dayMid,
			QuantityMl: ml,
		}))
	}

	// 1 workout: 60-minute ride, 600 kcal.
	wkID := uuid.New()
	_, err := f.workouts.Upsert(ctx, &workouts.Workout{
		ID:         wkID,
		Source:     workouts.SourceManual,
		Sport:      workouts.SportBike,
		StartedAt:  time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC),
		EndedAt:    time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC),
		KcalBurned: ptr(600.0),
	})
	require.NoError(t, err)

	// 2 workout-fuel entries: one linked to the workout, one freestanding.
	require.NoError(t, f.workoutFuel.Insert(ctx, &workoutfuel.Entry{
		LoggedAt:  time.Date(2026, 7, 15, 9, 30, 0, 0, time.UTC),
		Name:      "gel",
		CarbsG:    ptr(25.0),
		WorkoutID: &wkID,
	}))
	require.NoError(t, f.workoutFuel.Insert(ctx, &workoutfuel.Entry{
		LoggedAt: dayMid,
		Name:     "post-ride drink",
		CarbsG:   ptr(40.0),
		SodiumMg: ptr(300.0),
	}))

	// Body weight: 1 fresh today + 1 prior 5 days back.
	require.NoError(t, f.bodyWeight.Insert(ctx, &bodyweight.Entry{
		LoggedAt: time.Date(2026, 7, 10, 7, 0, 0, 0, time.UTC),
		WeightKg: 71.0,
	}))
	require.NoError(t, f.bodyWeight.Insert(ctx, &bodyweight.Entry{
		LoggedAt:   time.Date(2026, 7, 15, 7, 0, 0, 0, time.UTC),
		WeightKg:   70.5,
		BodyFatPct: ptr(14.2),
	}))

	// Template + phase covering today.
	tpl, err := f.templates.Upsert(ctx, &trainingphases.Template{
		Name: "build-default",
		Kcal: &goals.Range{Min: ptr(2400.0), Max: ptr(2600.0)},
	})
	require.NoError(t, err)
	tid := tpl.ID
	require.NoError(t, f.phases.Insert(ctx, &trainingphases.Phase{
		Name:              "build-block",
		Type:              trainingphases.PhaseTypeBuild,
		StartDate:         time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndDate:           time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
		DefaultTemplateID: &tid,
	}))

	// Override on today (wins over phase).
	require.NoError(t, f.goalsOverrides.Upsert(ctx, date, &goals.Goals{
		Kcal:   &goals.Range{Min: ptr(2800.0)},
		CarbsG: &goals.Range{Min: ptr(700.0)},
	}))

	out, err := f.svc.BuildFor(ctx, date, loc)
	require.NoError(t, err)
	require.NotNil(t, out)

	// Top-level echoes.
	assert.Equal(t, "2026-07-15", out.Date)
	assert.Equal(t, "UTC", out.TZ)

	// Adherence: override beats phase, so goal_source=override.
	assert.Equal(t, "override", out.Adherence.GoalSource)
	assert.Empty(t, out.Adherence.PhaseName)
	assert.Contains(t, out.Adherence.Adherence, "kcal")

	// Nutrition: 2 meals.
	assert.Equal(t, 2, out.Nutrition.EntriesCount)
	assert.Greater(t, out.Nutrition.Totals.Kcal, 0.0)

	// Hydration: 1500ml across 3 entries.
	assert.InDelta(t, 1500.0, out.Hydration.TotalMl, 0.001)
	assert.Equal(t, 3, out.Hydration.EntriesCount)

	// Workouts: one bike workout, 60 min.
	require.Len(t, out.Workouts, 1)
	w := out.Workouts[0]
	assert.Equal(t, "bike", w.Sport)
	assert.InDelta(t, 60.0, w.DurationMin, 0.001)
	require.NotNil(t, w.KcalBurned)
	assert.InDelta(t, 600.0, *w.KcalBurned, 0.001)

	// Workout-fuel: 2 entries.
	assert.Len(t, out.WorkoutFuel, 2)

	// Weight: fresh same-day entry, NOT carryover.
	require.NotNil(t, out.Weight)
	assert.False(t, out.Weight.IsCarryover)
	assert.InDelta(t, 70.5, out.Weight.WeightKg, 0.001)
	require.NotNil(t, out.Weight.BodyFatPct)
	assert.InDelta(t, 14.2, *out.Weight.BodyFatPct, 0.001)

	// Phase: the build-block phase row.
	require.NotNil(t, out.Phase)
	assert.Equal(t, "build-block", out.Phase.Name)
	assert.Equal(t, trainingphases.PhaseTypeBuild, out.Phase.Type)
	require.NotNil(t, out.Phase.DefaultTemplateName)
	assert.Equal(t, "build-default", *out.Phase.DefaultTemplateName)

	// Goal override: present with kcal+carbs.
	assert.True(t, out.GoalOverride.Present)
	require.NotNil(t, out.GoalOverride.Goals)
	require.NotNil(t, out.GoalOverride.Goals.Kcal)
	assert.InDelta(t, 2800.0, *out.GoalOverride.Goals.Kcal.Min, 0.001)
}

// TestBuildFor_EmptyDay returns the bundle with empty arrays and nulls.
func TestBuildFor_EmptyDay(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, loc)

	out, err := f.svc.BuildFor(ctx, date, loc)
	require.NoError(t, err)

	assert.Equal(t, "2026-07-15", out.Date)
	assert.Equal(t, "none", out.Adherence.GoalSource)
	assert.Empty(t, out.Adherence.PhaseName)
	assert.Equal(t, 0, out.Nutrition.EntriesCount)
	assert.InDelta(t, 0.0, out.Hydration.TotalMl, 0.001)
	assert.Equal(t, 0, out.Hydration.EntriesCount)
	// Empty arrays, NOT nil — agents branch on length.
	require.NotNil(t, out.Workouts)
	assert.Empty(t, out.Workouts)
	require.NotNil(t, out.WorkoutFuel)
	assert.Empty(t, out.WorkoutFuel)
	assert.Nil(t, out.Weight)
	assert.Nil(t, out.Phase)
	assert.False(t, out.GoalOverride.Present)
	assert.Nil(t, out.GoalOverride.Goals)
	// Memory is an empty (non-nil) array on a quiet day.
	require.NotNil(t, out.Memory)
	assert.Empty(t, out.Memory)
}

// TestBuildFor_FoldsInActiveMemory is the fold-in payoff: a dateless standing
// item rides the daily context, a same-day recommendation rides it, an
// out-of-window recommendation does not, an expired item is excluded, and an
// item past its review date is flagged needs_review.
func TestBuildFor_FoldsInActiveMemory(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, loc)

	mk := func(in coachmemory.CreateInput) {
		_, err := coachmemory.NewService(f.coachMemory).Create(ctx, in)
		require.NoError(t, err)
	}
	// Standing constraint, past its review date → needs_review.
	mk(coachmemory.CreateInput{Kind: "constraint", Text: "knee niggle", ReviewAt: ptr("2026-07-10")})
	// Recommendation dated to the requested day → included.
	mk(coachmemory.CreateInput{Kind: "recommendation", Text: "220g carbs", Date: ptr("2026-07-15")})
	// Recommendation on another day → excluded.
	mk(coachmemory.CreateInput{Kind: "recommendation", Text: "old advice", Date: ptr("2026-07-01")})
	// Expired observation → excluded.
	mk(coachmemory.CreateInput{Kind: "observation", Text: "stale", ExpiresAt: ptr("2026-07-01")})

	out, err := f.svc.BuildFor(ctx, date, loc)
	require.NoError(t, err)

	texts := map[string]bool{}
	var flaggedKnee bool
	for _, m := range out.Memory {
		texts[m.Text] = true
		if m.Text == "knee niggle" {
			flaggedKnee = m.NeedsReview
		}
	}
	assert.True(t, texts["knee niggle"], "standing item should ride the daily context")
	assert.True(t, texts["220g carbs"], "same-day recommendation should be included")
	assert.False(t, texts["old advice"], "out-of-window recommendation should be excluded")
	assert.False(t, texts["stale"], "expired item should be excluded")
	assert.True(t, flaggedKnee, "item past review_at should be flagged needs_review")
}

// TestBuildFor_WeightCarryover_PriorEntry: no entry today, one 5 days back.
func TestBuildFor_WeightCarryover_PriorEntry(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, loc)

	priorTS := time.Date(2026, 7, 10, 7, 0, 0, 0, time.UTC)
	require.NoError(t, f.bodyWeight.Insert(ctx, &bodyweight.Entry{
		LoggedAt: priorTS,
		WeightKg: 71.2,
	}))

	out, err := f.svc.BuildFor(ctx, date, loc)
	require.NoError(t, err)
	require.NotNil(t, out.Weight)
	assert.True(t, out.Weight.IsCarryover)
	assert.InDelta(t, 71.2, out.Weight.WeightKg, 0.001)
	assert.Equal(t, priorTS.UTC(), out.Weight.LoggedAt.UTC())
}

// TestBuildFor_PhaseDrivesAdherenceWhenNoOverride: phase template wins
// adherence; goal_source=phase_template; phase block also populated.
func TestBuildFor_PhaseDrivesAdherenceWhenNoOverride(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, loc)

	tpl, err := f.templates.Upsert(ctx, &trainingphases.Template{
		Name: "build-default",
		Kcal: &goals.Range{Min: ptr(2400.0), Max: ptr(2600.0)},
	})
	require.NoError(t, err)
	tid := tpl.ID
	require.NoError(t, f.phases.Insert(ctx, &trainingphases.Phase{
		Name:              "build-block",
		Type:              trainingphases.PhaseTypeBuild,
		StartDate:         time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndDate:           time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
		DefaultTemplateID: &tid,
	}))

	out, err := f.svc.BuildFor(ctx, date, loc)
	require.NoError(t, err)
	assert.Equal(t, "phase_template", out.Adherence.GoalSource)
	assert.Equal(t, "build-block", out.Adherence.PhaseName)
	require.NotNil(t, out.Phase)
	assert.Equal(t, "build-block", out.Phase.Name)
}

// TestBuildFor_PhasePersistsEvenWhenOverrideWins: phase block populated
// even when goal_source=override (the phase still covers the date).
func TestBuildFor_PhasePersistsEvenWhenOverrideWins(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, loc)

	tpl, err := f.templates.Upsert(ctx, &trainingphases.Template{
		Name: "build-default", Kcal: &goals.Range{Min: ptr(2400.0)},
	})
	require.NoError(t, err)
	tid := tpl.ID
	require.NoError(t, f.phases.Insert(ctx, &trainingphases.Phase{
		Name:              "build-block",
		Type:              trainingphases.PhaseTypeBuild,
		StartDate:         time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndDate:           time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
		DefaultTemplateID: &tid,
	}))
	require.NoError(t, f.goalsOverrides.Upsert(ctx, date, &goals.Goals{
		Kcal: &goals.Range{Min: ptr(2800.0)},
	}))

	out, err := f.svc.BuildFor(ctx, date, loc)
	require.NoError(t, err)
	assert.Equal(t, "override", out.Adherence.GoalSource)
	assert.Empty(t, out.Adherence.PhaseName)
	// Phase block still populated.
	require.NotNil(t, out.Phase)
	assert.Equal(t, "build-block", out.Phase.Name)
}

// TestBuildFor_NoGoroutineLeak_UnderRace runs the happy-path fixture many
// times to validate the errgroup composition under -race. Goroutine leaks
// or racy access would show up as a flake or `go test -race` complaint.
func TestBuildFor_NoGoroutineLeak_UnderRace(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, loc)

	// Light fixture — just enough that all goroutines have work.
	require.NoError(t, f.hydration.Insert(context.Background(), &hydration.Entry{
		LoggedAt: time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC), QuantityMl: 500,
	}))
	for i := 0; i < 50; i++ {
		_, err := f.svc.BuildFor(ctx, date, loc)
		require.NoError(t, err)
	}
}

// TestBuildFor_RecoveryAndFitnessAndWeightBiometrics covers the
// add-garmin-daily-metrics additions: same-day recovery + fitness snapshots
// surface, absence yields null (no carryover), and the weight block echoes the
// new smart-scale biometrics.
func TestBuildFor_RecoveryAndFitnessPresent(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, loc)

	_, err := f.recovery.Upsert(ctx, &recoverymetrics.Snapshot{
		Date: "2026-07-15", SleepSeconds: ptr(27000), RestingHR: ptr(48),
	})
	require.NoError(t, err)
	_, err = f.fitness.Upsert(ctx, &fitnessmetrics.Snapshot{
		Date: "2026-07-15", VO2MaxRunning: ptr(54.0),
	})
	require.NoError(t, err)
	// Weight entry on the day with smart-scale biometrics.
	require.NoError(t, f.bodyWeight.Insert(ctx, &bodyweight.Entry{
		LoggedAt: time.Date(2026, 7, 15, 7, 0, 0, 0, loc), WeightKg: 72.5,
		MuscleMassKg: ptr(58.4), BMI: ptr(22.4),
	}))

	out, err := f.svc.BuildFor(ctx, date, loc)
	require.NoError(t, err)

	require.NotNil(t, out.Recovery)
	require.NotNil(t, out.Recovery.RestingHR)
	assert.Equal(t, 48, *out.Recovery.RestingHR)
	require.NotNil(t, out.Fitness)
	require.NotNil(t, out.Fitness.VO2MaxRunning)
	assert.InDelta(t, 54.0, *out.Fitness.VO2MaxRunning, 0.05)
	require.NotNil(t, out.Weight)
	require.NotNil(t, out.Weight.MuscleMassKg)
	assert.InDelta(t, 58.4, *out.Weight.MuscleMassKg, 0.05)
	require.NotNil(t, out.Weight.BMI)
	assert.InDelta(t, 22.4, *out.Weight.BMI, 0.05)
}

func TestBuildFor_RecoveryAndFitnessNullWhenAbsentNoCarryover(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC

	// Snapshot exists for the PRIOR day only — must NOT carry over.
	_, err := f.recovery.Upsert(ctx, &recoverymetrics.Snapshot{Date: "2026-07-14", RestingHR: ptr(50)})
	require.NoError(t, err)
	_, err = f.fitness.Upsert(ctx, &fitnessmetrics.Snapshot{Date: "2026-07-14", VO2MaxRunning: ptr(53.0)})
	require.NoError(t, err)

	out, err := f.svc.BuildFor(ctx, time.Date(2026, 7, 15, 0, 0, 0, 0, loc), loc)
	require.NoError(t, err)
	assert.Nil(t, out.Recovery, "recovery is same-day-or-null, never carried over")
	assert.Nil(t, out.Fitness, "fitness is same-day-or-null, never carried over")
}

// TestBuildFor_HydrationBalancePresent covers the add-hydration-balance-metrics
// addition: the same-day snapshot surfaces in its own block, stays distinct from
// the logged-intake hydration block, and is null (no carryover) when absent.
func TestBuildFor_HydrationBalancePresent(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, loc)

	_, err := f.hydrationBalance.Upsert(ctx, &hydrationbalance.Snapshot{
		Date: "2026-07-15", SweatLossML: ptr(2400.0), ActivityIntakeML: ptr(1800.0), GoalML: ptr(3000.0),
	})
	require.NoError(t, err)
	// A logged hydration entry on the same day — the two blocks must stay distinct.
	require.NoError(t, f.hydration.Insert(ctx, &hydration.Entry{
		LoggedAt: time.Date(2026, 7, 15, 9, 0, 0, 0, loc), QuantityMl: 500,
	}))

	out, err := f.svc.BuildFor(ctx, date, loc)
	require.NoError(t, err)

	require.NotNil(t, out.HydrationBalance)
	require.NotNil(t, out.HydrationBalance.SweatLossML)
	assert.InDelta(t, 2400, *out.HydrationBalance.SweatLossML, 0.05)
	// Distinct from the logged-intake block.
	assert.InDelta(t, 500, out.Hydration.TotalMl, 0.05)
	assert.Equal(t, 1, out.Hydration.EntriesCount)
}

func TestBuildFor_HydrationBalanceNullWhenAbsentNoCarryover(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC

	// Snapshot exists for the PRIOR day only — must NOT carry over.
	_, err := f.hydrationBalance.Upsert(ctx, &hydrationbalance.Snapshot{Date: "2026-07-14", SweatLossML: ptr(2200.0)})
	require.NoError(t, err)

	out, err := f.svc.BuildFor(ctx, time.Date(2026, 7, 15, 0, 0, 0, 0, loc), loc)
	require.NoError(t, err)
	assert.Nil(t, out.HydrationBalance, "hydration_balance is same-day-or-null, never carried over")
}

// TestBuildFor_WellnessPresent covers add-wellness-diary: today's subjective
// entry surfaces verbatim in its own block, beside the objective recovery
// snapshot, and the rest of the payload is unaffected.
func TestBuildFor_WellnessPresent(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, loc)

	require.NoError(t, f.wellness.Upsert(ctx, date, &wellness.Entry{
		Fatigue: ptr(3), Note: ptr("legs heavy despite good TSB"),
	}))
	// An objective recovery snapshot on the same day — subjective + objective
	// must read side by side.
	_, err := f.recovery.Upsert(ctx, &recoverymetrics.Snapshot{Date: "2026-07-15", RestingHR: ptr(48)})
	require.NoError(t, err)

	out, err := f.svc.BuildFor(ctx, date, loc)
	require.NoError(t, err)

	require.NotNil(t, out.Wellness)
	require.NotNil(t, out.Wellness.Fatigue)
	assert.Equal(t, 3, *out.Wellness.Fatigue)
	require.NotNil(t, out.Wellness.Note)
	assert.Equal(t, "legs heavy despite good TSB", *out.Wellness.Note)
	assert.Nil(t, out.Wellness.Mood, "unreported score stays nil")
	// Rest of the payload unaffected.
	require.NotNil(t, out.Recovery)
	require.NotNil(t, out.Recovery.RestingHR)
	assert.Equal(t, 48, *out.Recovery.RestingHR)
}

func TestBuildFor_WellnessOmittedWhenUnlogged(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC

	// An entry exists for the PRIOR day only — today omits the field (no carryover).
	require.NoError(t, f.wellness.Upsert(ctx, time.Date(2026, 7, 14, 0, 0, 0, 0, loc), &wellness.Entry{Mood: ptr(4)}))

	out, err := f.svc.BuildFor(ctx, time.Date(2026, 7, 15, 0, 0, 0, 0, loc), loc)
	require.NoError(t, err)
	assert.Nil(t, out.Wellness, "wellness is same-day-or-omitted, never carried over")

	// Confirm the JSON payload has no `wellness` key at all (omitempty).
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), `"wellness"`)
}

func TestBuildFor_SupplementsPresent(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, loc)

	// Two supplements logged today; one before the window (yesterday) must NOT show.
	_, err := f.supplements.Insert(ctx, &supplements.Entry{
		LoggedAt: time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC), Name: "creatine", Dose: ptr(5.0), DoseUnit: ptr("g"),
	})
	require.NoError(t, err)
	_, err = f.supplements.Insert(ctx, &supplements.Entry{
		LoggedAt: time.Date(2026, 7, 15, 20, 0, 0, 0, time.UTC), Name: "magnesium",
	})
	require.NoError(t, err)
	_, err = f.supplements.Insert(ctx, &supplements.Entry{
		LoggedAt: time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC), Name: "yesterday",
	})
	require.NoError(t, err)

	out, err := f.svc.BuildFor(ctx, date, loc)
	require.NoError(t, err)
	require.Len(t, out.Supplements, 2, "only today's entries")
	assert.Equal(t, "creatine", out.Supplements[0].Name)
	assert.Equal(t, "magnesium", out.Supplements[1].Name)
}

func TestBuildFor_SupplementsOmittedWhenNone(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	loc := time.UTC
	out, err := f.svc.BuildFor(ctx, time.Date(2026, 7, 15, 0, 0, 0, 0, loc), loc)
	require.NoError(t, err)
	assert.Nil(t, out.Supplements)
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), `"supplements"`)
}
