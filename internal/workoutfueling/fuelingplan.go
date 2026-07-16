package workoutfueling

import (
	"context"
	"errors"
	"math"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/athleteconfig"
	"github.com/vinzenzs/kazper/internal/numfmt"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// Fueling-plan sentinels — mapped 1:1 to API error codes by the handler.
var (
	ErrWorkoutNotPlanned = errors.New("workout is not planned")
	ErrCarbsPerHrInvalid = errors.New("carbs_per_hr must be > 0 and <= 130")
)

// Degradation reasons. Every partial answer states what it lacks rather than
// filling the gap with a guess (design D5, the race-pacing posture).
const (
	// ReasonPlanDataMissing: neither planned TSS nor a duration — nothing is
	// computable, not even the intake ladder. Defense-in-depth at the HTTP
	// layer: the workouts table's `CHECK (ended_at > started_at)` means a stored
	// workout always has a duration, so this cannot currently fire through the
	// endpoint (tss_missing is the reachable floor). Kept because the pure math
	// must not depend on that constraint holding forever.
	ReasonPlanDataMissing = "plan_data_missing"
	// ReasonTSSMissing: a duration but no planned TSS — the intake ladder still
	// applies; burn does not.
	ReasonTSSMissing = "tss_missing"
	// ReasonFTPMissing: planned TSS but no effective FTP — same shape; TSS alone
	// can't be turned into work.
	ReasonFTPMissing = "ftp_missing"
)

// Conversion constants and the two ladders. Constants v1, echoed in the
// response with the input that selected them (design D2/D3).
const (
	// kjPerTSSHourAtFTP: TSS 100 ≈ one hour at FTP ≈ FTP × 3600 s ≈ FTP × 3.6 kJ.
	kjPerFTPHour = 3.6

	// kcalPerKJ: the standard cycling convention kJ_work ≈ kcal, since gross
	// efficiency (~24%) and the 4.184 kJ/kcal conversion very nearly cancel.
	// A planning anchor, not a metabolic measurement.
	kcalPerKJ = 1.0

	kcalPerCarbG = 4.0

	// MaxCarbsPerHr bounds the capacity param. Above this is not a gut, it's a
	// typo.
	MaxCarbsPerHr = 130.0
)

// CHO fraction ladder over planned IF — the crossover concept: the harder the
// effort, the more of it comes from carbohydrate rather than fat.
const (
	ifEasyMax     = 0.60 // < 0.60         → 45%
	ifModerateMax = 0.75 // 0.60 .. 0.75   → 55%
	ifHardMax     = 0.85 // 0.75 .. 0.85   → 70%; > 0.85 → 80%

	choFracEasy     = 0.45
	choFracModerate = 0.55
	choFracHard     = 0.70
	choFracVeryHard = 0.80
)

// Intake duration ladder (g/hr) — standard sports-nutrition guidance.
const (
	shortSessionMin  = 60.0  // < 60 min  → nothing
	mediumSessionMin = 150.0 // 60..150   → 30–60; > 150 → 60–90

	mediumIntakeMinGPerHr = 30.0
	mediumIntakeMaxGPerHr = 60.0
	longIntakeMinGPerHr   = 60.0
	longIntakeMaxGPerHr   = 90.0
)

// ConfigReader reads the EFFECTIVE athlete config (garmin-sourced when the
// athlete chose so). Mirrors racepacing.ConfigRepo — same adapter satisfies
// both. Optional: a nil reader degrades the plan to ftp_missing, never an
// error (fail-open, like every other config consumer).
type ConfigReader interface {
	Get(ctx context.Context) (*athleteconfig.AthleteConfig, error)
}

// SetConfigReader enables the FTP-derived burn estimate.
func (s *Service) SetConfigReader(r ConfigReader) { s.config = r }

// FuelingPlanInputs echoes everything the numbers were derived from, so a
// surprising plan is auditable against its inputs rather than argued with.
type FuelingPlanInputs struct {
	PlannedTSS      *float64 `json:"planned_tss,omitempty"`
	DurationMin     *float64 `json:"duration_min,omitempty"`
	PlannedIF       *float64 `json:"planned_if,omitempty"`
	FTPWatts        *int     `json:"ftp_watts,omitempty"`
	CHOFraction     *float64 `json:"cho_fraction,omitempty"`
	CarbsPerHrLimit *float64 `json:"carbs_per_hr_limit,omitempty"`
}

// Prescription is the intake plan: a per-hour range and its session totals.
// The range collapses to 0–0 for a session too short to need fuelling.
type Prescription struct {
	PerHourMinG      float64 `json:"per_hour_min_g"`
	PerHourMaxG      float64 `json:"per_hour_max_g"`
	SessionTotalMinG float64 `json:"session_total_min_g"`
	SessionTotalMaxG float64 `json:"session_total_max_g"`
}

// FuelingPlan is the response shape for GET /workouts/{id}/fueling-plan.
// Unit-isolated: grams and kJ/kcal of the SESSION — it feeds no daily
// nutrition total (the plan is intent; workout-fuel entries are the log).
type FuelingPlan struct {
	WorkoutID uuid.UUID         `json:"workout_id"`
	Inputs    FuelingPlanInputs `json:"inputs"`

	EstimatedKJ        *float64      `json:"estimated_kj,omitempty"`
	EstimatedKcal      *float64      `json:"estimated_kcal,omitempty"`
	EstimatedCarbBurnG *float64      `json:"estimated_carb_burn_g,omitempty"`
	Prescription       *Prescription `json:"prescription,omitempty"`
	ProjectedDeficitG  *float64      `json:"projected_deficit_g,omitempty"`

	Reason *string `json:"reason,omitempty"`
}

// choFraction selects the substrate ladder rung for a planned IF.
func choFraction(intensity float64) float64 {
	switch {
	case intensity > ifHardMax:
		return choFracVeryHard
	case intensity > ifModerateMax:
		return choFracHard
	case intensity >= ifEasyMax:
		return choFracModerate
	default:
		return choFracEasy
	}
}

// plannedIF derives intensity from planned TSS and duration:
// TSS = IF² × hours × 100, so IF = sqrt(TSS/100 / hours).
func plannedIF(tss, hours float64) float64 {
	if hours <= 0 {
		return 0
	}
	return math.Sqrt(tss / 100 / hours)
}

// prescribe applies the duration ladder, clamped by the athlete's tested gut
// capacity when supplied. The clamp caps the ceiling AND pulls the floor down
// with it — prescribing a 60 g/hr minimum to someone who tops out at 50 would
// be advice they cannot follow.
func prescribe(durationMin float64, capacity *float64) Prescription {
	var minG, maxG float64
	switch {
	case durationMin < shortSessionMin:
		minG, maxG = 0, 0
	case durationMin <= mediumSessionMin:
		minG, maxG = mediumIntakeMinGPerHr, mediumIntakeMaxGPerHr
	default:
		minG, maxG = longIntakeMinGPerHr, longIntakeMaxGPerHr
	}

	if capacity != nil && maxG > 0 {
		if *capacity < maxG {
			maxG = *capacity
		}
		if minG > maxG {
			minG = maxG
		}
	}

	hours := durationMin / 60
	return Prescription{
		PerHourMinG:      numfmt.Round1(minG),
		PerHourMaxG:      numfmt.Round1(maxG),
		SessionTotalMinG: numfmt.Round1(minG * hours),
		SessionTotalMaxG: numfmt.Round1(maxG * hours),
	}
}

// buildFuelingPlan is the pure computation over resolved inputs: planned TSS,
// duration, and effective FTP (each optional), plus the capacity clamp.
func buildFuelingPlan(id uuid.UUID, tss *float64, durationMin float64, ftp *int, capacity *float64) *FuelingPlan {
	out := &FuelingPlan{WorkoutID: id, Inputs: FuelingPlanInputs{CarbsPerHrLimit: capacity}}
	if durationMin > 0 {
		d := numfmt.Round1(durationMin)
		out.Inputs.DurationMin = &d
	}
	if tss != nil {
		t := numfmt.Round1(*tss)
		out.Inputs.PlannedTSS = &t
	}
	if ftp != nil {
		out.Inputs.FTPWatts = ftp
	}

	// Nothing to plan from: no load estimate and no duration.
	if tss == nil && durationMin <= 0 {
		reason := ReasonPlanDataMissing
		out.Reason = &reason
		return out
	}

	// The intake ladder needs only duration, so it survives every degradation
	// below — a plan that can still say "take 60–90 g/hr" is worth returning.
	if durationMin > 0 {
		pr := prescribe(durationMin, capacity)
		out.Prescription = &pr
	}

	switch {
	case tss == nil:
		reason := ReasonTSSMissing
		out.Reason = &reason
		return out
	case ftp == nil:
		reason := ReasonFTPMissing
		out.Reason = &reason
		return out
	}

	hours := durationMin / 60
	kj := *tss / 100 * float64(*ftp) * kjPerFTPHour
	kcal := kj * kcalPerKJ

	intensity := plannedIF(*tss, hours)
	frac := choFraction(intensity)
	burnG := kcal * frac / kcalPerCarbG

	roundedKJ := numfmt.Round1(kj)
	roundedKcal := numfmt.Round1(kcal)
	roundedBurn := numfmt.Round1(burnG)
	roundedIF := numfmt.Round2(intensity)
	roundedFrac := frac

	out.EstimatedKJ = &roundedKJ
	out.EstimatedKcal = &roundedKcal
	out.EstimatedCarbBurnG = &roundedBurn
	out.Inputs.PlannedIF = &roundedIF
	out.Inputs.CHOFraction = &roundedFrac

	// The deficit is what the ride burns beyond the most it prescribes taking
	// in — the number that says whether post-ride carbs need emphasis. Computed
	// against unrounded values and rounded once, at the boundary.
	if out.Prescription != nil {
		maxIntake := 0.0
		if durationMin > 0 {
			maxIntake = out.Prescription.PerHourMaxG * hours
		}
		deficit := numfmt.Round1(burnG - maxIntake)
		out.ProjectedDeficitG = &deficit
	}
	return out
}

// FuelingPlanFor computes a PLANNED workout's fueling plan. Persists nothing;
// feeds no daily total.
func (s *Service) FuelingPlanFor(ctx context.Context, id uuid.UUID, capacity *float64) (*FuelingPlan, error) {
	if capacity != nil && (*capacity <= 0 || *capacity > MaxCarbsPerHr) {
		return nil, ErrCarbsPerHrInvalid
	}

	w, err := s.workouts.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// The pre-session question is the product: a completed ride's retrospective
	// is a different question, deferred with compliance scoring (design D4).
	if w.Status != workouts.StatusPlanned {
		return nil, ErrWorkoutNotPlanned
	}

	ftp := s.effectiveFTP(ctx)
	durationMin := w.EndedAt.Sub(w.StartedAt).Minutes()
	if durationMin < 0 {
		durationMin = 0
	}
	return buildFuelingPlan(w.ID, w.TSS, durationMin, ftp, capacity), nil
}

// effectiveFTP resolves the effective config's FTP, fail-open: an unwired
// reader, a read error, or an unset value all degrade the plan to
// ftp_missing rather than failing it — an unset threshold has never failed a
// caller in this codebase and must not start here.
func (s *Service) effectiveFTP(ctx context.Context) *int {
	if s.config == nil {
		return nil
	}
	cfg, err := s.config.Get(ctx)
	if err != nil || cfg == nil {
		return nil
	}
	return cfg.FtpWatts
}
