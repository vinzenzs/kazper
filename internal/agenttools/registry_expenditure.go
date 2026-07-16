package agenttools

import (
	"encoding/json"
	"net/url"
)

// Adaptive expenditure read — the energy-balance TDEE estimate over a window.
// One GET, body forwarded verbatim.

func init() { registerMCPDomain(expenditureSpecs()) }

// EnergyExpenditureArgs is the input shape for `energy_expenditure`.
// `from`/`to` are required inclusive calendar dates; `tz` is optional.
type EnergyExpenditureArgs struct {
	From string `json:"from" jsonschema:"inclusive start date YYYY-MM-DD"`
	To   string `json:"to" jsonschema:"inclusive end date YYYY-MM-DD; max 92-day span"`
	TZ   string `json:"tz,omitempty" jsonschema:"IANA timezone for calendar-day boundaries (e.g. Europe/Berlin). If omitted, the REST server uses DEFAULT_USER_TZ."`
}

func expenditureSpecs() []Spec {
	return []Spec{
		{
			Name: "energy_expenditure",
			Description: "Estimate the athlete's ACTUAL average daily energy expenditure (TDEE) over a " +
				"window from energy balance, rather than from a formula: " +
				"`mean(intake over logged days) − Δ trend-weight × 7700 kcal/kg ÷ window_days`. " +
				"Intake comes from logged meals; the mass signal is the smoothed 7-day body-weight " +
				"trend (raw daily weight swings 1–2 kg on water alone and cannot be used directly). " +
				"A falling trend means expenditure ran ABOVE intake; a rising trend, below. " +
				"Use a 21–28 day window — that is the honest sweet spot. " +
				"Returns `expenditure_kcal_per_day` plus every input behind it: the trend endpoints " +
				"with the dates they were taken at, the intake mean, and the per-day series. " +
				"Honest degradation (200, `expenditure_kcal_per_day: null` + `reason`): " +
				"`insufficient_logged_days` (< 14 days with at least one meal) or " +
				"`insufficient_weigh_ins` (< 5 weigh-ins in the window) — a gated answer means the " +
				"data isn't there yet, NOT that expenditure is unknowable; say what's missing. " +
				"CAVEATS to carry into any advice: (1) unlogged snacks bias the estimate DOWN — " +
				"check `days_unlogged` and the athlete's logging discipline before trusting a low " +
				"number; (2) a window spanning deliberate glycogen manipulation (carb-load, race " +
				"taper, a big salt/water shift) moves water mass, not tissue, and corrupts the " +
				"trend delta — do not read expenditure across race week. " +
				"This tool is ADVISORY and knows nothing about goals: it reads no target and changes " +
				"nothing. To compare against the athlete's current target, read it with `get_goals` " +
				"(or `get_daily_goal_override` for a specific date); to act on a gap, propose an " +
				"explicit change via `set_goals` / `set_daily_goal_override` and let the athlete decide. " +
				"Read-only; no idempotency-key is sent.",
			SchemaType: EnergyExpenditureArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a EnergyExpenditureArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				if a.TZ != "" {
					q.Set("tz", a.TZ)
				}
				return HTTPCall{Method: "GET", Path: "/nutrition/expenditure", Query: q}, nil
			},
		},
	}
}
