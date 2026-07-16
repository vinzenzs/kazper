package agenttools

import (
	"encoding/json"
	"net/url"
)

// Location-period tools — the travel layer behind every weather/heat read.
// log_location_period is a create (Idempotency-Key auto-derived by the
// dispatcher); list_location_periods is a window read. One HTTP call each, per
// the REST↔MCP 1:1 convention.

func init() { registerMCPDomain(locationSpecs()) }

// LogLocationPeriodArgs is the input to log_location_period.
type LogLocationPeriodArgs struct {
	Name           string  `json:"name" jsonschema:"place name as the athlete would say it, e.g. 'Mallorca', 'Sierra Nevada camp' (required)"`
	StartDate      string  `json:"start_date" jsonschema:"first day at this location, YYYY-MM-DD (inclusive)"`
	EndDate        string  `json:"end_date" jsonschema:"last day at this location, YYYY-MM-DD (inclusive)"`
	Lat            float64 `json:"lat" jsonschema:"latitude in [-90, 90]; city-grade precision is enough"`
	Lon            float64 `json:"lon" jsonschema:"longitude in [-180, 180]; city-grade precision is enough"`
	Note           *string `json:"note,omitempty" jsonschema:"optional free-text note, e.g. 'altitude camp, 2320 m'"`
	IdempotencyKey string  `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args"`
}

// ListLocationPeriodsArgs is the input to list_location_periods.
type ListLocationPeriodsArgs struct {
	From string `json:"from" jsonschema:"inclusive start date YYYY-MM-DD"`
	To   string `json:"to" jsonschema:"inclusive end date YYYY-MM-DD; max 400-day span"`
}

func locationSpecs() []Spec {
	return []Spec{
		{
			Name: "log_location_period",
			Description: "Record where the athlete is over a date range, so weather and heat reads for " +
				"those days resolve to that place instead of home. Use it whenever the athlete mentions " +
				"travel — \"I'm in Mallorca July 20–28\", a training camp, a race trip. " +
				"YOU supply the coordinates from your own knowledge of the place; city-grade precision is " +
				"all a forecast needs. Say which city you used, so a wrong guess is visible to the athlete " +
				"rather than buried in a forecast. " +
				"Home does NOT go here — it is server configuration (HOME_LAT/HOME_LON). This is the travel " +
				"layer only; moving house is a config change, not a log entry. " +
				"Overlapping periods are fine: a weekend trip logged inside a training camp wins for its " +
				"own days (the later-starting period resolves). " +
				"Nothing downstream is precomputed, so sessions already scheduled in the range follow the " +
				"trip the moment you log it — do not touch the workouts. " +
				"There is NO edit: to fix or extend a trip, delete the period and log it again.",
			SchemaType: LogLocationPeriodArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a LogLocationPeriodArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				payload := struct {
					Name      string  `json:"name"`
					StartDate string  `json:"start_date"`
					EndDate   string  `json:"end_date"`
					Lat       float64 `json:"lat"`
					Lon       float64 `json:"lon"`
					Note      *string `json:"note,omitempty"`
				}{a.Name, a.StartDate, a.EndDate, a.Lat, a.Lon, a.Note}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/locations", Body: body}, nil
			},
		},
		{
			Name: "list_location_periods",
			Description: "List the athlete's logged travel periods overlapping a date window, ascending by " +
				"start date. Returns periods that merely intersect the window, not only those fully inside " +
				"it. Days with no covering period are spent at the configured home location — they simply " +
				"have no row here, which is why an empty list means \"at home\", not \"unknown\". " +
				"Read-only; no idempotency-key is sent.",
			SchemaType: ListLocationPeriodsArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ListLocationPeriodsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				return HTTPCall{Method: "GET", Path: "/locations", Query: q}, nil
			},
		},
	}
}
