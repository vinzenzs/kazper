package races_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/heat"
	"github.com/vinzenzs/kazper/internal/races"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

// weatherFixture is self-contained: the package's shared setup returns a bare
// engine, and these tests need a handle on the service to wire the provider.
type weatherFixture struct {
	r        *gin.Engine
	racesSvc *races.Service
}

func setupFueling(t *testing.T) *weatherFixture {
	t.Helper()
	pool := storetest.NewPool(t)
	svc := races.NewService(pool, races.NewRepo(pool))
	gin.SetMode(gin.TestMode)
	r := gin.New()
	races.NewHandlers(svc).Register(r.Group("/"))
	return &weatherFixture{r: r, racesSvc: svc}
}

func (f *weatherFixture) do(t *testing.T, method, path, body string, _ map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	f.r.ServeHTTP(rec, req)
	return rec
}

func (f *weatherFixture) createRace(t *testing.T, body string) races.Race {
	t.Helper()
	rec := f.do(t, http.MethodPost, "/races", body, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var race races.Race
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &race))
	return race
}

// fakeRaceHeat returns a canned race-heat picture.
type fakeRaceHeat struct{ rh *heat.RaceHeat }

func (f *fakeRaceHeat) RaceHeatFor(_ context.Context, _ string, _, _ time.Time) *heat.RaceHeat {
	return f.rh
}

func raceHeatAt(loadC float64) *heat.RaceHeat {
	at := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	return &heat.RaceHeat{
		LoadC:           loadC,
		HeatIndexC:      loadC - 1,
		Conditions:      &heat.Conditions{TemperatureC: 33, HumidityPct: 60},
		Acclimatization: &heat.AcclimEvidence{Level: heat.AcclimMedium, Count: 3},
		Location:        "Palma, Spain",
		ForecastAt:      &at,
	}
}

