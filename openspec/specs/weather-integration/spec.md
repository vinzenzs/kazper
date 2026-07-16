# weather-integration Specification

## Purpose

Give the rest of the system weather to reason about, without letting the network into its
correctness. This is the contract for Kazper's Open-Meteo client — hourly forecast, historical
archive, and geocoding — and the vendor is an implementation detail behind it.

Three properties define the capability. It is **keyless**: no account, no credential, nothing to
store or leak, which keeps the self-hosted posture intact (the second outbound integration
after Open Food Facts, and deliberately the same shape). It is **cached**: a forecast for ~30
minutes, an archive window for the process lifetime, because the past does not change. And it
is **fail-open by construction** — every operation returns `(data, ok)` rather than an error a
caller might propagate, so a timeout, a rate limit, a malformed payload or an unreachable host
degrades the consumer's answer to `weather_unavailable` and can never turn a coach's read into
a 5xx.

The client distinguishes "the lookup ran and found nothing" from "the lookup could not run" —
a geocode that matches no place is a real answer about the athlete's input, while a geocode
that fails is a fact about us, and consumers owe the user different words for each.

## Requirements
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

