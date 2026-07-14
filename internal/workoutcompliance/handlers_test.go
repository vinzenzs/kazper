package workoutcompliance_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/athleteconfig"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/trainingplan"
	"github.com/vinzenzs/kazper/internal/workoutcompliance"
	"github.com/vinzenzs/kazper/internal/workouts"
	"github.com/vinzenzs/kazper/internal/workouttemplates"
)

func init() { gin.SetMode(gin.TestMode) }

type fixture struct {
	r     *gin.Engine
	pool  *pgxpool.Pool
	wrepo *workouts.Repo
	trepo *workouttemplates.Repo
}

func setup(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	r := gin.New()
	g := r.Group("/")

	trepo := workouttemplates.NewRepo(pool)
	wrepo := workouts.NewRepo(pool)
	acRepo := athleteconfig.NewRepo(pool)
	// Full training-plan wiring (HTTP + athlete-config cross-inject) so plan/slot
	// creation and zone resolution work for the override test.
	athleteconfig.NewHandlers(athleteconfig.NewService(acRepo)).Register(g)
	workouttemplates.NewHandlers(workouttemplates.NewService(trepo)).Register(g)
	workouts.NewHandlers(workouts.NewService(wrepo, pool, "UTC")).Register(g)
	tpSvc := trainingplan.NewService(trainingplan.NewRepo(pool), pool, wrepo, trepo, "UTC")
	tpSvc.SetAthleteConfigRepo(acRepo)
	trainingplan.NewHandlers(tpSvc).Register(g)
	workoutcompliance.NewHandlers(workoutcompliance.NewService(wrepo, tpSvc)).Register(g)

	return &fixture{r: r, pool: pool, wrepo: wrepo, trepo: trepo}
}

func iptr(i int) *int         { return &i }
func fptr(f float64) *float64 { return &f }

func single(intent string, secs int, target *workouttemplates.Target) workouttemplates.Step {
	return workouttemplates.Step{
		Type:     workouttemplates.NodeStep,
		Intent:   intent,
		Duration: &workouttemplates.Duration{Kind: workouttemplates.DurationTime, Seconds: iptr(secs)},
		Target:   target,
	}
}

func powerW(lo, hi int) *workouttemplates.Target {
	return &workouttemplates.Target{Kind: workouttemplates.TargetPowerW, Low: iptr(lo), High: iptr(hi)}
}

func (f *fixture) createTemplate(t *testing.T, sport string, steps []workouttemplates.Step) uuid.UUID {
	t.Helper()
	name := "T"
	tmpl, err := f.trepo.Create(context.Background(), &workouttemplates.Template{Sport: sport, Name: name, Steps: steps})
	require.NoError(t, err)
	return uuid.MustParse(tmpl.ID)
}

// seedWorkout upserts a workout, optionally links a template (raw UPDATE, since
// Upsert doesn't own template_id), and replaces its splits. status is set as
// given so the error-path tests can seed a planned row.
func (f *fixture) seedWorkout(t *testing.T, sport string, status workouts.Status, templateID *uuid.UUID, splits []workouts.Split) uuid.UUID {
	t.Helper()
	start := time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC)
	w := &workouts.Workout{
		Source:    workouts.SourceManual,
		Sport:     workouts.Sport(sport),
		Status:    status,
		StartedAt: start,
		EndedAt:   start.Add(time.Hour),
	}
	_, err := f.wrepo.Upsert(context.Background(), w)
	require.NoError(t, err)
	if templateID != nil {
		_, err = f.pool.Exec(context.Background(), `UPDATE workouts SET template_id = $1 WHERE id = $2`, *templateID, w.ID)
		require.NoError(t, err)
	}
	if splits != nil {
		require.NoError(t, f.wrepo.ReplaceChildren(context.Background(), f.pool, w.ID, splits, nil))
	}
	return w.ID
}

func powerSplit(idx, watts, durS int) workouts.Split {
	return workouts.Split{SplitIndex: idx, AvgPowerW: iptr(watts), DurationS: fptr(float64(durS))}
}

func (f *fixture) get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	f.r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func (f *fixture) compliance(t *testing.T, id uuid.UUID) (workoutcompliance.Result, int) {
	t.Helper()
	rec := f.get(t, "/workouts/"+id.String()+"/compliance")
	if rec.Code != http.StatusOK {
		return workoutcompliance.Result{}, rec.Code
	}
	var res workoutcompliance.Result
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	return res, rec.Code
}

