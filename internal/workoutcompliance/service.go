package workoutcompliance

import (
	"context"
	"errors"
	"math"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/trainingplan"
	"github.com/vinzenzs/kazper/internal/workouts"
	"github.com/vinzenzs/kazper/internal/workouttemplates"
)

// Sentinel errors mapping 1:1 to API error codes. The multisport check precedes
// the template-link check so a brick reports multisport_unsupported, not
// no_template_link.
var (
	ErrNotCompleted          = errors.New("workout not completed")   // workout_not_completed
	ErrMultisportUnsupported = errors.New("multisport unsupported")  // multisport_unsupported
	ErrNoTemplateLink        = errors.New("no template link")        // no_template_link
	ErrSplitsMissing         = errors.New("splits missing")          // splits_missing
)

// Tolerance constants (judgment calls, exercised by table-driven tests). Tuning
// any of these changes no response shapes.
const (
	// targetZeroPct is the deviation past the band edge at which a target score
	// hits 0 (linear falloff from 100 at the edge).
	targetZeroPct = 0.25
	// durationBandPct is the ±ratio tolerance inside which a duration scores 100.
	durationBandPct = 0.10
	// durationZeroPct is the ±ratio deviation at which a duration score hits 0.
	durationZeroPct = 0.25
	// targetWeight/durationWeight combine the two dimensions into a step score.
	targetWeight   = 0.7
	durationWeight = 0.3
)

// WorkoutsRepo loads a workout together with its splits (single-get shape).
type WorkoutsRepo interface {
	GetByIDWithChildren(ctx context.Context, id uuid.UUID) (*workouts.Workout, error)
}

// ProgramProvider resolves a workout's effective program (template steps + slot
// overrides + athlete-config zone→absolute pass). Satisfied by *trainingplan.Service.
type ProgramProvider interface {
	EffectiveProgram(ctx context.Context, workoutID uuid.UUID) (*trainingplan.Program, error)
}

// Service computes per-step compliance on read. It owns no state beyond its two
// injected dependencies.
type Service struct {
	workouts WorkoutsRepo
	program  ProgramProvider
}

func NewService(w WorkoutsRepo, p ProgramProvider) *Service {
	return &Service{workouts: w, program: p}
}

// Compliance scores workout id against its linked template. Prerequisite
// failures return the sentinel errors above; a lap/step count mismatch returns a
// Result with Status "unavailable" (not an error) and no per-step array.
func (s *Service) Compliance(ctx context.Context, id uuid.UUID) (*Result, error) {
	w, err := s.workouts.GetByIDWithChildren(ctx, id)
	if err != nil {
		return nil, err // workouts.ErrNotFound flows through to the handler's 404 map
	}
	if w.Status != workouts.StatusCompleted {
		return nil, ErrNotCompleted
	}
	// Multisport check precedes the template-link check (a brick must not report
	// no_template_link).
	if w.Sport == workouts.SportMultisport || w.MultisportTemplateID != nil {
		return nil, ErrMultisportUnsupported
	}
	if w.TemplateID == nil {
		return nil, ErrNoTemplateLink
	}
	if len(w.Splits) == 0 {
		return nil, ErrSplitsMissing
	}

	prog, err := s.program.EffectiveProgram(ctx, id)
	if err != nil {
		return nil, err
	}
	expanded := expand(prog.Steps)

	res := &Result{
		WorkoutID:    w.ID,
		TemplateID:   w.TemplateID,
		PlannedSteps: len(expanded),
		ExecutedLaps: len(w.Splits),
	}

	// Strict positional matching: one lap per executed step, or nothing.
	if len(expanded) != len(w.Splits) {
		res.Status = StatusUnavailable
		reason := "lap_count_mismatch"
		res.Reason = &reason
		return res, nil
	}

	res.Status = StatusScored
	res.Steps = make([]StepResult, len(expanded))
	var weightedSum, weightTotal float64
	for i, es := range expanded {
		sr := scoreStep(es, w.Splits[i], i)
		res.Steps[i] = sr
		if sr.Score != nil {
			wt := stepWeight(es, w.Splits[i])
			weightedSum += *sr.Score * wt
			weightTotal += wt
			res.StepsScored++
			if inBand(sr) {
				res.StepsInBand++
			}
		}
	}
	if weightTotal > 0 {
		score := weightedSum / weightTotal
		res.Score = &score
	}
	return res, nil
}

// expandedStep is one flattened executed step with its repeat provenance.
type expandedStep struct {
	step      workouttemplates.Step
	iteration *int // 1-based iteration within a repeat group, nil for a single step
	of        *int // the repeat group's count, nil for a single step
}

