package dailycontext_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/fuelplan"
)

// fakeFuelPlan stands in for the planned-workouts / weight-trend / goals stack
// so the fold's own behaviour is what's under test.
type fakeFuelPlan struct {
	plan   *fuelplan.Plan
	err    error
	called bool
	params fuelplan.Params
}

func (f *fakeFuelPlan) PlanFor(_ context.Context, p fuelplan.Params) (*fuelplan.Plan, error) {
	f.called = true
	f.params = p
	return f.plan, f.err
}

func fuelDay(date string, tier fuelplan.Tier, gPerKg float64, grams *float64, planMissing bool) fuelplan.Day {
	return fuelplan.Day{
		Date:            date,
		Tier:            tier,
		CarbsGPerKg:     gPerKg,
		SuggestedCarbsG: grams,
		PlanMissing:     planMissing,
		Sessions:        []fuelplan.Session{},
	}
}

func f64(v float64) *float64 { return &v }

// The check-in reads today and tomorrow at a glance: easy today, heavy
// tomorrow — the "front-load tonight" conversation.
func TestBuildFor_FuelPlanBlock_TodayAndTomorrow(t *testing.T) {
	f := setup(t)
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)

	fake := &fakeFuelPlan{plan: &fuelplan.Plan{
		From: "2026-07-15",
		To:   "2026-07-16",
		Days: []fuelplan.Day{
			fuelDay("2026-07-15", fuelplan.TierEasy, 5, f64(350), false),
			fuelDay("2026-07-16", fuelplan.TierHeavy, 9, f64(630), false),
		},
	}}
	f.svc.SetFuelPlanProvider(fake)

	out, err := f.svc.BuildFor(context.Background(), date, time.UTC)
	require.NoError(t, err)

	require.NotNil(t, out.FuelPlan)
	require.NotNil(t, out.FuelPlan.Today)
	assert.Equal(t, "2026-07-15", out.FuelPlan.Today.Date)
	assert.Equal(t, "easy", out.FuelPlan.Today.Tier)
	assert.Equal(t, 5.0, out.FuelPlan.Today.CarbsGPerKg)
	require.NotNil(t, out.FuelPlan.Today.SuggestedCarbsG)
	assert.Equal(t, 350.0, *out.FuelPlan.Today.SuggestedCarbsG)

	require.NotNil(t, out.FuelPlan.Tomorrow)
	assert.Equal(t, "2026-07-16", out.FuelPlan.Tomorrow.Date)
	assert.Equal(t, "heavy", out.FuelPlan.Tomorrow.Tier)
	assert.Equal(t, 9.0, out.FuelPlan.Tomorrow.CarbsGPerKg)
	require.NotNil(t, out.FuelPlan.Tomorrow.SuggestedCarbsG)
	assert.Equal(t, 630.0, *out.FuelPlan.Tomorrow.SuggestedCarbsG)

	// The fold asks for exactly today + tomorrow, in the caller's timezone.
	assert.True(t, fake.called)
	assert.Equal(t, date, fake.params.From)
	assert.Equal(t, date.AddDate(0, 0, 1), fake.params.To)
	assert.Equal(t, time.UTC, fake.params.Loc)
}

func TestBuildFor_FuelPlanBlock_OmittedWithoutProvider(t *testing.T) {
	f := setup(t)
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)

	out, err := f.svc.BuildFor(context.Background(), date, time.UTC)
	require.NoError(t, err)

	assert.Nil(t, out.FuelPlan)
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), `"fuel_plan"`, "never an empty key")
}

