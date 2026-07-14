package activitystreams_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vinzenzs/kazper/internal/activitystreams"
	"github.com/vinzenzs/kazper/internal/effortanalytics"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/workouts"
)

func init() { gin.SetMode(gin.TestMode) }

// noWeight satisfies effortanalytics.WeightProvider for the effort-analytics
// handlers registered alongside the stream routes; these tests never hit the
// power-profile endpoint, so it always reports no stored weight.
type noWeight struct{}

func (noWeight) LatestWeightKg(context.Context) (float64, bool, error) { return 0, false, nil }

type fixture struct {
	r    *gin.Engine
	repo *workouts.Repo
	pool *pgxpool.Pool
}

func setup(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	wrepo := workouts.NewRepo(pool)
	effortSvc := effortanalytics.NewService(effortanalytics.NewRepo(pool))
	svc := activitystreams.NewService(activitystreams.NewRepo(pool), wrepo, effortSvc)
	r := gin.New()
	activitystreams.NewHandlers(svc).Register(r.Group("/"))
	effortanalytics.NewHandlers(effortSvc, noWeight{}, "UTC", slog.Default()).Register(r.Group("/"))
	return &fixture{r: r, repo: wrepo, pool: pool}
}

func seedWorkout(t *testing.T, repo *workouts.Repo) uuid.UUID {
	t.Helper()
	start := time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC)
	w := &workouts.Workout{
		Source:    workouts.SourceManual,
		Sport:     workouts.SportBike,
		Status:    workouts.StatusCompleted,
		StartedAt: start,
		EndedAt:   start.Add(time.Hour),
	}
	_, err := repo.Upsert(context.Background(), w)
	require.NoError(t, err)
	return w.ID
}

