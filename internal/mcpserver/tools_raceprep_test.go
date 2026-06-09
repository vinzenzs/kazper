package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type racePrepRecord struct {
	method   string
	path     string
	rawQuery string
	idemKey  string
	body     string
}

func newRacePrepRecorder(t *testing.T, status int, respBody string) (*apiClient, *[]racePrepRecord) {
	t.Helper()
	var (
		mu      sync.Mutex
		records []racePrepRecord
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		records = append(records, racePrepRecord{
			method:   r.Method,
			path:     r.URL.Path,
			rawQuery: r.URL.RawQuery,
			idemKey:  r.Header.Get("Idempotency-Key"),
			body:     string(raw),
		})
		mu.Unlock()
		w.WriteHeader(status)
		_, _ = io.WriteString(w, respBody)
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c := &apiClient{
		baseURL:   u,
		token:     "t",
		userAgent: "ua",
		http:      &http.Client{Timeout: 5 * time.Second},
	}
	return c, &records
}

func TestPlanCarbLoad_RequiredParamsOnly(t *testing.T) {
	c, recs := newRacePrepRecorder(t, 200, `{"race_date":"2026-07-24"}`)
	r := handlePlanCarbLoad(context.Background(), c, PlanCarbLoadArgs{
		RaceDate:     "2026-07-24",
		BodyWeightKg: 70,
	})
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/race-prep/carb-load", rec.path)
	q, err := url.ParseQuery(rec.rawQuery)
	require.NoError(t, err)
	assert.Equal(t, "2026-07-24", q.Get("race_date"))
	assert.Equal(t, "70", q.Get("body_weight_kg"))
	assert.Empty(t, q.Get("days_before"), "unset optionals must not be sent")
	assert.Empty(t, q.Get("carbs_per_kg_per_day"))
	assert.Empty(t, q.Get("race_day_carbs_per_kg"))
	assert.Empty(t, rec.idemKey, "read-only — no Idempotency-Key")
}

func TestPlanCarbLoad_OptionalParamsForwarded(t *testing.T) {
	c, recs := newRacePrepRecorder(t, 200, `{}`)
	db := 2
	cpd := 8.0
	rdc := 2.5
	r := handlePlanCarbLoad(context.Background(), c, PlanCarbLoadArgs{
		RaceDate:          "2026-07-24",
		BodyWeightKg:      70,
		DaysBefore:        &db,
		CarbsPerKgPerDay:  &cpd,
		RaceDayCarbsPerKg: &rdc,
	})
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	q, err := url.ParseQuery((*recs)[0].rawQuery)
	require.NoError(t, err)
	assert.Equal(t, "2", q.Get("days_before"))
	assert.Equal(t, "8", q.Get("carbs_per_kg_per_day"))
	assert.Equal(t, "2.5", q.Get("race_day_carbs_per_kg"))
}

func TestPlanCarbLoad_400FromBackendForwarded(t *testing.T) {
	c, _ := newRacePrepRecorder(t, 400, `{"error":"body_weight_kg_invalid","range":{"min":30,"max":200}}`)
	r := handlePlanCarbLoad(context.Background(), c, PlanCarbLoadArgs{
		RaceDate: "2026-07-24", BodyWeightKg: 25,
	})
	assert.True(t, r.IsError)
}

func TestPlanCarbLoad_ResponseBodyForwardedVerbatim(t *testing.T) {
	body := `{"race_date":"2026-07-24","body_weight_kg":70,"params":{"days_before":3,"carbs_per_kg_per_day":10,"race_day_carbs_per_kg":2},"schedule":[]}`
	c, _ := newRacePrepRecorder(t, 200, body)
	r := handlePlanCarbLoad(context.Background(), c, PlanCarbLoadArgs{
		RaceDate: "2026-07-24", BodyWeightKg: 70,
	})
	assert.False(t, r.IsError)
	require.Len(t, r.Content, 1)
	// The text content is the body verbatim — no envelope, no transformation.
}

// ----- apply branch -----

func TestPlanCarbLoad_ApplyFalseHitsGET(t *testing.T) {
	c, recs := newRacePrepRecorder(t, 200, `{}`)
	apply := false
	r := handlePlanCarbLoad(context.Background(), c, PlanCarbLoadArgs{
		RaceDate:     "2026-07-24",
		BodyWeightKg: 70,
		Apply:        &apply,
	})
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/race-prep/carb-load", rec.path)
	assert.Empty(t, rec.idemKey, "apply=false stays read-only — no Idempotency-Key")
	assert.Empty(t, rec.body, "GET sends no body")
}

func TestPlanCarbLoad_ApplyAbsentHitsGET(t *testing.T) {
	c, recs := newRacePrepRecorder(t, 200, `{}`)
	r := handlePlanCarbLoad(context.Background(), c, PlanCarbLoadArgs{
		RaceDate:     "2026-07-24",
		BodyWeightKg: 70,
		// Apply omitted entirely.
	})
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	assert.Equal(t, http.MethodGet, (*recs)[0].method)
	assert.Empty(t, (*recs)[0].idemKey)
}

func TestPlanCarbLoad_ApplyTrueHitsPOSTWithDerivedKey(t *testing.T) {
	c, recs := newRacePrepRecorder(t, 200,
		`{"race_date":"2026-07-24","body_weight_kg":70,"params":{"days_before":3,"carbs_per_kg_per_day":10,"race_day_carbs_per_kg":2},"schedule":[],"applied":[]}`)
	apply := true
	r := handlePlanCarbLoad(context.Background(), c, PlanCarbLoadArgs{
		RaceDate:     "2026-07-24",
		BodyWeightKg: 70,
		Apply:        &apply,
	})
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/race-prep/carb-load/apply", rec.path)
	assert.NotEmpty(t, rec.idemKey, "apply=true is a write — Idempotency-Key must be set")

	// Body must carry race_date + body_weight_kg; apply itself is NOT
	// forwarded (it's a wrapper-side switch).
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(rec.body), &body))
	assert.Equal(t, "2026-07-24", body["race_date"])
	assert.InDelta(t, 70.0, body["body_weight_kg"], 0.001)
	_, hasApply := body["apply"]
	assert.False(t, hasApply, "wrapper consumed `apply`; do not forward to backend")
	_, hasIdemKey := body["idempotency_key"]
	assert.False(t, hasIdemKey, "idempotency_key is a header, not a body field")
}

