// Package wellness stores the athlete's subjective daily wellness log — five
// optional self-reported 1-5 scores plus a free-text note, one entry per date.
// It is the self-reported counterpart to the Garmin-fed objective recovery
// picture (HRV/sleep/RHR/readiness): the coach collects it in conversation and
// holds it against the device data ("TSB says fresh, legs say otherwise"). A
// per-date singleton with PUT full-replace upsert (the goals-overrides pattern).
package wellness

import (
	"context"
	"time"
)

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

// PMCDayValue is one day's PMC metrics — the narrow slice the correlation read
// needs from the performance-management capability.
type PMCDayValue struct {
	Date     string // YYYY-MM-DD (the PMC's tz bucketing)
	TSB      float64
	CTL      float64
	RampRate float64
}

// PMCProvider supplies the daily PMC series for correlation. Implemented by the
// pmc service, injected at wiring so internal/wellness carries no pmc import
// (the cross-injection precedent) — no package cycle, no duplicate EWMA.
type PMCProvider interface {
	PMCValues(ctx context.Context, from, to time.Time, loc *time.Location) ([]PMCDayValue, error)
}

// FieldCorrelation is one wellness field's association with the chosen PMC
// metric over the window. Rho is present only when n meets the minimum; below it
// Reason is "insufficient_pairs" (visible progress toward usefulness).
type FieldCorrelation struct {
	N      int      `json:"n"`
	Rho    *float64 `json:"rho,omitempty"`
	Reason string   `json:"reason,omitempty"`
}

// CorrelationResult is the GET /wellness/correlation response: per wellness field
// its Spearman rank correlation with the metric. Compute-on-read; nothing
// persisted.
type CorrelationResult struct {
	Metric string                      `json:"metric"`
	From   string                      `json:"from"`
	To     string                      `json:"to"`
	Fields map[string]FieldCorrelation `json:"fields"`
}
