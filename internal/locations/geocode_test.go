package locations_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/locations"
	"github.com/vinzenzs/kazper/internal/weather"
)

// fakeGeocoder stands in for the weather client's Geocode.
type fakeGeocoder struct {
	places []weather.Place
	ok     bool
	calls  int
}

func (f *fakeGeocoder) Geocode(_ context.Context, _ string) ([]weather.Place, bool) {
	f.calls++
	return f.places, f.ok
}

func mallorca() *fakeGeocoder {
	return &fakeGeocoder{
		places: []weather.Place{{Name: "Palma", Country: "Spain", Lat: 39.5696, Lon: 2.6502}},
		ok:     true,
	}
}

// setupGeo mounts the endpoints with a geocoder wired.
func setupGeo(t *testing.T, g locations.Geocoder) *fixture {
	t.Helper()
	f := setup(t)
	f.svc.SetGeocoder(g)
	return f
}

func postLocation(t *testing.T, f *fixture, body string) *struct {
	Location locations.Period `json:"location"`
} {
	t.Helper()
	rec := doReq(t, f.r, http.MethodPost, "/locations", body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	out := &struct {
		Location locations.Period `json:"location"`
	}{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), out))
	return out
}

// ============================================================================

func TestCreate_PlaceIsGeocoded(t *testing.T) {
	geo := mallorca()
	f := setupGeo(t, geo)

	out := postLocation(t, f, `{"place":"Mallorca","start_date":"2026-07-20","end_date":"2026-07-28"}`)

	assert.InDelta(t, 39.5696, out.Location.Lat, 0.0001)
	assert.InDelta(t, 2.6502, out.Location.Lon, 0.0001)
	// With no explicit name, the resolved match names the period.
	assert.Equal(t, "Palma", out.Location.Name)
	assert.Equal(t, 1, geo.calls)

	// And the dates it covers now resolve there.
	got, code := resolve(t, f, "2026-07-22")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, locations.SourceTravel, got.Source)
	assert.InDelta(t, 39.5696, got.Lat, 0.0001)
}

func TestCreate_PlaceKeepsAnExplicitName(t *testing.T) {
	f := setupGeo(t, mallorca())

	out := postLocation(t, f,
		`{"place":"Mallorca","name":"Summer camp","start_date":"2026-07-20","end_date":"2026-07-28"}`)

	assert.Equal(t, "Summer camp", out.Location.Name, "an explicit name is not overwritten")
	assert.InDelta(t, 39.5696, out.Location.Lat, 0.0001)
}

// Explicit coordinates are the athlete being specific — they win, and cost no
// geocoding call.
func TestCreate_ExplicitCoordinatesWinOverPlace(t *testing.T) {
	geo := mallorca()
	f := setupGeo(t, geo)

	out := postLocation(t, f,
		`{"place":"Mallorca","name":"Camp","start_date":"2026-07-20","end_date":"2026-07-28","lat":10.5,"lon":20.5}`)

	assert.InDelta(t, 10.5, out.Location.Lat, 0.0001)
	assert.InDelta(t, 20.5, out.Location.Lon, 0.0001)
	assert.Zero(t, geo.calls, "explicit coordinates need no lookup")
}

func TestCreate_UnknownPlaceIsRejectedAndStoresNothing(t *testing.T) {
	f := setupGeo(t, &fakeGeocoder{places: nil, ok: true}) // lookup worked, no match

	rec := doReq(t, f.r, http.MethodPost, "/locations",
		`{"place":"Nowhereville","start_date":"2026-07-20","end_date":"2026-07-28"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "place_not_found")

	// Nothing was stored.
	list := doReq(t, f.r, http.MethodGet, "/locations?from=2026-07-01&to=2026-07-31", "")
	assert.JSONEq(t, `{"locations":[]}`, list.Body.String())
}

// "No such place" and "the lookup is down" are different answers: the first is
// the athlete's typo, the second is our problem.
func TestCreate_GeocodingDownIsRefusedNotStoredUngeocoded(t *testing.T) {
	f := setupGeo(t, &fakeGeocoder{ok: false})

	rec := doReq(t, f.r, http.MethodPost, "/locations",
		`{"place":"Mallorca","start_date":"2026-07-20","end_date":"2026-07-28"}`)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "geocoding_unavailable")

	list := doReq(t, f.r, http.MethodGet, "/locations?from=2026-07-01&to=2026-07-31", "")
	assert.JSONEq(t, `{"locations":[]}`, list.Body.String(),
		"a period without real coordinates would resolve every forecast to the wrong city")
}

// An unwired geocoder must not break the explicit-coordinates path.
func TestCreate_WithoutGeocoderExplicitCoordinatesStillWork(t *testing.T) {
	f := setup(t) // no geocoder

	p := logPeriod(t, f, "Mallorca", "2026-07-20", "2026-07-28", 39.57, 2.65)
	assert.InDelta(t, 39.57, p.Lat, 0.001)

	// ...but a place-only write says why it can't be served.
	rec := doReq(t, f.r, http.MethodPost, "/locations",
		`{"place":"Mallorca","start_date":"2026-07-20","end_date":"2026-07-28"}`)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "geocoding_unavailable")
}

// A place-only write with a blank place is just a write missing coordinates.
func TestCreate_BlankPlaceFallsThroughToLatLonValidation(t *testing.T) {
	f := setupGeo(t, mallorca())

	rec := doReq(t, f.r, http.MethodPost, "/locations",
		`{"place":"   ","name":"X","start_date":"2026-07-20","end_date":"2026-07-28"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "lat_lon_invalid")
}