func TestPlanCarbLoad_ApplyTrueWithExplicitKeyForwardsVerbatim(t *testing.T) {
	c, recs := newRacePrepRecorder(t, 200, `{}`)
	apply := true
	r := handlePlanCarbLoad(context.Background(), c, PlanCarbLoadArgs{
		RaceDate:       "2026-07-24",
		BodyWeightKg:   70,
		Apply:          &apply,
		IdempotencyKey: "race-week-2026-07",
	})
	assert.False(t, r.IsError)
	require.Len(t, *recs, 1)
	assert.Equal(t, "race-week-2026-07", (*recs)[0].idemKey)
}

func TestPlanCarbLoad_ApplyTrueOptionalParamsForwardedInBody(t *testing.T) {
	c, recs := newRacePrepRecorder(t, 200, `{}`)
	apply := true
	db := 2
	cpd := 8.0
	rdc := 2.5
	_ = handlePlanCarbLoad(context.Background(), c, PlanCarbLoadArgs{
		RaceDate:          "2026-07-24",
		BodyWeightKg:      70,
		DaysBefore:        &db,
		CarbsPerKgPerDay:  &cpd,
		RaceDayCarbsPerKg: &rdc,
		Apply:             &apply,
	})
	require.Len(t, *recs, 1)
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte((*recs)[0].body), &body))
	assert.InDelta(t, 2.0, body["days_before"], 0.001)
	assert.InDelta(t, 8.0, body["carbs_per_kg_per_day"], 0.001)
	assert.InDelta(t, 2.5, body["race_day_carbs_per_kg"], 0.001)
}

func TestPlanCarbLoad_ApplyTrue400Forwarded(t *testing.T) {
	c, _ := newRacePrepRecorder(t, 400,
		`{"error":"body_weight_kg_invalid","range":{"min":30,"max":200}}`)
	apply := true
	r := handlePlanCarbLoad(context.Background(), c, PlanCarbLoadArgs{
		RaceDate: "2026-07-24", BodyWeightKg: 25, Apply: &apply,
	})
	assert.True(t, r.IsError)
}

