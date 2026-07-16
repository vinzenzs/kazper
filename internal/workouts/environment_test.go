package workouts_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/workouts"
)

// ============================================================================
// environment — does ambient weather apply to this session (indoor/outdoor)
// ============================================================================

const envBase = `"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z"`

func postWorkout(t *testing.T, f *fixture, body string) workouts.Workout {
	t.Helper()
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var w workouts.Workout
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &w))
	return w
}

func TestPost_WithEnvironment_EchoesField(t *testing.T) {
	f := setup(t)
	for _, v := range []workouts.Environment{workouts.EnvironmentIndoor, workouts.EnvironmentOutdoor} {
		w := postWorkout(t, f, fmt.Sprintf(`{%s,"environment":%q}`, envBase, v))
		require.NotNil(t, w.Environment, "environment=%s", v)
		assert.Equal(t, v, *w.Environment)
	}
}

func TestPost_WithoutEnvironment_OmitsFromResponse(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodPost, "/workouts", fmt.Sprintf(`{%s}`, envBase))
	require.Equal(t, http.StatusCreated, rec.Code)
	// Null means "not stated" — it must never serialize as a value.
	assert.NotContains(t, rec.Body.String(), `"environment"`)
}

func TestPost_InvalidEnvironment_Returns400(t *testing.T) {
	f := setup(t)
	// "" is rejected on CREATE: the empty-string clear is a PATCH affordance,
	// not a way to spell "not stated" on a write that can simply omit the key.
	for _, bad := range []string{"garage", "gym", "INDOOR", "Outdoor", "true", ""} {
		rec := doReq(t, f.r, http.MethodPost, "/workouts",
			fmt.Sprintf(`{%s,"environment":%q}`, envBase, bad))
		require.Equal(t, http.StatusBadRequest, rec.Code, "environment=%q", bad)
		assert.JSONEq(t, `{"error":"environment_invalid"}`, rec.Body.String())
	}
}

func TestPost_EnvironmentIndependentOfSport(t *testing.T) {
	f := setup(t)
	// A pool swim is "indoor" because ambient weather doesn't apply — the field
	// is not sport-coupled and claims nothing about roofs.
	cases := []struct{ sport, env string }{
		{"swim", "indoor"},
		{"run", "indoor"},   // treadmill
		{"bike", "outdoor"}, // road ride
		{"strength", "indoor"},
	}
	for _, c := range cases {
		rec := doReq(t, f.r, http.MethodPost, "/workouts", fmt.Sprintf(
			`{"source":"manual","sport":%q,"started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z","environment":%q}`,
			c.sport, c.env))
		require.Equal(t, http.StatusCreated, rec.Code, "%s/%s: %s", c.sport, c.env, rec.Body.String())
	}
}

