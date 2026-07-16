package racepacing_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/heat"
	"github.com/vinzenzs/kazper/internal/racepacing"
)

// fakeRaceHeat returns a canned race-heat picture.
type fakeRaceHeat struct {
	rh    *heat.RaceHeat
	calls int
}

func (f *fakeRaceHeat) RaceHeatFor(_ context.Context, _ string, _, _ time.Time) *heat.RaceHeat {
	f.calls++
	return f.rh
}

func hotRaceHeat(loadC float64, level heat.Acclimatization) *heat.RaceHeat {
	at := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	return &heat.RaceHeat{
		LoadC:           loadC,
		HeatIndexC:      loadC - 1,
		Conditions:      &heat.Conditions{TemperatureC: 33, HumidityPct: 60},
		Acclimatization: &heat.AcclimEvidence{Level: level, Count: 6},
		Location:        "Palma, Spain",
		ForecastAt:      &at,
	}
}

func reasonRaceHeat(reason string) *heat.RaceHeat {
	r := reason
	return &heat.RaceHeat{Reason: &r}
}

// ============================================================================

// Without the flag, the response must not change at all — the deterministic
// cool-weather plan is the contract every existing consumer holds.
func TestWeatherMode_WithoutFlagIsByteIdentical(t *testing.T) {
	f := setup(t)
	f.setConfig(t, allThresholds)
	race := f.createRace(t, fullTri)
	f.svc.SetHeatProvider(&fakeRaceHeat{rh: hotRaceHeat(34, heat.AcclimMedium)})

	before := f.do(t, http.MethodGet, "/races/"+race.ID.String()+"/pacing-plan", "", nil)
	require.Equal(t, http.StatusOK, before.Code, before.Body.String())

	// The same request, with the flag explicitly false.
	after := f.do(t, http.MethodGet, "/races/"+race.ID.String()+"/pacing-plan?weather=false", "", nil)
	require.Equal(t, http.StatusOK, after.Code)

	assert.JSONEq(t, before.Body.String(), after.Body.String())
	assert.NotContains(t, before.Body.String(), "heat_adjusted")
	assert.NotContains(t, before.Body.String(), `"heat"`)
	assert.NotContains(t, before.Body.String(), "heat_reason")
}

func TestWeatherMode_HotForecastAnnotatesEveryLeg(t *testing.T) {
	f := setup(t)
	f.setConfig(t, allThresholds)
	race := f.createRace(t, fullTri)
	fake := &fakeRaceHeat{rh: hotRaceHeat(34, heat.AcclimMedium)}
	f.svc.SetHeatProvider(fake)

	rec := f.do(t, http.MethodGet, "/races/"+race.ID.String()+"/pacing-plan?weather=true", "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var plan racepacing.PacingPlan
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &plan))

	require.NotNil(t, plan.Heat, "the race-level heat block names the load and place")
	assert.InDelta(t, 34, plan.Heat.LoadC, 0.01)
	assert.Equal(t, "Palma, Spain", plan.Heat.Location)
	assert.Equal(t, "medium", plan.Heat.Acclimatization)
	assert.NotNil(t, plan.Heat.ForecastAt)
	assert.Greater(t, plan.Heat.ReductionPct, 0.0)
	assert.Nil(t, plan.HeatReason)
	assert.Equal(t, 1, fake.calls)

	// Every leg with a computable band gains a heat-adjusted sibling, and the
	// ORIGINAL is untouched — that's the point: plan A cool / plan B hot.
	adjusted := 0
	for _, leg := range plan.Legs {
		if leg.HeatAdjusted == nil {
			continue
		}
		adjusted++
		assert.Greater(t, leg.HeatAdjusted.ReductionPct, 0.0)

		if leg.TargetPowerHighW != nil {
			require.NotNil(t, leg.HeatAdjusted.TargetPowerHighW)
			assert.Less(t, *leg.HeatAdjusted.TargetPowerHighW, *leg.TargetPowerHighW,
				"heat lowers the power target")
		}
		if leg.TargetPaceHighSecPerKM != nil {
			require.NotNil(t, leg.HeatAdjusted.TargetPaceHighSecPerKM)
			assert.Greater(t, *leg.HeatAdjusted.TargetPaceHighSecPerKM, *leg.TargetPaceHighSecPerKM,
				"backing off means MORE sec/km, not fewer")
		}
		if leg.TargetPaceHighSecPer100m != nil {
			require.NotNil(t, leg.HeatAdjusted.TargetPaceHighSecPer100m)
			assert.Greater(t, *leg.HeatAdjusted.TargetPaceHighSecPer100m, *leg.TargetPaceHighSecPer100m)
		}
		// IF and TSS follow the intensity down.
		if leg.IntensityFactor != nil && leg.HeatAdjusted.IntensityFactor != nil {
			assert.Less(t, *leg.HeatAdjusted.IntensityFactor, *leg.IntensityFactor)
		}
		if leg.EstimatedTSS != nil && leg.HeatAdjusted.EstimatedTSS != nil {
			assert.Less(t, *leg.HeatAdjusted.EstimatedTSS, *leg.EstimatedTSS)
		}
	}
	assert.Positive(t, adjusted, "at least one leg must be annotated")
}

