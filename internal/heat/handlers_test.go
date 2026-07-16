package heat_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/athleteconfig"
	"github.com/vinzenzs/kazper/internal/heat"
	"github.com/vinzenzs/kazper/internal/locations"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/weather"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// fakeConfig is the effective-config stand-in.
type fakeConfig struct{ cfg *athleteconfig.AthleteConfig }

func (f fakeConfig) Get(context.Context) (*athleteconfig.AthleteConfig, error) { return f.cfg, nil }

// fakeSweat supplies (or withholds) a personal sweat signal.
type fakeSweat struct{ ml *float64 }

func (f fakeSweat) LatestSweatSignal(context.Context) *heat.SweatSignal {
	if f.ml == nil {
		return nil
	}
	return &heat.SweatSignal{MlPerHour: *f.ml, Source: heat.SourceGarminSweatLoss, Sessions: 2}
}

type fixture struct {
	r            *gin.Engine
	workoutsRepo *workouts.Repo
	locRepo      *locations.Repo
	calls        *int
}

func fp(v float64) *float64 { return &v }
func ip(v int) *int         { return &v }

// hourlyJSON builds an Open-Meteo forecast covering 08:00–12:00 UTC on the
// fixture day at fixed conditions.
func hourlyJSON(tempC, humidity, wind, cloud float64) string {
	var times, temps, hums, winds, clouds []string
	for h := 6; h <= 14; h++ {
		times = append(times, fmt.Sprintf(`"2026-07-20T%02d:00"`, h))
		temps = append(temps, fmt.Sprintf("%v", tempC))
		hums = append(hums, fmt.Sprintf("%v", humidity))
		winds = append(winds, fmt.Sprintf("%v", wind))
		clouds = append(clouds, fmt.Sprintf("%v", cloud))
	}
	return fmt.Sprintf(`{"hourly":{"time":[%s],"temperature_2m":[%s],"relative_humidity_2m":[%s],"wind_speed_10m":[%s],"cloud_cover":[%s]}}`,
		strings.Join(times, ","), strings.Join(temps, ","), strings.Join(hums, ","),
		strings.Join(winds, ","), strings.Join(clouds, ","))
}

