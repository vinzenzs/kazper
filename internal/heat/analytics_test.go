package heat_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/heat"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// analyticsSession inserts a completed session with the metrics the evidence
// read aggregates.
type sessOpts struct {
	daysAgo    int
	mins       float64
	tempC      *float64
	humidity   *float64
	ef         *float64
	decoupling *float64
	powerW     *int
	env        *workouts.Environment
}

func insertSession(t *testing.T, f *fixture, o sessOpts) {
	t.Helper()
	start := time.Date(2026, 7, 20, 8, 0, 0, 0, time.UTC).AddDate(0, 0, -o.daysAgo)
	w := &workouts.Workout{
		Source:       workouts.SourceManual,
		Sport:        workouts.SportBike,
		Status:       workouts.StatusCompleted,
		StartedAt:    start,
		EndedAt:      start.Add(time.Duration(o.mins) * time.Minute),
		TemperatureC: o.tempC,
		HumidityPct:  o.humidity,
		AvgPowerW:    o.powerW,
		Environment:  o.env,
	}
	_, err := f.workoutsRepo.Upsert(context.Background(), w)
	require.NoError(t, err)

	// EF and decoupling are stream-derived: the upsert deliberately refuses
	// them (they're written only by the activity-streams ingest/recompute path),
	// so the fixture writes them through that same door.
	if o.ef != nil || o.decoupling != nil {
		require.NoError(t, f.workoutsRepo.SetExecutionMetrics(context.Background(), w.ID, nil, o.ef, o.decoupling))
	}
}

func getAnalytics(t *testing.T, f *fixture, query string) heat.Analytics {
	t.Helper()
	rec := doGet(t, f.r, "/workouts/heat-analytics?"+query)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out heat.Analytics
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out
}

const analyticsWindow = "from=2026-06-01&to=2026-07-20&tz=UTC"

func bucketByName(t *testing.T, a heat.Analytics, name string) heat.Bucket {
	t.Helper()
	for _, b := range a.Buckets {
		if b.Bucket == name {
			return b
		}
	}
	t.Fatalf("bucket %q not present in %v", name, a.Buckets)
	return heat.Bucket{}
}

// ============================================================================

// The whole point: as it gets hotter, output and EF fall.
func TestAnalytics_SeasonShowsTheDegradationGradient(t *testing.T) {
	f := setup(t, hourlyJSON(20, 50, 1, 50))

	// Cool sessions: strong power + EF.
	for i := 1; i <= 4; i++ {
		insertSession(t, f, sessOpts{daysAgo: i, mins: 120, tempC: fp(14), humidity: fp(50),
			ef: fp(1.9), decoupling: fp(3), powerW: ip(240), env: outdoor()})
	}
	// Mild.
	for i := 5; i <= 8; i++ {
		insertSession(t, f, sessOpts{daysAgo: i, mins: 120, tempC: fp(22), humidity: fp(50),
			ef: fp(1.8), decoupling: fp(5), powerW: ip(230), env: outdoor()})
	}
	// Warm.
	for i := 9; i <= 12; i++ {
		insertSession(t, f, sessOpts{daysAgo: i, mins: 120, tempC: fp(27), humidity: fp(50),
			ef: fp(1.6), decoupling: fp(8), powerW: ip(215), env: outdoor()})
	}
	// Hot: heat index well above 30.
	for i := 13; i <= 16; i++ {
		insertSession(t, f, sessOpts{daysAgo: i, mins: 120, tempC: fp(34), humidity: fp(60),
			ef: fp(1.4), decoupling: fp(12), powerW: ip(200), env: outdoor()})
	}

	out := getAnalytics(t, f, analyticsWindow)

	assert.Equal(t, 16, out.Sessions)
	require.Len(t, out.Buckets, 4, "all four bands are represented")
	assert.Equal(t, "<20", out.Buckets[0].Bucket, "buckets read in gradient order")
	assert.Equal(t, ">30", out.Buckets[3].Bucket)

	cool := bucketByName(t, out, "<20")
	hot := bucketByName(t, out, ">30")
	assert.Equal(t, 4, cool.Sessions)
	require.NotNil(t, cool.MeanEF)
	require.NotNil(t, hot.MeanEF)
	assert.Greater(t, *cool.MeanEF, *hot.MeanEF, "EF falls in the heat")
	require.NotNil(t, hot.MeanDecoupPct)
	assert.Greater(t, *hot.MeanDecoupPct, *cool.MeanDecoupPct, "decoupling rises")

	// Output is relative to the window's own baseline: cool above 100, hot below.
	require.NotNil(t, cool.MeanOutputRelPct)
	require.NotNil(t, hot.MeanOutputRelPct)
	assert.Greater(t, *cool.MeanOutputRelPct, 100.0)
	assert.Less(t, *hot.MeanOutputRelPct, 100.0)

	// 16 pairs clears the gate, and the association is negative for EF.
	require.NotNil(t, out.EFVsHeat.Rho)
	assert.Equal(t, 16, out.EFVsHeat.N)
	assert.Less(t, *out.EFVsHeat.Rho, -0.8, "hotter → lower EF")
	require.NotNil(t, out.DecouplingVsHeat.Rho)
	assert.Greater(t, *out.DecouplingVsHeat.Rho, 0.8, "hotter → more decoupling")

	// The duration confound is visible: every bucket reports its mean duration.
	require.NotNil(t, hot.MeanDurationM)
	assert.InDelta(t, 120, *hot.MeanDurationM, 0.1)
}

