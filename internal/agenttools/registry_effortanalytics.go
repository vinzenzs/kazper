package agenttools

import (
	"encoding/json"
	"net/url"
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
	}
}