// setup mounts the heat endpoint over a fake Open-Meteo. `body` is the forecast
// payload; an empty body makes the fake return 503 (the unavailable case).
func setup(t *testing.T, body string, opts ...func(*setupOpts)) *fixture {
	t.Helper()
	o := &setupOpts{
		home:   locations.Home{Lat: 39.57, Lon: 2.65, Set: true},
		config: &athleteconfig.AthleteConfig{FtpWatts: ip(280)},
	}
	for _, fn := range opts {
		fn(o)
	}

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if body == "" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	pool := storetest.NewPool(t)
	wRepo := workouts.NewRepo(pool)
	locRepo := locations.NewRepo(pool)
	locSvc := locations.NewService(locRepo, o.home)
	wx := weather.New(weather.Config{ForecastBaseURL: srv.URL, ArchiveBaseURL: srv.URL, GeocodeBaseURL: srv.URL}, nil)

	svc := heat.NewService(wRepo, locSvc, wx, fakeConfig{cfg: o.config})
	if o.sweatSet {
		svc.SetSweatRateProvider(fakeSweat{ml: o.sweat})
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/")
	heat.NewHandlers(svc, nil).Register(rg)
	return &fixture{r: r, workoutsRepo: wRepo, locRepo: locRepo, calls: &calls}
}

type setupOpts struct {
	home     locations.Home
	config   *athleteconfig.AthleteConfig
	sweat    *float64
	sweatSet bool
}

func noHome(o *setupOpts) { o.home = locations.Home{} }
func withSweat(ml float64) func(*setupOpts) {
	return func(o *setupOpts) { o.sweat = fp(ml); o.sweatSet = true }
}
func withConfig(c *athleteconfig.AthleteConfig) func(*setupOpts) {
	return func(o *setupOpts) { o.config = c }
}

// planSession materializes a planned session on the fixture day.
func planSession(t *testing.T, f *fixture, sport workouts.Sport, mins float64, env *workouts.Environment) uuid.UUID {
	t.Helper()
	start := time.Date(2026, 7, 20, 8, 0, 0, 0, time.UTC)
	w := &workouts.Workout{
		Source:      workouts.SourceManual,
		Sport:       sport,
		Status:      workouts.StatusPlanned,
		StartedAt:   start,
		EndedAt:     start.Add(time.Duration(mins) * time.Minute),
		Environment: env,
	}
	_, err := f.workoutsRepo.Upsert(context.Background(), w)
	require.NoError(t, err)
	return w.ID
}

// completedHot inserts a completed outdoor session that qualifies for
// acclimatization (long + hot), `daysAgo` before the fixture day.
func completedHot(t *testing.T, f *fixture, daysAgo int, mins, tempC float64, env *workouts.Environment) uuid.UUID {
	t.Helper()
	start := time.Date(2026, 7, 20, 8, 0, 0, 0, time.UTC).AddDate(0, 0, -daysAgo)
	w := &workouts.Workout{
		Source:       workouts.SourceManual,
		Sport:        workouts.SportBike,
		Status:       workouts.StatusCompleted,
		StartedAt:    start,
		EndedAt:      start.Add(time.Duration(mins) * time.Minute),
		TemperatureC: fp(tempC),
		HumidityPct:  fp(55),
		Environment:  env,
	}
	_, err := f.workoutsRepo.Upsert(context.Background(), w)
	require.NoError(t, err)
	return w.ID
}

func outdoor() *workouts.Environment { e := workouts.EnvironmentOutdoor; return &e }
func indoor() *workouts.Environment  { e := workouts.EnvironmentIndoor; return &e }

func getHeat(t *testing.T, f *fixture, id uuid.UUID) heat.Report {
	t.Helper()
	rec := doGet(t, f.r, "/workouts/"+id.String()+"/heat")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out heat.Report
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out
}

func doGet(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// ============================================================================

func TestHeat_HotDayHappyPath(t *testing.T) {
	// 31 °C / 55% RH, 2 m/s wind, 40% cloud; six qualifying hot sessions.
	f := setup(t, hourlyJSON(31, 55, 2, 40), withSweat(1200))
	for i := 1; i <= 6; i++ {
		completedHot(t, f, i, 90, 30, outdoor())
	}
	id := planSession(t, f, workouts.SportBike, 120, outdoor())

	out := getHeat(t, f, id)

	assert.Equal(t, id, out.WorkoutID)
	assert.Equal(t, "2026-07-20", out.Date)
	assert.Nil(t, out.Reason)
	assert.False(t, out.NotApplicable)
	assert.False(t, out.AssumedOutdoor, "a stated outdoor session assumes nothing")

	// The location the forecast came from is echoed.
	require.NotNil(t, out.Location)
	assert.Equal(t, "home", out.Location.Name)
	assert.Equal(t, "home", out.Location.Source)

	require.NotNil(t, out.Conditions)
	assert.InDelta(t, 31, out.Conditions.TemperatureC, 0.01)
	assert.InDelta(t, 55, out.Conditions.HumidityPct, 0.01)

	require.NotNil(t, out.Load)
	assert.Greater(t, out.Load.HeatLoadC, 31.0, "hot + humid + sun reads above dry bulb")
	assert.InDelta(t, 1.8, out.Load.SolarPenaltyC, 0.01) // 3 × (1 − 0.40)
	assert.Zero(t, out.Load.WindCoolingC, "2 m/s is below the cooling threshold")

	// Acclimatization is derived and auditable.
	require.NotNil(t, out.Acclim)
	assert.Equal(t, heat.AcclimGood, out.Acclim.Level)
	assert.Equal(t, 6, out.Acclim.Count)
	assert.Len(t, out.Acclim.Sessions, 6)
	assert.Equal(t, 14, out.Acclim.WindowDays)

	require.NotNil(t, out.Adjustment)
	assert.Greater(t, out.Adjustment.ReductionPct, 0.0)
	assert.Equal(t, "ftp_watts", out.Adjustment.Baseline)
	assert.InDelta(t, 280, out.Adjustment.BaselineFrom, 0.01)
	assert.Less(t, out.Adjustment.SuggestedTo, 280.0, "the suggestion is a reduction")

	require.NotNil(t, out.Fluid)
	assert.Equal(t, heat.SourceGarminSweatLoss, out.Fluid.Source)
	assert.Greater(t, out.Fluid.MlPerHour, 1200.0, "heat scales the personal rate up")

	assert.InDelta(t, 120, out.DurationMin, 0.01)
}

func TestHeat_IndoorIsNotApplicableAndFetchesNoWeather(t *testing.T) {
	f := setup(t, hourlyJSON(31, 55, 2, 40))
	id := planSession(t, f, workouts.SportBike, 120, indoor())

	out := getHeat(t, f, id)

	assert.True(t, out.NotApplicable)
	assert.Nil(t, out.Load)
	assert.Nil(t, out.Adjustment)
	assert.Nil(t, out.Location)
	assert.Zero(t, *f.calls, "an indoor session must not cost a forecast call")
}

func TestHeat_NullEnvironmentAssumesOutdoorVisibly(t *testing.T) {
	f := setup(t, hourlyJSON(31, 55, 2, 40))
	id := planSession(t, f, workouts.SportBike, 120, nil)

	out := getHeat(t, f, id)

	assert.True(t, out.AssumedOutdoor, "the assumption must be stated, never silent")
	assert.False(t, out.NotApplicable)
	require.NotNil(t, out.Load)
	assert.Greater(t, out.Load.HeatLoadC, 20.0)
}

func TestHeat_WeatherUnavailableDegradesHonestly(t *testing.T) {
	f := setup(t, "") // the fake returns 503
	id := planSession(t, f, workouts.SportBike, 120, outdoor())

	rec := doGet(t, f.r, "/workouts/"+id.String()+"/heat")
	require.Equal(t, http.StatusOK, rec.Code, "weather trouble must never 5xx the read")

	var out heat.Report
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.NotNil(t, out.Reason)
	assert.Equal(t, "weather_unavailable", *out.Reason)
	assert.Nil(t, out.Load)
	assert.Nil(t, out.Adjustment)
	assert.Nil(t, out.Conditions)
	// The location it did resolve stays visible.
	assert.NotNil(t, out.Location)
}

func TestHeat_LocationUnconfiguredDegradesHonestly(t *testing.T) {
	f := setup(t, hourlyJSON(31, 55, 2, 40), noHome)
	id := planSession(t, f, workouts.SportBike, 120, outdoor())

	out := getHeat(t, f, id)

	require.NotNil(t, out.Reason)
	assert.Equal(t, "location_unconfigured", *out.Reason)
	assert.Nil(t, out.Load)
	assert.Nil(t, out.Location)
	assert.Zero(t, *f.calls, "no location, no forecast to fetch")
}

// A logged trip moves the forecast: the session follows the athlete.
func TestHeat_TravelPeriodMovesTheForecast(t *testing.T) {
	f := setup(t, hourlyJSON(31, 55, 2, 40), noHome)
	_, err := f.locRepo.Insert(context.Background(), &locations.Period{
		StartDate: "2026-07-18", EndDate: "2026-07-25",
		Name: "Mallorca", Lat: 39.57, Lon: 2.65,
	})
	require.NoError(t, err)
	id := planSession(t, f, workouts.SportBike, 120, outdoor())

	out := getHeat(t, f, id)

	require.NotNil(t, out.Location)
	assert.Equal(t, "Mallorca", out.Location.Name)
	assert.Equal(t, "travel", out.Location.Source)
	assert.NotNil(t, out.Load, "with a trip logged, home config isn't needed")
}

func TestHeat_CompletedWorkoutIsRefused(t *testing.T) {
	f := setup(t, hourlyJSON(31, 55, 2, 40))
	id := completedHot(t, f, 1, 120, 30, outdoor())

	rec := doGet(t, f.r, "/workouts/"+id.String()+"/heat")

	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "workout_not_planned")
}

func TestHeat_NotFound(t *testing.T) {
	f := setup(t, hourlyJSON(31, 55, 2, 40))

	rec := doGet(t, f.r, "/workouts/"+uuid.New().String()+"/heat")
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = doGet(t, f.r, "/workouts/not-a-uuid/heat")
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "not_found")
}

