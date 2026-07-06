package effortanalytics_test

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

	"github.com/vinzenzs/kazper/internal/effortanalytics"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/workouts"
)

func init() { gin.SetMode(gin.TestMode) }

type fixture struct {
	r    *gin.Engine
	repo *workouts.Repo
}

func setup(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	wrepo := workouts.NewRepo(pool)
	svc := effortanalytics.NewService(effortanalytics.NewRepo(pool), wrepo)
	r := gin.New()
	effortanalytics.NewHandlers(svc, "UTC", slog.Default()).Register(r.Group("/"))
	return &fixture{r: r, repo: wrepo}
}

// seedWorkout inserts a completed bike workout and returns its id.
func seedWorkout(t *testing.T, repo *workouts.Repo, start time.Time) uuid.UUID {
	t.Helper()
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

// constPower builds a JSON body with a power array of n samples all at v.
func constPower(n int, v float64) string {
	vals := make([]string, n)
	for i := range vals {
		vals[i] = fmt.Sprintf("%g", v)
	}
	return `{"power":[` + strings.Join(vals, ",") + `]}`
}

func TestIngest_ComputesAndStoresBestEfforts(t *testing.T) {
	f := setup(t)
	id := seedWorkout(t, f.repo, time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC))

	rec := post(t, f.r, "/workouts/"+id.String()+"/streams", constPower(3600, 250))
	require.Equal(t, http.StatusOK, rec.Code)
	var out effortanalytics.IngestResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, len(effortanalytics.Ladder), out.RecordsWritten)

	// The curve reflects them.
	crec := get(t, f.r, "/workouts/power-curve?from=2026-03-10&to=2026-03-10&sport=bike&tz=UTC")
	require.Equal(t, http.StatusOK, crec.Code)
	var curve effortanalytics.Curve
	require.NoError(t, json.Unmarshal(crec.Body.Bytes(), &curve))
	assert.Equal(t, effortanalytics.MetricPower, curve.Metric)
	require.Len(t, curve.Points, len(effortanalytics.Ladder))
	assert.InDelta(t, 250.0, curve.Points[0].Value, 0.001)
	assert.Equal(t, id.String(), curve.Points[0].WorkoutID)
}

func TestIngest_RepostReplacesNotDuplicates(t *testing.T) {
	f := setup(t)
	id := seedWorkout(t, f.repo, time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC))
	path := "/workouts/" + id.String() + "/streams"

	require.Equal(t, http.StatusOK, post(t, f.r, path, constPower(600, 200)).Code)
	require.Equal(t, http.StatusOK, post(t, f.r, path, constPower(600, 300)).Code)

	// After re-post the curve still has one point per duration, at the new value.
	crec := get(t, f.r, "/workouts/power-curve?from=2026-03-10&to=2026-03-10&sport=bike&tz=UTC")
	var curve effortanalytics.Curve
	require.NoError(t, json.Unmarshal(crec.Body.Bytes(), &curve))
	seen := map[int]int{}
	for _, p := range curve.Points {
		seen[p.DurationS]++
		assert.InDelta(t, 300.0, p.Value, 0.001) // replaced, not the old 200
	}
	for dur, n := range seen {
		assert.Equalf(t, 1, n, "duration %d duplicated", dur)
	}
}

func TestIngest_UnknownWorkoutIs404(t *testing.T) {
	f := setup(t)
	rec := post(t, f.r, "/workouts/"+uuid.New().String()+"/streams", constPower(60, 100))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestIngest_EmptyBodyWritesNothing(t *testing.T) {
	f := setup(t)
	id := seedWorkout(t, f.repo, time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC))
	rec := post(t, f.r, "/workouts/"+id.String()+"/streams", `{}`)
	require.Equal(t, http.StatusOK, rec.Code)
	var out effortanalytics.IngestResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, 0, out.RecordsWritten)
}

func TestCurve_EmptyWindowReturnsEmptyPoints(t *testing.T) {
	f := setup(t)
	rec := get(t, f.r, "/workouts/power-curve?from=2026-03-10&to=2026-03-12&sport=bike&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	var curve effortanalytics.Curve
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &curve))
	assert.Empty(t, curve.Points)
}

func TestCurve_ErrorContract(t *testing.T) {
	f := setup(t)
	cases := []struct{ name, path, wantErr string }{
		{"missing range", "/workouts/power-curve?sport=bike", "range_required"},
		{"bad date", "/workouts/power-curve?from=03/10/2026&to=2026-03-12", "date_invalid"},
		{"from after to", "/workouts/power-curve?from=2026-03-12&to=2026-03-10", "range_invalid"},
		{"range too large", "/workouts/power-curve?from=2025-01-01&to=2026-12-31", "range_too_large"},
		{"bad tz", "/workouts/power-curve?from=2026-03-10&to=2026-03-12&tz=NowhereLand", "tz_invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := get(t, f.r, tc.path)
			require.Equal(t, http.StatusBadRequest, rec.Code)
			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tc.wantErr, body["error"])
		})
	}
}
