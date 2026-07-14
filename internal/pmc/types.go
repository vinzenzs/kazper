// Package pmc computes the classic Coggan Performance Management Chart —
// CTL (fitness), ATL (fatigue), TSB (form) — as a compute-on-read daily series
// over stored completed-workout TSS, plus ramp-rate safety flags. No tables, no
// persistence (same posture as energy-availability): one aggregate query + a
// linear EWMA pass. See openspec/specs/performance-management.
package pmc

// Series is the windowed PMC response.
type Series struct {
	From               string      `json:"from"`
	To                 string      `json:"to"`
	TZ                 string      `json:"tz"`
	SeedDate           *string     `json:"seed_date,omitempty"`
	Days               []Day       `json:"days"`
	RampAlerts         []RampAlert `json:"ramp_alerts"`
	MissingTSSWorkouts int         `json:"missing_tss_workouts"`
}

// Day is one calendar day's PMC values.
type Day struct {
	Date            string  `json:"date"`
	TSSTotal        float64 `json:"tss_total"`
	CTL             float64 `json:"ctl"`
	ATL             float64 `json:"atl"`
	TSB             float64 `json:"tsb"`
	RampRate        float64 `json:"ramp_rate"`
	MissingTSSCount int     `json:"missing_tss_count,omitempty"`
}

// RampAlert flags a Monday-start week whose CTL rose faster than the safe ceiling.
type RampAlert struct {
	WeekStart string  `json:"week_start"`
	CTLStart  float64 `json:"ctl_start"`
	CTLEnd    float64 `json:"ctl_end"`
	CTLDelta  float64 `json:"ctl_delta"`
}

const (
	// Classic Coggan time constants (days) and the published safe-ramp ceiling
	// (CTL/week). Constants, not parameters — comparability is the point of a PMC.
	ctlTimeConstantDays = 42
	atlTimeConstantDays = 7
	rampAlertThreshold  = 8.0
	maxWindowDays       = 400
)
