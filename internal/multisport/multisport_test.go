package multisport

import (
	"errors"
	"testing"

	wt "github.com/vinzenzs/kazper/internal/workouttemplates"
)

func ptrInt(i int) *int { return &i }

func swimSeg() Segment {
	return Segment{Sport: wt.SportSwim, Steps: []wt.Step{
		{Type: wt.NodeStep, Intent: wt.IntentActive,
			Duration: &wt.Duration{Kind: wt.DurationDistance, Meters: ptrInt(750)},
			Target:   &wt.Target{Kind: wt.TargetSwimPace, LowSecPer100m: ptrInt(100), HighSecPer100m: ptrInt(100)}},
	}}
}

func bikeSeg() Segment {
	return Segment{Sport: wt.SportBike, Steps: []wt.Step{
		{Type: wt.NodeStep, Intent: wt.IntentActive,
			Duration: &wt.Duration{Kind: wt.DurationTime, Seconds: ptrInt(1200)},
			Target:   &wt.Target{Kind: wt.TargetPowerW, Low: ptrInt(200), High: ptrInt(230)}},
	}}
}

func runSeg() Segment {
	return Segment{Sport: wt.SportRun, Steps: []wt.Step{
		{Type: wt.NodeStep, Intent: wt.IntentActive,
			Duration: &wt.Duration{Kind: wt.DurationTime, Seconds: ptrInt(900)},
			Target:   &wt.Target{Kind: wt.TargetNone}},
	}}
}

func transition() Segment {
	return Segment{Sport: SportTransition, Duration: &wt.Duration{Kind: wt.DurationLapButton}}
}

func validTriathlon() *Template {
	return &Template{Name: "Olympic Tri Race Sim", Segments: []Segment{
		swimSeg(), transition(), bikeSeg(), transition(), runSeg(),
	}}
}

func assertErr(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestValidate_AcceptsTriathlon(t *testing.T) {
	if err := validateTemplate(validTriathlon()); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestValidate_RejectsFewerThanTwoSportSegments(t *testing.T) {
	tpl := &Template{Name: "Solo", Segments: []Segment{swimSeg(), transition()}}
	assertErr(t, validateTemplate(tpl), ErrTooFewSports)
}

func TestValidate_RejectsEmptySegments(t *testing.T) {
	assertErr(t, validateTemplate(&Template{Name: "x"}), ErrSegmentsEmpty)
}

func TestValidate_RejectsMissingName(t *testing.T) {
	tpl := validTriathlon()
	tpl.Name = "  "
	assertErr(t, validateTemplate(tpl), ErrNameRequired)
}

// Per-segment validation runs under the segment's own sport: a km-pace target on
// a swim segment is rejected exactly as on a single-sport swim template.
func TestValidate_RejectsKmPaceOnSwimSegment(t *testing.T) {
	tpl := validTriathlon()
	tpl.Segments[0] = Segment{Sport: wt.SportSwim, Steps: []wt.Step{
		{Type: wt.NodeStep, Intent: wt.IntentActive,
			Duration: &wt.Duration{Kind: wt.DurationDistance, Meters: ptrInt(750)},
			Target:   &wt.Target{Kind: wt.TargetPace, LowSecPerKM: ptrInt(300), HighSecPerKM: ptrInt(300)}}},
	}
	assertErr(t, validateTemplate(tpl), wt.ErrTargetSportMismatch)
}

// A secondary target is bike-only; on a run segment it is rejected.
func TestValidate_RejectsSecondaryTargetOnRunSegment(t *testing.T) {
	tpl := validTriathlon()
	tpl.Segments[4] = Segment{Sport: wt.SportRun, Steps: []wt.Step{
		{Type: wt.NodeStep, Intent: wt.IntentActive,
			Duration:        &wt.Duration{Kind: wt.DurationTime, Seconds: ptrInt(900)},
			Target:          &wt.Target{Kind: wt.TargetHRZone, Low: ptrInt(2), High: ptrInt(2)},
			SecondaryTarget: &wt.Target{Kind: wt.TargetCadence, Low: ptrInt(85), High: ptrInt(90)}}},
	}
	assertErr(t, validateTemplate(tpl), wt.ErrSecondaryTarget)
}

// A bike segment MAY carry a secondary target (power + cadence).
func TestValidate_AcceptsSecondaryTargetOnBikeSegment(t *testing.T) {
	tpl := validTriathlon()
	tpl.Segments[2] = Segment{Sport: wt.SportBike, Steps: []wt.Step{
		{Type: wt.NodeStep, Intent: wt.IntentActive,
			Duration:        &wt.Duration{Kind: wt.DurationTime, Seconds: ptrInt(1200)},
			Target:          &wt.Target{Kind: wt.TargetPowerZone, Low: ptrInt(3), High: ptrInt(3)},
			SecondaryTarget: &wt.Target{Kind: wt.TargetCadence, Low: ptrInt(85), High: ptrInt(95)}}},
	}
	if err := validateTemplate(tpl); err != nil {
		t.Fatalf("expected valid bike secondary, got %v", err)
	}
}

func TestValidate_RejectsTransitionWithSteps(t *testing.T) {
	tpl := validTriathlon()
	tpl.Segments[1] = Segment{Sport: SportTransition,
		Duration: &wt.Duration{Kind: wt.DurationLapButton},
		Steps:    runSeg().Steps}
	assertErr(t, validateTemplate(tpl), ErrTransitionShape)
}

func TestValidate_RejectsTransitionWithoutDuration(t *testing.T) {
	tpl := validTriathlon()
	tpl.Segments[1] = Segment{Sport: SportTransition}
	assertErr(t, validateTemplate(tpl), ErrTransitionShape)
}

func TestValidate_RejectsUnknownSegmentSport(t *testing.T) {
	tpl := validTriathlon()
	tpl.Segments[0] = Segment{Sport: "kayak", Steps: runSeg().Steps}
	assertErr(t, validateTemplate(tpl), ErrSegmentSport)
}