// expand flattens the program's step tree: each repeat group contributes `count`
// consecutive copies of its inner steps, in order, carrying iteration/of
// provenance; single steps contribute themselves. One level deep (templates
// never nest repeats).
func expand(steps []workouttemplates.Step) []expandedStep {
	var out []expandedStep
	for _, s := range steps {
		if s.Type == workouttemplates.NodeRepeat {
			for it := 1; it <= s.Count; it++ {
				for _, inner := range s.Steps {
					iter, of := it, s.Count
					out = append(out, expandedStep{step: inner, iteration: &iter, of: &of})
				}
			}
			continue
		}
		out = append(out, expandedStep{step: s})
	}
	return out
}

// scoreStep scores one matched (step, split) pair: primary target, optional
// secondary target, duration, and the combined step score.
func scoreStep(es expandedStep, split workouts.Split, stepIndex int) StepResult {
	st := es.step
	sr := StepResult{
		StepIndex: stepIndex, // flat 0-based position in the expanded program
		Intent:    st.Intent,
		Iteration: es.iteration,
		Of:        es.of,
		Planned:   PlannedStep{Duration: st.Duration, Target: st.Target, SecondaryTarget: st.SecondaryTarget},
		Actual:    actualLap(split),
	}

	sr.Target = scoreTarget(st.Target, split)
	if st.SecondaryTarget != nil {
		sr.Secondary = scoreTarget(st.SecondaryTarget, split)
	}
	sr.Duration = scoreDuration(st.Duration, split)

	// Combine: 0.7 target + 0.3 duration when both scored; the present one
	// otherwise; nil when neither.
	tScored := sr.Target != nil && sr.Target.Scorable && sr.Target.Score != nil
	dScored := sr.Duration != nil && sr.Duration.Score != nil
	switch {
	case tScored && dScored:
		v := targetWeight*(*sr.Target.Score) + durationWeight*(*sr.Duration.Score)
		sr.Score = &v
	case tScored:
		v := *sr.Target.Score
		sr.Score = &v
	case dScored:
		v := *sr.Duration.Score
		sr.Score = &v
	}
	return sr
}

func actualLap(s workouts.Split) ActualLap {
	return ActualLap{
		DurationS:   s.DurationS,
		DistanceM:   s.DistanceM,
		AvgHR:       s.AvgHR,
		AvgPowerW:   s.AvgPowerW,
		AvgSpeedMPS: s.AvgSpeedMPS,
	}
}

// scoreTarget selects the actual metric by resolved target kind, classifies it
// against the band, and scores it. Returns an unscorable result (Scorable=false,
// Reason set) rather than nil for kinds/actuals that can't be compared, so the
// step still surfaces why.
func scoreTarget(t *workouttemplates.Target, split workouts.Split) *TargetResult {
	if t == nil || t.Kind == workouttemplates.TargetNone {
		return &TargetResult{Scorable: false, Reason: ptr(ReasonNoTarget)}
	}

	var metric string
	var actual *float64
	var low, high *float64
	switch t.Kind {
	case workouttemplates.TargetPowerW:
		metric = "power_w"
		actual = intToF(split.AvgPowerW)
		low, high = intToF(t.Low), intToF(t.High)
	case workouttemplates.TargetHRBpm:
		metric = "hr_bpm"
		actual = intToF(split.AvgHR)
		low, high = intToF(t.Low), intToF(t.High)
	case workouttemplates.TargetPace:
		metric = "pace"
		actual = paceFromSpeed(split.AvgSpeedMPS, 1000)
		low, high = intToF(t.LowSecPerKM), intToF(t.HighSecPerKM)
	case workouttemplates.TargetSwimPace:
		metric = "swim_pace"
		actual = paceFromSpeed(split.AvgSpeedMPS, 100)
		low, high = intToF(t.LowSecPer100m), intToF(t.HighSecPer100m)
	case workouttemplates.TargetHRZone, workouttemplates.TargetPowerZone:
		// Reached the read still zone-shaped → the resolver couldn't rewrite it.
		return &TargetResult{Scorable: false, Reason: ptr(ReasonZoneUnresolved)}
	default: // cadence, rpe, anything else — no per-lap actual to compare
		return &TargetResult{Scorable: false, Reason: ptr(ReasonUnsupportedKind)}
	}

	if actual == nil {
		return &TargetResult{Scorable: false, Reason: ptr(ReasonActualMissing), Metric: metric, Low: low, High: high}
	}
	if low == nil && high == nil {
		return &TargetResult{Scorable: false, Reason: ptr(ReasonZoneUnresolved), Metric: metric, Actual: actual}
	}

	class, delta, devPct := classify(*actual, low, high)
	score := targetScore(devPct)
	return &TargetResult{
		Scorable:       true,
		Metric:         metric,
		Low:            low,
		High:           high,
		Actual:         actual,
		Classification: class,
		Delta:          &delta,
		DeviationPct:   &devPct,
		Score:          &score,
	}
}

