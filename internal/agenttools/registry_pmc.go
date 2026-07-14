package agenttools

import (
	"encoding/json"
	"net/url"
)

// Performance Management Chart read — the classic Coggan CTL/ATL/TSB daily series
// computed on-read from stored completed-workout TSS. One HTTP call to
// GET /performance/pmc, per the REST↔MCP 1:1 convention.

func init() { registerMCPDomain(pmcSpecs()) }

// PMCSeriesArgs is the input to pmc_series. from/to are inclusive calendar dates.
type PMCSeriesArgs struct {
	From string `json:"from" jsonschema:"inclusive start date YYYY-MM-DD"`
	To   string `json:"to" jsonschema:"inclusive end date YYYY-MM-DD; up to 400 days from 'from'"`
	TZ   string `json:"tz,omitempty" jsonschema:"IANA timezone for calendar-day boundaries (e.g. Europe/Berlin). If omitted, the REST server uses DEFAULT_USER_TZ."`
}

func pmcSpecs() []Spec {
	return []Spec{
		{
			Name: "pmc_series",
			Description: "Performance Management Chart: the classic Coggan CTL/ATL/TSB daily series computed " +
				"from stored COMPLETED-workout TSS. Per day returns tss_total, ctl (42-day EWMA = fitness), " +
				"atl (7-day EWMA = fatigue), tsb (yesterday's ctl−atl = form; positive is fresh, negative is " +
				"fatigued), and ramp_rate (ctl change over 7 days). Warm-up runs from the earliest workout so " +
				"values are window-independent (seed_date reports coverage). ramp_alerts flags Monday-start " +
				"weeks whose CTL rose more than 8/week (overreaching). A completed workout with no TSS counts " +
				"as 0 but is surfaced via per-day missing_tss_count + window missing_tss_workouts, so a " +
				"deflated CTL is visible. This is Kazper's OWN Coggan computation — distinct from the stored " +
				"Garmin acute/chronic load (garmin_load / fitness metrics), which is a different metric. " +
				"Range up to 400 days. Read-only; no idempotency key is sent.",
			SchemaType: PMCSeriesArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a PMCSeriesArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				if a.TZ != "" {
					q.Set("tz", a.TZ)
				}
				return HTTPCall{Method: "GET", Path: "/performance/pmc", Query: q}, nil
			},
		},
	}
}