func TestPlanCarbLoad_ApplyTrue500RollbackForwarded(t *testing.T) {
	c, _ := newRacePrepRecorder(t, 500, `{"error":"apply_failed"}`)
	apply := true
	r := handlePlanCarbLoad(context.Background(), c, PlanCarbLoadArgs{
		RaceDate: "2026-07-24", BodyWeightKg: 70, Apply: &apply,
	})
	assert.True(t, r.IsError, "rollback's 500 must surface as an error to the agent")
}

func TestPlanCarbLoad_DerivedKeyIgnoresApplyAndIdempotencyKey(t *testing.T) {
	// The derived key should be the same whether or not the caller
	// supplied an explicit `idempotency_key` (the canonical-JSON helper
	// strips it). Two calls with the same other args must produce the
	// same Idempotency-Key.
	c, recs := newRacePrepRecorder(t, 200, `{}`)
	apply := true
	_ = handlePlanCarbLoad(context.Background(), c, PlanCarbLoadArgs{
		RaceDate: "2026-07-24", BodyWeightKg: 70, Apply: &apply,
	})
	_ = handlePlanCarbLoad(context.Background(), c, PlanCarbLoadArgs{
		RaceDate: "2026-07-24", BodyWeightKg: 70, Apply: &apply,
	})
	require.Len(t, *recs, 2)
	assert.Equal(t, (*recs)[0].idemKey, (*recs)[1].idemKey,
		"identical args must produce identical derived keys")
}

// ----- recommend_workout_fuel -----

func TestRecommendWorkoutFuel_WorkoutModeForwardsOnlyWorkoutID(t *testing.T) {
	c, recs := newRacePrepRecorder(t, 200, `{}`)
	wid := "11111111-1111-1111-1111-111111111111"
	_ = handleRecommendWorkoutFuel(context.Background(), c, RecommendWorkoutFuelArgs{
		WorkoutID: &wid,
	})
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/race-prep/recommend-workout-fuel", rec.path)
	q, err := url.ParseQuery(rec.rawQuery)
	require.NoError(t, err)
	assert.Equal(t, wid, q.Get("workout_id"))
	assert.Empty(t, q.Get("sport"))
	assert.Empty(t, q.Get("duration_min"))
	assert.Empty(t, q.Get("intensity_zone"))
	assert.Empty(t, q.Get("body_weight_kg"))
	assert.Empty(t, rec.idemKey, "read-only; no idempotency-key")
}

func TestRecommendWorkoutFuel_ExplicitModeForwardsTriplet(t *testing.T) {
	c, recs := newRacePrepRecorder(t, 200, `{}`)
	sport := "bike"
	dur := 90
	zone := 3
	_ = handleRecommendWorkoutFuel(context.Background(), c, RecommendWorkoutFuelArgs{
		Sport: &sport, DurationMin: &dur, IntensityZone: &zone,
	})
	require.Len(t, *recs, 1)
	q, err := url.ParseQuery((*recs)[0].rawQuery)
	require.NoError(t, err)
	assert.Equal(t, "bike", q.Get("sport"))
	assert.Equal(t, "90", q.Get("duration_min"))
	assert.Equal(t, "3", q.Get("intensity_zone"))
	assert.Empty(t, q.Get("workout_id"))
}

func TestRecommendWorkoutFuel_BodyWeightKgForwardedWhenSupplied(t *testing.T) {
	c, recs := newRacePrepRecorder(t, 200, `{}`)
	sport := "bike"
	dur := 60
	zone := 2
	bw := 72.5
	_ = handleRecommendWorkoutFuel(context.Background(), c, RecommendWorkoutFuelArgs{
		Sport: &sport, DurationMin: &dur, IntensityZone: &zone, BodyWeightKg: &bw,
	})
	require.Len(t, *recs, 1)
	q, err := url.ParseQuery((*recs)[0].rawQuery)
	require.NoError(t, err)
	assert.Equal(t, "72.5", q.Get("body_weight_kg"))
}

func TestRecommendWorkoutFuel_400Forwarded(t *testing.T) {
	c, _ := newRacePrepRecorder(t, 400, `{"error":"input_conflict"}`)
	wid := "11111111-1111-1111-1111-111111111111"
	sport := "bike"
	r := handleRecommendWorkoutFuel(context.Background(), c, RecommendWorkoutFuelArgs{
		WorkoutID: &wid, Sport: &sport,
	})
	assert.True(t, r.IsError)
	// Body is forwarded verbatim — check it surfaces in the result content.
	require.Len(t, r.Content, 1)
}
