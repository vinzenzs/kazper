package fitnessmetrics_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/nutrition-api/internal/fitnessmetrics"
	"github.com/vinzenzs/nutrition-api/internal/store/storetest"
)

func init() { gin.SetMode(gin.TestMode) }

func setup(t *testing.T) *gin.Engine {
	t.Helper()
	pool := storetest.NewPool(t)
	svc := fitnessmetrics.NewService(fitnessmetrics.NewRepo(pool))
	r := gin.New()
	fitnessmetrics.NewHandlers(svc).Register(r.Group("/"))
	return r
}

func do(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Buffer
	if body != "" {
		rdr = bytes.NewBufferString(body)
	} else {
		rdr = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestUpsert_InsertThenUpdateInPlace(t *testing.T) {
	r := setup(t)
	body := `{"date":"2026-06-09","vo2max_running":54.0,"race_predictor_5k_seconds":1230,"acute_load":420.5,"chronic_load":380.0}`
	rec := do(t, r, http.MethodPost, "/fitness-metrics", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var s fitnessmetrics.Snapshot
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &s))
	require.NotNil(t, s.RacePredictor5kSeconds)
	assert.Equal(t, 1230, *s.RacePredictor5kSeconds)

	rec2 := do(t, r, http.MethodPost, "/fitness-metrics", `{"date":"2026-06-09","vo2max_cycling":58.0}`)
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
	var s2 fitnessmetrics.Snapshot
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &s2))
	require.NotNil(t, s2.VO2MaxCycling)
	assert.InDelta(t, 58.0, *s2.VO2MaxCycling, 0.05)
	assert.Nil(t, s2.VO2MaxRunning, "full-replace upsert nulls omitted fields")
	assert.Nil(t, s2.RacePredictor5kSeconds)
}

func TestUpsert_NoAcwrField(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/fitness-metrics", `{"date":"2026-06-09","acute_load":420,"chronic_load":380}`)
	require.Equal(t, http.StatusCreated, rec.Code)
	body := rec.Body.String()
	assert.NotContains(t, body, "acwr")
	assert.NotContains(t, body, "acute_chronic_ratio")
}

func TestUpsert_MissingDateAndNonPositiveMetrics(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/fitness-metrics", `{"vo2max_running":54}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"date_invalid"}`, rec.Body.String())

	cases := map[string]string{
		`{"date":"2026-06-09","vo2max_running":0}`:              "vo2max_running_invalid",
		`{"date":"2026-06-09","race_predictor_10k_seconds":-1}`: "race_predictor_10k_seconds_invalid",
		`{"date":"2026-06-09","acute_load":-5}`:                 "acute_load_invalid",
	}
	for body, want := range cases {
		rec := do(t, r, http.MethodPost, "/fitness-metrics", body)
		require.Equal(t, http.StatusBadRequest, rec.Code, body)
		var got map[string]string
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		assert.Equal(t, want, got["error"], body)
	}
}

func TestList_WindowAndCaps(t *testing.T) {
	r := setup(t)
	for _, d := range []string{"2026-06-01", "2026-06-15", "2026-07-05"} {
		require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/fitness-metrics", `{"date":"`+d+`","vo2max_running":54}`).Code)
	}
	rec := do(t, r, http.MethodGet, "/fitness-metrics?from=2026-06-01&to=2026-06-30", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out struct {
		FitnessMetrics []fitnessmetrics.Snapshot `json:"fitness_metrics"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.FitnessMetrics, 2)

	rec = do(t, r, http.MethodGet, "/fitness-metrics?to=2026-06-30", "")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"window_required"}`, rec.Body.String())

	rec = do(t, r, http.MethodGet, "/fitness-metrics?from=2026-01-01&to=2026-12-31", "")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var m map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
	assert.Equal(t, "range_too_large", m["error"])
}

func TestGetAndDeleteByDate(t *testing.T) {
	r := setup(t)
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/fitness-metrics", `{"date":"2026-06-09","vo2max_running":54}`).Code)

	require.Equal(t, http.StatusOK, do(t, r, http.MethodGet, "/fitness-metrics/2026-06-09", "").Code)

	rec := do(t, r, http.MethodGet, "/fitness-metrics/2026-06-10", "")
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":"fitness_metrics_not_found"}`, rec.Body.String())

	require.Equal(t, http.StatusNoContent, do(t, r, http.MethodDelete, "/fitness-metrics/2026-06-09", "").Code)
	require.Equal(t, http.StatusNotFound, do(t, r, http.MethodDelete, "/fitness-metrics/2026-06-09", "").Code)
}

func TestUnitIsolation_NoForeignFields(t *testing.T) {
	r := setup(t)
	rec := do(t, r, http.MethodPost, "/fitness-metrics", `{"date":"2026-06-09","vo2max_running":54}`)
	require.Equal(t, http.StatusCreated, rec.Code)
	body := rec.Body.String()
	for _, k := range []string{`"sleep_seconds"`, `"hrv_ms"`, `"kcal"`, `"weight_kg"`} {
		assert.NotContains(t, body, k)
	}
}