// classify places actual against [low, high] (either bound may be nil = open).
// delta is the signed distance from the violated bound (0 in band): negative
// when under low, positive when over high. deviation_pct is |delta| over the
// violated bound.
func classify(actual float64, low, high *float64) (class string, delta, devPct float64) {
	if low != nil && actual < *low {
		delta = actual - *low
		if *low != 0 {
			devPct = math.Abs(delta) / math.Abs(*low)
		}
		return ClassUnder, delta, devPct
	}
	if high != nil && actual > *high {
		delta = actual - *high
		if *high != 0 {
			devPct = math.Abs(delta) / math.Abs(*high)
		}
		return ClassOver, delta, devPct
	}
	return ClassInBand, 0, 0
}

// targetScore is 100 in band, falling off linearly to 0 at targetZeroPct past
// the band edge.
func targetScore(devPct float64) float64 {
	return 100 * math.Max(0, 1-devPct/targetZeroPct)
}

// scoreDuration compares planned vs actual for time/distance steps. lap_button/
// open steps have no planned duration → no DurationResult.
func scoreDuration(d *workouttemplates.Duration, split workouts.Split) *DurationResult {
	if d == nil {
		return nil
	}
	var planned, actual *float64
	switch d.Kind {
	case workouttemplates.DurationTime:
		planned = intToF(d.Seconds)
		actual = split.DurationS
	case workouttemplates.DurationDistance:
		planned = intToF(d.Meters)
		actual = split.DistanceM
	default: // lap_button / open — athlete decides when it ends, not scored
		return nil
	}
	dr := &DurationResult{Kind: d.Kind, Planned: planned, Actual: actual}
	if planned == nil || *planned == 0 || actual == nil {
		return dr // reported, but unscored (nothing to compare against)
	}
	ratio := *actual / *planned
	dr.Ratio = &ratio
	dev := math.Abs(ratio - 1)
	dr.Classification = ClassInBand
	if ratio < 1-durationBandPct {
		dr.Classification = ClassUnder
	} else if ratio > 1+durationBandPct {
		dr.Classification = ClassOver
	}
	dr.Score = ptrF(durationScore(dev))
	return dr
}

// durationScore is 100 within ±durationBandPct, falling off linearly to 0 at
// ±durationZeroPct.
func durationScore(dev float64) float64 {
	if dev <= durationBandPct {
		return 100
	}
	span := durationZeroPct - durationBandPct
	return 100 * math.Max(0, 1-(dev-durationBandPct)/span)
}

// stepWeight is the planned-duration weight of a step in the overall mean: the
// planned seconds for a time step; an estimated duration for a distance step
// (planned metres at the target pace midpoint when pace-targeted, else the
// actual lap duration); the actual lap duration for lap_button/open steps.
// Falls back to the actual lap duration, then 1, so a weight is always positive.
func stepWeight(es expandedStep, split workouts.Split) float64 {
	d := es.step.Duration
	if d != nil {
		switch d.Kind {
		case workouttemplates.DurationTime:
			if d.Seconds != nil && *d.Seconds > 0 {
				return float64(*d.Seconds)
			}
		case workouttemplates.DurationDistance:
			if d.Meters != nil && *d.Meters > 0 {
				if secs := estimateDistanceSeconds(*d.Meters, es.step.Target); secs > 0 {
					return secs
				}
			}
		}
	}
	if split.DurationS != nil && *split.DurationS > 0 {
		return *split.DurationS
	}
	return 1
}

// estimateDistanceSeconds estimates how long a distance step should take from
// the target pace midpoint (pace → sec/km, swim_pace → sec/100m). Returns 0 when
// the step is not pace-targeted (caller falls back to the actual lap duration).
func estimateDistanceSeconds(meters int, t *workouttemplates.Target) float64 {
	if t == nil {
		return 0
	}
	switch t.Kind {
	case workouttemplates.TargetPace:
		if t.LowSecPerKM != nil && t.HighSecPerKM != nil {
			mid := float64(*t.LowSecPerKM+*t.HighSecPerKM) / 2
			return float64(meters) / 1000 * mid
		}
	case workouttemplates.TargetSwimPace:
		if t.LowSecPer100m != nil && t.HighSecPer100m != nil {
			mid := float64(*t.LowSecPer100m+*t.HighSecPer100m) / 2
			return float64(meters) / 100 * mid
		}
	}
	return 0
}

// inBand reports whether the step's primary target landed in band (used for the
// steps_in_band count — a step with no scorable target is not counted).
func inBand(sr StepResult) bool {
	return sr.Target != nil && sr.Target.Scorable && sr.Target.Classification == ClassInBand
}

func ptr(s string) *string    { return &s }
func ptrF(v float64) *float64 { return &v }

func intToF(p *int) *float64 {
	if p == nil {
		return nil
	}
	v := float64(*p)
	return &v
}

// paceFromSpeed converts m/s to seconds per `unit` metres (1000 = sec/km, 100 =
// sec/100m). Zero/negative speed → nil (no meaningful pace).
func paceFromSpeed(speedMPS *float64, unit float64) *float64 {
	if speedMPS == nil || *speedMPS <= 0 {
		return nil
	}
	v := unit / *speedMPS
	return &v
}
