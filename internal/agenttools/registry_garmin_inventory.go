package agenttools

import (
	"encoding/json"
	"net/url"
	"strconv"
)

// Garmin inventory reads — singleton/list reference data the desktop coach
// reads for context. Ported from internal/mcpserver onto the shared registry
// (unify-mcp-tool-registry, pilot domain). These are MCP-only (not chat-exposed)
// reads; the descriptions and arg structs are byte-identical to the prior
// bespoke registrations so the announced schema is unchanged.

func init() { registerMCPDomain(garminInventorySpecs()) }

// ListGearArgs is the input to gear_list.
type ListGearArgs struct {
	Retired *bool `json:"retired,omitempty" jsonschema:"optional filter by retirement state (true returns only retired gear, false only active)"`
}

// GetAthleteConfigArgs is the (empty) input to athlete_config_get.
type GetAthleteConfigArgs struct{}

// AthleteConfigHistoryGetArgs is the input to athlete_config_history_get:
// optional inclusive date bounds on the snapshots' effective_from.
type AthleteConfigHistoryGetArgs struct {
	From *string `json:"from,omitempty" jsonschema:"optional inclusive lower bound on effective_from (YYYY-MM-DD)"`
	To   *string `json:"to,omitempty" jsonschema:"optional inclusive upper bound on effective_from (YYYY-MM-DD)"`
}

// AthleteConfigUpdateArgs is the full PUT /athlete-config body. Every field is
// optional; an omitted field is CLEARED on the server (full-replace, PUT not
// PATCH), mirroring the AthleteConfig REST shape. The json tags match the REST
// body verbatim so the Build step can marshal these args directly.
type AthleteConfigUpdateArgs struct {
	FtpWatts                    *int     `json:"ftp_watts,omitempty" jsonschema:"functional threshold power in watts"`
	ThresholdHR                 *int     `json:"threshold_hr,omitempty" jsonschema:"threshold heart rate in bpm"`
	LactateThresholdHR          *int     `json:"lactate_threshold_hr,omitempty" jsonschema:"lactate-threshold heart rate in bpm"`
	MaxHR                       *int     `json:"max_hr,omitempty" jsonschema:"maximum heart rate in bpm"`
	ThresholdPaceSecPerKm       *float64 `json:"threshold_pace_sec_per_km,omitempty" jsonschema:"run threshold pace in seconds per km"`
	ThresholdSwimPaceSecPer100m *float64 `json:"threshold_swim_pace_sec_per_100m,omitempty" jsonschema:"swim threshold pace in seconds per 100m"`

	HRZone1Max *int `json:"hr_zone_1_max,omitempty" jsonschema:"upper HR bound of zone 1 (bpm)"`
	HRZone2Max *int `json:"hr_zone_2_max,omitempty" jsonschema:"upper HR bound of zone 2 (bpm)"`
	HRZone3Max *int `json:"hr_zone_3_max,omitempty" jsonschema:"upper HR bound of zone 3 (bpm)"`
	HRZone4Max *int `json:"hr_zone_4_max,omitempty" jsonschema:"upper HR bound of zone 4 (bpm)"`
	HRZone5Max *int `json:"hr_zone_5_max,omitempty" jsonschema:"upper HR bound of zone 5 (bpm)"`

	PowerZone1Max *int `json:"power_zone_1_max,omitempty" jsonschema:"upper power bound of zone 1 (watts)"`
	PowerZone2Max *int `json:"power_zone_2_max,omitempty" jsonschema:"upper power bound of zone 2 (watts)"`
	PowerZone3Max *int `json:"power_zone_3_max,omitempty" jsonschema:"upper power bound of zone 3 (watts)"`
	PowerZone4Max *int `json:"power_zone_4_max,omitempty" jsonschema:"upper power bound of zone 4 (watts)"`
	PowerZone5Max *int `json:"power_zone_5_max,omitempty" jsonschema:"upper power bound of zone 5 (watts)"`
}

// ListPersonalRecordsArgs is the input to personal_records_list.
type ListPersonalRecordsArgs struct {
	PRType string `json:"pr_type,omitempty" jsonschema:"optional filter to a single PR type (e.g. 5k, 10k, longest-ride)"`
}

// SetThresholdSourcesArgs is the PUT /athlete-config/sources body: the full
// replacement list of garmin-sourced field tokens.
type SetThresholdSourcesArgs struct {
	Sources []string `json:"sources" jsonschema:"full replacement list of garmin-sourced fields; whitelisted tokens: ftp_watts, lactate_threshold_hr, max_hr, threshold_pace_sec_per_km, hr_zones, power_zones. Empty = all manual."`
}

