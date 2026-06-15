package trainingplan_test

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/athleteconfig"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/trainingplan"
	"github.com/vinzenzs/kazper/internal/workouts"
	"github.com/vinzenzs/kazper/internal/workouttemplates"
)

// setupResolve wires the same routes as setup() plus the athlete-config handlers
// and cross-injects its repo into the training-plan service (mirroring
// httpserver.Run), so EffectiveProgram resolves zone targets. Returns the engine
// and the athlete-config repo so a test can opt out of wiring config.
func setupResolve(t *testing.T, wireConfig bool) *gin.Engine {
	t.Helper()
	pool := storetest.NewPool(t)
	r := gin.New()
	g := r.Group("/")
	tr := workouttemplates.NewRepo(pool)
	workouttemplates.NewHandlers(workouttemplates.NewService(tr)).Register(g)
	wr := workouts.NewRepo(pool)
	workouts.NewHandlers(workouts.NewService(wr, pool, "UTC")).Register(g)
	acRepo := athleteconfig.NewRepo(pool)
	athleteconfig.NewHandlers(athleteconfig.NewService(acRepo)).Register(g)
	tpSvc := trainingplan.NewService(trainingplan.NewRepo(pool), pool, wr, tr, "UTC")
	if wireConfig {
		tpSvc.SetAthleteConfigRepo(acRepo)
	}
	trainingplan.NewHandlers(tpSvc).Register(g)
	return r
}

// createBikeTemplate makes a bike template whose single active step targets
// power_zone 4, and returns its id.
func createBikeTemplate(t *testing.T, r *gin.Engine) string {
	body := `{"sport":"bike","name":"FTP intervals","estimated_duration_sec":3600,"steps":[{"type":"step","intent":"active","duration":{"kind":"time","seconds":3600},"target":{"kind":"power_zone","low":4,"high":4}}]}`
	return mustID(t, do(t, r, http.MethodPost, "/workout-templates", body))
}

func planFromTemplate(t *testing.T, r *gin.Engine, templateID string) string {
	t.Helper()
	planID := mustID(t, do(t, r, http.MethodPost, "/training-plans", `{"name":"p","start_date":"2026-06-01"}`))
	weekID := mustID(t, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks", `{"ordinal":1}`))
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/training-plans/"+planID+"/weeks/"+weekID+"/slots",
		`{"weekday":0,"ordinal":0,"template_id":"`+templateID+`"}`).Code)
	return planID
}

func TestWorkoutProgram_ZoneTargetResolvesWithOrigin(t *testing.T) {
	r := setupResolve(t, true)
	require.Equal(t, http.StatusOK, do(t, r, http.MethodPut, "/athlete-config",
		`{"power_zone_3_max":230,"power_zone_4_max":268}`).Code)
	templateID := createBikeTemplate(t, r)
	planID := planFromTemplate(t, r, templateID)
	ws := materialize(t, r, planID, `{"scope":"all"}`)
	require.Len(t, ws, 1)

	prog := program(t, r, ws[0]["id"].(string))
	target := prog["steps"].([]any)[0].(map[string]any)["target"].(map[string]any)
	assert.Equal(t, "power_w", target["kind"], "power_zone resolved to absolute watts")
	assert.Equal(t, float64(230), target["low"]) // power_zone_3_max
	assert.Equal(t, float64(268), target["high"]) // power_zone_4_max
	assert.Equal(t, "Z4", target["origin"])
}

func TestWorkoutProgram_MissingConfigLeavesZoneTargetUnchanged(t *testing.T) {
	// Config wired but never populated → zone target passes through unchanged.
	r := setupResolve(t, true)
	templateID := createBikeTemplate(t, r)
	planID := planFromTemplate(t, r, templateID)
	ws := materialize(t, r, planID, `{"scope":"all"}`)
	require.Len(t, ws, 1)

	prog := program(t, r, ws[0]["id"].(string))
	target := prog["steps"].([]any)[0].(map[string]any)["target"].(map[string]any)
	assert.Equal(t, "power_zone", target["kind"], "no config → zone reference unchanged")
	assert.Equal(t, float64(4), target["low"])
	assert.Nil(t, target["origin"])
}