func post(t *testing.T, r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func get(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// arr renders a JSON array of n samples all at v.
func arr(n int, v float64) string {
	vals := make([]string, n)
	for i := range vals {
		vals[i] = fmt.Sprintf("%g", v)
	}
	return "[" + strings.Join(vals, ",") + "]"
}

func TestIngest_PersistsStreamsBestEffortsAndMetrics(t *testing.T) {
	f := setup(t)
	id := seedWorkout(t, f.repo)

	// A full 1-hour series so every ladder rung (through 60m) yields a record.
	body := fmt.Sprintf(`{"power":%s,"heart_rate":%s}`, arr(3600, 200), arr(3600, 150))
	rec := post(t, f.r, "/workouts/"+id.String()+"/streams", body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out activitystreams.IngestResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, len(effortanalytics.Ladder), out.RecordsWritten)
	assert.Equal(t, 2, out.StreamsStored)

	// Streams persisted (retrievable).
	grec := get(t, f.r, "/workouts/"+id.String()+"/streams")
	require.Equal(t, http.StatusOK, grec.Code)
	var sr activitystreams.StreamsResponse
	require.NoError(t, json.Unmarshal(grec.Body.Bytes(), &sr))
	assert.Equal(t, 3600, sr.DurationS)
	assert.Len(t, sr.Streams[activitystreams.StreamPower], 3600)
	assert.Len(t, sr.Streams[activitystreams.StreamHeartRate], 3600)

	// Execution metrics written back onto the workout.
	w, err := f.repo.GetByID(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, w.VariabilityIndex)
	assert.InDelta(t, 1.00, *w.VariabilityIndex, 0.01)
	require.NotNil(t, w.EfficiencyFactor)
	assert.InDelta(t, 200.0/150.0, *w.EfficiencyFactor, 0.01)
	require.NotNil(t, w.DecouplingPct)
	assert.InDelta(t, 0.0, *w.DecouplingPct, 0.1)

	// Best-effort ladder is queryable via the power curve.
	crec := get(t, f.r, "/workouts/power-curve?from=2026-03-10&to=2026-03-10&sport=bike&tz=UTC")
	var curve effortanalytics.Curve
	require.NoError(t, json.Unmarshal(crec.Body.Bytes(), &curve))
	require.Len(t, curve.Points, len(effortanalytics.Ladder))
	assert.InDelta(t, 200.0, curve.Points[0].Value, 0.001)
}

func TestIngest_RepostReplacesStreams(t *testing.T) {
	f := setup(t)
	id := seedWorkout(t, f.repo)
	path := "/workouts/" + id.String() + "/streams"

	require.Equal(t, http.StatusOK, post(t, f.r, path, fmt.Sprintf(`{"power":%s}`, arr(1800, 200))).Code)
	// Re-post with a shorter, higher series replaces (not appends).
	require.Equal(t, http.StatusOK, post(t, f.r, path, fmt.Sprintf(`{"power":%s}`, arr(1500, 300))).Code)

	grec := get(t, f.r, path)
	var sr activitystreams.StreamsResponse
	require.NoError(t, json.Unmarshal(grec.Body.Bytes(), &sr))
	assert.Len(t, sr.Streams[activitystreams.StreamPower], 1500)
	assert.InDelta(t, 300.0, sr.Streams[activitystreams.StreamPower][0], 0.001)

	// Exactly one stored row for the workout (replaced, not duplicated).
	var n int
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM workout_streams WHERE workout_id = $1`, id).Scan(&n))
	assert.Equal(t, 1, n)
}

func TestIngest_LegacyTwoSeriesPost(t *testing.T) {
	f := setup(t)
	id := seedWorkout(t, f.repo)
	body := fmt.Sprintf(`{"power":%s,"speed":%s}`, arr(1800, 210), arr(1800, 8))
	rec := post(t, f.r, "/workouts/"+id.String()+"/streams", body)
	require.Equal(t, http.StatusOK, rec.Code)
	var out activitystreams.IngestResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 2, out.StreamsStored)

	// No HR → efficiency_factor and decoupling stay NULL, VI still set.
	w, err := f.repo.GetByID(context.Background(), id)
	require.NoError(t, err)
	assert.NotNil(t, w.VariabilityIndex)
	assert.Nil(t, w.EfficiencyFactor)
	assert.Nil(t, w.DecouplingPct)
}

func TestIngest_EmptyBodyIsNoOp(t *testing.T) {
	f := setup(t)
	id := seedWorkout(t, f.repo)
	rec := post(t, f.r, "/workouts/"+id.String()+"/streams", `{}`)
	require.Equal(t, http.StatusOK, rec.Code)
	var out activitystreams.IngestResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 0, out.RecordsWritten)
	assert.Equal(t, 0, out.StreamsStored)

	// Nothing stored → GET is streams_not_found.
	assert.Equal(t, http.StatusNotFound, get(t, f.r, "/workouts/"+id.String()+"/streams").Code)
}

func TestIngest_UnknownWorkoutIs404(t *testing.T) {
	f := setup(t)
	rec := post(t, f.r, "/workouts/"+uuid.New().String()+"/streams", fmt.Sprintf(`{"power":%s}`, arr(60, 100)))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRetrieve_Downsample(t *testing.T) {
	f := setup(t)
	id := seedWorkout(t, f.repo)
	require.Equal(t, http.StatusOK, post(t, f.r, "/workouts/"+id.String()+"/streams", fmt.Sprintf(`{"power":%s}`, arr(1800, 200))).Code)

	grec := get(t, f.r, "/workouts/"+id.String()+"/streams?downsample=100")
	require.Equal(t, http.StatusOK, grec.Code)
	var sr activitystreams.StreamsResponse
	require.NoError(t, json.Unmarshal(grec.Body.Bytes(), &sr))
	require.NotNil(t, sr.Downsample)
	assert.Equal(t, 100, *sr.Downsample)
	assert.Len(t, sr.Streams[activitystreams.StreamPower], 100)
	assert.Equal(t, 1800, sr.DurationS) // full-resolution length preserved
}

func TestRetrieve_DownsampleBounds(t *testing.T) {
	f := setup(t)
	id := seedWorkout(t, f.repo)
	require.Equal(t, http.StatusOK, post(t, f.r, "/workouts/"+id.String()+"/streams", fmt.Sprintf(`{"power":%s}`, arr(1800, 200))).Code)

	for _, ds := range []string{"5", "9", "5001", "abc"} {
		rec := get(t, f.r, "/workouts/"+id.String()+"/streams?downsample="+ds)
		require.Equal(t, http.StatusBadRequest, rec.Code, "downsample=%s", ds)
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "downsample_invalid", body["error"])
	}
}

func TestRetrieve_StreamsNotFound(t *testing.T) {
	f := setup(t)
	id := seedWorkout(t, f.repo)
	rec := get(t, f.r, "/workouts/"+id.String()+"/streams")
	require.Equal(t, http.StatusNotFound, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "streams_not_found", body["error"])
}

func TestRecompute_HappyPath(t *testing.T) {
	f := setup(t)
	id := seedWorkout(t, f.repo)
	require.Equal(t, http.StatusOK, post(t, f.r, "/workouts/"+id.String()+"/streams",
		fmt.Sprintf(`{"power":%s,"heart_rate":%s}`, arr(3600, 200), arr(3600, 150))).Code)

	rec := post(t, f.r, "/workouts/"+id.String()+"/streams/recompute", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out activitystreams.RecomputeResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, len(effortanalytics.Ladder), out.RecordsWritten)
	assert.Equal(t, 2, out.StreamsUsed)
}

func TestRecompute_404s(t *testing.T) {
	f := setup(t)
	// Unknown workout.
	assert.Equal(t, http.StatusNotFound,
		post(t, f.r, "/workouts/"+uuid.New().String()+"/streams/recompute", "").Code)
	// Known workout, no stored streams → streams_not_found.
	id := seedWorkout(t, f.repo)
	rec := post(t, f.r, "/workouts/"+id.String()+"/streams/recompute", "")
	require.Equal(t, http.StatusNotFound, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "streams_not_found", body["error"])
}

func TestWorkoutDeleteCascadesStreams(t *testing.T) {
	f := setup(t)
	id := seedWorkout(t, f.repo)
	require.Equal(t, http.StatusOK, post(t, f.r, "/workouts/"+id.String()+"/streams", fmt.Sprintf(`{"power":%s}`, arr(1800, 200))).Code)

	require.NoError(t, f.repo.Delete(context.Background(), id))

	var n int
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM workout_streams WHERE workout_id = $1`, id).Scan(&n))
	assert.Equal(t, 0, n)
}