// A fuel-plan failure must not take the whole check-in down with it: the block
// is supplementary, the bundle is not.
func TestBuildFor_FuelPlanBlock_ErrorOmitsBlockAndKeepsPayload(t *testing.T) {
	f := setup(t)
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	f.svc.SetFuelPlanProvider(&fakeFuelPlan{err: errors.New("weight trend exploded")})

	out, err := f.svc.BuildFor(context.Background(), date, time.UTC)

	require.NoError(t, err, "a fuel-plan failure is not a check-in failure")
	assert.Nil(t, out.FuelPlan)
	// The rest of the payload is intact.
	assert.Equal(t, "2026-07-15", out.Date)
	assert.NotNil(t, out.Workouts)
	assert.NotNil(t, out.Memory)
}

func TestBuildFor_FuelPlanBlock_EmptyPlanOmitsBlock(t *testing.T) {
	f := setup(t)
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	f.svc.SetFuelPlanProvider(&fakeFuelPlan{plan: &fuelplan.Plan{Days: []fuelplan.Day{}}})

	out, err := f.svc.BuildFor(context.Background(), date, time.UTC)
	require.NoError(t, err)
	assert.Nil(t, out.FuelPlan)
}

// No weight data: the tier still ships, the gram target doesn't.
func TestBuildFor_FuelPlanBlock_WeightMissingKeepsTier(t *testing.T) {
	f := setup(t)
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	f.svc.SetFuelPlanProvider(&fakeFuelPlan{plan: &fuelplan.Plan{
		Days: []fuelplan.Day{
			fuelDay("2026-07-15", fuelplan.TierModerate, 7, nil, false),
			fuelDay("2026-07-16", fuelplan.TierRest, 3, nil, false),
		},
	}})

	out, err := f.svc.BuildFor(context.Background(), date, time.UTC)
	require.NoError(t, err)

	require.NotNil(t, out.FuelPlan)
	require.NotNil(t, out.FuelPlan.Today)
	assert.Equal(t, "moderate", out.FuelPlan.Today.Tier)
	assert.Equal(t, 7.0, out.FuelPlan.Today.CarbsGPerKg)
	assert.Nil(t, out.FuelPlan.Today.SuggestedCarbsG)

	raw, err := json.Marshal(out.FuelPlan)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "suggested_carbs_g")
}

// plan_missing must survive into the compact block — otherwise tomorrow's
// "rest" reads as a planned rest day when the plan simply doesn't reach it.
func TestBuildFor_FuelPlanBlock_PlanMissingSurvives(t *testing.T) {
	f := setup(t)
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	f.svc.SetFuelPlanProvider(&fakeFuelPlan{plan: &fuelplan.Plan{
		Days: []fuelplan.Day{
			fuelDay("2026-07-15", fuelplan.TierEasy, 5, f64(350), false),
			fuelDay("2026-07-16", fuelplan.TierRest, 3, f64(210), true),
		},
	}})

	out, err := f.svc.BuildFor(context.Background(), date, time.UTC)
	require.NoError(t, err)

	require.NotNil(t, out.FuelPlan)
	assert.False(t, out.FuelPlan.Today.PlanMissing)
	require.NotNil(t, out.FuelPlan.Tomorrow)
	assert.True(t, out.FuelPlan.Tomorrow.PlanMissing)

	// A planned rest day carries no flag at all, so the two are distinguishable
	// on the wire.
	raw, err := json.Marshal(out.FuelPlan.Today)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "plan_missing")
}

// A one-day plan (defensive: the provider is free to return fewer days) yields
// today without a tomorrow, not a panic.
func TestBuildFor_FuelPlanBlock_SingleDayPlan(t *testing.T) {
	f := setup(t)
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	f.svc.SetFuelPlanProvider(&fakeFuelPlan{plan: &fuelplan.Plan{
		Days: []fuelplan.Day{fuelDay("2026-07-15", fuelplan.TierEasy, 5, f64(350), false)},
	}})

	out, err := f.svc.BuildFor(context.Background(), date, time.UTC)
	require.NoError(t, err)

	require.NotNil(t, out.FuelPlan)
	assert.NotNil(t, out.FuelPlan.Today)
	assert.Nil(t, out.FuelPlan.Tomorrow)
}