func TestGet_ReturnsEnvironmentWhenSet(t *testing.T) {
	f := setup(t)
	w := postWorkout(t, f, fmt.Sprintf(`{%s,"environment":"indoor"}`, envBase))

	rec := doReq(t, f.r, http.MethodGet, "/workouts/"+w.ID.String(), "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"environment":"indoor"`)
}

func TestPatch_SetEnvironment(t *testing.T) {
	f := setup(t)
	w := postWorkout(t, f, fmt.Sprintf(`{%s}`, envBase))

	rec := doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(), `{"environment":"outdoor"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var got workouts.Workout
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.NotNil(t, got.Environment)
	assert.Equal(t, workouts.EnvironmentOutdoor, *got.Environment)
}

func TestPatch_AbsentEnvironment_LeavesUnchanged(t *testing.T) {
	f := setup(t)
	w := postWorkout(t, f, fmt.Sprintf(`{%s,"environment":"indoor"}`, envBase))

	rec := doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(), `{"notes":"trainer session"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"environment":"indoor"`)
}

// The spec names the empty string as the clearing sentinel; the endpoint's
// every other nullable field clears with JSON null. Both are accepted, and
// neither is ambiguous — "" is not a valid environment.
func TestPatch_ClearEnvironment_BothSentinelsWork(t *testing.T) {
	for _, clear := range []string{`""`, `null`} {
		t.Run(clear, func(t *testing.T) {
			f := setup(t)
			w := postWorkout(t, f, fmt.Sprintf(`{%s,"environment":"outdoor"}`, envBase))

			rec := doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(),
				fmt.Sprintf(`{"environment":%s}`, clear))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.NotContains(t, rec.Body.String(), `"environment"`)

			// And it's actually cleared in the store, not just in the echo.
			getRec := doReq(t, f.r, http.MethodGet, "/workouts/"+w.ID.String(), "")
			require.Equal(t, http.StatusOK, getRec.Code)
			assert.NotContains(t, getRec.Body.String(), `"environment"`)
		})
	}
}

func TestPatch_InvalidEnvironment_Returns400(t *testing.T) {
	f := setup(t)
	w := postWorkout(t, f, fmt.Sprintf(`{%s}`, envBase))

	for _, bad := range []string{`"garage"`, `"INDOOR"`, `7`, `true`} {
		rec := doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(),
			fmt.Sprintf(`{"environment":%s}`, bad))
		require.Equal(t, http.StatusBadRequest, rec.Code, "environment=%s", bad)
		assert.JSONEq(t, `{"error":"environment_invalid"}`, rec.Body.String())
	}
}

func TestPatch_EnvironmentRoundTripSetClearSet(t *testing.T) {
	f := setup(t)
	w := postWorkout(t, f, fmt.Sprintf(`{%s}`, envBase))
	url := "/workouts/" + w.ID.String()

	// The spec's scenario, end to end: set → clear → the omitted key leaves it.
	rec := doReq(t, f.r, http.MethodPatch, url, `{"environment":"indoor"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"environment":"indoor"`)

	rec = doReq(t, f.r, http.MethodPatch, url, `{"environment":""}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), `"environment"`)

	rec = doReq(t, f.r, http.MethodPatch, url, `{"name":"unrelated"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), `"environment"`, "an omitted key leaves a cleared field cleared")

	rec = doReq(t, f.r, http.MethodPatch, url, `{"environment":"outdoor"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"environment":"outdoor"`)
}

func TestBulk_AcceptsEnvironment(t *testing.T) {
	f := setup(t)
	body := `{"workouts":[
        {"source":"garmin","external_id":"env-indoor-1","sport":"bike","status":"completed",
         "started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:00:00Z","environment":"indoor"},
        {"source":"garmin","external_id":"env-outdoor-1","sport":"bike","status":"completed",
         "started_at":"2026-06-08T08:00:00Z","ended_at":"2026-06-08T09:00:00Z","environment":"outdoor"},
        {"source":"garmin","external_id":"env-unstated-1","sport":"bike","status":"completed",
         "started_at":"2026-06-09T08:00:00Z","ended_at":"2026-06-09T09:00:00Z"}
    ]}`
	rec := doReq(t, f.r, http.MethodPost, "/workouts/bulk", body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out struct {
		Results []map[string]any `json:"results"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Results, 3)

	want := []*workouts.Environment{
		ptrEnv(workouts.EnvironmentIndoor),
		ptrEnv(workouts.EnvironmentOutdoor),
		nil, // unstated stays unstated — bulk invents nothing
	}
	for i, r := range out.Results {
		require.Nil(t, r["error"], "item %d: %v", i, r["error"])
		id, ok := r["id"].(string)
		require.True(t, ok, "item %d has no id", i)

		getRec := doReq(t, f.r, http.MethodGet, "/workouts/"+id, "")
		require.Equal(t, http.StatusOK, getRec.Code)
		var w workouts.Workout
		require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &w))
		if want[i] == nil {
			assert.Nil(t, w.Environment, "item %d must stay unstated", i)
			continue
		}
		require.NotNil(t, w.Environment, "item %d", i)
		assert.Equal(t, *want[i], *w.Environment)
	}
}

