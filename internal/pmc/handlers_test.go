package pmc_test

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

	"github.com/vinzenzs/kazper/internal/pmc"
	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/workouts"
)

func init() { gin.SetMode(gin.TestMode) }

// fakeResolver is a settable pmc.MacroResolver for the target-trajectory tests.
// Default: no active macrocycle (ErrMacrocycleNotFound).
type fakeResolver struct {
	macro *pmc.Macro
	err   error
}

func (f *fakeResolver) Resolve(context.Context, *string, time.Time) (*pmc.Macro, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.macro == nil {
		return nil, pmc.ErrMacrocycleNotFound
	}
	return f.macro, nil
}

type fixture struct {
	r        *gin.Engine
	repo     *workouts.Repo
	resolver *fakeResolver
}

func setup(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	repo := workouts.NewRepo(pool)
	svc := pmc.NewService(pmc.NewRepo(pool))
	res := &fakeResolver{}
	r := gin.New()
	pmc.NewHandlers(svc, res, "UTC", slog.Default()).Register(r.Group("/"))
	return &fixture{r: r, repo: repo, resolver: res}
}

func fp(v float64) *float64 { return &v }

// seedW inserts a workout via the repo. tss nil = a NULL-tss (missing) row.
func seedW(t *testing.T, repo *workouts.Repo, sport, status string, start time.Time, tss *float64) {
	t.Helper()
	var src *string
	if tss != nil {
		s := "manual"
		src = &s
	}
	_, err := repo.Upsert(context.Background(), &workouts.Workout{
		Source:    workouts.SourceManual,
		Sport:     workouts.Sport(sport),
		Status:    workouts.Status(status),
		StartedAt: start,
		EndedAt:   start.Add(time.Hour),
		TSS:       tss,
		TSSSource: src,
	})
	require.NoError(t, err)
}

func get(t *testing.T, f *fixture, query string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/performance/pmc?"+query, nil)
	rec := httptest.NewRecorder()
	f.r.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) pmc.Series {
	t.Helper()
	var s pmc.Series
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &s))
	return s
}

// utc builds a UTC instant.
func utc(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestPMC_PopulatedWindowShape(t *testing.T) {
	f := setup(t)
	seedW(t, f.repo, "bike", "completed", utc("2026-05-20T09:00:00Z"), fp(60)) // warm-up (before window)
	seedW(t, f.repo, "bike", "completed", utc("2026-06-05T09:00:00Z"), fp(120))
	seedW(t, f.repo, "run", "completed", utc("2026-06-10T09:00:00Z"), fp(80))

	rec := get(t, f, "from=2026-06-01&to=2026-06-30&tz=Europe/Berlin")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	s := decode(t, rec)

	assert.Equal(t, "Europe/Berlin", s.TZ)
	require.Len(t, s.Days, 30, "one entry per day in the inclusive window")
	require.NotNil(t, s.SeedDate)
	assert.Equal(t, "2026-05-19", *s.SeedDate, "seed = earliest (05-20) − 1")
	// Ordered ascending; the 06-05 day carries its tss and non-zero warmed ctl.
	assert.Equal(t, "2026-06-01", s.Days[0].Date)
	assert.Equal(t, "2026-06-30", s.Days[29].Date)
	d5 := s.Days[4] // 2026-06-05
	assert.Equal(t, "2026-06-05", d5.Date)
	assert.Equal(t, 120.0, d5.TSSTotal)
	assert.Greater(t, d5.CTL, 0.0)
	assert.NotNil(t, s.RampAlerts)
}

func TestPMC_PlannedExcludedAndMissingSurfaced(t *testing.T) {
	f := setup(t)
	day := utc("2026-06-10T09:00:00Z")
	seedW(t, f.repo, "bike", "completed", day, fp(80))
	seedW(t, f.repo, "run", "planned", day.Add(time.Hour), fp(999))    // planned: no load
	seedW(t, f.repo, "strength", "completed", day.Add(2*time.Hour), nil) // completed, NULL tss

	rec := get(t, f, "from=2026-06-10&to=2026-06-10&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	s := decode(t, rec)
	require.Len(t, s.Days, 1)
	assert.Equal(t, 80.0, s.Days[0].TSSTotal, "planned session contributes no load")
	assert.Equal(t, 1, s.Days[0].MissingTSSCount, "the NULL-tss completed session is counted")
	assert.Equal(t, 1, s.MissingTSSWorkouts)
}

func TestPMC_TimezoneBucketing(t *testing.T) {
	f := setup(t)
	// 22:30Z on Jun 7 = 00:30 Jun 8 in Europe/Berlin (UTC+2 in June).
	seedW(t, f.repo, "bike", "completed", utc("2026-06-07T22:30:00Z"), fp(90))

	rec := get(t, f, "from=2026-06-07&to=2026-06-08&tz=Europe/Berlin")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	s := decode(t, rec)
	assert.Equal(t, 0.0, s.Days[0].TSSTotal, "not attributed to Jun 7 local")
	assert.Equal(t, 90.0, s.Days[1].TSSTotal, "attributed to Jun 8 local (start-day)")
}

func TestPMC_EmptyHistory(t *testing.T) {
	f := setup(t)
	rec := get(t, f, "from=2026-06-01&to=2026-06-03&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	s := decode(t, rec)
	assert.Nil(t, s.SeedDate)
	require.Len(t, s.Days, 3)
	for _, d := range s.Days {
		assert.Equal(t, 0.0, d.CTL)
		assert.Equal(t, 0.0, d.ATL)
	}
	assert.NotNil(t, s.RampAlerts)
	assert.Empty(t, s.RampAlerts)
}

func TestPMC_ValidationCodes(t *testing.T) {
	f := setup(t)
	cases := []struct {
		query, code string
		max         bool
	}{
		{"to=2026-06-30", "range_required", false},
		{"from=2026-06-01", "range_required", false},
		{"from=nope&to=2026-06-30", "date_invalid", false},
		{"from=2026-06-30&to=2026-06-01", "range_invalid", false},
		{"from=2025-01-01&to=2026-06-30", "range_too_large", true},
		{"from=2026-06-01&to=2026-06-30&tz=Mars/Phobos", "tz_invalid", false},
	}
	for _, c := range cases {
		rec := get(t, f, c.query)
		require.Equal(t, http.StatusBadRequest, rec.Code, c.query)
		assert.Contains(t, rec.Body.String(), c.code, c.query)
		if c.max {
			assert.Contains(t, rec.Body.String(), `"max_days":400`)
		}
	}
}

func TestPMC_ReadOnlyIgnoresIdempotencyKey(t *testing.T) {
	f := setup(t)
	req := httptest.NewRequest(http.MethodGet, "/performance/pmc?from=2026-06-01&to=2026-06-02&tz=UTC", nil)
	req.Header.Set("Idempotency-Key", "some-key")
	rec := httptest.NewRecorder()
	f.r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "a GET ignores Idempotency-Key")
}
