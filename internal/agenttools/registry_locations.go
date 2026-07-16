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
	StartDate string `json:"start_date" jsonschema:"first day at this location, YYYY-MM-DD (inclusive)"`
	EndDate   string `json:"end_date" jsonschema:"last day at this location, YYYY-MM-DD (inclusive)"`
	// Place geocodes server-side; lat/lon are the explicit alternative. Supply
	// one or the other — see the tool description.
	Place          string   `json:"place,omitempty" jsonschema:"place name to geocode server-side, e.g. 'Mallorca'. Use this OR lat+lon. Also names the period when name is omitted."`
	Name           string   `json:"name,omitempty" jsonschema:"optional label for the period, e.g. 'Sierra Nevada camp'. Required when passing lat/lon instead of place; with place, defaults to the geocoded name."`
	Lat            *float64 `json:"lat,omitempty" jsonschema:"latitude in [-90, 90]; city-grade precision is enough. Pass with lon instead of place."`
	Lon            *float64 `json:"lon,omitempty" jsonschema:"longitude in [-180, 180]; city-grade precision is enough. Pass with lat instead of place."`
	Note           *string  `json:"note,omitempty" jsonschema:"optional free-text note, e.g. 'altitude camp, 2320 m'"`
	IdempotencyKey string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args"`
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
				"Pass `place` and the server geocodes it (simplest, and it names the period for you); or " +
				"pass explicit `lat`+`lon`+`name` when you know the coordinates or the place name is " +
				"ambiguous. Explicit coordinates win if you send both. City-grade precision is all a " +
				"forecast needs — say which place resolved, so a wrong city is visible to the athlete " +
				"rather than buried in a forecast. An unknown `place` is rejected rather than stored " +
				"without coordinates. " +
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
					Name      string   `json:"name,omitempty"`
					StartDate string   `json:"start_date"`
					EndDate   string   `json:"end_date"`
					Place     string   `json:"place,omitempty"`
					Lat       *float64 `json:"lat,omitempty"`
					Lon       *float64 `json:"lon,omitempty"`
					Note      *string  `json:"note,omitempty"`
				}{a.Name, a.StartDate, a.EndDate, a.Place, a.Lat, a.Lon, a.Note}
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
