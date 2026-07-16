package expenditure_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/bodyweight"
	"github.com/vinzenzs/kazper/internal/expenditure"
	"github.com/vinzenzs/kazper/internal/meals"
	"github.com/vinzenzs/kazper/internal/products"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

type fixture struct {
	r            *gin.Engine
	mealsRepo    *meals.Repo
	productsRepo *products.Repo
	weightRepo   *bodyweight.Repo
}

func setup(t *testing.T) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	mRepo := meals.NewRepo(pool)
	pRepo := products.NewRepo(pool)
	wRepo := bodyweight.NewRepo(pool)
	wSvc := bodyweight.NewService(wRepo)
	svc := expenditure.NewService(mRepo, wSvc, wRepo)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/")
	expenditure.NewHandlers(svc, "UTC", nil).Register(rg)
	return &fixture{r: r, mealsRepo: mRepo, productsRepo: pRepo, weightRepo: wRepo}
}

// makeProduct returns a product at exactly 100 kcal per 100 g, so a meal of
// N grams logs N kcal — keeps the fixtures readable.
func makeProduct(t *testing.T, repo *products.Repo) uuid.UUID {
	t.Helper()
	kcal := 100.0
	p := &products.Product{
		Name:       "kcal-per-gram",
		Source:     products.SourceManual,
		Nutriments: products.Nutriments{KcalPer100g: &kcal},
	}
	require.NoError(t, repo.Insert(context.Background(), p))
	return p.ID
}

func logMeal(t *testing.T, repo *meals.Repo, pid uuid.UUID, at time.Time, kcal float64) {
	t.Helper()
	_, err := repo.Insert(context.Background(), meals.InsertParams{
		ProductID: &pid,
		LoggedAt:  at,
		QuantityG: kcal, // 100 kcal/100 g → grams == kcal
	})
	require.NoError(t, err)
}

func logWeight(t *testing.T, repo *bodyweight.Repo, at time.Time, kg float64) {
	t.Helper()
	require.NoError(t, repo.Insert(context.Background(), &bodyweight.Entry{LoggedAt: at, WeightKg: kg}))
}

func day(n int) time.Time {
	return time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC).AddDate(0, 0, n-1)
}