// Indoor history must not count as heat adaptation, however hot the room was.
func TestHeat_AcclimatizationExcludesIndoorSessions(t *testing.T) {
	f := setup(t, hourlyJSON(31, 55, 2, 40))
	for i := 1; i <= 6; i++ {
		completedHot(t, f, i, 90, 32, indoor())
	}
	id := planSession(t, f, workouts.SportBike, 120, outdoor())

	out := getHeat(t, f, id)

	require.NotNil(t, out.Acclim)
	assert.Equal(t, heat.AcclimLow, out.Acclim.Level, "a hot pain cave is not heat acclimatization")
	assert.Equal(t, 0, out.Acclim.Count)
}

func TestHeat_AcclimatizationExcludesSessionsOutsideTheWindow(t *testing.T) {
	f := setup(t, hourlyJSON(31, 55, 2, 40))
	// Six qualifying sessions, but all older than 14 days.
	for i := 15; i <= 20; i++ {
		completedHot(t, f, i, 90, 30, outdoor())
	}
	id := planSession(t, f, workouts.SportBike, 120, outdoor())

	out := getHeat(t, f, id)
	assert.Equal(t, heat.AcclimLow, out.Acclim.Level, "adaptation fades — old sessions don't count")
	assert.Equal(t, 0, out.Acclim.Count)
}

