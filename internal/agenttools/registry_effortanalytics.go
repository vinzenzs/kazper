package agenttools

import (
	"encoding/json"
	"net/url"
	"strconv"
)

// Mean-maximal power/pace curve read — the best average power (cycling) or speed
// (run/swim) sustained for each ladder duration across a window. One HTTP call to
// GET /workouts/power-curve, per the REST↔MCP 1:1 convention.

func init() { registerMCPDomain(effortAnalyticsSpecs()) }

// PowerCurveArgs is the input to power_curve. `from`/`to` are inclusive calendar
// dates (YYYY-MM-DD). `sport` selects the metric: bike → power (W), run/swim →
// speed (m/s, i.e. pace).
type PowerCurveArgs struct {
	From  string `json:"from" jsonschema:"inclusive start date YYYY-MM-DD"`
	To    string `json:"to" jsonschema:"inclusive end date YYYY-MM-DD; up to 400 days from 'from'"`
	Sport string `json:"sport,omitempty" jsonschema:"bike (→ power) | run | swim (→ speed/pace); defaults to bike"`
	TZ    string `json:"tz,omitempty" jsonschema:"IANA timezone for calendar-day boundaries (e.g. Europe/Berlin). If omitted, the REST server uses DEFAULT_USER_TZ."`
}

// CPModelArgs is the input to cp_model. `from`/`to` are inclusive calendar dates.
type CPModelArgs struct {
	From string `json:"from" jsonschema:"inclusive start date YYYY-MM-DD"`
	To   string `json:"to" jsonschema:"inclusive end date YYYY-MM-DD; up to 400 days from 'from'"`
	TZ   string `json:"tz,omitempty" jsonschema:"IANA timezone for calendar-day boundaries (e.g. Europe/Berlin). If omitted, the REST server uses DEFAULT_USER_TZ."`
}

// PowerProfileArgs is the input to power_profile. `weight_kg` overrides the W/kg
// denominator; omitted, the endpoint uses the latest stored body weight.
type PowerProfileArgs struct {
	From     string  `json:"from" jsonschema:"inclusive start date YYYY-MM-DD"`
	To       string  `json:"to" jsonschema:"inclusive end date YYYY-MM-DD; up to 400 days from 'from'"`
	WeightKg float64 `json:"weight_kg,omitempty" jsonschema:"body weight in kg for the W/kg denominator (> 0); if omitted, the latest stored body weight is used"`
	Sex      string  `json:"sex,omitempty" jsonschema:"male (default) | female — selects the Coggan reference table"`
	TZ       string  `json:"tz,omitempty" jsonschema:"IANA timezone for calendar-day boundaries (e.g. Europe/Berlin). If omitted, the REST server uses DEFAULT_USER_TZ."`
}

func effortAnalyticsSpecs() []Spec {
	return []Spec{
		{
			Name: "power_curve",
			Description: "Return the mean-maximal power/pace curve over a date window: for each ladder duration " +
				"(5s, 15s, 30s, 1m, 5m, 10m, 20m, 30m, 60m) the BEST average value achieved across completed " +
				"workouts in the range, with the contributing workout id and date. Metric follows `sport`: " +
				"bike → power in watts, run/swim → speed in m/s (pace). This is the backbone of FTP/threshold " +
				"reasoning ('what's my best 20-minute power this season'). It is built from per-activity " +
				"best-effort records ingested from Garmin streams; a workout with no power/speed stream " +
				"contributes nothing. Empty window returns an empty curve. Read-only; no idempotency key.",
			SchemaType: PowerCurveArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a PowerCurveArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				if a.Sport != "" {
					q.Set("sport", a.Sport)
				}
				if a.TZ != "" {
					q.Set("tz", a.TZ)
				}
				return HTTPCall{Method: "GET", Path: "/workouts/power-curve", Query: q}, nil
			},
		},
		{
			Name: "cp_model",
			Description: "Fit the 2-parameter critical-power model (CP2) over a date window's power best-efforts " +
				"(cycling only). Returns the estimated CRITICAL POWER `cp_watts` (the athlete's data-derived " +
				"sustainable threshold, ≈ FTP) and `w_prime_kj` (anaerobic work capacity above CP), with fit " +
				"quality (`r_squared`, `rmse_w`) and the exact effort points used (2–30 min durations). Use it to " +
				"sanity-check the CONFIGURED `ftp_watts` (read via athlete_config_get) against what recent racing/" +
				"training actually shows — e.g. 'your best efforts fit CP 268 W but your configured FTP is 250 W, " +
				"worth re-testing'. ADVISORY ONLY: this does not read or change athlete-config; to APPLY a new " +
				"threshold, use the athlete-config update flow (athlete_config_update). When the window lacks " +
				"enough spread of long efforts the response is 200 with `model: null` and a `reason` " +
				"(`insufficient_points` / `span_too_narrow`) plus whatever points were found. Read-only; no " +
				"idempotency key. Typical call: the trailing 90 days.",
			SchemaType: CPModelArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a CPModelArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				if a.TZ != "" {
					q.Set("tz", a.TZ)
				}
				return HTTPCall{Method: "GET", Path: "/workouts/cp-model", Query: q}, nil
			},
		},
		{
			Name: "power_profile",
			Description: "Rank the athlete's windowed best power efforts against the Coggan power-profile tables. " +
				"For four benchmark durations — 5 s (neuromuscular), 1 min (anaerobic), 5 min (VO₂max), and the " +
				"20-minute best as a functional-threshold proxy (no 0.95 haircut) — it reports W/kg, the Coggan " +
				"CATEGORY band (untrained → world class) which is the authoritative output, and an interpolated " +
				"`percentile` (an estimate). It also returns a rider `phenotype` (sprinter / time_trialist / " +
				"climber / all_rounder), null unless all four anchors are present. The W/kg denominator is the " +
				"`weight_kg` arg (> 0) or, if omitted, the latest stored body weight (else `weight_data_missing`); " +
				"`weight_source` is echoed. `sex` selects the table (male default). Cycling power only. ADVISORY: " +
				"does not read or change athlete-config. Read-only; no idempotency key. Typical call: the trailing " +
				"90 days.",
			SchemaType: PowerProfileArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a PowerProfileArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				if a.WeightKg > 0 {
					q.Set("weight_kg", strconv.FormatFloat(a.WeightKg, 'f', -1, 64))
				}
				if a.Sex != "" {
					q.Set("sex", a.Sex)
				}
				if a.TZ != "" {
					q.Set("tz", a.TZ)
				}
				return HTTPCall{Method: "GET", Path: "/workouts/power-profile", Query: q}, nil
			},
		},
	}
}
