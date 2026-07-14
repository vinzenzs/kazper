package agenttools

import (
	"encoding/json"
	"net/url"
)

// Wellness-diary tools — the athlete's subjective daily state, collected in
// conversation. log_wellness is a full-replace PUT (no Idempotency-Key — the
// REST backend rejects it on PUT, handled centrally by the generic dispatcher);
// list_wellness is a pure window read. Each is one HTTP call, per the REST↔MCP
// 1:1 convention. The objective recovery picture (HRV/sleep/RHR) is Garmin-fed
// and lives elsewhere — this is only the self-reported half.

func init() { registerMCPDomain(wellnessSpecs()) }

// LogWellnessArgs is the input to log_wellness. Every score is optional — a
// partial entry is first-class; nil fields are stored as NULL (full-replace).
type LogWellnessArgs struct {
	Date       string  `json:"date" jsonschema:"calendar date for the entry in YYYY-MM-DD (the athlete's day)"`
	Fatigue    *int    `json:"fatigue,omitempty" jsonschema:"how tired, 1-5 (1 = fresh/none, 5 = severe)"`
	Soreness   *int    `json:"soreness,omitempty" jsonschema:"muscle soreness, 1-5 (1 = none, 5 = severe)"`
	Stress     *int    `json:"stress,omitempty" jsonschema:"life/training stress, 1-5 (1 = none, 5 = severe)"`
	Mood       *int    `json:"mood,omitempty" jsonschema:"overall mood, 1-5 (1 = low, 5 = great)"`
	Motivation *int    `json:"motivation,omitempty" jsonschema:"motivation to train, 1-5 (1 = low, 5 = high)"`
	Note       *string `json:"note,omitempty" jsonschema:"optional free-text note (injuries, context); max 2000 chars"`
}

// ListWellnessArgs is the input to list_wellness.
type ListWellnessArgs struct {
	From string `json:"from" jsonschema:"inclusive start date YYYY-MM-DD"`
	To   string `json:"to" jsonschema:"inclusive end date YYYY-MM-DD; max 92 days from from"`
}

// WellnessCorrelationArgs is the input to wellness_correlation.
type WellnessCorrelationArgs struct {
	From   string `json:"from" jsonschema:"inclusive start date YYYY-MM-DD"`
	To     string `json:"to" jsonschema:"inclusive end date YYYY-MM-DD; max 92 days from from"`
	Metric string `json:"metric,omitempty" jsonschema:"PMC metric to correlate against: tsb (default) | ctl | ramp_rate"`
	TZ     string `json:"tz,omitempty" jsonschema:"IANA timezone for the PMC day bucketing; defaults to UTC"`
}

func wellnessSpecs() []Spec {
	return []Spec{
		{
			Name: "log_wellness",
			Description: "Record the athlete's subjective wellness for a date — the self-reported half of " +
				"recovery, beside the objective Garmin vitals (HRV/sleep/RHR). Five optional 1-5 scores: " +
				"fatigue, soreness, stress (1 = none → 5 = severe) and mood, motivation (1 = low → 5 = high), " +
				"plus an optional free-text note. FULL-REPLACE semantics: the entry replaces (not merges " +
				"with) any existing entry for that date, so include everything known; fields left out are " +
				"cleared. Prefer a PARTIAL entry over skipping the day entirely — logging just soreness is " +
				"more useful than nothing. At least one field must be present (empty entries are rejected). " +
				"Retries are NOT safe (the backend rejects Idempotency-Key on PUT). Date format: YYYY-MM-DD.",
			SchemaType: LogWellnessArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a LogWellnessArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				// PUT /wellness/{date} — no Idempotency-Key (the dispatcher skips
				// the header on PUT). nil score/note fields are omitted, matching
				// the handler's absent-means-cleared full-replace contract.
				payload := struct {
					Fatigue    *int    `json:"fatigue,omitempty"`
					Soreness   *int    `json:"soreness,omitempty"`
					Stress     *int    `json:"stress,omitempty"`
					Mood       *int    `json:"mood,omitempty"`
					Motivation *int    `json:"motivation,omitempty"`
					Note       *string `json:"note,omitempty"`
				}{
					Fatigue:    a.Fatigue,
					Soreness:   a.Soreness,
					Stress:     a.Stress,
					Mood:       a.Mood,
					Motivation: a.Motivation,
					Note:       a.Note,
				}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "PUT", Path: "/wellness/" + url.PathEscape(a.Date), Body: body}, nil
			},
		},
		{
			Name: "list_wellness",
			Description: "List the athlete's subjective wellness entries over a date range [from, to] " +
				"(inclusive, max 92 days), ascending. Use this to read the recent block — hold it against " +
				"the objective load/recovery trend ('TSB says fresh, but fatigue's been 4 all week'). Dates " +
				"without an entry are simply absent from the response.",
			SchemaType: ListWellnessArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ListWellnessArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				return HTTPCall{Method: "GET", Path: "/wellness", Query: q}, nil
			},
		},
		{
			Name: "wellness_correlation",
			Description: "Correlate the athlete's subjective wellness fields against an objective PMC metric over a " +
				"window: for each field (fatigue/soreness/stress/mood/motivation) the Spearman rank correlation with " +
				"that day's `metric` — `tsb` (default; the 'does form feel like something' question) | `ctl` | " +
				"`ramp_rate`. Returns per field `{n, rho}`; a field with fewer than 14 paired days returns " +
				"`{n, reason: 'insufficient_pairs'}` and no rho (early sparse data can't produce a confident number). " +
				"IMPORTANT: this is ASSOCIATION, not causation — confounders (illness, life stress) abound, and rho on " +
				"5-level data saturates, so read direction + rough magnitude, not the third decimal. Read-only; no " +
				"idempotency key. Max 92-day window.",
			SchemaType: WellnessCorrelationArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a WellnessCorrelationArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				if a.Metric != "" {
					q.Set("metric", a.Metric)
				}
				if a.TZ != "" {
					q.Set("tz", a.TZ)
				}
				return HTTPCall{Method: "GET", Path: "/wellness/correlation", Query: q}, nil
			},
		},
	}
}