func garminInventorySpecs() []Spec {
	return []Spec{
		{
			Name: "gear_list",
			Description: "List the athlete's Garmin gear inventory (shoes, bikes, other equipment) with " +
				"accumulated distance, activity count, and retirement state. Use for gear-rotation context — " +
				"e.g. flagging shoes that are near or past their mileage budget. Optional `retired` filter.",
			SchemaType: ListGearArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ListGearArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				if a.Retired != nil {
					q.Set("retired", strconv.FormatBool(*a.Retired))
				}
				return HTTPCall{Method: "GET", Path: "/gear", Query: q}, nil
			},
		},
		{
			Name: "athlete_config_get",
			Description: "Fetch the athlete's physiology configuration (singleton): FTP, threshold HR and " +
				"run/swim paces, max HR, lactate-threshold HR, and HR-zone (and optional power-zone) " +
				"boundaries. Returns null before any config has been set. Use to interpret workout " +
				"detail — e.g. to know what heart rate a zone-4 second actually corresponds to.",
			SchemaType: GetAthleteConfigArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				return HTTPCall{Method: "GET", Path: "/athlete-config"}, nil
			},
		},
		{
			Name: "athlete_config_history_get",
			Description: "Fetch the DATED history of the athlete's physiology configuration — the snapshots " +
				"recorded each time a PUT changed FTP / thresholds / zone boundaries — ascending by " +
				"effective_from. Use this to answer 'how has my FTP developed this season' from data instead " +
				"of memory. A snapshot exists only per change (the daily Garmin re-PUT of an unchanged config " +
				"records nothing); the seed baseline is dated 1970-01-01 (the oldest known state). Optional " +
				"from/to (YYYY-MM-DD) filter inclusively on effective_from. Empty history returns an empty " +
				"list, not an error. Read-only.",
			SchemaType: AthleteConfigHistoryGetArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a AthleteConfigHistoryGetArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				if a.From != nil {
					q.Set("from", *a.From)
				}
				if a.To != nil {
					q.Set("to", *a.To)
				}
				return HTTPCall{Method: "GET", Path: "/athlete-config/history", Query: q}, nil
			},
		},
		{
			Name: "athlete_config_update",
			Description: "Set or replace the athlete's physiology configuration (FTP, threshold HR and " +
				"run/swim paces, max HR, lactate-threshold HR, and HR/power zone boundaries). Full-replace " +
				"semantics: a field omitted from the call is CLEARED on the server (PUT, not PATCH) — send " +
				"the complete desired config every time, not just the field you're changing. These values " +
				"drive workout-target resolution pushed to the watch, so wrong FTP/zone numbers silently " +
				"corrupt every subsequent prescribed session. Retries are NOT safe (the backend rejects " +
				"Idempotency-Key on PUT); re-issue only if you're sure the previous call didn't land.",
			SchemaType: AthleteConfigUpdateArgs{},
			Tier:       TierWriteConfirm,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var args AthleteConfigUpdateArgs
				if err := DecodeInto(in, &args); err != nil {
					return HTTPCall{}, err
				}
				// The args json tags mirror the REST body exactly; marshal
				// directly. Nil fields are omitted (omitempty) → full-replace
				// clear-on-omit. No Idempotency-Key: PUT /athlete-config rejects
				// it (400 idempotency_unsupported_for_put) and the generic
				// dispatcher already skips the header on PUT.
				body, err := json.Marshal(args)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "PUT", Path: "/athlete-config", Body: body}, nil
			},
		},
		{
			Name: "set_threshold_sources",
			Description: "Set the per-field source policy — which physiology fields the app's computations " +
				"read from Garmin's DETECTED values instead of the deliberately-confirmed athlete config. " +
				"Full-replace list (send the complete desired set every time; empty = all manual). " +
				"Whitelisted tokens: `ftp_watts`, `lactate_threshold_hr`, `max_hr`, " +
				"`threshold_pace_sec_per_km`, and the zone groups `hr_zones` / `power_zones` (zones flip " +
				"as whole sets). This changes ONLY the policy — it never overwrites a confirmed value and " +
				"never writes threshold history. Flipping a source changes the thresholds computations " +
				"use, so when derived TSS should follow the flip, run `recompute_workout_tss` afterward. " +
				"Retries are NOT safe (the backend rejects Idempotency-Key on PUT).",
			SchemaType: SetThresholdSourcesArgs{},
			Tier:       TierWriteConfirm,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var args SetThresholdSourcesArgs
				if err := DecodeInto(in, &args); err != nil {
					return HTTPCall{}, err
				}
				if args.Sources == nil {
					args.Sources = []string{}
				}
				body, err := json.Marshal(args)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "PUT", Path: "/athlete-config/sources", Body: body}, nil
			},
		},
		{
			Name: "personal_records_list",
			Description: "List the athlete's Garmin personal records (fastest 5k/10k, longest ride, …) with " +
				"value, unit, and when each was achieved, most recent first. Use for PR-freshness coaching " +
				"context — e.g. framing race-prep advice around how sharp the athlete's top-end is. Optional " +
				"`pr_type` filter.",
			SchemaType: ListPersonalRecordsArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ListPersonalRecordsArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				if a.PRType != "" {
					q.Set("pr_type", a.PRType)
				}
				return HTTPCall{Method: "GET", Path: "/personal-records", Query: q}, nil
			},
		},
	}
}