func TestBulk_InvalidEnvironment_ReportsPerItemError(t *testing.T) {
	f := setup(t)
	// A bad environment fails only its own item — the sibling still lands.
	body := `{"workouts":[
        {"source":"garmin","external_id":"env-bad-1","sport":"bike","status":"completed",
         "started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:00:00Z","environment":"garage"},
        {"source":"garmin","external_id":"env-good-1","sport":"bike","status":"completed",
         "started_at":"2026-06-08T08:00:00Z","ended_at":"2026-06-08T09:00:00Z","environment":"outdoor"}
    ]}`
	rec := doReq(t, f.r, http.MethodPost, "/workouts/bulk", body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var out struct {
		Results []map[string]any `json:"results"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Results, 2)
	assert.Equal(t, "environment_invalid", out.Results[0]["error"])
	assert.Nil(t, out.Results[1]["error"])
	assert.NotNil(t, out.Results[1]["id"])
}

// A Garmin re-sync full-replaces by external_id, so the derived environment
// wins over a manual override — the accepted training_focus caveat, pinned so
// the trade-off is visible if it ever bites.
func TestBulk_ResyncReplacesManualEnvironmentOverride(t *testing.T) {
	f := setup(t)
	item := `{"source":"garmin","external_id":"env-resync-1","sport":"bike","status":"completed",
        "started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:00:00Z","environment":"outdoor"}`
	rec := doReq(t, f.r, http.MethodPost, "/workouts/bulk", fmt.Sprintf(`{"workouts":[%s]}`, item))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out struct {
		Results []map[string]any `json:"results"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Results, 1)
	id, ok := out.Results[0]["id"].(string)
	require.True(t, ok)

	// The athlete corrects it: that "road ride" was actually on rollers.
	patchRec := doReq(t, f.r, http.MethodPatch, "/workouts/"+id, `{"environment":"indoor"}`)
	require.Equal(t, http.StatusOK, patchRec.Code)
	assert.Contains(t, patchRec.Body.String(), `"environment":"indoor"`)

	// The next re-sync of that day re-derives and clobbers the override.
	rec = doReq(t, f.r, http.MethodPost, "/workouts/bulk", fmt.Sprintf(`{"workouts":[%s]}`, item))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	getRec := doReq(t, f.r, http.MethodGet, "/workouts/"+id, "")
	require.Equal(t, http.StatusOK, getRec.Code)
	assert.Contains(t, getRec.Body.String(), `"environment":"outdoor"`,
		"accepted caveat (training_focus precedent): a re-sync re-applies the derived value")
}

// Reconciliation must leave environment alone: neither Merge (fulfill) nor
// RestorePlanned (unfulfill) names the column, so a stated environment survives
// both. Pinned rather than assumed — a future column-list edit that swept it in
// would silently wipe the heat arc's input.
func TestReconciliation_PreservesEnvironment(t *testing.T) {
	f := setup(t)
	planned, _, _ := seedPlannedFromSlot(t, f, "run", at(18))

	// The athlete states up front that this planned run is a treadmill session.
	rec := doReq(t, f.r, http.MethodPatch, "/workouts/"+planned.ID.String(), `{"environment":"indoor"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Fulfilling it with the Garmin actual must not disturb the field.
	ingestGarmin(t, f, "garmin:env-r1", "run", at(7), http.StatusOK)
	merged, code := getWorkout(t, f, planned.ID)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, workouts.StatusCompleted, merged.Status)
	require.NotNil(t, merged.Environment, "fulfill must not clear environment")
	assert.Equal(t, workouts.EnvironmentIndoor, *merged.Environment)

	// Nor must unfulfilling, which clears the actuals but keeps plan identity.
	rec = doReq(t, f.r, http.MethodPost, "/workouts/"+planned.ID.String()+"/unfulfill", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	restored := decodeWorkout(t, rec.Body.Bytes())
	assert.Equal(t, workouts.StatusPlanned, restored.Status)
	assert.Nil(t, restored.KcalBurned, "actuals are cleared")
	require.NotNil(t, restored.Environment, "unfulfill must not clear environment")
	assert.Equal(t, workouts.EnvironmentIndoor, *restored.Environment)
}

func ptrEnv(e workouts.Environment) *workouts.Environment { return &e }
