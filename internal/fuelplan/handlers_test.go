package fuelplan_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/bodyweight"
	"github.com/vinzenzs/kazper/internal/fuelplan"
	"github.com/vinzenzs/kazper/internal/goals"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/workouts"
)

type fixture struct {
	r             *gin.Engine
	workoutsRepo  *workouts.Repo
	weightRepo    *bodyweight.Repo
	goalsRepo     *goals.Repo
	overridesRepo *goals.OverridesRepo
}

func setup(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	wRepo := workouts.NewRepo(pool)
	bwRepo := bodyweight.NewRepo(pool)
	bwSvc := bodyweight.NewService(bwRepo)
	gRepo := goals.NewRepo(pool)
	goRepo := goals.NewOverridesRepo(pool)
	resolver := goals.NewResolver(gRepo, goRepo, nil, nil)
	svc := fuelplan.NewService(wRepo, bwSvc, resolver)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/")
	fuelplan.NewHandlers(svc, "UTC", nil).Register(rg)
	return &fixture{r: r, workoutsRepo: wRepo, weightRepo: bwRepo, goalsRepo: gRepo, overridesRepo: goRepo}
}

func p(v float64) *float64 { return &v }

// day n of the fixture week, at 09:00 UTC.
func day(n int) time.Time {
	return time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC).AddDate(0, 0, n-1)
}

func dateStr(n int) string { return day(n).Format("2006-01-02") }

// planSession materializes a planned workout: `tss` planned TSS over `mins`.
// tss_source is paired with tss by a DB CHECK, so a planned TSS must declare
// where it came from.
func planSession(t *testing.T, repo *workouts.Repo, at time.Time, tss *float64, mins float64) uuid.UUID {
	t.Helper()
	w := &workouts.Workout{
		Source:    workouts.SourceManual,
		Sport:     workouts.SportBike,
		Status:    workouts.StatusPlanned,
		StartedAt: at,
		EndedAt:   at.Add(time.Duration(mins) * time.Minute),
		TSS:       tss,
	}
	if tss != nil {
		src := "manual"
		w.TSSSource = &src
	}
	_, err := repo.Upsert(context.Background(), w)
	require.NoError(t, err)
	return w.ID
}

func logWeight(t *testing.T, repo *bodyweight.Repo, at time.Time, kg float64) {
	t.Helper()
	require.NoError(t, repo.Insert(context.Background(), &bodyweight.Entry{LoggedAt: at, WeightKg: kg}))
}

// seedWeight puts the trend at exactly `kg` across the days before the window,
// so the g/kg denominator is a known number.
func seedWeight(t *testing.T, f *fixture, kg float64) {
	t.Helper()
	for i := 0; i < 7; i++ {
		logWeight(t, f.weightRepo, day(1).AddDate(0, 0, -i), kg)
	}
}

func setGoalCarbs(t *testing.T, f *fixture, min, max float64) {
	t.Helper()
	require.NoError(t, f.goalsRepo.Upsert(context.Background(), &goals.Goals{
		CarbsG: &goals.Range{Min: &min, Max: &max},
	}))
}

