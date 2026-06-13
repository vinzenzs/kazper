// Package healthvitals stores daily health vitals — blood pressure, all-day
// resting/min/max heart rate, and all-day average/max stress — one snapshot per
// calendar date (upsert by date), distinct from recovery-metrics. Reference/
// coaching context, unit-isolated: never merged into nutrition/recovery totals.
package healthvitals

import "time"

// Snapshot mirrors a health_vitals row. Date is the identity (YYYY-MM-DD). Every
// metric is a nullable pointer so absent stays distinct from a real zero.
type Snapshot struct {
	Date        string    `json:"date"`
	BPSystolic  *int      `json:"bp_systolic,omitempty"`
	BPDiastolic *int      `json:"bp_diastolic,omitempty"`
	BPPulse     *int      `json:"bp_pulse,omitempty"`
	RestingHR   *int      `json:"resting_hr,omitempty"`
	MinHR       *int      `json:"min_hr,omitempty"`
	MaxHR       *int      `json:"max_hr,omitempty"`
	StressAvg   *int      `json:"stress_avg,omitempty"`
	StressMax   *int      `json:"stress_max,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