func TestWeatherMode_DegradationsKeepTheBasePlan(t *testing.T) {
	// One fixture, re-provisioned per reason: a Postgres per subtest buys
	// nothing here and only invites boot contention.
	f := setup(t)
	f.setConfig(t, allThresholds)
	race := f.createRace(t, fullTri)

	for _, reason := range []string{"location_ungeocodable", "forecast_out_of_range", "weather_unavailable"} {
		t.Run(reason, func(t *testing.T) {
			f.svc.SetHeatProvider(&fakeRaceHeat{rh: reasonRaceHeat(reason)})

			rec := f.do(t, http.MethodGet, "/races/"+race.ID.String()+"/pacing-plan?weather=true", "", nil)
			require.Equal(t, http.StatusOK, rec.Code, "a degradation is never an error")
			var plan racepacing.PacingPlan
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &plan))

			require.NotNil(t, plan.HeatReason)
			assert.Equal(t, reason, *plan.HeatReason)
			assert.Nil(t, plan.Heat)

			// The base plan is intact: bands still there, no adjustments.
			require.NotEmpty(t, plan.Legs)
			for _, leg := range plan.Legs {
				assert.Nil(t, leg.HeatAdjusted)
			}
			assert.NotContains(t, rec.Body.String(), "heat_adjusted")
		})
	}
}

// Weather mode with the provider unwired must not error — it just does nothing.
func TestWeatherMode_UnwiredProviderIsInert(t *testing.T) {
	f := setup(t)
	f.setConfig(t, allThresholds)
	race := f.createRace(t, fullTri)

	rec := f.do(t, http.MethodGet, "/races/"+race.ID.String()+"/pacing-plan?weather=true", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), "heat_adjusted")
}

// A cool forecast annotates with a zero reduction rather than pretending the
// day is hot — the bands come back equal to the originals.
func TestWeatherMode_CoolRaceLeavesBandsEqual(t *testing.T) {
	f := setup(t)
	f.setConfig(t, allThresholds)
	race := f.createRace(t, fullTri)
	f.svc.SetHeatProvider(&fakeRaceHeat{rh: hotRaceHeat(18, heat.AcclimGood)})

	rec := f.do(t, http.MethodGet, "/races/"+race.ID.String()+"/pacing-plan?weather=true", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var plan racepacing.PacingPlan
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &plan))

	require.NotNil(t, plan.Heat)
	assert.Zero(t, plan.Heat.ReductionPct)
	for _, leg := range plan.Legs {
		if leg.HeatAdjusted == nil || leg.TargetPowerHighW == nil {
			continue
		}
		assert.Zero(t, leg.HeatAdjusted.ReductionPct)
		assert.Equal(t, *leg.TargetPowerHighW, *leg.HeatAdjusted.TargetPowerHighW,
			"a cool race costs nothing")
	}
}

// Less adaptation → a deeper cut for the same race.
func TestWeatherMode_AcclimatizationMovesTheAdjustment(t *testing.T) {
	f := setup(t)
	f.setConfig(t, allThresholds)
	race := f.createRace(t, fullTri)

	read := func(level heat.Acclimatization) float64 {
		f.svc.SetHeatProvider(&fakeRaceHeat{rh: hotRaceHeat(34, level)})
		rec := f.do(t, http.MethodGet, "/races/"+race.ID.String()+"/pacing-plan?weather=true", "", nil)
		require.Equal(t, http.StatusOK, rec.Code)
		var plan racepacing.PacingPlan
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &plan))
		require.NotNil(t, plan.Heat)
		return plan.Heat.ReductionPct
	}

	assert.Greater(t, read(heat.AcclimLow), read(heat.AcclimGood))
}