func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body["error"].(string)
}

// --- happy path -----------------------------------------------------------

func TestCompliance_ScoresTemplateLinkedWorkout(t *testing.T) {
	f := setup(t)
	tmpl := f.createTemplate(t, "bike", []workouttemplates.Step{
		single("warmup", 600, nil),
		single("active", 1800, powerW(250, 265)),
		single("cooldown", 300, nil),
	})
	id := f.seedWorkout(t, "bike", workouts.StatusCompleted, &tmpl, []workouts.Split{
		powerSplit(0, 120, 600),
		powerSplit(1, 230, 1800), // 20W under the 250 floor
		powerSplit(2, 110, 300),
	})

	res, code := f.compliance(t, id)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, workoutcompliance.StatusScored, res.Status)
	require.Len(t, res.Steps, 3)

	active := res.Steps[1]
	require.NotNil(t, active.Target)
	require.True(t, active.Target.Scorable)
	assert.Equal(t, "power_w", active.Target.Metric)
	assert.Equal(t, workoutcompliance.ClassUnder, active.Target.Classification)
	require.NotNil(t, active.Target.Delta)
	assert.InDelta(t, -20, *active.Target.Delta, 0.01)
	require.NotNil(t, res.Score)
}

func TestCompliance_ComputeOnReadDoesNotMutate(t *testing.T) {
	f := setup(t)
	tmpl := f.createTemplate(t, "bike", []workouttemplates.Step{single("active", 1800, powerW(250, 265))})
	id := f.seedWorkout(t, "bike", workouts.StatusCompleted, &tmpl, []workouts.Split{powerSplit(0, 258, 1800)})

	var before time.Time
	require.NoError(t, f.pool.QueryRow(context.Background(), `SELECT updated_at FROM workouts WHERE id=$1`, id).Scan(&before))
	_, code := f.compliance(t, id)
	require.Equal(t, http.StatusOK, code)
	var after time.Time
	require.NoError(t, f.pool.QueryRow(context.Background(), `SELECT updated_at FROM workouts WHERE id=$1`, id).Scan(&after))
	assert.True(t, before.Equal(after), "compliance read must not mutate the workout row")
}

// --- repeat expansion end-to-end ------------------------------------------

func TestCompliance_RepeatExpansion(t *testing.T) {
	f := setup(t)
	tmpl := f.createTemplate(t, "bike", []workouttemplates.Step{
		single("warmup", 600, nil),
		{Type: workouttemplates.NodeRepeat, Count: 5, Steps: []workouttemplates.Step{
			single("interval", 180, powerW(250, 265)),
			single("recovery", 120, nil),
		}},
		single("cooldown", 300, nil),
	})
	// 1 + 5*2 + 1 = 12 laps.
	splits := make([]workouts.Split, 12)
	for i := range splits {
		splits[i] = powerSplit(i, 255, 180)
	}
	id := f.seedWorkout(t, "bike", workouts.StatusCompleted, &tmpl, splits)

	res, code := f.compliance(t, id)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, workoutcompliance.StatusScored, res.Status)
	require.Len(t, res.Steps, 12)

	// The third interval sits at flat index 1 + (3-1)*2 = 5, iteration 3 of 5.
	third := res.Steps[5]
	assert.Equal(t, "interval", third.Intent)
	require.NotNil(t, third.Iteration)
	assert.Equal(t, 3, *third.Iteration)
	require.NotNil(t, third.Of)
	assert.Equal(t, 5, *third.Of)
}

// --- lap-count mismatch ---------------------------------------------------

func TestCompliance_LapCountMismatchUnavailable(t *testing.T) {
	f := setup(t)
	tmpl := f.createTemplate(t, "bike", []workouttemplates.Step{
		single("warmup", 600, nil),
		{Type: workouttemplates.NodeRepeat, Count: 5, Steps: []workouttemplates.Step{
			single("interval", 180, powerW(250, 265)),
			single("recovery", 120, nil),
		}},
		single("cooldown", 300, nil),
	})
	splits := make([]workouts.Split, 9) // 9 laps vs 12 steps
	for i := range splits {
		splits[i] = powerSplit(i, 255, 120)
	}
	id := f.seedWorkout(t, "bike", workouts.StatusCompleted, &tmpl, splits)

	res, code := f.compliance(t, id)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, workoutcompliance.StatusUnavailable, res.Status)
	require.NotNil(t, res.Reason)
	assert.Equal(t, "lap_count_mismatch", *res.Reason)
	assert.Equal(t, 12, res.PlannedSteps)
	assert.Equal(t, 9, res.ExecutedLaps)
	assert.Empty(t, res.Steps)
}