func TestAnalytics_ThinExposureGatesTheCorrelation(t *testing.T) {
	f := setup(t, hourlyJSON(20, 50, 1, 50))
	// Only 6 sessions carry EF — below the 10-pair floor.
	for i := 1; i <= 6; i++ {
		insertSession(t, f, sessOpts{daysAgo: i, mins: 120, tempC: fp(28), humidity: fp(55),
			ef: fp(1.6), powerW: ip(220), env: outdoor()})
	}

	out := getAnalytics(t, f, analyticsWindow)

	// Bucket means still report — the descriptive half survives the gate.
	require.NotEmpty(t, out.Buckets)
	assert.Equal(t, 6, out.Buckets[0].Sessions)
	assert.NotNil(t, out.Buckets[0].MeanEF)

	assert.Nil(t, out.EFVsHeat.Rho, "6 pairs can't produce a confident rho")
	assert.Equal(t, 6, out.EFVsHeat.N)
	assert.Equal(t, "insufficient_pairs", out.EFVsHeat.Reason)
}

// A trainer's temperature says nothing about racing in the heat.
func TestAnalytics_IndoorExcluded(t *testing.T) {
	f := setup(t, hourlyJSON(20, 50, 1, 50))
	for i := 1; i <= 5; i++ {
		insertSession(t, f, sessOpts{daysAgo: i, mins: 120, tempC: fp(30), humidity: fp(60),
			ef: fp(1.5), powerW: ip(210), env: indoor()})
	}
	insertSession(t, f, sessOpts{daysAgo: 6, mins: 120, tempC: fp(30), humidity: fp(60),
		ef: fp(1.5), powerW: ip(210), env: outdoor()})

	out := getAnalytics(t, f, analyticsWindow)

	assert.Equal(t, 1, out.Sessions, "only the outdoor session is evidence")
	assert.Zero(t, out.AssumedOutdoor)
}

// Null-environment sessions count, but the assumption is tallied so the caveat
// is visible rather than buried.
func TestAnalytics_NullEnvironmentCountsAndIsTallied(t *testing.T) {
	f := setup(t, hourlyJSON(20, 50, 1, 50))
	for i := 1; i <= 3; i++ {
		insertSession(t, f, sessOpts{daysAgo: i, mins: 120, tempC: fp(28), humidity: fp(55),
			ef: fp(1.6), powerW: ip(220), env: nil})
	}
	insertSession(t, f, sessOpts{daysAgo: 4, mins: 120, tempC: fp(28), humidity: fp(55),
		ef: fp(1.6), powerW: ip(220), env: outdoor()})

	out := getAnalytics(t, f, analyticsWindow)

	assert.Equal(t, 4, out.Sessions)
	assert.Equal(t, 3, out.AssumedOutdoor)
}

// No temperature, no heat index, no evidence.
func TestAnalytics_SessionsWithoutTemperatureAreSkipped(t *testing.T) {
	f := setup(t, hourlyJSON(20, 50, 1, 50))
	insertSession(t, f, sessOpts{daysAgo: 1, mins: 120, tempC: nil, ef: fp(1.8), powerW: ip(230), env: outdoor()})
	insertSession(t, f, sessOpts{daysAgo: 2, mins: 120, tempC: fp(28), humidity: fp(55), ef: fp(1.6), powerW: ip(220), env: outdoor()})

	out := getAnalytics(t, f, analyticsWindow)
	assert.Equal(t, 1, out.Sessions)
}

