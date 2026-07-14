package trainingplan_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/athleteconfig"
	"github.com/vinzenzs/kazper/internal/multisport"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/trainingplan"
	"github.com/vinzenzs/kazper/internal/workouts"
	"github.com/vinzenzs/kazper/internal/workouttemplates"
)

// setupMultisport wires the single-sport routes plus the multisport-template and
// athlete-config handlers, cross-injecting both repos into the training-plan
// service (mirroring httpserver.Run) so a plan slot can reference a multisport
// template and EffectiveProgram resolves each segment per its own sport. Returns
// the engine and the pool (so a test can flip a row to completed directly).
func setupMultisport(t *testing.T) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	pool := storetest.NewPool(t)
	r := gin.New()
	g := r.Group("/")
	tr := workouttemplates.NewRepo(pool)
	workouttemplates.NewHandlers(workouttemplates.NewService(tr)).Register(g)
	wr := workouts.NewRepo(pool)
	workouts.NewHandlers(workouts.NewService(wr, pool, "UTC")).Register(g)
	acRepo := athleteconfig.NewRepo(pool)
	athleteconfig.NewHandlers(athleteconfig.NewService(acRepo, pool)).Register(g)
	msRepo := multisport.NewRepo(pool)
	multisport.NewHandlers(multisport.NewService(msRepo)).Register(g)
	tpSvc := trainingplan.NewService(trainingplan.NewRepo(pool), pool, wr, tr, "UTC")
	tpSvc.SetAthleteConfigRepo(acRepo)
	tpSvc.SetMultisportRepo(msRepo)
	trainingplan.NewHandlers(tpSvc).Register(g)
	return r, pool
}

// createBrickTemplate makes a bike→T2→run multisport template (a bike segment
// targeting power_zone 4, a 5-minute transition, a run segment with an hr_zone
// target) and returns its id. Total bounded length = 1800 + 300 + 1200 = 3300s.
func createBrickTemplate(t *testing.T, r *gin.Engine) string {
	body := `{"name":"Brick","segments":[` +
		`{"sport":"bike","steps":[{"type":"step","intent":"active","duration":{"kind":"time","seconds":1800},"target":{"kind":"power_zone","low":4,"high":4}}]},` +
		`{"sport":"transition","duration":{"kind":"time","seconds":300}},` +
		`{"sport":"run","steps":[{"type":"step","intent":"active","duration":{"kind":"time","seconds":1200},"target":{"kind":"hr_zone","low":2,"high":3}}]}` +
		`]}`
	return mustID(t, do(t, r, http.MethodPost, "/multisport-templates", body))
}

