package workoutstats_test

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

	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/workouts"
	"github.com/vinzenzs/kazper/internal/workoutstats"
)

func init() { gin.SetMode(gin.TestMode) }

type fixture struct {
	r    *gin.Engine
	repo *workouts.Repo
}

func setup(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	svc := workoutstats.NewService(repo)
	r := gin.New()
	workoutstats.NewHandlers(svc, "UTC", slog.Default()).Register(r.Group("/"))
	return &fixture{r: r, repo: repo}
}

func ptr(f float64) *float64 { return &f }

// seed inserts a workout directly via the repo. distance/elevation/kcal are
// optional (nil = unmeasured) to exercise present-only summation.
func seed(t *testing.T, repo *workouts.Repo, sport, status string, start time.Time, dur time.Duration, dist, elev, kcal *float64) {
	t.Helper()
	_, err := repo.Upsert(context.Background(), &workouts.Workout{
		Source:         workouts.SourceManual,
		Sport:          workouts.Sport(sport),
		Status:         workouts.Status(status),
		StartedAt:      start,
		EndedAt:        start.Add(dur),
		DistanceM:      dist,
		ElevationGainM: elev,
		KcalBurned:     kcal,
	})
	require.NoError(t, err)
}

func get(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) workoutstats.Summary {
	t.Helper()
	var s workoutstats.Summary
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &s))
	return s
}

func TestSummary_RangeTotalsAndPerDaySeries(t *testing.T) {
	f := setup(t)
	dayA := time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC)
	dayB := time.Date(2026, 3, 12, 7, 0, 0, 0, time.UTC)
	// Two bike sessions on day A, one run on day B.
	seed(t, f.repo, "bike", "completed", dayA, 90*time.Minute, ptr(45000), ptr(600), ptr(1100))
	seed(t, f.repo, "bike", "completed", dayA.Add(3*time.Hour), 60*time.Minute, ptr(30000), ptr(300), ptr(700))
	seed(t, f.repo, "run", "completed", dayB, 45*time.Minute, ptr(10000), nil, ptr(500))

	rec := get(t, f.r, "/workouts/summary?from=2026-03-10&to=2026-03-12&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	s := decode(t, rec)

	// Window total.
	assert.Equal(t, 3, s.Total.Count)
	assert.InDelta(t, 195.0, s.Total.TotalDurationMin, 0.01) // 90+60+45
	assert.InDelta(t, 85000.0, s.Total.TotalDistanceM, 0.01) // 45000+30000+10000
	assert.Equal(t, 2, s.Total.BySport["bike"])
	assert.Equal(t, 1, s.Total.BySport["run"])

	// Per-day series covers every calendar day (10, 11, 12).
	require.Len(t, s.Days, 3)
	assert.Equal(t, "2026-03-10", s.Days[0].Date)
	assert.Equal(t, 2, s.Days[0].Count)
	assert.Equal(t, 0, s.Days[1].Count) // the 11th is a zero-filled gap day
	assert.Equal(t, 1, s.Days[2].Count)
}

func TestSummary_PresentOnlySums(t *testing.T) {
	f := setup(t)
	day := time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC)
	// One workout WITH elevation, one WITHOUT — the missing one must not zero
	// the day's elevation sum.
	seed(t, f.repo, "bike", "completed", day, 60*time.Minute, ptr(30000), ptr(400), ptr(700))
	seed(t, f.repo, "run", "completed", day.Add(2*time.Hour), 30*time.Minute, ptr(8000), nil, nil)

	rec := get(t, f.r, "/workouts/summary?from=2026-03-10&to=2026-03-10&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	s := decode(t, rec)

	assert.Equal(t, 2, s.Total.Count)
	assert.InDelta(t, 400.0, s.Total.TotalElevationGainM, 0.01) // only the bike's elevation
	assert.InDelta(t, 38000.0, s.Total.TotalDistanceM, 0.01)
	assert.InDelta(t, 700.0, s.Total.TotalKcal, 0.01) // only the bike's kcal
}

func TestSummary_PlannedExcluded(t *testing.T) {
	f := setup(t)
	day := time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC)
	seed(t, f.repo, "bike", "completed", day, 60*time.Minute, ptr(30000), ptr(400), ptr(700))
	seed(t, f.repo, "run", "planned", day.Add(2*time.Hour), 30*time.Minute, ptr(8000), ptr(50), ptr(400))

	rec := get(t, f.r, "/workouts/summary?from=2026-03-10&to=2026-03-10&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	s := decode(t, rec)

	assert.Equal(t, 1, s.Total.Count) // planned run not counted
	assert.Equal(t, 1, s.Total.BySport["bike"])
	assert.Equal(t, 0, s.Total.BySport["run"])
}

func TestSummary_YearToDateRangeAccepted(t *testing.T) {
	f := setup(t)
	rec := get(t, f.r, "/workouts/summary?from=2026-01-01&to=2026-12-31&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	s := decode(t, rec)
	assert.Len(t, s.Days, 365) // 2026 is not a leap year
}

func TestSummary_ErrorContract(t *testing.T) {
	f := setup(t)
	cases := []struct {
		name, path, wantErr string
	}{
		{"missing range", "/workouts/summary?tz=UTC", "range_required"},
		{"bad date", "/workouts/summary?from=03/10/2026&to=2026-03-12", "date_invalid"},
		{"from after to", "/workouts/summary?from=2026-03-12&to=2026-03-10", "range_invalid"},
		{"range too large", "/workouts/summary?from=2025-01-01&to=2026-12-31", "range_too_large"},
		{"bad tz", "/workouts/summary?from=2026-03-10&to=2026-03-12&tz=NowhereLand", "tz_invalid"},
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
