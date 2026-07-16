package locations_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/locations"
	"github.com/vinzenzs/kazper/internal/store/storetest"
)

type fixture struct {
	r    *gin.Engine
	svc  *locations.Service
	repo *locations.Repo
}

// setup mounts the endpoints with home configured (Vienna).
func setup(t *testing.T) *fixture {
	return setupWithHome(t, locations.Home{Lat: 48.21, Lon: 16.37, Set: true})
}

// setupNoHome mounts them with HOME_LAT/HOME_LON unset.
func setupNoHome(t *testing.T) *fixture { return setupWithHome(t, locations.Home{}) }

func setupWithHome(t *testing.T, home locations.Home) *fixture {
	t.Helper()
	pool := storetest.NewPool(t)
	repo := locations.NewRepo(pool)
	svc := locations.NewService(repo, home)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/")
	locations.NewHandlers(svc, "UTC").Register(rg)
	return &fixture{r: r, svc: svc, repo: repo}
}

func doReq(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// logPeriod creates a period and returns it.
func logPeriod(t *testing.T, f *fixture, name, start, end string, lat, lon float64) locations.Period {
	t.Helper()
	body := fmt.Sprintf(`{"name":%q,"start_date":%q,"end_date":%q,"lat":%v,"lon":%v}`,
		name, start, end, lat, lon)
	rec := doReq(t, f.r, http.MethodPost, "/locations", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var out struct {
		Location locations.Period `json:"location"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out.Location
}

func resolve(t *testing.T, f *fixture, date string) (locations.Resolved, int) {
	t.Helper()
	rec := doReq(t, f.r, http.MethodGet, "/locations/resolve?date="+date, "")
	var out locations.Resolved
	if rec.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	}
	return out, rec.Code
}

// ============================================================================

func TestCreate_TrainingCampLoggedConversationally(t *testing.T) {
	f := setup(t)

	p := logPeriod(t, f, "Mallorca", "2026-07-20", "2026-07-28", 39.57, 2.65)

	assert.Equal(t, "Mallorca", p.Name)
	assert.Equal(t, "2026-07-20", p.StartDate)
	assert.Equal(t, "2026-07-28", p.EndDate)
	assert.InDelta(t, 39.57, p.Lat, 0.001)
	assert.InDelta(t, 2.65, p.Lon, 0.001)
	assert.NotEqual(t, "", p.ID.String())
	assert.Nil(t, p.Note)
}

func TestCreate_WithNote(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodPost, "/locations",
		`{"name":"Sierra Nevada","start_date":"2026-05-01","end_date":"2026-05-21","lat":37.09,"lon":-3.40,"note":"altitude camp, 2320 m"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var out struct {
		Location locations.Period `json:"location"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.NotNil(t, out.Location.Note)
	assert.Equal(t, "altitude camp, 2320 m", *out.Location.Note)
}

func TestCreate_SingleDayPeriod(t *testing.T) {
	f := setup(t)
	p := logPeriod(t, f, "Race day trip", "2026-08-02", "2026-08-02", 47.0, 15.4)
	assert.Equal(t, p.StartDate, p.EndDate)

	got, code := resolve(t, f, "2026-08-02")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, locations.SourceTravel, got.Source)
}

func TestCreate_ValidationMatrix(t *testing.T) {
	f := setup(t)
	cases := []struct {
		name string
		body string
		want string
	}{
		{"lat out of range high", `{"name":"X","start_date":"2026-07-20","end_date":"2026-07-28","lat":95,"lon":2.65}`, "lat_lon_invalid"},
		{"lat out of range low", `{"name":"X","start_date":"2026-07-20","end_date":"2026-07-28","lat":-95,"lon":2.65}`, "lat_lon_invalid"},
		{"lon out of range high", `{"name":"X","start_date":"2026-07-20","end_date":"2026-07-28","lat":39,"lon":181}`, "lat_lon_invalid"},
		{"lon out of range low", `{"name":"X","start_date":"2026-07-20","end_date":"2026-07-28","lat":39,"lon":-181}`, "lat_lon_invalid"},
		{"lat missing", `{"name":"X","start_date":"2026-07-20","end_date":"2026-07-28","lon":2.65}`, "lat_lon_invalid"},
		{"lon missing", `{"name":"X","start_date":"2026-07-20","end_date":"2026-07-28","lat":39}`, "lat_lon_invalid"},
		{"name missing", `{"start_date":"2026-07-20","end_date":"2026-07-28","lat":39,"lon":2.65}`, "name_required"},
		{"name blank", `{"name":"   ","start_date":"2026-07-20","end_date":"2026-07-28","lat":39,"lon":2.65}`, "name_required"},
		{"start missing", `{"name":"X","end_date":"2026-07-28","lat":39,"lon":2.65}`, "date_invalid"},
		{"end missing", `{"name":"X","start_date":"2026-07-20","lat":39,"lon":2.65}`, "date_invalid"},
		{"start unparseable", `{"name":"X","start_date":"July 20","end_date":"2026-07-28","lat":39,"lon":2.65}`, "date_invalid"},
		{"end before start", `{"name":"X","start_date":"2026-07-28","end_date":"2026-07-20","lat":39,"lon":2.65}`, "range_invalid"},
		{"malformed json", `{"name":`, "invalid_json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doReq(t, f.r, http.MethodPost, "/locations", tc.body)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tc.want, body["error"])
		})
	}
}

// 0,0 is a real coordinate (Null Island) — the validator must not confuse it
// with an absent field.
func TestCreate_ZeroCoordinatesAreValid(t *testing.T) {
	f := setup(t)
	p := logPeriod(t, f, "Null Island", "2026-07-20", "2026-07-21", 0, 0)
	assert.Zero(t, p.Lat)
	assert.Zero(t, p.Lon)

	got, code := resolve(t, f, "2026-07-20")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, locations.SourceTravel, got.Source)
	assert.Zero(t, got.Lat)
}

func TestCreate_NoteTooLong(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodPost, "/locations", fmt.Sprintf(
		`{"name":"X","start_date":"2026-07-20","end_date":"2026-07-28","lat":39,"lon":2.65,"note":%q}`,
		strings.Repeat("a", 2001)))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "note_too_long")
}

func TestResolve_TripCoversItsDatesElseHome(t *testing.T) {
	f := setup(t)
	logPeriod(t, f, "Mallorca", "2026-07-20", "2026-07-28", 39.57, 2.65)

	// Inside the trip.
	got, code := resolve(t, f, "2026-07-22")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "Mallorca", got.Name)
	assert.Equal(t, locations.SourceTravel, got.Source)
	assert.InDelta(t, 39.57, got.Lat, 0.001)
	assert.Equal(t, "2026-07-22", got.Date)

	// After it → home.
	got, code = resolve(t, f, "2026-07-30")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, locations.SourceHome, got.Source)
	assert.Equal(t, "home", got.Name)
	assert.InDelta(t, 48.21, got.Lat, 0.001)
}

// Inclusive on BOTH ends — a trip's first and last day are still the trip.
func TestResolve_RangeIsInclusive(t *testing.T) {
	f := setup(t)
	logPeriod(t, f, "Mallorca", "2026-07-20", "2026-07-28", 39.57, 2.65)

	for _, d := range []string{"2026-07-20", "2026-07-28"} {
		got, code := resolve(t, f, d)
		require.Equal(t, http.StatusOK, code, d)
		assert.Equal(t, locations.SourceTravel, got.Source, "%s must be inside the trip", d)
	}
	for _, d := range []string{"2026-07-19", "2026-07-29"} {
		got, code := resolve(t, f, d)
		require.Equal(t, http.StatusOK, code, d)
		assert.Equal(t, locations.SourceHome, got.Source, "%s must be outside the trip", d)
	}
}

// The latest-start rule: a weekend trip nested inside a camp wins for its days.
func TestResolve_NestedTripsResolveToLatestStart(t *testing.T) {
	f := setup(t)
	logPeriod(t, f, "Camp", "2026-07-20", "2026-07-28", 39.57, 2.65)
	logPeriod(t, f, "Weekend", "2026-07-24", "2026-07-26", 41.39, 2.16)

	got, _ := resolve(t, f, "2026-07-25")
	assert.Equal(t, "Weekend", got.Name, "the later-starting period wins")
	assert.InDelta(t, 41.39, got.Lat, 0.001)

	// Days of the camp outside the weekend still resolve to the camp.
	got, _ = resolve(t, f, "2026-07-22")
	assert.Equal(t, "Camp", got.Name)
	got, _ = resolve(t, f, "2026-07-27")
	assert.Equal(t, "Camp", got.Name)
}

// Insertion order must not decide the answer — start_date does.
func TestResolve_NestedTripWinsRegardlessOfInsertOrder(t *testing.T) {
	f := setup(t)
	logPeriod(t, f, "Weekend", "2026-07-24", "2026-07-26", 41.39, 2.16)
	logPeriod(t, f, "Camp", "2026-07-20", "2026-07-28", 39.57, 2.65)

	got, _ := resolve(t, f, "2026-07-25")
	assert.Equal(t, "Weekend", got.Name)
}

func TestResolve_UnconfiguredIsHonest404(t *testing.T) {
	f := setupNoHome(t)

	rec := doReq(t, f.r, http.MethodGet, "/locations/resolve?date=2026-07-22", "")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "location_unconfigured")

	// A covering trip still resolves without home configured.
	logPeriod(t, f, "Mallorca", "2026-07-20", "2026-07-28", 39.57, 2.65)
	got, code := resolve(t, f, "2026-07-22")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, locations.SourceTravel, got.Source)
}

func TestResolve_DefaultsToToday(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodGet, "/locations/resolve", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var got locations.Resolved
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, time.Now().UTC().Format("2006-01-02"), got.Date)
	assert.Equal(t, locations.SourceHome, got.Source)
}

func TestResolve_DateInvalid(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodGet, "/locations/resolve?date=tomorrow", "")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "date_invalid")
}

// The endpoint and the primitive weather consumers read are the same code, so
// they can never disagree about which location produced a forecast.
func TestResolve_EndpointMatchesPrimitive(t *testing.T) {
	f := setup(t)
	logPeriod(t, f, "Camp", "2026-07-20", "2026-07-28", 39.57, 2.65)
	logPeriod(t, f, "Weekend", "2026-07-24", "2026-07-26", 41.39, 2.16)

	for _, d := range []string{"2026-07-19", "2026-07-22", "2026-07-25", "2026-07-30"} {
		date, err := time.Parse("2006-01-02", d)
		require.NoError(t, err)
		direct, err := f.svc.LocationOn(context.Background(), date)
		require.NoError(t, err)

		viaHTTP, code := resolve(t, f, d)
		require.Equal(t, http.StatusOK, code, d)
		assert.Equal(t, *direct, viaHTTP, "endpoint and primitive disagree on %s", d)
	}
}

func TestList_OverlappingWindowAscending(t *testing.T) {
	f := setup(t)
	logPeriod(t, f, "Camp", "2026-07-20", "2026-07-28", 39.57, 2.65)
	logPeriod(t, f, "Weekend", "2026-07-24", "2026-07-26", 41.39, 2.16)
	logPeriod(t, f, "Later", "2026-09-01", "2026-09-05", 45.0, 9.0)

	rec := doReq(t, f.r, http.MethodGet, "/locations?from=2026-07-01&to=2026-07-31", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out struct {
		Locations []locations.Period `json:"locations"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Locations, 2, "the September trip is outside the window")
	assert.Equal(t, "Camp", out.Locations[0].Name, "ascending by start_date")
	assert.Equal(t, "Weekend", out.Locations[1].Name)
}

// Overlap, not containment: a trip straddling the window edge is still where
// the athlete was.
func TestList_ReturnsPartiallyOverlappingPeriods(t *testing.T) {
	f := setup(t)
	logPeriod(t, f, "Straddles start", "2026-06-25", "2026-07-02", 39.57, 2.65)
	logPeriod(t, f, "Straddles end", "2026-07-28", "2026-08-04", 41.39, 2.16)
	logPeriod(t, f, "Spans whole window", "2026-06-01", "2026-09-01", 45.0, 9.0)

	rec := doReq(t, f.r, http.MethodGet, "/locations?from=2026-07-01&to=2026-07-31", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var out struct {
		Locations []locations.Period `json:"locations"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Len(t, out.Locations, 3)
}

func TestList_EmptyWindowIs200(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodGet, "/locations?from=2026-07-01&to=2026-07-31", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"locations":[]}`, rec.Body.String(), "empty array, never null")
}

func TestList_RangeErrors(t *testing.T) {
	f := setup(t)
	cases := []struct{ name, query, want string }{
		{"missing both", "", "range_required"},
		{"missing to", "from=2026-07-01", "range_required"},
		{"unparseable", "from=nope&to=2026-07-31", "date_invalid"},
		{"inverted", "from=2026-07-31&to=2026-07-01", "range_invalid"},
		{"too large", "from=2026-01-01&to=2027-06-01", "range_too_large"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doReq(t, f.r, http.MethodGet, "/locations?"+tc.query, "")
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), tc.want)
		})
	}
}

func TestGet_AndDelete(t *testing.T) {
	f := setup(t)
	p := logPeriod(t, f, "Mallorca", "2026-07-20", "2026-07-28", 39.57, 2.65)

	rec := doReq(t, f.r, http.MethodGet, "/locations/"+p.ID.String(), "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Mallorca")

	rec = doReq(t, f.r, http.MethodDelete, "/locations/"+p.ID.String(), "")
	require.Equal(t, http.StatusNoContent, rec.Code)

	rec = doReq(t, f.r, http.MethodGet, "/locations/"+p.ID.String(), "")
	assert.Equal(t, http.StatusNotFound, rec.Code)

	// The date it covered falls back to home once the trip is gone.
	got, code := resolve(t, f, "2026-07-22")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, locations.SourceHome, got.Source)
}

func TestGet_Delete_NotFound(t *testing.T) {
	f := setup(t)
	missing := "6f1c2b7e-6c9a-4c1e-9f3a-0d2f7b8e1a11"

	for _, m := range []string{http.MethodGet, http.MethodDelete} {
		rec := doReq(t, f.r, m, "/locations/"+missing, "")
		require.Equal(t, http.StatusNotFound, rec.Code, m)
		assert.Contains(t, rec.Body.String(), "not_found")

		rec = doReq(t, f.r, m, "/locations/not-a-uuid", "")
		require.Equal(t, http.StatusNotFound, rec.Code, m)
	}
}

// Corrections are delete + re-log; there is no PATCH to reach for.
func TestPatch_NotRouted(t *testing.T) {
	f := setup(t)
	p := logPeriod(t, f, "Mallorca", "2026-07-20", "2026-07-28", 39.57, 2.65)

	rec := doReq(t, f.r, http.MethodPatch, "/locations/"+p.ID.String(), `{"name":"Ibiza"}`)
	assert.NotEqual(t, http.StatusOK, rec.Code, "PATCH must not be a route")
}

// Extending a trip is delete + re-log — the documented correction path.
func TestExtendTripByRelogging(t *testing.T) {
	f := setup(t)
	p := logPeriod(t, f, "Mallorca", "2026-07-20", "2026-07-28", 39.57, 2.65)

	got, _ := resolve(t, f, "2026-07-30")
	require.Equal(t, locations.SourceHome, got.Source, "the 30th is outside the original trip")

	rec := doReq(t, f.r, http.MethodDelete, "/locations/"+p.ID.String(), "")
	require.Equal(t, http.StatusNoContent, rec.Code)
	logPeriod(t, f, "Mallorca", "2026-07-20", "2026-07-31", 39.57, 2.65)

	got, _ = resolve(t, f, "2026-07-30")
	assert.Equal(t, locations.SourceTravel, got.Source)
	assert.Equal(t, "Mallorca", got.Name)
}
