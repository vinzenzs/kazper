// Package wellness stores the athlete's subjective daily wellness log — five
// optional self-reported 1-5 scores plus a free-text note, one entry per date.
// It is the self-reported counterpart to the Garmin-fed objective recovery
// picture (HRV/sleep/RHR/readiness): the coach collects it in conversation and
// holds it against the device data ("TSB says fresh, legs say otherwise"). A
// per-date singleton with PUT full-replace upsert (the goals-overrides pattern).
package wellness

import "time"

// Entry mirrors a wellness_entries row. Every score is nullable — absent means
// "not reported", never defaulted — and serializes with omitempty. Symptom-like
// fields (fatigue/soreness/stress) read 1 = none → 5 = severe; state-like fields
// (mood/motivation) read 1 = low → 5 = high, each keeping its natural direction.
type Entry struct {
	Date       string    `json:"date"`
	Fatigue    *int      `json:"fatigue,omitempty"`
	Soreness   *int      `json:"soreness,omitempty"`
	Stress     *int      `json:"stress,omitempty"`
	Mood       *int      `json:"mood,omitempty"`
	Motivation *int      `json:"motivation,omitempty"`
	Note       *string   `json:"note,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
