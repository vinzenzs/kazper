package effortanalytics_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
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
	svc  *effortanalytics.Service
}

func setup(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	wrepo := workouts.NewRepo(pool)
	svc := effortanalytics.NewService(effortanalytics.NewRepo(pool))
	r := gin.New()
	effortanalytics.NewHandlers(svc, "UTC", slog.Default()).Register(r.Group("/"))
	return &fixture{r: r, repo: wrepo, svc: svc}
}

// seedWorkout inserts a completed bike workout and returns it.
func seedWorkout(t *testing.T, repo *workouts.Repo, start time.Time) *workouts.Workout {
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
	return w
}

func get(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// constSlice builds a slice of n samples all at v.
func constSlice(n int, v float64) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = v
	}
	return s
}

// The ingest entrypoint moved to the activity-streams capability; here we
// exercise ComputeAndReplace directly and confirm the curve reflects the stored
// best efforts (one point per ladder duration, at the constant value).
func TestComputeAndReplace_CurveReflectsStoredEfforts(t *testing.T) {
	f := setup(t)
	w := seedWorkout(t, f.repo, time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC))

	n, err := f.svc.ComputeAndReplace(context.Background(), w, constSlice(3600, 250), nil)
	require.NoError(t, err)
	assert.Equal(t, len(effortanalytics.Ladder), n)

	crec := get(t, f.r, "/workouts/power-curve?from=2026-03-10&to=2026-03-10&sport=bike&tz=UTC")
	require.Equal(t, http.StatusOK, crec.Code)
	var curve effortanalytics.Curve
	require.NoError(t, json.Unmarshal(crec.Body.Bytes(), &curve))
	assert.Equal(t, effortanalytics.MetricPower, curve.Metric)
	require.Len(t, curve.Points, len(effortanalytics.Ladder))
	assert.InDelta(t, 250.0, curve.Points[0].Value, 0.001)
	assert.Equal(t, w.ID.String(), curve.Points[0].WorkoutID)
}

func TestComputeAndReplace_ReplacesNotDuplicates(t *testing.T) {
	f := setup(t)
	w := seedWorkout(t, f.repo, time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC))

	_, err := f.svc.ComputeAndReplace(context.Background(), w, constSlice(600, 200), nil)
	require.NoError(t, err)
	_, err = f.svc.ComputeAndReplace(context.Background(), w, constSlice(600, 300), nil)
	require.NoError(t, err)

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

func TestComputeAndReplace_EmptyInputWritesNothing(t *testing.T) {
	f := setup(t)
	w := seedWorkout(t, f.repo, time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC))
	n, err := f.svc.ComputeAndReplace(context.Background(), w, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
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