// emptyPlanWeek creates a plan starting Mon 2026-06-01 with one week and returns
// the plan and week ids (no slots).
func emptyPlanWeek(t *testing.T, r *gin.Engine) (planID, weekID string) {
	planID = mustID(t, do(t, r, http.MethodPost, "/training-plans", `{"name":"p","start_date":"2026-06-01"}`))
	weekID = mustID(t, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks", `{"ordinal":1}`))
	return
}

// ----- 8.1 slot XOR validation -----

func TestCreateSlot_NeitherTemplateRejected(t *testing.T) {
	r, _ := setupMultisport(t)
	_, weekID := emptyPlanWeek(t, r)
	rec := do(t, r, http.MethodPost, "/training-plans/x/weeks/"+weekID+"/slots", `{"weekday":0,"ordinal":0}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "template_id_required")
}

func TestCreateSlot_BothTemplatesRejected(t *testing.T) {
	r, _ := setupMultisport(t)
	_, weekID := emptyPlanWeek(t, r)
	tmplID := createTemplate(t, r)
	msID := createBrickTemplate(t, r)
	rec := do(t, r, http.MethodPost, "/training-plans/x/weeks/"+weekID+"/slots",
		`{"weekday":0,"ordinal":0,"template_id":"`+tmplID+`","multisport_template_id":"`+msID+`"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "template_reference_ambiguous")
}

func TestCreateSlot_MultisportSlotAccepted(t *testing.T) {
	r, _ := setupMultisport(t)
	_, weekID := emptyPlanWeek(t, r)
	msID := createBrickTemplate(t, r)
	rec := do(t, r, http.MethodPost, "/training-plans/x/weeks/"+weekID+"/slots",
		`{"weekday":0,"ordinal":0,"multisport_template_id":"`+msID+`"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var slot map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &slot))
	assert.Equal(t, msID, slot["multisport_template_id"])
	_, hasTmpl := slot["template_id"]
	assert.False(t, hasTmpl, "single-sport template_id absent on a multisport slot")
}

func TestCreateSlot_OverridesOnMultisportRejected(t *testing.T) {
	r, _ := setupMultisport(t)
	_, weekID := emptyPlanWeek(t, r)
	msID := createBrickTemplate(t, r)
	override := `"target_overrides":[{"intent":"active","target":{"kind":"hr_zone","low":2,"high":2}}]`
	rec := do(t, r, http.MethodPost, "/training-plans/x/weeks/"+weekID+"/slots",
		`{"weekday":0,"ordinal":0,"multisport_template_id":"`+msID+`",`+override+`}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "overrides_unsupported_for_multisport")
}

// ----- 8.2 materialize a multisport slot -----

func TestMaterialize_MultisportSlotEmitsMultisportWorkout(t *testing.T) {
	r, pool := setupMultisport(t)
	planID, weekID := emptyPlanWeek(t, r)
	msID := createBrickTemplate(t, r)
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks/"+weekID+"/slots",
		`{"weekday":0,"ordinal":0,"multisport_template_id":"`+msID+`"}`).Code)

	ws := materialize(t, r, planID, `{"scope":"all"}`)
	require.Len(t, ws, 1)
	w := ws[0]
	assert.Equal(t, "multisport", w["sport"])
	assert.Equal(t, "Brick", w["name"])
	assert.Equal(t, "planned", w["status"])
	assert.Equal(t, msID, w["multisport_template_id"])
	_, hasTmpl := w["template_id"]
	assert.False(t, hasTmpl, "no single-sport template_id on a multisport workout")
	// Session length = summed segment step + transition durations (1800+300+1200).
	assert.Equal(t, "2026-06-01T06:00:00Z", utcInstant(t, w["started_at"].(string)))
	assert.Equal(t, "2026-06-01T06:55:00Z", utcInstant(t, w["ended_at"].(string)))

	// Idempotent: re-materializing updates the same row, not a duplicate.
	ws2 := materialize(t, r, planID, `{"scope":"all"}`)
	require.Len(t, ws2, 1)
	assert.Equal(t, w["id"], ws2[0]["id"])

	// Never reverts a completed row: flip to completed, re-materialize, confirm
	// the row is left completed (the upsert's status='planned' guard holds).
	_, err := pool.Exec(context.Background(),
		`UPDATE workouts SET status='completed' WHERE id=$1`, w["id"])
	require.NoError(t, err)
	materialize(t, r, planID, `{"scope":"all"}`)
	var status string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT status FROM workouts WHERE id=$1`, w["id"]).Scan(&status))
	assert.Equal(t, "completed", status, "completed multisport row not reverted by re-materialize")
}

// ----- 8.3 effective program per-segment resolution -----

func TestWorkoutProgram_MultisportResolvesPerSegment(t *testing.T) {
	r, _ := setupMultisport(t)
	require.Equal(t, http.StatusOK, do(t, r, http.MethodPut, "/athlete-config",
		`{"power_zone_3_max":230,"power_zone_4_max":268}`).Code)
	planID, weekID := emptyPlanWeek(t, r)
	msID := createBrickTemplate(t, r)
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks/"+weekID+"/slots",
		`{"weekday":0,"ordinal":0,"multisport_template_id":"`+msID+`"}`).Code)
	ws := materialize(t, r, planID, `{"scope":"all"}`)
	require.Len(t, ws, 1)

	prog := program(t, r, ws[0]["id"].(string))
	assert.Equal(t, "multisport", prog["sport"])
	_, hasSteps := prog["steps"].([]any)
	assert.False(t, hasSteps, "multisport program has segments, not a flat step list")

	segs := prog["segments"].([]any)
	require.Len(t, segs, 3, "bike, transition, run in order")

	bike := segs[0].(map[string]any)
	assert.Equal(t, "bike", bike["sport"])
	bikeTarget := bike["steps"].([]any)[0].(map[string]any)["target"].(map[string]any)
	assert.Equal(t, "power_w", bikeTarget["kind"], "bike power_zone resolved to watts")
	assert.Equal(t, float64(230), bikeTarget["low"])
	assert.Equal(t, float64(268), bikeTarget["high"])

	transition := segs[1].(map[string]any)
	assert.Equal(t, "transition", transition["sport"])
	assert.NotNil(t, transition["duration"], "transition segment carries its duration")

	run := segs[2].(map[string]any)
	assert.Equal(t, "run", run["sport"])
	runTarget := run["steps"].([]any)[0].(map[string]any)["target"].(map[string]any)
	assert.Equal(t, "hr_zone", runTarget["kind"], "run hr_zone passes through unchanged")
}