func TestAnalytics_EmptyWindow(t *testing.T) {
	f := setup(t, hourlyJSON(20, 50, 1, 50))

	out := getAnalytics(t, f, analyticsWindow)

	assert.Zero(t, out.Sessions)
	assert.NotNil(t, out.Buckets, "serializes as [] not null")
	assert.Empty(t, out.Buckets)
	assert.Equal(t, "insufficient_pairs", out.EFVsHeat.Reason)
	assert.Equal(t, "2026-06-01", out.From)
	assert.Equal(t, "2026-07-20", out.To)
}

// A bucket with sessions but no EF must report the count without faking a mean.
func TestAnalytics_MissingMetricsStayNull(t *testing.T) {
	f := setup(t, hourlyJSON(20, 50, 1, 50))
	for i := 1; i <= 3; i++ {
		insertSession(t, f, sessOpts{daysAgo: i, mins: 120, tempC: fp(28), humidity: fp(55), env: outdoor()})
	}

	out := getAnalytics(t, f, analyticsWindow)

	require.Len(t, out.Buckets, 1)
	assert.Equal(t, 3, out.Buckets[0].Sessions)
	assert.Nil(t, out.Buckets[0].MeanEF, "no EF is null, never a zero that reads as terrible")
	assert.Nil(t, out.Buckets[0].MeanDecoupPct)
	assert.Nil(t, out.Buckets[0].MeanOutputRelPct)
}

func TestAnalytics_RangeErrors(t *testing.T) {
	f := setup(t, hourlyJSON(20, 50, 1, 50))
	cases := []struct{ name, query, want string }{
		{"missing both", "", "range_required"},
		{"missing to", "from=2026-06-01", "range_required"},
		{"unparseable", "from=nope&to=2026-07-20", "date_invalid"},
		{"inverted", "from=2026-07-20&to=2026-06-01", "range_invalid"},
		{"too large", "from=2025-01-01&to=2026-07-20", "range_too_large"},
		{"bad tz", "from=2026-06-01&to=2026-07-20&tz=Mars/Olympus", "tz_invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doGet(t, f.r, "/workouts/heat-analytics?"+tc.query)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), tc.want)
		})
	}
}

func TestAnalytics_ReadOnly(t *testing.T) {
	f := setup(t, hourlyJSON(20, 50, 1, 50))
	insertSession(t, f, sessOpts{daysAgo: 1, mins: 120, tempC: fp(30), humidity: fp(60),
		ef: fp(1.6), powerW: ip(220), env: outdoor()})

	getAnalytics(t, f, analyticsWindow)

	status := string(workouts.StatusCompleted)
	after, err := f.workoutsRepo.List(context.Background(),
		time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC), nil, &status)
	require.NoError(t, err)
	require.Len(t, after, 1)
	require.NotNil(t, after[0].EfficiencyFactor)
	assert.InDelta(t, 1.6, *after[0].EfficiencyFactor, 0.001)
	assert.Zero(t, *f.calls, "analytics reads history — it fetches no forecast")
}

// The bucket boundaries themselves.
func TestAnalytics_BucketBoundaries(t *testing.T) {
	f := setup(t, hourlyJSON(20, 50, 1, 50))
	// Heat index ≈ temperature at low humidity/cool temps, so these land where
	// their temperatures suggest. 10% humidity keeps the index near dry-bulb.
	insertSession(t, f, sessOpts{daysAgo: 1, mins: 90, tempC: fp(15), humidity: fp(40), env: outdoor()})
	insertSession(t, f, sessOpts{daysAgo: 2, mins: 90, tempC: fp(22), humidity: fp(40), env: outdoor()})

	out := getAnalytics(t, f, analyticsWindow)

	require.Len(t, out.Buckets, 2)
	assert.Equal(t, "<20", out.Buckets[0].Bucket)
	assert.Equal(t, 1, out.Buckets[0].Sessions)
	assert.Equal(t, "20-25", out.Buckets[1].Bucket)
	assert.Equal(t, 1, out.Buckets[1].Sessions)
}
