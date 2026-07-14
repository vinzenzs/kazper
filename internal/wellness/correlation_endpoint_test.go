package wellness_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/store/storetest"
	"github.com/vinzenzs/kazper/internal/wellness"
)

// fakePMC returns a fixed daily series regardless of the requested window.
type fakePMC struct {
	days []wellness.PMCDayValue
}

func (f fakePMC) PMCValues(context.Context, time.Time, time.Time, *time.Location) ([]wellness.PMCDayValue, error) {
	return f.days, nil
}

// setupCorrelation builds a wellness service with a fake PMC provider and returns
// the engine + the fake so tests can seed both sides.
func setupCorrelation(t *testing.T, pmcDays []wellness.PMCDayValue) *gin.Engine {
	t.Helper()
	pool := storetest.NewPool(t)
	svc := wellness.NewService(wellness.NewRepo(pool))
	svc.SetPMCProvider(fakePMC{days: pmcDays})
	r := gin.New()
	wellness.NewHandlers(svc).Register(r.Group("/"))
	return r
}

func dateStr(base time.Time, i int) string {
	return base.AddDate(0, 0, i).Format("2006-01-02")
}

func TestCorrelation_NegativeAssociation(t *testing.T) {
	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	// 16 days: fatigue steps up (1→4) as TSB steps down (+10 → -20) → strong
	// negative rank correlation.
	var pmcDays []wellness.PMCDayValue
	for i := 0; i < 16; i++ {
		pmcDays = append(pmcDays, wellness.PMCDayValue{Date: dateStr(base, i), TSB: float64(10 - i*2)})
	}
	r := setupCorrelation(t, pmcDays)

	for i := 0; i < 16; i++ {
		fatigue := 1 + i/4 // 1,1,1,1,2,2,2,2,3,3,3,3,4,4,4,4
		body := fmt.Sprintf(`{"fatigue":%d}`, fatigue)
		require.Equal(t, http.StatusOK, doReq(r, http.MethodPut, "/wellness/"+dateStr(base, i), body, nil).Code)
	}

	rec := doReq(r, http.MethodGet, "/wellness/correlation?from=2026-06-01&to=2026-06-20", "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var res wellness.CorrelationResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))

	assert.Equal(t, "tsb", res.Metric)
	fat := res.Fields["fatigue"]
	assert.Equal(t, 16, fat.N)
	require.NotNil(t, fat.Rho)
	assert.Less(t, *fat.Rho, -0.8, "fatigue rises as TSB falls → strong negative rho")
	// Soreness had no entries → insufficient.
	sore := res.Fields["soreness"]
	assert.Equal(t, 0, sore.N)
	assert.Equal(t, "insufficient_pairs", sore.Reason)
	assert.Nil(t, sore.Rho)
}

func TestCorrelation_SparseFieldGated(t *testing.T) {
	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	var pmcDays []wellness.PMCDayValue
	for i := 0; i < 20; i++ {
		pmcDays = append(pmcDays, wellness.PMCDayValue{Date: dateStr(base, i), TSB: float64(i)})
	}
	r := setupCorrelation(t, pmcDays)

	// Only 5 mood entries — below the n=14 floor.
	for i := 0; i < 5; i++ {
		doReq(r, http.MethodPut, "/wellness/"+dateStr(base, i), `{"mood":3}`, nil)
	}
	rec := doReq(r, http.MethodGet, "/wellness/correlation?from=2026-06-01&to=2026-06-21", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var res wellness.CorrelationResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	mood := res.Fields["mood"]
	assert.Equal(t, 5, mood.N)
	assert.Equal(t, "insufficient_pairs", mood.Reason)
	assert.Nil(t, mood.Rho)
}

func TestCorrelation_MetricMatrix(t *testing.T) {
	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	days := []wellness.PMCDayValue{}
	for i := 0; i < 3; i++ {
		days = append(days, wellness.PMCDayValue{Date: dateStr(base, i), TSB: 1, CTL: 2, RampRate: 3})
	}
	r := setupCorrelation(t, days)

	for _, m := range []string{"tsb", "ctl", "ramp_rate", ""} {
		rec := doReq(r, http.MethodGet, "/wellness/correlation?from=2026-06-01&to=2026-06-10&metric="+m, "", nil)
		require.Equal(t, http.StatusOK, rec.Code, m)
		var res wellness.CorrelationResult
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
		want := m
		if m == "" {
			want = "tsb"
		}
		assert.Equal(t, want, res.Metric)
	}

	bad := doReq(r, http.MethodGet, "/wellness/correlation?from=2026-06-01&to=2026-06-10&metric=vo2", "", nil)
	require.Equal(t, http.StatusBadRequest, bad.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(bad.Body.Bytes(), &body))
	assert.Equal(t, "metric_invalid", body["error"])
}

func TestCorrelation_RangeContract(t *testing.T) {
	r := setupCorrelation(t, nil)
	cases := []struct{ path, code string }{
		{"/wellness/correlation", "range_required"},
		{"/wellness/correlation?from=2026-06-10&to=2026-06-01", "range_invalid"},
		{"/wellness/correlation?from=2026-01-01&to=2026-12-31", "range_too_large"},
	}
	for _, tc := range cases {
		rec := doReq(r, http.MethodGet, tc.path, "", nil)
		require.Equal(t, http.StatusBadRequest, rec.Code, tc.path)
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, tc.code, body["error"], tc.path)
	}
}
