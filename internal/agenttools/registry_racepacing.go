package agenttools

import (
	"encoding/json"
	"net/url"
	"strconv"
)

// Race-pacing tools (add-race-pacing-plan): the compute-on-read per-leg pacing
// plan and its persisted per-leg overrides. plan_race_pacing is a pure read;
// set_race_leg_pacing_override is a full-replace PUT (no Idempotency-Key — the
// REST backend rejects it on PUT, and the generic dispatcher skips the header on
// PUT centrally); clear_race_leg_pacing_override is a DELETE with the standard
// derived idempotency key. Both writes are write-confirm (a training-target the
// user should confirm before it is pinned).

func init() { registerMCPDomain(racePacingSpecs()) }

// PlanRacePacingArgs is the input to plan_race_pacing.
type PlanRacePacingArgs struct {
	RaceID  string `json:"race_id" jsonschema:"the race id (uuid) to compute a per-leg pacing plan for"`
	Weather bool   `json:"weather,omitempty" jsonschema:"opt-in: also annotate each leg with a heat-adjusted band from the race-day forecast. Only useful inside ~16 days of the race."`
}

// SetRaceLegPacingOverrideArgs is the input to set_race_leg_pacing_override.
// Exactly one unit family (both low and high) must be populated and must match
// the leg's discipline: power_* for bike, sec_per_km for run, sec_per_100m for
// swim. `idempotency_key` is intentionally NOT exposed (PUT rejects it).
type SetRaceLegPacingOverrideArgs struct {
	RaceID  string `json:"race_id" jsonschema:"the race id (uuid)"`
	Ordinal int    `json:"ordinal" jsonschema:"the leg ordinal to override (1-based, matching the leg's position)"`

	TargetPowerLowW          *int     `json:"target_power_low_w,omitempty" jsonschema:"bike leg: low end of the power band in watts"`
	TargetPowerHighW         *int     `json:"target_power_high_w,omitempty" jsonschema:"bike leg: high end of the power band in watts"`
	TargetPaceLowSecPerKM    *float64 `json:"target_pace_low_sec_per_km,omitempty" jsonschema:"run leg: fast end of the pace band in sec/km"`
	TargetPaceHighSecPerKM   *float64 `json:"target_pace_high_sec_per_km,omitempty" jsonschema:"run leg: slow end of the pace band in sec/km"`
	TargetPaceLowSecPer100m  *float64 `json:"target_pace_low_sec_per_100m,omitempty" jsonschema:"swim leg: fast end of the pace band in sec/100m"`
	TargetPaceHighSecPer100m *float64 `json:"target_pace_high_sec_per_100m,omitempty" jsonschema:"swim leg: slow end of the pace band in sec/100m"`
	Note                     *string  `json:"note,omitempty" jsonschema:"optional free-text note on why this override was set"`
}

// ClearRaceLegPacingOverrideArgs is the input to clear_race_leg_pacing_override.
type ClearRaceLegPacingOverrideArgs struct {
	RaceID         string `json:"race_id" jsonschema:"the race id (uuid)"`
	Ordinal        int    `json:"ordinal" jsonschema:"the leg ordinal whose override to clear"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args"`
}

