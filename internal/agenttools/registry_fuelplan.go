package agenttools

import (
	"encoding/json"
	"net/url"
)

// Fuel periodization read — the planned-load day classifier and its carb
// suggestions. One GET, body forwarded verbatim.

func init() { registerMCPDomain(fuelPlanSpecs()) }

// FuelPlanArgs is the input shape for `fuel_plan`. Every field is optional:
// with no args the REST server classifies today plus six days.
type FuelPlanArgs struct {
	From string `json:"from,omitempty" jsonschema:"inclusive start date YYYY-MM-DD; defaults to today. Pass with 'to'."`
	To   string `json:"to,omitempty" jsonschema:"inclusive end date YYYY-MM-DD; defaults to from + 6 days. Max 14-day span."`
	TZ   string `json:"tz,omitempty" jsonschema:"IANA timezone for calendar-day boundaries (e.g. Europe/Berlin). If omitted, the REST server uses DEFAULT_USER_TZ."`
}

func fuelPlanSpecs() []Spec {
	return []Spec{
		{
			Name: "fuel_plan",
			Description: "Classify each upcoming day by its PLANNED training load and get a matching " +
				"carbohydrate target — 'fuel for the work required'. With no arguments: today plus six days. " +
				"Tier from the day's total planned TSS: `rest` (no planned session) / `easy` (< 60) / " +
				"`moderate` (60–150) / `heavy` (> 150, or any single planned session of 150 min or more — " +
				"a long endurance day is glycogen-expensive even at low intensity). Tiers map to a fixed " +
				"3 / 5 / 7 / 9 g/kg ladder × the smoothed body-weight trend, returned as " +
				"`suggested_carbs_g` beside that date's currently-effective goal carbs and the `delta_g` " +
				"between them. Each day echoes the planned sessions behind its tier, so a classification " +
				"you disagree with can be checked against its inputs rather than argued with. " +
				"Honest degradations: `plan_missing: true` marks a day the plan doesn't reach — its `rest` " +
				"tier means 'no data', NOT 'planned rest day', and must not be presented as one; " +
				"`reason: \"weight_missing\"` returns tiers and g/kg without gram targets. " +
				"SUGGESTIONS ONLY — this tool writes NOTHING. To act on a day, propose the number to the " +
				"athlete, and only once they confirm apply it with `set_daily_goal_override` for that date. " +
				"Never apply a suggestion silently. " +
				"SCOPE: this periodizes CARBS WITHIN the standing kcal target; it does not estimate the " +
				"target itself — that's `energy_expenditure`, a separate concern. Protein stays flat by " +
				"design (the evidence supports it), so don't periodize it from these tiers. " +
				"Read-only; no idempotency-key is sent.",
			SchemaType: FuelPlanArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a FuelPlanArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				if a.From != "" {
					q.Set("from", a.From)
				}
				if a.To != "" {
					q.Set("to", a.To)
				}
				if a.TZ != "" {
					q.Set("tz", a.TZ)
				}
				return HTTPCall{Method: "GET", Path: "/nutrition/fuel-plan", Query: q}, nil
			},
		},
	}
}
