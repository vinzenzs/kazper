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
// training_focus — 7-zone Trainingsbereiche classification
// ============================================================================

func TestPost_WithTrainingFocus_Returns201AndEchoesField(t *testing.T) {
	f := setup(t)
	body := `{
        "source":"manual","sport":"bike",
        "started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z",
        "training_focus":"basic_endurance_1"
    }`
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var w workouts.Workout
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &w))
	require.NotNil(t, w.TrainingFocus)
	assert.Equal(t, workouts.TrainingFocusBasicEndurance1, *w.TrainingFocus)
}

func TestPost_WithoutTrainingFocus_OmitsFromResponse(t *testing.T) {
	f := setup(t)
	body := `{"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z"}`
	rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.NotContains(t, rec.Body.String(), `"training_focus"`)
}

func TestPost_UnknownTrainingFocus_Returns400(t *testing.T) {
	f := setup(t)
	for _, bad := range []string{"ga1", "zone2", "sweet_spot", "tempo", ""} {
		body := fmt.Sprintf(`{"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z","training_focus":%q}`, bad)
		rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
		require.Equal(t, http.StatusBadRequest, rec.Code, "training_focus=%q", bad)
		assert.JSONEq(t, `{"error":"training_focus_invalid"}`, rec.Body.String())
	}
}

func TestPost_AllSevenTrainingFocusValuesAccepted(t *testing.T) {
	f := setup(t)
	values := []workouts.TrainingFocus{
		workouts.TrainingFocusRecovery,
		workouts.TrainingFocusBasicEndurance1,
		workouts.TrainingFocusBasicEndurance2,
		workouts.TrainingFocusDevelopment,
		workouts.TrainingFocusCompetitionSpecific,
		workouts.TrainingFocusPeak,
		workouts.TrainingFocusStrengthEndurance,
	}
	for _, v := range values {
		body := fmt.Sprintf(`{"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z","training_focus":%q}`, v)
		rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
		require.Equal(t, http.StatusCreated, rec.Code, "training_focus=%s: %s", v, rec.Body.String())
		var w workouts.Workout
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &w))
		require.NotNil(t, w.TrainingFocus)
		assert.Equal(t, v, *w.TrainingFocus)
	}
}

func TestPost_TrainingFocusIndependentOfSport(t *testing.T) {
	f := setup(t)
	// strength_endurance on a strength session, competition_specific on a run —
	// validated only against the enum, with no sport-coupling.
	cases := []struct{ sport, focus string }{
		{"strength", "strength_endurance"},
		{"run", "competition_specific"},
	}
	for _, c := range cases {
		body := fmt.Sprintf(`{"source":"manual","sport":%q,"started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z","training_focus":%q}`, c.sport, c.focus)
		rec := doReq(t, f.r, http.MethodPost, "/workouts", body)
		require.Equal(t, http.StatusCreated, rec.Code, "%s/%s: %s", c.sport, c.focus, rec.Body.String())
	}
}

func TestGet_ReturnsTrainingFocusWhenSet(t *testing.T) {
	f := setup(t)
	postRec := doReq(t, f.r, http.MethodPost, "/workouts",
		`{"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z","training_focus":"peak"}`)
	require.Equal(t, http.StatusCreated, postRec.Code)
	var w workouts.Workout
	require.NoError(t, json.Unmarshal(postRec.Body.Bytes(), &w))

	getRec := doReq(t, f.r, http.MethodGet, "/workouts/"+w.ID.String(), "")
	require.Equal(t, http.StatusOK, getRec.Code)
	assert.Contains(t, getRec.Body.String(), `"training_focus":"peak"`)
}

func TestPatch_SetTrainingFocus_OnExistingRow(t *testing.T) {
	f := setup(t)
	postRec := doReq(t, f.r, http.MethodPost, "/workouts",
		`{"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z"}`)
	require.Equal(t, http.StatusCreated, postRec.Code)
	var w workouts.Workout
	require.NoError(t, json.Unmarshal(postRec.Body.Bytes(), &w))

	patchRec := doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(), `{"training_focus":"competition_specific"}`)
	require.Equal(t, http.StatusOK, patchRec.Code, patchRec.Body.String())
	var got workouts.Workout
	require.NoError(t, json.Unmarshal(patchRec.Body.Bytes(), &got))
	require.NotNil(t, got.TrainingFocus)
	assert.Equal(t, workouts.TrainingFocusCompetitionSpecific, *got.TrainingFocus)
}