func racePacingSpecs() []Spec {
	return []Spec{
		{
			Name: "plan_race_pacing",
			Description: "Compute a deterministic per-leg pacing plan for a race from the athlete-config thresholds: " +
				"bike legs get a power band as a duration-banded % of FTP, run legs a pace band vs threshold pace, " +
				"swim legs a pace band per 100 m vs CSS. Each leg reports its band, `source` (computed/override/" +
				"none), `intensity_factor`, `estimated_tss`, and a `rationale`; the race reports " +
				"`estimated_tss_total`, `tss_complete`, and a `missing_thresholds` union. An unset threshold " +
				"degrades only the affected legs (still 200) and names the missing field — tell the user to set it " +
				"in athlete-config rather than guessing. This is a duration-banded baseline, NOT course-specific " +
				"(no gradient/wind/aero); use it to anchor pacing advice instead of estimating watts/pace from " +
				"scratch. " +
				"Pass `weather: true` inside ~16 days of the race to ALSO get a `heat_adjusted` band beside " +
				"each original band, plus a race-level `heat` block (load, acclimatization, resolved location). " +
				"The originals never change — present it as \"plan A if it's cool / plan B if it's hot\", not as " +
				"a replacement. Further out there is no reliable forecast and you'll get " +
				"`heat_reason: \"forecast_out_of_range\"` with the base plan intact; `location_ungeocodable` " +
				"means the race's `location` text doesn't resolve to a place (fix the race, don't guess). " +
				"Read-only.",
			SchemaType: PlanRacePacingArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a PlanRacePacingArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				if a.Weather {
					q.Set("weather", "true")
				}
				return HTTPCall{Method: "GET", Path: "/races/" + url.PathEscape(a.RaceID) + "/pacing-plan", Query: q}, nil
			},
		},
		{
			Name: "set_race_leg_pacing_override",
			Description: "Pin a manual pacing target on one race leg (full-replace), overriding the computed band — " +
				"e.g. 'I'm holding 195–200 W on the bike, not the computed 180–207'. Populate exactly ONE unit " +
				"family matching the leg's discipline: target_power_low_w/high_w for a bike leg, " +
				"target_pace_low/high_sec_per_km for a run leg, target_pace_low/high_sec_per_100m for a swim leg. " +
				"The override is keyed by the leg ordinal and survives race/leg edits. Errors: 400 " +
				"override_discipline_mismatch / override_target_required / override_unit_conflict / " +
				"override_band_invalid, 404 race_not_found / leg_not_found. Retries are not safe (PUT). Confirm the " +
				"numbers with the athlete before pinning.",
			SchemaType: SetRaceLegPacingOverrideArgs{},
			Tier:       TierWriteConfirm,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a SetRaceLegPacingOverrideArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				payload := struct {
					TargetPowerLowW          *int     `json:"target_power_low_w,omitempty"`
					TargetPowerHighW         *int     `json:"target_power_high_w,omitempty"`
					TargetPaceLowSecPerKM    *float64 `json:"target_pace_low_sec_per_km,omitempty"`
					TargetPaceHighSecPerKM   *float64 `json:"target_pace_high_sec_per_km,omitempty"`
					TargetPaceLowSecPer100m  *float64 `json:"target_pace_low_sec_per_100m,omitempty"`
					TargetPaceHighSecPer100m *float64 `json:"target_pace_high_sec_per_100m,omitempty"`
					Note                     *string  `json:"note,omitempty"`
				}{
					TargetPowerLowW:          a.TargetPowerLowW,
					TargetPowerHighW:         a.TargetPowerHighW,
					TargetPaceLowSecPerKM:    a.TargetPaceLowSecPerKM,
					TargetPaceHighSecPerKM:   a.TargetPaceHighSecPerKM,
					TargetPaceLowSecPer100m:  a.TargetPaceLowSecPer100m,
					TargetPaceHighSecPer100m: a.TargetPaceHighSecPer100m,
					Note:                     a.Note,
				}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				// PUT — no Idempotency-Key (dispatcher skips the header on PUT).
				return HTTPCall{
					Method: "PUT",
					Path:   "/races/" + url.PathEscape(a.RaceID) + "/pacing-plan/overrides/" + strconv.Itoa(a.Ordinal),
					Body:   body,
				}, nil
			},
		},
		{
			Name: "clear_race_leg_pacing_override",
			Description: "Remove a race leg's manual pacing override so it reverts to the computed band on the next " +
				"plan read. 404 override_not_found when there was nothing pinned, 404 race_not_found for an unknown " +
				"race.",
			SchemaType: ClearRaceLegPacingOverrideArgs{},
			Tier:       TierWriteConfirm,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ClearRaceLegPacingOverrideArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{
					Method: "DELETE",
					Path:   "/races/" + url.PathEscape(a.RaceID) + "/pacing-plan/overrides/" + strconv.Itoa(a.Ordinal),
				}, nil
			},
		},
	}
}