func doGet(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func getEstimate(t *testing.T, f *fixture, query string) expenditure.Expenditure {
	t.Helper()
	rec := doGet(t, f.r, "/nutrition/expenditure?"+query)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out expenditure.Expenditure
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out
}

const fullWindow = "from=2026-03-01&to=2026-03-28&tz=UTC"

// ============================================================================

func TestExpenditure_WellLoggedWindowReturnsBalanceEstimate(t *testing.T) {
	f := setup(t)
	pid := makeProduct(t, f.productsRepo)

	// 28 days at 2,800 kcal, weighed daily, trending 72.0 → 71.5 kg.
	for i := 1; i <= 28; i++ {
		logMeal(t, f.mealsRepo, pid, day(i), 2800)
		kg := 72.0 - 0.5*float64(i-1)/27.0
		logWeight(t, f.weightRepo, day(i), kg)
	}

	out := getEstimate(t, f, fullWindow)

	require.NotNil(t, out.ExpenditureKcalPerDay, "well-logged window must estimate")
	assert.Nil(t, out.Reason)
	assert.Equal(t, 28, out.Intake.DaysLogged)
	assert.Equal(t, 0, out.Intake.DaysUnlogged)
	assert.InDelta(t, 2800, out.Intake.MeanKcalLoggedDays, 1)
	assert.Equal(t, 28, out.Window.Days)
	assert.Equal(t, "2026-03-01", out.Window.From)
	assert.Equal(t, "2026-03-28", out.Window.To)

	// The trend endpoints the balance used are echoed with their dates. The
	// 7-day trailing average lags the raw series, so the observed delta is
	// smaller than the raw 0.5 kg — the estimate must sit above intake either
	// way, since the athlete lost mass.
	require.NotNil(t, out.Trend)
	assert.Equal(t, "2026-03-01", out.Trend.StartDate)
	assert.Equal(t, "2026-03-28", out.Trend.EndDate)
	assert.Less(t, out.Trend.DeltaKg, 0.0, "trend fell")
	assert.Greater(t, *out.ExpenditureKcalPerDay, 2800.0)

	// The reported number is exactly the balance over the reported inputs.
	want := out.Intake.MeanKcalLoggedDays - out.Trend.DeltaKg*7700/28
	assert.InDelta(t, want, *out.ExpenditureKcalPerDay, 1.0)
}

func TestExpenditure_UnloggedDaysExcludedAndCounted(t *testing.T) {
	f := setup(t)
	pid := makeProduct(t, f.productsRepo)

	// 22 logged days at 3,000 kcal; days 23–28 hold no meals at all.
	for i := 1; i <= 28; i++ {
		if i <= 22 {
			logMeal(t, f.mealsRepo, pid, day(i), 3000)
		}
		logWeight(t, f.weightRepo, day(i), 70.0)
	}

	out := getEstimate(t, f, fullWindow)

	assert.Equal(t, 22, out.Intake.DaysLogged)
	assert.Equal(t, 6, out.Intake.DaysUnlogged)
	// Read as zero-kcal days the mean would fall to ~2,357.
	assert.InDelta(t, 3000, out.Intake.MeanKcalLoggedDays, 0.5)
	require.NotNil(t, out.ExpenditureKcalPerDay)
	assert.InDelta(t, 3000, *out.ExpenditureKcalPerDay, 0.5, "flat trend → expenditure is the intake mean")

	// The per-day series states which days were which.
	require.Len(t, out.Intake.Days, 28)
	assert.True(t, out.Intake.Days[0].Logged)
	assert.False(t, out.Intake.Days[27].Logged)
	assert.Equal(t, "2026-03-28", out.Intake.Days[27].Date)
}

func TestExpenditure_SparseLoggingDegradesHonestly(t *testing.T) {
	f := setup(t)
	pid := makeProduct(t, f.productsRepo)

	for i := 1; i <= 9; i++ {
		logMeal(t, f.mealsRepo, pid, day(i), 2800)
	}
	for i := 1; i <= 28; i++ {
		logWeight(t, f.weightRepo, day(i), 70.0)
	}

	out := getEstimate(t, f, fullWindow)

	assert.Nil(t, out.ExpenditureKcalPerDay)
	require.NotNil(t, out.Reason)
	assert.Equal(t, "insufficient_logged_days", *out.Reason)
	assert.Equal(t, 9, out.Intake.DaysLogged)
	assert.Equal(t, 19, out.Intake.DaysUnlogged)
}

func TestExpenditure_TooFewWeighInsDegradesHonestly(t *testing.T) {
	f := setup(t)
	pid := makeProduct(t, f.productsRepo)

	for i := 1; i <= 28; i++ {
		logMeal(t, f.mealsRepo, pid, day(i), 2800)
	}
	logWeight(t, f.weightRepo, day(1), 72.0)
	logWeight(t, f.weightRepo, day(14), 71.8)
	logWeight(t, f.weightRepo, day(28), 71.5)

	out := getEstimate(t, f, fullWindow)

	assert.Nil(t, out.ExpenditureKcalPerDay)
	require.NotNil(t, out.Reason)
	assert.Equal(t, "insufficient_weigh_ins", *out.Reason)
	assert.Equal(t, 3, out.Intake.WeighIns)
	// Intake counts and the trend it did resolve stay visible under the gate.
	assert.Equal(t, 28, out.Intake.DaysLogged)
	assert.NotNil(t, out.Trend)
}

func TestExpenditure_NoWeighInsAtAll(t *testing.T) {
	f := setup(t)
	pid := makeProduct(t, f.productsRepo)
	for i := 1; i <= 28; i++ {
		logMeal(t, f.mealsRepo, pid, day(i), 2800)
	}

	out := getEstimate(t, f, fullWindow)

	assert.Nil(t, out.ExpenditureKcalPerDay)
	require.NotNil(t, out.Reason)
	assert.Equal(t, "insufficient_weigh_ins", *out.Reason)
	assert.Nil(t, out.Trend, "no mass signal at all → no trend block")
}

func TestExpenditure_EmptyWindow(t *testing.T) {
	f := setup(t)

	out := getEstimate(t, f, fullWindow)

	assert.Nil(t, out.ExpenditureKcalPerDay)
	require.NotNil(t, out.Reason)
	assert.Equal(t, "insufficient_logged_days", *out.Reason)
	assert.Equal(t, 0, out.Intake.DaysLogged)
	assert.Equal(t, 28, out.Intake.DaysUnlogged)
}

func TestExpenditure_WindowBoundsAreInclusiveAndTZLocal(t *testing.T) {
	f := setup(t)
	pid := makeProduct(t, f.productsRepo)

	// A meal at 23:30 UTC on the last day belongs to that day in UTC, but to
	// the NEXT day in Berlin (+1) — and so falls outside a Berlin window.
	for i := 1; i <= 28; i++ {
		logMeal(t, f.mealsRepo, pid, day(i), 2500)
		logWeight(t, f.weightRepo, day(i), 70.0)
	}
	logMeal(t, f.mealsRepo, pid, time.Date(2026, 3, 28, 23, 30, 0, 0, time.UTC), 500)

	utc := getEstimate(t, f, fullWindow)
	// Day 28 carries 2,500 + 500 in UTC.
	assert.InDelta(t, 3000, utc.Intake.Days[27].Kcal, 0.5)

	berlin := getEstimate(t, f, "from=2026-03-01&to=2026-03-28&tz=Europe/Berlin")
	assert.InDelta(t, 2500, berlin.Intake.Days[27].Kcal, 0.5, "23:30 UTC is next-day in Berlin")
}

func TestExpenditure_ReadOnly(t *testing.T) {
	f := setup(t)
	pid := makeProduct(t, f.productsRepo)
	for i := 1; i <= 28; i++ {
		logMeal(t, f.mealsRepo, pid, day(i), 2800)
		logWeight(t, f.weightRepo, day(i), 70.0)
	}

	before, err := f.mealsRepo.List(context.Background(), meals.ListParams{
		From: day(1).AddDate(0, 0, -1), To: day(28).AddDate(0, 0, 1),
	})
	require.NoError(t, err)
	weightsBefore, err := f.weightRepo.ListInRange(context.Background(),
		day(1).AddDate(0, 0, -1), day(28).AddDate(0, 0, 1))
	require.NoError(t, err)

	getEstimate(t, f, fullWindow)

	after, err := f.mealsRepo.List(context.Background(), meals.ListParams{
		From: day(1).AddDate(0, 0, -1), To: day(28).AddDate(0, 0, 1),
	})
	require.NoError(t, err)
	weightsAfter, err := f.weightRepo.ListInRange(context.Background(),
		day(1).AddDate(0, 0, -1), day(28).AddDate(0, 0, 1))
	require.NoError(t, err)

	assert.Len(t, after, len(before))
	assert.Len(t, weightsAfter, len(weightsBefore))
}

// The estimate is advisory: no goal target, no goal comparison, no
// athlete-config field may appear in the payload (design D4).
func TestExpenditure_UncoupledFromGoals(t *testing.T) {
	f := setup(t)
	pid := makeProduct(t, f.productsRepo)
	for i := 1; i <= 28; i++ {
		logMeal(t, f.mealsRepo, pid, day(i), 2800)
		logWeight(t, f.weightRepo, day(i), 70.0)
	}

	rec := doGet(t, f.r, "/nutrition/expenditure?"+fullWindow)
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	assert.NotContains(t, body, "goal")
	assert.NotContains(t, body, "target")
	assert.NotContains(t, body, "adherence")
	assert.NotContains(t, body, "ftp")
}

func TestExpenditure_RangeErrors(t *testing.T) {
	f := setup(t)

	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"missing both", "", "range_required"},
		{"missing to", "from=2026-03-01", "range_required"},
		{"missing from", "to=2026-03-28", "range_required"},
		{"unparseable from", "from=nope&to=2026-03-28", "date_invalid"},
		{"unparseable to", "from=2026-03-01&to=nope", "date_invalid"},
		{"rfc3339 not accepted", "from=2026-03-01T00:00:00Z&to=2026-03-28", "date_invalid"},
		{"inverted", "from=2026-03-28&to=2026-03-01", "range_invalid"},
		{"too large", "from=2026-01-01&to=2026-06-30", "range_too_large"},
		{"bad tz", "from=2026-03-01&to=2026-03-28&tz=Mars/Olympus", "tz_invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doGet(t, f.r, "/nutrition/expenditure?"+tc.query)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tc.want, body["error"])
		})
	}
}