func doGet(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func getPlan(t *testing.T, f *fixture, query string) fuelplan.Plan {
	t.Helper()
	rec := doGet(t, f.r, "/nutrition/fuel-plan?"+query)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out fuelplan.Plan
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out
}

// week is the fixture's explicit 7-day window.
const week = "from=2026-07-20&to=2026-07-26&tz=UTC"

// ============================================================================

func TestFuelPlan_ClassifiedWeek(t *testing.T) {
	f := setup(t)
	seedWeight(t, f, 70.0)
	setGoalCarbs(t, f, 400, 500)

	planSession(t, f.workoutsRepo, day(1), p(40), 45)  // easy
	planSession(t, f.workoutsRepo, day(2), p(90), 75)  // moderate
	planSession(t, f.workoutsRepo, day(3), p(180), 90) // heavy
	// day 4: rest inside the plan
	planSession(t, f.workoutsRepo, day(5), p(120), 60) // moderate
	planSession(t, f.workoutsRepo, day(6), p(30), 30)  // easy
	planSession(t, f.workoutsRepo, day(7), p(200), 240)

	out := getPlan(t, f, week)

	require.Len(t, out.Days, 7)
	require.NotNil(t, out.Weight)
	assert.InDelta(t, 70.0, out.Weight.TrendKg, 0.001)
	assert.Nil(t, out.Reason)

	want := []struct {
		tier  fuelplan.Tier
		gPerK float64
		grams float64
	}{
		{fuelplan.TierEasy, 5, 350},
		{fuelplan.TierModerate, 7, 490},
		{fuelplan.TierHeavy, 9, 630},
		{fuelplan.TierRest, 3, 210},
		{fuelplan.TierModerate, 7, 490},
		{fuelplan.TierEasy, 5, 350},
		{fuelplan.TierHeavy, 9, 630},
	}
	for i, w := range want {
		d := out.Days[i]
		assert.Equal(t, dateStr(i+1), d.Date)
		assert.Equal(t, w.tier, d.Tier, "day %d tier", i+1)
		assert.Equal(t, w.gPerK, d.CarbsGPerKg, "day %d g/kg", i+1)
		require.NotNil(t, d.SuggestedCarbsG, "day %d suggestion", i+1)
		assert.InDelta(t, w.grams, *d.SuggestedCarbsG, 0.001, "day %d grams", i+1)
		assert.False(t, d.PlanMissing, "day %d is inside the plan", i+1)
		// Every day compares against the effective goal midpoint (450).
		require.NotNil(t, d.DeltaG)
		assert.InDelta(t, w.grams-450, *d.DeltaG, 0.001, "day %d delta", i+1)
	}

	// The rest day inside the plan carries no sessions but is not plan_missing.
	assert.Empty(t, out.Days[3].Sessions)
	assert.False(t, out.Days[3].PlanMissing)
}

func TestFuelPlan_HeavyDaySuggestsHeavyPrescription(t *testing.T) {
	f := setup(t)
	seedWeight(t, f, 70.0)
	setGoalCarbs(t, f, 400, 500)
	wid := planSession(t, f.workoutsRepo, day(2), p(180), 120)

	out := getPlan(t, f, week)

	d := out.Days[1]
	assert.Equal(t, fuelplan.TierHeavy, d.Tier)
	require.NotNil(t, d.SuggestedCarbsG)
	assert.InDelta(t, 630, *d.SuggestedCarbsG, 0.001)
	require.NotNil(t, d.GoalCarbsG)
	assert.InDelta(t, 400, *d.GoalCarbsG.Min, 0.001)
	assert.InDelta(t, 500, *d.GoalCarbsG.Max, 0.001)
	require.NotNil(t, d.DeltaG)
	assert.InDelta(t, 180, *d.DeltaG, 0.001)

	// The inputs behind the tier are echoed, so the classification is auditable.
	require.Len(t, d.Sessions, 1)
	assert.Equal(t, wid, d.Sessions[0].WorkoutID)
	assert.Equal(t, "bike", d.Sessions[0].Sport)
	require.NotNil(t, d.Sessions[0].PlannedTSS)
	assert.InDelta(t, 180, *d.Sessions[0].PlannedTSS, 0.001)
	require.NotNil(t, d.Sessions[0].PlannedDurationMin)
	assert.InDelta(t, 120, *d.Sessions[0].PlannedDurationMin, 0.001)
	assert.InDelta(t, 180, d.PlannedTSSTotal, 0.001)
}

func TestFuelPlan_LongLowIntensitySessionIsHeavy(t *testing.T) {
	f := setup(t)
	seedWeight(t, f, 70.0)
	// 160 minutes at 90 TSS: moderate on TSS alone, heavy by the long rule.
	planSession(t, f.workoutsRepo, day(1), p(90), 160)

	out := getPlan(t, f, week)

	assert.Equal(t, fuelplan.TierHeavy, out.Days[0].Tier)
	assert.InDelta(t, 90, out.Days[0].PlannedTSSTotal, 0.001)
	require.NotNil(t, out.Days[0].SuggestedCarbsG)
	assert.InDelta(t, 630, *out.Days[0].SuggestedCarbsG, 0.001)
}

func TestFuelPlan_MultipleSessionsSumOnADay(t *testing.T) {
	f := setup(t)
	seedWeight(t, f, 70.0)
	// Two sessions, neither moderate alone; together 85 TSS → moderate.
	planSession(t, f.workoutsRepo, day(1), p(40), 45)
	planSession(t, f.workoutsRepo, day(1).Add(6*time.Hour), p(45), 50)

	out := getPlan(t, f, week)

	assert.Equal(t, fuelplan.TierModerate, out.Days[0].Tier)
	assert.InDelta(t, 85, out.Days[0].PlannedTSSTotal, 0.001)
	assert.Len(t, out.Days[0].Sessions, 2)
}

func TestFuelPlan_BeyondThePlanIsFlaggedNotDisguisedAsRest(t *testing.T) {
	f := setup(t)
	seedWeight(t, f, 70.0)
	// The plan is materialized through day 3 only.
	planSession(t, f.workoutsRepo, day(1), p(40), 45)
	planSession(t, f.workoutsRepo, day(3), p(90), 60)

	out := getPlan(t, f, week)

	// Day 2 is a rest day INSIDE the plan.
	assert.Equal(t, fuelplan.TierRest, out.Days[1].Tier)
	assert.False(t, out.Days[1].PlanMissing)

	// Days 4–7 are past the last materialized day: rest tier, but flagged.
	for i := 3; i < 7; i++ {
		assert.Equal(t, fuelplan.TierRest, out.Days[i].Tier, "day %d", i+1)
		assert.True(t, out.Days[i].PlanMissing, "day %d must be flagged", i+1)
		// A plan-missing day still prices its rest tier — the flag qualifies the
		// number rather than withholding it.
		require.NotNil(t, out.Days[i].SuggestedCarbsG)
		assert.InDelta(t, 210, *out.Days[i].SuggestedCarbsG, 0.001)
	}
}

func TestFuelPlan_NoPlanDataAtAllFlagsEveryDay(t *testing.T) {
	f := setup(t)
	seedWeight(t, f, 70.0)

	out := getPlan(t, f, week)

	require.Len(t, out.Days, 7)
	for i, d := range out.Days {
		assert.Equal(t, fuelplan.TierRest, d.Tier, "day %d", i+1)
		assert.True(t, d.PlanMissing, "no plan data means we don't know, day %d", i+1)
	}
}

func TestFuelPlan_CompletedWorkoutsDoNotClassify(t *testing.T) {
	f := setup(t)
	seedWeight(t, f, 70.0)
	// A COMPLETED heavy session: the tier reads intent, not results (D1).
	src := "manual"
	w := &workouts.Workout{
		Source:    workouts.SourceManual,
		Sport:     workouts.SportBike,
		Status:    workouts.StatusCompleted,
		StartedAt: day(1),
		EndedAt:   day(1).Add(3 * time.Hour),
		TSS:       p(200),
		TSSSource: &src,
	}
	_, err := f.workoutsRepo.Upsert(context.Background(), w)
	require.NoError(t, err)
	planSession(t, f.workoutsRepo, day(2), p(180), 90)

	out := getPlan(t, f, week)

	assert.Equal(t, fuelplan.TierRest, out.Days[0].Tier, "a completed session is not a plan")
	assert.Empty(t, out.Days[0].Sessions)
	assert.Equal(t, fuelplan.TierHeavy, out.Days[1].Tier)
}

func TestFuelPlan_WeightMissingDegradesToTiersOnly(t *testing.T) {
	f := setup(t)
	setGoalCarbs(t, f, 400, 500)
	planSession(t, f.workoutsRepo, day(1), p(180), 90)

	out := getPlan(t, f, week)

	require.NotNil(t, out.Reason)
	assert.Equal(t, "weight_missing", *out.Reason)
	assert.Nil(t, out.Weight)

	d := out.Days[0]
	assert.Equal(t, fuelplan.TierHeavy, d.Tier, "tiers are weight-free")
	assert.Equal(t, 9.0, d.CarbsGPerKg, "the g/kg prescription still ships")
	assert.Nil(t, d.SuggestedCarbsG)
	assert.Nil(t, d.DeltaG)
	assert.NotNil(t, d.GoalCarbsG, "the goal is still echoed")
}

func TestFuelPlan_StaleWeightIsUsedAndDated(t *testing.T) {
	f := setup(t)
	// One weigh-in, 20 days before the window: in reach of the 30-day lookback,
	// and its date is echoed so the staleness is visible rather than hidden.
	logWeight(t, f.weightRepo, day(1).AddDate(0, 0, -20), 68.0)
	planSession(t, f.workoutsRepo, day(1), p(180), 90)

	out := getPlan(t, f, week)

	require.NotNil(t, out.Weight)
	assert.InDelta(t, 68.0, out.Weight.TrendKg, 0.001)
	// The trailing 7-day average keeps reporting for 6 days after the weigh-in.
	assert.Equal(t, day(1).AddDate(0, 0, -14).Format("2006-01-02"), out.Weight.Date)
	require.NotNil(t, out.Days[0].SuggestedCarbsG)
	assert.InDelta(t, 612, *out.Days[0].SuggestedCarbsG, 0.001) // 9 × 68
}

func TestFuelPlan_EffectiveGoalHonoursPerDateOverride(t *testing.T) {
	f := setup(t)
	seedWeight(t, f, 70.0)
	setGoalCarbs(t, f, 400, 500) // base: midpoint 450
	// Day 2 has an override: midpoint 600.
	require.NoError(t, f.overridesRepo.Upsert(context.Background(), day(2),
		&goals.Goals{CarbsG: &goals.Range{Min: p(550), Max: p(650)}}))

	planSession(t, f.workoutsRepo, day(1), p(180), 90)
	planSession(t, f.workoutsRepo, day(2), p(180), 90)

	out := getPlan(t, f, week)

	// Same heavy tier, same 630 g suggestion — different effective goal, so a
	// different delta. The comparison must be against what actually applies.
	require.NotNil(t, out.Days[0].DeltaG)
	assert.InDelta(t, 180, *out.Days[0].DeltaG, 0.001)

	require.NotNil(t, out.Days[1].DeltaG)
	assert.InDelta(t, 30, *out.Days[1].DeltaG, 0.001)
	assert.InDelta(t, 550, *out.Days[1].GoalCarbsG.Min, 0.001)
}

func TestFuelPlan_NoGoalsConfiguredStillSuggests(t *testing.T) {
	f := setup(t)
	seedWeight(t, f, 70.0)
	planSession(t, f.workoutsRepo, day(1), p(180), 90)

	out := getPlan(t, f, week)

	require.NotNil(t, out.Days[0].SuggestedCarbsG)
	assert.InDelta(t, 630, *out.Days[0].SuggestedCarbsG, 0.001)
	assert.Nil(t, out.Days[0].GoalCarbsG)
	assert.Nil(t, out.Days[0].DeltaG, "nothing to compare against")
}

func TestFuelPlan_DefaultWindowIsTodayPlusSix(t *testing.T) {
	f := setup(t)

	out := getPlan(t, f, "")

	require.Len(t, out.Days, 7)
	today := time.Now().UTC().Format("2006-01-02")
	assert.Equal(t, today, out.From)
	assert.Equal(t, today, out.Days[0].Date)
	sixth := time.Now().UTC().AddDate(0, 0, 6).Format("2006-01-02")
	assert.Equal(t, sixth, out.To)
	assert.Equal(t, sixth, out.Days[6].Date)
}

func TestFuelPlan_TZLocalDayBucketing(t *testing.T) {
	f := setup(t)
	seedWeight(t, f, 70.0)
	// 23:30 UTC on day 1 is day 2 in Berlin (+2 in July).
	planSession(t, f.workoutsRepo, day(1).Add(14*time.Hour+30*time.Minute), p(180), 90)

	// In UTC the session belongs to day 1.
	utc := getPlan(t, f, week)
	assert.Equal(t, fuelplan.TierHeavy, utc.Days[0].Tier)
	assert.Equal(t, fuelplan.TierRest, utc.Days[1].Tier)

	// In Berlin (UTC+2 in July) 23:30 UTC is 01:30 the next morning, so the
	// same session fuels day 2 — the athlete's calendar decides, not UTC.
	berlin := getPlan(t, f, "from=2026-07-20&to=2026-07-26&tz=Europe/Berlin")
	assert.Equal(t, "Europe/Berlin", berlin.TZ)
	assert.Equal(t, fuelplan.TierRest, berlin.Days[0].Tier)
	assert.Equal(t, fuelplan.TierHeavy, berlin.Days[1].Tier)
	assert.Len(t, berlin.Days[1].Sessions, 1)
}

func TestFuelPlan_ReadOnly(t *testing.T) {
	f := setup(t)
	seedWeight(t, f, 70.0)
	setGoalCarbs(t, f, 400, 500)
	planSession(t, f.workoutsRepo, day(1), p(180), 90)

	getPlan(t, f, week)

	// The suggestion must not have become an override.
	overrides, err := f.overridesRepo.List(context.Background(), day(1).AddDate(0, 0, -1), day(7))
	require.NoError(t, err)
	assert.Empty(t, overrides, "fuel-plan suggestions never write overrides")

	// The base goal is untouched.
	g, err := f.goalsRepo.Get(context.Background())
	require.NoError(t, err)
	assert.InDelta(t, 400, *g.CarbsG.Min, 0.001)
	assert.InDelta(t, 500, *g.CarbsG.Max, 0.001)

	// The plan itself is unchanged.
	status := string(workouts.StatusPlanned)
	planned, err := f.workoutsRepo.List(context.Background(), day(1).AddDate(0, 0, -1), day(7), nil, &status)
	require.NoError(t, err)
	assert.Len(t, planned, 1)
	assert.InDelta(t, 180, *planned[0].TSS, 0.001)
}

func TestFuelPlan_RangeErrors(t *testing.T) {
	f := setup(t)

	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"from without to", "from=2026-07-20", "range_required"},
		{"to without from", "to=2026-07-26", "range_required"},
		{"unparseable from", "from=nope&to=2026-07-26", "date_invalid"},
		{"unparseable to", "from=2026-07-20&to=nope", "date_invalid"},
		{"inverted", "from=2026-07-26&to=2026-07-20", "range_invalid"},
		{"too large", "from=2026-07-01&to=2026-07-20", "range_too_large"},
		{"bad tz", "tz=Mars/Olympus", "tz_invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doGet(t, f.r, "/nutrition/fuel-plan?"+tc.query)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tc.want, body["error"])
		})
	}
}

func TestFuelPlan_RangeCapBoundary(t *testing.T) {
	f := setup(t)

	// 14 days inclusive passes; 15 is over the cap.
	rec := doGet(t, f.r, "/nutrition/fuel-plan?from=2026-07-20&to=2026-08-02")
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = doGet(t, f.r, "/nutrition/fuel-plan?from=2026-07-20&to=2026-08-03")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "range_too_large", body["error"])
	assert.Equal(t, float64(14), body["max_days"])
}

func TestFuelPlan_SingleDayWindow(t *testing.T) {
	f := setup(t)
	seedWeight(t, f, 70.0)
	planSession(t, f.workoutsRepo, day(1), p(180), 90)

	out := getPlan(t, f, "from=2026-07-20&to=2026-07-20&tz=UTC")

	require.Len(t, out.Days, 1)
	assert.Equal(t, fuelplan.TierHeavy, out.Days[0].Tier)
}
