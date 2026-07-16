package weather_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/weather"
)

const hourlyBody = `{
  "hourly": {
    "time": ["2026-07-20T08:00","2026-07-20T09:00","2026-07-20T10:00"],
    "temperature_2m": [24.0, 28.0, 32.0],
    "relative_humidity_2m": [70.0, 60.0, 50.0],
    "wind_speed_10m": [1.0, 2.0, 3.0],
    "cloud_cover": [10.0, 20.0, 30.0]
  }
}`

const geocodeBody = `{
  "results": [
    {"name":"Palma","country":"Spain","latitude":39.5696,"longitude":2.6502}
  ]
}`

// newClient points every base URL at one test server.
func newClient(t *testing.T, srv *httptest.Server, cfg weather.Config) *weather.Client {
	t.Helper()
	cfg.ForecastBaseURL = srv.URL
	cfg.ArchiveBaseURL = srv.URL
	cfg.GeocodeBaseURL = srv.URL
	return weather.New(cfg, nil)
}

func window() weather.Window {
	return weather.Window{
		From: time.Date(2026, 7, 20, 8, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC),
	}
}

// ============================================================================

func TestForecast_HappyPath(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(hourlyBody))
	}))
	defer srv.Close()

	hours, ok := newClient(t, srv, weather.Config{}).Forecast(context.Background(), 39.57, 2.65, window())

	require.True(t, ok)
	require.Len(t, hours, 3)
	assert.Equal(t, time.Date(2026, 7, 20, 8, 0, 0, 0, time.UTC), hours[0].Time)
	assert.InDelta(t, 24.0, hours[0].TemperatureC, 0.001)
	assert.InDelta(t, 70.0, hours[0].HumidityPct, 0.001)
	assert.InDelta(t, 3.0, hours[2].WindSpeedMPS, 0.001)
	assert.InDelta(t, 30.0, hours[2].CloudCovPct, 0.001)

	// Wind must be requested in m/s — the repo's unit — so nothing downstream
	// has to know Open-Meteo's km/h default.
	assert.Contains(t, gotQuery, "wind_speed_unit=ms")
	assert.Contains(t, gotQuery, "timezone=UTC")
	assert.Contains(t, gotQuery, "latitude=39.5700")
	assert.Contains(t, gotQuery, "start_date=2026-07-20")
}

// Every failure mode must degrade, never error: these are the cases that would
// otherwise turn a coach's check-in into a 5xx.
func TestForecast_FailsOpen(t *testing.T) {
	cases := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"non-200", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }},
		{"rate limited", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTooManyRequests) }},
		{"malformed json", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{not json`)) }},
		{"empty hourly", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"hourly":{}}`)) }},
		{"ragged arrays", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"hourly":{"time":["2026-07-20T08:00","2026-07-20T09:00"],
				"temperature_2m":[24.0],"relative_humidity_2m":[70.0,60.0],
				"wind_speed_10m":[1.0,2.0],"cloud_cover":[10.0,20.0]}}`))
		}},
		{"unparseable timestamp", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"hourly":{"time":["yesterday"],"temperature_2m":[24.0],
				"relative_humidity_2m":[70.0],"wind_speed_10m":[1.0],"cloud_cover":[10.0]}}`))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			hours, ok := newClient(t, srv, weather.Config{}).Forecast(context.Background(), 39.57, 2.65, window())

			assert.False(t, ok, "must report no-data")
			assert.Nil(t, hours)
		})
	}
}

func TestForecast_UnreachableHostFailsOpen(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing is listening now

	c := weather.New(weather.Config{ForecastBaseURL: url, Timeout: 200 * time.Millisecond}, nil)
	hours, ok := c.Forecast(context.Background(), 39.57, 2.65, window())

	assert.False(t, ok)
	assert.Nil(t, hours)
}

func TestForecast_TimeoutFailsOpen(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte(hourlyBody))
	}))
	defer srv.Close()

	c := newClient(t, srv, weather.Config{Timeout: 50 * time.Millisecond})
	hours, ok := c.Forecast(context.Background(), 39.57, 2.65, window())

	assert.False(t, ok, "a slow forecast degrades rather than hanging the read")
	assert.Nil(t, hours)
}

func TestForecast_CancelledContextFailsOpen(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(hourlyBody))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	hours, ok := newClient(t, srv, weather.Config{}).Forecast(ctx, 39.57, 2.65, window())
	assert.False(t, ok)
	assert.Nil(t, hours)
}

func TestForecast_CachedWithinTTL(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		_, _ = w.Write([]byte(hourlyBody))
	}))
	defer srv.Close()

	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	c := newClient(t, srv, weather.Config{
		ForecastTTL: 30 * time.Minute,
		Now:         func() time.Time { return now },
	})

	// Two reads for the same location/window inside the TTL → one upstream call.
	_, ok := c.Forecast(context.Background(), 39.57, 2.65, window())
	require.True(t, ok)
	_, ok = c.Forecast(context.Background(), 39.57, 2.65, window())
	require.True(t, ok)
	assert.EqualValues(t, 1, atomic.LoadInt32(&calls), "second read must hit the cache")

	// A different location is a different key.
	_, ok = c.Forecast(context.Background(), 48.21, 16.37, window())
	require.True(t, ok)
	assert.EqualValues(t, 2, atomic.LoadInt32(&calls))
}

