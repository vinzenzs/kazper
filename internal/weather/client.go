// Package weather is a guarded client for Open-Meteo's keyless HTTPS API —
// forecast, historical archive, and geocoding.
//
// It is the second outbound integration in the repo after Open Food Facts, and
// keeps the same posture: bounded timeouts, an in-memory cache, and FAIL-OPEN
// results. Every operation returns (data, ok) rather than an error the caller
// might accidentally propagate as a 5xx — weather trouble degrades the
// consumer's answer (`weather_unavailable`), it never breaks the request.
//
// No credentials are stored or required: Open-Meteo is keyless, which keeps the
// self-hosted posture intact (no account, no secret to leak).
package weather

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// Defaults. The timeout is deliberately short: this sits on a read path, and a
// slow forecast must degrade rather than hang a coach's morning check-in.
const (
	DefaultTimeout      = 5 * time.Second
	DefaultForecastTTL  = 30 * time.Minute
	defaultForecastBase = "https://api.open-meteo.com"
	defaultArchiveBase  = "https://archive-api.open-meteo.com"
	defaultGeocodeBase  = "https://geocoding-api.open-meteo.com"
)

// Hour is one hourly observation/forecast sample.
type Hour struct {
	Time         time.Time `json:"time"`
	TemperatureC float64   `json:"temperature_c"`
	HumidityPct  float64   `json:"humidity_pct"`
	WindSpeedMPS float64   `json:"wind_speed_mps"`
	CloudCovPct  float64   `json:"cloud_cover_pct"`
}