func fuelingPlan(t *testing.T, f *weatherFixture, raceID, query string) races.FuelingPlan {
	t.Helper()
	rec := f.do(t, http.MethodGet, "/races/"+raceID+"/fueling-plan?"+query, "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out races.FuelingPlan
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out
}

func legFuel(t *testing.T, p races.FuelingPlan, discipline races.Discipline) *races.LegFuelingPlan {
	t.Helper()
	for _, l := range p.Legs {
		if l.Discipline == discipline {
			return l
		}
	}
	t.Fatalf("no %s leg in plan", discipline)
	return nil
}

// ============================================================================

// The base contract holds: no flag, no change.
func TestFuelingWeather_WithoutFlagIsByteIdentical(t *testing.T) {
	f := setupFueling(t)
	race := f.createRace(t, weatherTri)
	f.racesSvc.SetHeatProvider(&fakeRaceHeat{rh: raceHeatAt(34)})

	base := f.do(t, http.MethodGet, "/races/"+race.ID.String()+"/fueling-plan?body_weight_kg=70&sweat_rate_ml_per_hr=800", "", nil)
	require.Equal(t, http.StatusOK, base.Code, base.Body.String())
	off := f.do(t, http.MethodGet, "/races/"+race.ID.String()+"/fueling-plan?body_weight_kg=70&sweat_rate_ml_per_hr=800&weather=false", "", nil)
	require.Equal(t, http.StatusOK, off.Code)

	assert.JSONEq(t, base.Body.String(), off.Body.String())
	assert.NotContains(t, base.Body.String(), `"heat"`)
	assert.NotContains(t, base.Body.String(), "heat_reason")
}

func TestFuelingWeather_HotRaceScalesFluidAndSodiumNotCarbs(t *testing.T) {
	f := setupFueling(t)
	race := f.createRace(t, weatherTri)
	f.racesSvc.SetHeatProvider(&fakeRaceHeat{rh: raceHeatAt(34)})

	cool := fuelingPlan(t, f, race.ID.String(), "body_weight_kg=70&sweat_rate_ml_per_hr=800")
	hot := fuelingPlan(t, f, race.ID.String(), "body_weight_kg=70&sweat_rate_ml_per_hr=800&weather=true")

	require.NotNil(t, hot.Heat)
	assert.InDelta(t, 34, hot.Heat.LoadC, 0.01)
	assert.Equal(t, "Palma, Spain", hot.Heat.Location)
	assert.Equal(t, "medium", hot.Heat.Acclimatization)
	// 1 + (34−24)×0.03 = 1.3
	assert.InDelta(t, 1.3, hot.Heat.FluidMultiplier, 0.001)
	assert.Nil(t, hot.HeatReason)

	coolBike := legFuel(t, cool, races.DisciplineBike)
	hotBike := legFuel(t, hot, races.DisciplineBike)

	assert.Greater(t, hotBike.FluidMlPerHr, coolBike.FluidMlPerHr, "heat raises fluid")
	assert.Greater(t, hotBike.SodiumMgPerHr, coolBike.SodiumMgPerHr, "and sodium with it")
	// Carbs are NOT a function of the weather: heat drives sweat loss, not
	// carbohydrate oxidation.
	assert.Equal(t, coolBike.CarbsGPerHr, hotBike.CarbsGPerHr)
	assert.Equal(t, coolBike.CarbsGTotal, hotBike.CarbsGTotal)
	assert.Equal(t, cool.Total.CarbsGTotal, hot.Total.CarbsGTotal)
	assert.Greater(t, hot.Total.FluidMlTotal, cool.Total.FluidMlTotal)
}

// The absorption ceiling is physiology: heat cannot make a gut drink faster.
func TestFuelingWeather_AbsorptionCapStillApplies(t *testing.T) {
	f := setupFueling(t)
	race := f.createRace(t, weatherTri)
	f.racesSvc.SetHeatProvider(&fakeRaceHeat{rh: raceHeatAt(40)})

	hot := fuelingPlan(t, f, race.ID.String(), "body_weight_kg=70&sweat_rate_ml_per_hr=950&weather=true")

	bike := legFuel(t, hot, races.DisciplineBike)
	assert.LessOrEqual(t, bike.FluidMlPerHr, 1000.0, "the 1000 ml/hr cap survives the multiplier")
}

// Scaling a generic default by the heat does not make it a measured number:
// the flagged rationale must survive.
func TestFuelingWeather_DefaultsStayFlagged(t *testing.T) {
	f := setupFueling(t)
	race := f.createRace(t, weatherTri)
	f.racesSvc.SetHeatProvider(&fakeRaceHeat{rh: raceHeatAt(34)})

	noSweat := fuelingPlan(t, f, race.ID.String(), "body_weight_kg=70")
	hot := fuelingPlan(t, f, race.ID.String(), "body_weight_kg=70&weather=true")

	coolBike := legFuel(t, noSweat, races.DisciplineBike)
	hotBike := legFuel(t, hot, races.DisciplineBike)

	// The number moved...
	assert.Greater(t, hotBike.FluidMlPerHr, coolBike.FluidMlPerHr)
	// ...but its provenance did not.
	assert.Equal(t, coolBike.Rationale, hotBike.Rationale,
		"a scaled default is still a default — the flag must not silently upgrade")
	assert.Nil(t, hot.SweatRateMlPerHr)
}

func TestFuelingWeather_DegradationsKeepTheBasePlan(t *testing.T) {
	f := setupFueling(t)
	race := f.createRace(t, weatherTri)

	for _, reason := range []string{"location_ungeocodable", "forecast_out_of_range", "weather_unavailable"} {
		t.Run(reason, func(t *testing.T) {
			r := reason
			f.racesSvc.SetHeatProvider(&fakeRaceHeat{rh: &heat.RaceHeat{Reason: &r}})

			base := fuelingPlan(t, f, race.ID.String(), "body_weight_kg=70&sweat_rate_ml_per_hr=800")
			degraded := fuelingPlan(t, f, race.ID.String(), "body_weight_kg=70&sweat_rate_ml_per_hr=800&weather=true")

			require.NotNil(t, degraded.HeatReason)
			assert.Equal(t, reason, *degraded.HeatReason)
			assert.Nil(t, degraded.Heat)
			// The plan itself is the unadjusted one.
			assert.Equal(t, legFuel(t, base, races.DisciplineBike).FluidMlPerHr,
				legFuel(t, degraded, races.DisciplineBike).FluidMlPerHr)
		})
	}
}

func TestFuelingWeather_CoolRaceDoesNotScale(t *testing.T) {
	f := setupFueling(t)
	race := f.createRace(t, weatherTri)
	f.racesSvc.SetHeatProvider(&fakeRaceHeat{rh: raceHeatAt(18)})

	cool := fuelingPlan(t, f, race.ID.String(), "body_weight_kg=70&sweat_rate_ml_per_hr=800")
	hot := fuelingPlan(t, f, race.ID.String(), "body_weight_kg=70&sweat_rate_ml_per_hr=800&weather=true")

	require.NotNil(t, hot.Heat)
	assert.InDelta(t, 1.0, hot.Heat.FluidMultiplier, 0.001)
	assert.Equal(t, legFuel(t, cool, races.DisciplineBike).FluidMlPerHr,
		legFuel(t, hot, races.DisciplineBike).FluidMlPerHr)
}

func TestFuelingWeather_UnwiredProviderIsInert(t *testing.T) {
	f := setupFueling(t)
	race := f.createRace(t, weatherTri)

	rec := f.do(t, http.MethodGet, "/races/"+race.ID.String()+"/fueling-plan?body_weight_kg=70&weather=true", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), `"heat"`)
}

const weatherTri = `{"name":"IM Mallorca","race_date":"2026-09-01","location":"Palma, Mallorca","legs":[
	{"ordinal":1,"discipline":"swim","expected_duration_min":70},
	{"ordinal":2,"discipline":"bike","expected_duration_min":300},
	{"ordinal":3,"discipline":"run","expected_duration_min":240}]}`