func TestPatch_AbsentTrainingFocus_LeavesUnchanged(t *testing.T) {
	f := setup(t)
	postRec := doReq(t, f.r, http.MethodPost, "/workouts",
		`{"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z","training_focus":"development"}`)
	require.Equal(t, http.StatusCreated, postRec.Code)
	var w workouts.Workout
	require.NoError(t, json.Unmarshal(postRec.Body.Bytes(), &w))

	// PATCH a different field; training_focus must survive untouched.
	patchRec := doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(), `{"notes":"updated"}`)
	require.Equal(t, http.StatusOK, patchRec.Code, patchRec.Body.String())
	assert.Contains(t, patchRec.Body.String(), `"training_focus":"development"`)
}

func TestPatch_ClearTrainingFocusViaJSONNull(t *testing.T) {
	f := setup(t)
	postRec := doReq(t, f.r, http.MethodPost, "/workouts",
		`{"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z","training_focus":"recovery"}`)
	require.Equal(t, http.StatusCreated, postRec.Code)
	var w workouts.Workout
	require.NoError(t, json.Unmarshal(postRec.Body.Bytes(), &w))

	patchRec := doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(), `{"training_focus":null}`)
	require.Equal(t, http.StatusOK, patchRec.Code, patchRec.Body.String())
	assert.NotContains(t, patchRec.Body.String(), `"training_focus"`)

	// Confirm it's persisted as cleared.
	getRec := doReq(t, f.r, http.MethodGet, "/workouts/"+w.ID.String(), "")
	require.Equal(t, http.StatusOK, getRec.Code)
	assert.NotContains(t, getRec.Body.String(), `"training_focus"`)
}

func TestPatch_UnknownTrainingFocus_RejectedWithoutTouchingRow(t *testing.T) {
	f := setup(t)
	postRec := doReq(t, f.r, http.MethodPost, "/workouts",
		`{"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:30:00Z","training_focus":"recovery","notes":"keep"}`)
	require.Equal(t, http.StatusCreated, postRec.Code)
	var w workouts.Workout
	require.NoError(t, json.Unmarshal(postRec.Body.Bytes(), &w))

	patchRec := doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(), `{"training_focus":"tempo"}`)
	require.Equal(t, http.StatusBadRequest, patchRec.Code)
	assert.JSONEq(t, `{"error":"training_focus_invalid"}`, patchRec.Body.String())

	// Existing value is untouched.
	getRec := doReq(t, f.r, http.MethodGet, "/workouts/"+w.ID.String(), "")
	require.Equal(t, http.StatusOK, getRec.Code)
	assert.Contains(t, getRec.Body.String(), `"training_focus":"recovery"`)
}

func TestBulk_TrainingFocusValidatedPerItem(t *testing.T) {
	f := setup(t)
	body := `{"workouts":[
        {"source":"manual","sport":"bike","started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:00:00Z","training_focus":"basic_endurance_2"},
        {"source":"manual","sport":"run","started_at":"2026-06-07T10:00:00Z","ended_at":"2026-06-07T10:30:00Z"},
        {"source":"manual","sport":"swim","started_at":"2026-06-07T12:00:00Z","ended_at":"2026-06-07T12:30:00Z","training_focus":"bogus"}
    ]}`
	rec := doReq(t, f.r, http.MethodPost, "/workouts/bulk", body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp struct {
		Results []struct {
			Index   int    `json:"index"`
			ID      string `json:"id"`
			Created bool   `json:"created"`
			Error   string `json:"error"`
		} `json:"results"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Results, 3)

	// Item 0: valid value stored.
	assert.True(t, resp.Results[0].Created)
	assert.Empty(t, resp.Results[0].Error)
	// Item 1: omitted → stored with NULL (no error).
	assert.True(t, resp.Results[1].Created)
	assert.Empty(t, resp.Results[1].Error)
	// Item 2: invalid value → per-item training_focus_invalid.
	assert.Equal(t, "training_focus_invalid", resp.Results[2].Error)

	// Spot-check item 0 round-trips its value.
	getRec := doReq(t, f.r, http.MethodGet, "/workouts/"+resp.Results[0].ID, "")
	require.Equal(t, http.StatusOK, getRec.Code)
	assert.Contains(t, getRec.Body.String(), `"training_focus":"basic_endurance_2"`)
}