// Place is a geocoding match.
type Place struct {
	Name    string  `json:"name"`
	Country string  `json:"country,omitempty"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
}

// Config controls the client's HTTP behavior. The three base URLs are separate
// because Open-Meteo serves forecast, archive and geocoding from distinct
// hosts; tests point all three at one httptest server.
type Config struct {
	ForecastBaseURL string
	ArchiveBaseURL  string
	GeocodeBaseURL  string
	// Timeout per request. Zero means DefaultTimeout.
	Timeout time.Duration
	// ForecastTTL bounds forecast cache entries. Zero means DefaultForecastTTL.
	// Archive data is immutable, so it is cached for the process lifetime.
	ForecastTTL time.Duration
	// HTTPClient overrides the underlying client. Tests inject a stub here.
	HTTPClient *http.Client
	// Now is injectable so cache-expiry tests don't sleep.
	Now func() time.Time
}

// Client fetches weather from Open-Meteo.
type Client struct {
	forecastBase string
	archiveBase  string
	geocodeBase  string
	httpClient   *http.Client
	forecastTTL  time.Duration
	now          func() time.Time
	logger       *slog.Logger

	mu    sync.Mutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	hours   []Hour
	places  []Place
	expires time.Time // zero means "never" (archive: the past doesn't change)
}

// New returns a configured client. Logger may be nil.
func New(cfg Config, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: timeout}
	}
	ttl := cfg.ForecastTTL
	if ttl == 0 {
		ttl = DefaultForecastTTL
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Client{
		forecastBase: orDefault(cfg.ForecastBaseURL, defaultForecastBase),
		archiveBase:  orDefault(cfg.ArchiveBaseURL, defaultArchiveBase),
		geocodeBase:  orDefault(cfg.GeocodeBaseURL, defaultGeocodeBase),
		httpClient:   hc,
		forecastTTL:  ttl,
		now:          now,
		logger:       logger,
		cache:        map[string]cacheEntry{},
	}
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// Window is an inclusive UTC time range to fetch hourly data for.
type Window struct {
	From time.Time
	To   time.Time
}

// Forecast returns hourly weather covering the window. ok=false on any
// network, timeout, status or shape failure — the caller degrades, never errors.
func (c *Client) Forecast(ctx context.Context, lat, lon float64, w Window) (hours []Hour, ok bool) {
	return c.hourly(ctx, c.forecastBase+"/v1/forecast", lat, lon, w, c.forecastTTL)
}

// Archive returns hourly historical weather covering the window. Cached for the
// process lifetime: the past does not change.
func (c *Client) Archive(ctx context.Context, lat, lon float64, w Window) (hours []Hour, ok bool) {
	return c.hourly(ctx, c.archiveBase+"/v1/archive", lat, lon, w, 0)
}

// hourlyResponse mirrors Open-Meteo's hourly block. Arrays are parallel and
// indexed by hour.
type hourlyResponse struct {
	Hourly struct {
		Time               []string  `json:"time"`
		Temperature2m      []float64 `json:"temperature_2m"`
		RelativeHumidity2m []float64 `json:"relative_humidity_2m"`
		WindSpeed10m       []float64 `json:"wind_speed_10m"`
		CloudCover         []float64 `json:"cloud_cover"`
	} `json:"hourly"`
}

// hourly fetches and parses an hourly endpoint. ttl == 0 caches forever.
func (c *Client) hourly(ctx context.Context, endpoint string, lat, lon float64, w Window, ttl time.Duration) ([]Hour, bool) {
	q := url.Values{}
	q.Set("latitude", strconv.FormatFloat(lat, 'f', 4, 64))
	q.Set("longitude", strconv.FormatFloat(lon, 'f', 4, 64))
	q.Set("hourly", "temperature_2m,relative_humidity_2m,wind_speed_10m,cloud_cover")
	q.Set("wind_speed_unit", "ms") // the repo stores wind in m/s; don't convert twice
	q.Set("timezone", "UTC")
	q.Set("start_date", w.From.UTC().Format("2006-01-02"))
	q.Set("end_date", w.To.UTC().Format("2006-01-02"))
	target := endpoint + "?" + q.Encode()

	if hours, hit := c.cachedHours(target); hit {
		return hours, true
	}

	body, ok := c.get(ctx, target)
	if !ok {
		return nil, false
	}
	var parsed hourlyResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		c.logger.Warn("weather: malformed hourly response", "err", err)
		return nil, false
	}

	h := parsed.Hourly
	n := len(h.Time)
	// Parallel arrays must actually be parallel; a short one means a shape we
	// don't understand, and guessing which hour a value belongs to would be
	// worse than degrading.
	if n == 0 || len(h.Temperature2m) != n || len(h.RelativeHumidity2m) != n ||
		len(h.WindSpeed10m) != n || len(h.CloudCover) != n {
		c.logger.Warn("weather: unexpected hourly shape", "hours", n)
		return nil, false
	}

	out := make([]Hour, 0, n)
	for i := 0; i < n; i++ {
		ts, err := time.ParseInLocation("2006-01-02T15:04", h.Time[i], time.UTC)
		if err != nil {
			c.logger.Warn("weather: unparseable hour", "value", h.Time[i])
			return nil, false
		}
		out = append(out, Hour{
			Time:         ts,
			TemperatureC: h.Temperature2m[i],
			HumidityPct:  h.RelativeHumidity2m[i],
			WindSpeedMPS: h.WindSpeed10m[i],
			CloudCovPct:  h.CloudCover[i],
		})
	}
	c.storeHours(target, out, ttl)
	return out, true
}

// geocodeResponse mirrors Open-Meteo's geocoding search block.
type geocodeResponse struct {
	Results []struct {
		Name      string  `json:"name"`
		Country   string  `json:"country"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"results"`
}

// Geocode returns matches for a place name, best first. ok=false means the
// lookup could not be performed (network/shape); an empty slice with ok=true
// means the lookup worked and matched nothing — a real answer, and a different
// thing from "geocoding is down".
func (c *Client) Geocode(ctx context.Context, place string) ([]Place, bool) {
	q := url.Values{}
	q.Set("name", place)
	q.Set("count", "1")
	q.Set("format", "json")
	target := c.geocodeBase + "/v1/search?" + q.Encode()

	if places, hit := c.cachedPlaces(target); hit {
		return places, true
	}

	body, ok := c.get(ctx, target)
	if !ok {
		return nil, false
	}
	var parsed geocodeResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		c.logger.Warn("weather: malformed geocode response", "err", err)
		return nil, false
	}
	out := make([]Place, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		out = append(out, Place{Name: r.Name, Country: r.Country, Lat: r.Latitude, Lon: r.Longitude})
	}
	// Place names don't move; cache for the process lifetime.
	c.storePlaces(target, out)
	return out, true
}

// get performs the HTTP GET, returning ok=false for every failure mode.
func (c *Client) get(ctx context.Context, target string) ([]byte, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		c.logger.Warn("weather: build request", "err", err)
		return nil, false
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Warn("weather: request failed", "err", err)
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("weather: non-200", "status", resp.StatusCode)
		return nil, false
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Warn("weather: read body", "err", err)
		return nil, false
	}
	return body, true
}

func (c *Client) cachedHours(key string) ([]Hour, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.cache[key]
	if !ok || e.hours == nil {
		return nil, false
	}
	if !e.expires.IsZero() && c.now().After(e.expires) {
		delete(c.cache, key)
		return nil, false
	}
	return e.hours, true
}

func (c *Client) storeHours(key string, hours []Hour, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := cacheEntry{hours: hours}
	if ttl > 0 {
		e.expires = c.now().Add(ttl)
	}
	c.cache[key] = e
}

func (c *Client) cachedPlaces(key string) ([]Place, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.cache[key]
	if !ok || e.places == nil {
		return nil, false
	}
	return e.places, true
}

func (c *Client) storePlaces(key string, places []Place) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = cacheEntry{places: places}
}

// MeanOver averages the hours whose timestamp falls inside [from, to]. Returns
// ok=false when the window catches no samples — an empty mean is not zero.
func MeanOver(hours []Hour, from, to time.Time) (Hour, bool) {
	var (
		sum Hour
		n   int
	)
	for _, h := range hours {
		if h.Time.Before(from) || h.Time.After(to) {
			continue
		}
		sum.TemperatureC += h.TemperatureC
		sum.HumidityPct += h.HumidityPct
		sum.WindSpeedMPS += h.WindSpeedMPS
		sum.CloudCovPct += h.CloudCovPct
		n++
	}
	if n == 0 {
		return Hour{}, false
	}
	f := float64(n)
	return Hour{
		Time:         from,
		TemperatureC: sum.TemperatureC / f,
		HumidityPct:  sum.HumidityPct / f,
		WindSpeedMPS: sum.WindSpeedMPS / f,
		CloudCovPct:  sum.CloudCovPct / f,
	}, true
}
