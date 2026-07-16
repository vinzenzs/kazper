## ADDED Requirements

### Requirement: A guarded Open-Meteo client serves forecast, archive, and geocoding

The system SHALL provide an internal weather client over Open-Meteo's keyless HTTPS API with
three operations: `Forecast(lat, lon, window)` and `Archive(lat, lon, window)` returning hourly
temperature (°C), relative humidity (%), wind speed (m/s), and cloud cover (%) for the window,
and `Geocode(place)` returning the top match's name, country, lat, and lon. Requests SHALL
carry a bounded timeout (~5 s); responses SHALL be cached in memory (forecast ~30 minutes,
archive for the process lifetime). The client SHALL be fail-open: any network, timeout, or
shape failure yields a no-data result that consumers surface as `weather_unavailable` —
weather trouble SHALL never produce a 5xx from a consuming endpoint. No credentials are stored
or required.

#### Scenario: A forecast failure degrades the consumer, not the request

- **WHEN** Open-Meteo is unreachable while a heat read runs
- **THEN** the heat endpoint returns `200` with `reason: "weather_unavailable"` and no
  weather-derived fields

#### Scenario: Repeated reads inside the TTL hit the cache

- **WHEN** two heat reads for the same location/window run within 30 minutes
- **THEN** Open-Meteo is called once
