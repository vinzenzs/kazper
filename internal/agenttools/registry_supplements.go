package agenttools

import (
	"encoding/json"
	"net/url"
)

// Supplement-log tools — a dated log of supplement intakes (creatine, iron,
// vitamin D, out-of-session caffeine). Distinct from meals (no macros) and
// workout-fuel (in-session). log_supplement is a create (Idempotency-Key
// auto-derived by the dispatcher); list_supplements is a window read. One HTTP
// call each, per the REST↔MCP 1:1 convention. Supplements feed no macro total.

func init() { registerMCPDomain(supplementsSpecs()) }

// LogSupplementArgs is the input to log_supplement.
type LogSupplementArgs struct {
	Name           string   `json:"name" jsonschema:"the supplement name, e.g. 'creatine', 'vitamin D' (required)"`
	LoggedAt       string   `json:"logged_at" jsonschema:"when it was taken, RFC 3339 timestamp"`
	Dose           *float64 `json:"dose,omitempty" jsonschema:"optional amount (> 0); MUST be paired with dose_unit"`
	DoseUnit       *string  `json:"dose_unit,omitempty" jsonschema:"optional unit for dose, e.g. 'g', 'mg', 'IU'; MUST be paired with dose"`
	Note           *string  `json:"note,omitempty" jsonschema:"optional free-text note"`
	IdempotencyKey string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args"`
}

// ListSupplementsArgs is the input to list_supplements.
type ListSupplementsArgs struct {
	From string `json:"from" jsonschema:"inclusive RFC 3339 lower bound"`
	To   string `json:"to" jsonschema:"inclusive RFC 3339 upper bound; max 92 days from from"`
}

func supplementsSpecs() []Spec {
	return []Spec{
		{
			Name: "log_supplement",
			Description: "Record a supplement intake (creatine, iron, vitamin D, magnesium, out-of-session caffeine " +
				"tablets). `name` is required; `dose` + `dose_unit` are paired (pass both or neither); `note` is " +
				"optional. Supplements are their own log — NOT meals (no macros) and NOT workout-fuel (in-session) — " +
				"and feed no nutrition total. There is no edit: to correct an entry, delete it and log again. Multiple " +
				"per day are fine.",
			SchemaType: LogSupplementArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a LogSupplementArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				payload := struct {
					Name     string   `json:"name"`
					LoggedAt string   `json:"logged_at"`
					Dose     *float64 `json:"dose,omitempty"`
					DoseUnit *string  `json:"dose_unit,omitempty"`
					Note     *string  `json:"note,omitempty"`
				}{a.Name, a.LoggedAt, a.Dose, a.DoseUnit, a.Note}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/supplements", Body: body}, nil
			},
		},
		{
			Name: "list_supplements",
			Description: "List the athlete's supplement intakes over a time window [from, to] (inclusive, max 92 " +
				"days), ascending by logged_at. Use this to check protocol adherence — 'did the iron hold through the " +
				"block?'. Entries carry name, optional dose/dose_unit, and note.",
			SchemaType: ListSupplementsArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ListSupplementsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				return HTTPCall{Method: "GET", Path: "/supplements", Query: q}, nil
			},
		},
	}
}
