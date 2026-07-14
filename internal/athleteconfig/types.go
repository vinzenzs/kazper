// Package athleteconfig stores the athlete's slowly-changing physiology
// configuration (FTP, threshold HR/paces, max HR, lactate-threshold HR, and
// HR-zone / power-zone boundaries) as a singleton row, modeled on the
// nutrition_goals singleton. It is CAPTURE ONLY: nothing in this package's
// change consumes these values, and they are unit-isolated — never merged into
// summary totals or any fueling-math input.
package athleteconfig

import "time"

// AthleteConfig is the singleton athlete_config row. Every field is a nullable
// pointer so absent stays distinct from a real zero; the JSON marshaller omits
// the empty ones so callers see only populated fields.
type AthleteConfig struct {
	FtpWatts                    *int     `json:"ftp_watts,omitempty"`
	ThresholdHR                 *int     `json:"threshold_hr,omitempty"`
	LactateThresholdHR          *int     `json:"lactate_threshold_hr,omitempty"`
	MaxHR                       *int     `json:"max_hr,omitempty"`
	ThresholdPaceSecPerKm       *float64 `json:"threshold_pace_sec_per_km,omitempty"`
	ThresholdSwimPaceSecPer100m *float64 `json:"threshold_swim_pace_sec_per_100m,omitempty"`

	HRZone1Max *int `json:"hr_zone_1_max,omitempty"`
	HRZone2Max *int `json:"hr_zone_2_max,omitempty"`
	HRZone3Max *int `json:"hr_zone_3_max,omitempty"`
	HRZone4Max *int `json:"hr_zone_4_max,omitempty"`
	HRZone5Max *int `json:"hr_zone_5_max,omitempty"`

	PowerZone1Max *int `json:"power_zone_1_max,omitempty"`
	PowerZone2Max *int `json:"power_zone_2_max,omitempty"`
	PowerZone3Max *int `json:"power_zone_3_max,omitempty"`
	PowerZone4Max *int `json:"power_zone_4_max,omitempty"`
	PowerZone5Max *int `json:"power_zone_5_max,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ThresholdSnapshot is one dated row of athlete_config_history — the full
// physiology state (the 16 fields, embedded and inlined into the JSON) that was
// in effect from EffectiveFrom onward. CreatedAt/UpdatedAt on the embedded config
// carry the history ROW's timestamps (when the snapshot was written/replaced),
// not the singleton's. The seed baseline uses effective_from 1970-01-01.
type ThresholdSnapshot struct {
	EffectiveFrom string `json:"effective_from"` // YYYY-MM-DD
	AthleteConfig
}

const dateLayout = "2006-01-02"