// Less adaptation → a deeper suggested cut for the identical session.
func TestHeat_LowAcclimatizationSuggestsBiggerReduction(t *testing.T) {
	hot := hourlyJSON(33, 60, 1, 0)

	unadapted := setup(t, hot)
	idA := planSession(t, unadapted, workouts.SportBike, 120, outdoor())
	low := getHeat(t, unadapted, idA)

	adapted := setup(t, hot)
	for i := 1; i <= 6; i++ {
		completedHot(t, adapted, i, 90, 32, outdoor())
	}
	idB := planSession(t, adapted, workouts.SportBike, 120, outdoor())
	good := getHeat(t, adapted, idB)

	assert.Equal(t, heat.AcclimLow, low.Acclim.Level)
	assert.Equal(t, heat.AcclimGood, good.Acclim.Level)
	assert.InDelta(t, low.Load.HeatLoadC, good.Load.HeatLoadC, 0.01, "same weather")
	assert.Greater(t, low.Adjustment.ReductionPct, good.Adjustment.ReductionPct)
}

func TestHeat_CoolDaySuggestsNoReduction(t *testing.T) {
	f := setup(t, hourlyJSON(14, 60, 2, 80))
	id := planSession(t, f, workouts.SportBike, 120, outdoor())

	out := getHeat(t, f, id)

	require.NotNil(t, out.Load)
	assert.Less(t, out.Load.HeatLoadC, 24.0)
	require.NotNil(t, out.Adjustment)
	assert.Zero(t, out.Adjustment.ReductionPct, "a cool day costs nothing")
	assert.InDelta(t, 280, out.Adjustment.SuggestedTo, 0.01, "baseline unchanged")
}

// Pace is inverted: a run's suggestion must be SLOWER (more sec/km), not faster.
func TestHeat_RunBaselineIsThresholdPaceAndSlowsDown(t *testing.T) {
	f := setup(t, hourlyJSON(34, 60, 0, 0),
		withConfig(&athleteconfig.AthleteConfig{ThresholdPaceSecPerKm: fp(240)}))
	id := planSession(t, f, workouts.SportRun, 90, outdoor())

	out := getHeat(t, f, id)

	require.NotNil(t, out.Adjustment)
	assert.Equal(t, "threshold_pace_sec_per_km", out.Adjustment.Baseline)
	assert.InDelta(t, 240, out.Adjustment.BaselineFrom, 0.01)
	assert.Greater(t, out.Adjustment.SuggestedTo, 240.0,
		"backing off in the heat means more seconds per km, not fewer")
}

// Fail-open: no FTP configured still yields a percentage — the advice stands
// without the arithmetic.
func TestHeat_MissingBaselineStillSuggestsAPercentage(t *testing.T) {
	f := setup(t, hourlyJSON(34, 60, 0, 0), withConfig(&athleteconfig.AthleteConfig{}))
	id := planSession(t, f, workouts.SportBike, 120, outdoor())

	out := getHeat(t, f, id)

	require.NotNil(t, out.Adjustment)
	assert.Greater(t, out.Adjustment.ReductionPct, 0.0)
	assert.Empty(t, out.Adjustment.Baseline)
	assert.Zero(t, out.Adjustment.SuggestedTo)
}

func TestHeat_GenericFluidIsFlaggedWhenNoSweatRate(t *testing.T) {
	f := setup(t, hourlyJSON(31, 55, 2, 40)) // no sweat provider wired
	id := planSession(t, f, workouts.SportBike, 120, outdoor())

	out := getHeat(t, f, id)

	require.NotNil(t, out.Fluid)
	assert.Equal(t, heat.SourceGenericDefault, out.Fluid.Source)
	assert.Contains(t, out.Fluid.Note, "generic")
}

// The forecast is fetched once per location/window even across reads.
func TestHeat_ForecastIsCachedAcrossReads(t *testing.T) {
	f := setup(t, hourlyJSON(31, 55, 2, 40))
	id := planSession(t, f, workouts.SportBike, 120, outdoor())

	getHeat(t, f, id)
	getHeat(t, f, id)

	assert.Equal(t, 1, *f.calls, "the second read must hit the weather cache")
}

func TestHeat_ReadOnly(t *testing.T) {
	f := setup(t, hourlyJSON(31, 55, 2, 40))
	id := planSession(t, f, workouts.SportBike, 120, outdoor())

	getHeat(t, f, id)

	// The session is untouched: still planned, no target rewritten.
	w, err := f.workoutsRepo.GetByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, workouts.StatusPlanned, w.Status)
	assert.Nil(t, w.TSS)
	require.NotNil(t, w.Environment)
	assert.Equal(t, workouts.EnvironmentOutdoor, *w.Environment)
}