func TestForecast_CacheExpires(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		_, _ = w.Write([]byte(hourlyBody))
	}))
	defer srv.Close()

	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	c := newClient(t, srv, weather.Config{
		ForecastTTL: 30 * time.Minute,
		Now:         func() time.Time { return now },
	})

	_, _ = c.Forecast(context.Background(), 39.57, 2.65, window())
	require.EqualValues(t, 1, atomic.LoadInt32(&calls))

	now = now.Add(31 * time.Minute) // past the TTL
	_, ok := c.Forecast(context.Background(), 39.57, 2.65, window())
	require.True(t, ok)
	assert.EqualValues(t, 2, atomic.LoadInt32(&calls), "a stale forecast must be refetched")
}

// The past doesn't change, so archive entries never expire.
func TestArchive_CachedForProcessLifetime(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		_, _ = w.Write([]byte(hourlyBody))
	}))
	defer srv.Close()

	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	c := newClient(t, srv, weather.Config{Now: func() time.Time { return now }})

	_, ok := c.Archive(context.Background(), 39.57, 2.65, window())
	require.True(t, ok)

	now = now.Add(72 * time.Hour)
	_, ok = c.Archive(context.Background(), 39.57, 2.65, window())
	require.True(t, ok)
	assert.EqualValues(t, 1, atomic.LoadInt32(&calls), "archive data is immutable — never refetched")
}

func TestGeocode_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "name=Mallorca")
		_, _ = w.Write([]byte(geocodeBody))
	}))
	defer srv.Close()

	places, ok := newClient(t, srv, weather.Config{}).Geocode(context.Background(), "Mallorca")

	require.True(t, ok)
	require.Len(t, places, 1)
	assert.Equal(t, "Palma", places[0].Name)
	assert.Equal(t, "Spain", places[0].Country)
	assert.InDelta(t, 39.5696, places[0].Lat, 0.0001)
	assert.InDelta(t, 2.6502, places[0].Lon, 0.0001)
}

// "matched nothing" and "geocoding is down" are different answers and must not
// collapse into each other.
func TestGeocode_NoMatchIsOkWithEmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`)) // Open-Meteo omits "results" entirely on no match
	}))
	defer srv.Close()

	places, ok := newClient(t, srv, weather.Config{}).Geocode(context.Background(), "Nowhereville")

	assert.True(t, ok, "the lookup succeeded — it just matched nothing")
	assert.Empty(t, places)
}

func TestGeocode_FailsOpen(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	places, ok := newClient(t, srv, weather.Config{}).Geocode(context.Background(), "Mallorca")

	assert.False(t, ok)
	assert.Nil(t, places)
}

func TestGeocode_Cached(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		_, _ = w.Write([]byte(geocodeBody))
	}))
	defer srv.Close()

	c := newClient(t, srv, weather.Config{})
	for i := 0; i < 3; i++ {
		_, ok := c.Geocode(context.Background(), "Mallorca")
		require.True(t, ok)
	}
	assert.EqualValues(t, 1, atomic.LoadInt32(&calls), "place names don't move")
}

func TestMeanOver(t *testing.T) {
	hours := hour3Fixture()

	got, ok := weather.MeanOver(hours, time.Date(2026, 7, 20, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC))
	require.True(t, ok)
	assert.InDelta(t, 28.0, got.TemperatureC, 0.001) // (24+28+32)/3
	assert.InDelta(t, 60.0, got.HumidityPct, 0.001)
	assert.InDelta(t, 2.0, got.WindSpeedMPS, 0.001)

	// A sub-window averages only the hours it covers.
	got, ok = weather.MeanOver(hours, time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC))
	require.True(t, ok)
	assert.InDelta(t, 30.0, got.TemperatureC, 0.001) // (28+32)/2

	// A window catching no samples is not a zero mean — it's no answer.
	_, ok = weather.MeanOver(hours, time.Date(2026, 7, 21, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC))
	assert.False(t, ok)

	_, ok = weather.MeanOver(nil, time.Date(2026, 7, 20, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC))
	assert.False(t, ok)
}

// hour3Fixture is the fixture behind hourlyBody, as parsed.
func hour3Fixture() []weather.Hour {
	return []weather.Hour{
		{Time: time.Date(2026, 7, 20, 8, 0, 0, 0, time.UTC), TemperatureC: 24, HumidityPct: 70, WindSpeedMPS: 1, CloudCovPct: 10},
		{Time: time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC), TemperatureC: 28, HumidityPct: 60, WindSpeedMPS: 2, CloudCovPct: 20},
		{Time: time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC), TemperatureC: 32, HumidityPct: 50, WindSpeedMPS: 3, CloudCovPct: 30},
	}
}