func TestExpenditure_RangeCapBoundary(t *testing.T) {
	f := setup(t)

	// 92 days inclusive passes; 93 is over the nutrition-tier cap.
	rec := doGet(t, f.r, "/nutrition/expenditure?from=2026-01-01&to=2026-04-02")
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = doGet(t, f.r, "/nutrition/expenditure?from=2026-01-01&to=2026-04-03")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "range_too_large", body["error"])
	assert.Equal(t, float64(92), body["max_days"])
}

func TestExpenditure_SingleDayWindow(t *testing.T) {
	f := setup(t)
	pid := makeProduct(t, f.productsRepo)
	logMeal(t, f.mealsRepo, pid, day(1), 2800)

	out := getEstimate(t, f, "from=2026-03-01&to=2026-03-01&tz=UTC")

	assert.Equal(t, 1, out.Window.Days)
	assert.Nil(t, out.ExpenditureKcalPerDay)
	require.NotNil(t, out.Reason)
	assert.Equal(t, "insufficient_logged_days", *out.Reason)
	assert.Len(t, out.Intake.Days, 1)
}

// A window that opens on a weighing gap reports the dates it actually used,
// rather than pretending the trend was measured at the window bound.
func TestExpenditure_TrendEndpointDatesReflectActualCoverage(t *testing.T) {
	f := setup(t)
	pid := makeProduct(t, f.productsRepo)
	for i := 1; i <= 28; i++ {
		logMeal(t, f.mealsRepo, pid, day(i), 2800)
	}
	// No weigh-in until day 10; the 7-day trailing window reaches back at most
	// to day 10 itself, so no earlier date carries a trend value.
	for i := 10; i <= 24; i++ {
		logWeight(t, f.weightRepo, day(i), 71.0)
	}

	out := getEstimate(t, f, fullWindow)

	require.NotNil(t, out.Trend)
	assert.Equal(t, "2026-03-10", out.Trend.StartDate)
	// The 7-day trailing average keeps reporting past the last weigh-in (day
	// 24), so the window's own end is still covered.
	assert.Equal(t, "2026-03-28", out.Trend.EndDate)
	require.NotNil(t, out.ExpenditureKcalPerDay)
}