// --- slot overrides shape the compared band -------------------------------

func TestCompliance_SlotOverridesShapeTarget(t *testing.T) {
	f := setup(t)
	// Template's active step targets a low band; the slot override raises it.
	tmpl := f.createTemplate(t, "bike", []workouttemplates.Step{single("active", 1800, powerW(180, 190))})

	planID := mustID(t, f.post(t, "/training-plans", `{"name":"p","start_date":"2026-06-01"}`))
	weekID := mustID(t, f.post(t, "/training-plans/"+planID+"/weeks", `{"ordinal":1}`))
	slotID := mustID(t, f.post(t, "/training-plans/"+planID+"/weeks/"+weekID+"/slots",
		`{"weekday":0,"ordinal":0,"template_id":"`+tmpl.String()+`","target_overrides":[{"intent":"active","target":{"kind":"power_w","low":250,"high":265}}]}`))

	id := f.seedWorkout(t, "bike", workouts.StatusCompleted, &tmpl, []workouts.Split{powerSplit(0, 258, 1800)})
	// Link the plan slot so EffectiveProgram applies its overrides.
	_, err := f.pool.Exec(context.Background(), `UPDATE workouts SET plan_slot_id = $1 WHERE id = $2`, slotID, id)
	require.NoError(t, err)

	res, code := f.compliance(t, id)
	require.Equal(t, http.StatusOK, code)
	require.Len(t, res.Steps, 1)
	tr := res.Steps[0].Target
	require.NotNil(t, tr)
	require.NotNil(t, tr.Low)
	assert.InDelta(t, 250, *tr.Low, 0.01, "compared against the overridden band, not 180–190")
	assert.InDelta(t, 265, *tr.High, 0.01)
	assert.Equal(t, workoutcompliance.ClassInBand, tr.Classification) // 258 within 250–265
}

// --- error paths ----------------------------------------------------------

func TestCompliance_ErrorPaths(t *testing.T) {
	f := setup(t)

	// Unknown id → 404.
	assert.Equal(t, http.StatusNotFound, f.get(t, "/workouts/"+uuid.New().String()+"/compliance").Code)

	// Malformed id → 400 workout_id_invalid.
	badRec := f.get(t, "/workouts/not-a-uuid/compliance")
	require.Equal(t, http.StatusBadRequest, badRec.Code)
	assert.Equal(t, "workout_id_invalid", errCode(t, badRec))

	// Planned workout → 409 workout_not_completed.
	tmpl := f.createTemplate(t, "bike", []workouttemplates.Step{single("active", 1800, powerW(250, 265))})
	planned := f.seedWorkout(t, "bike", workouts.StatusPlanned, &tmpl, []workouts.Split{powerSplit(0, 258, 1800)})
	rec := f.get(t, "/workouts/"+planned.String()+"/compliance")
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "workout_not_completed", errCode(t, rec))

	// Multisport → 409 multisport_unsupported (before the template check).
	ms := f.seedWorkout(t, "multisport", workouts.StatusCompleted, nil, nil)
	rec = f.get(t, "/workouts/"+ms.String()+"/compliance")
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "multisport_unsupported", errCode(t, rec))

	// Completed but no template link → 409 no_template_link.
	noTmpl := f.seedWorkout(t, "bike", workouts.StatusCompleted, nil, []workouts.Split{powerSplit(0, 258, 1800)})
	rec = f.get(t, "/workouts/"+noTmpl.String()+"/compliance")
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "no_template_link", errCode(t, rec))

	// Completed + linked but zero splits → 409 splits_missing.
	splitless := f.seedWorkout(t, "bike", workouts.StatusCompleted, &tmpl, nil)
	rec = f.get(t, "/workouts/"+splitless.String()+"/compliance")
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "splits_missing", errCode(t, rec))
}

// --- small HTTP helpers ---------------------------------------------------

func (f *fixture) post(t *testing.T, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	f.r.ServeHTTP(rec, req)
	return rec
}

func mustID(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	id, _ := body["id"].(string)
	require.NotEmpty(t, id)
	return id
}
