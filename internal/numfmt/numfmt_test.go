package numfmt

import (
	"math"
	"testing"
)

func TestRound1(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{0, 0},
		{2.34, 2.3},
		{2.46, 2.5},
		{70.44969999999999, 70.4}, // the float artefact the package exists to hide
		{-1.25, -1.3},             // half away from zero (12.5 → 13)
		{-2.34, -2.3},
		{999.95, 1000.0},
	}
	for _, c := range cases {
		if got := Round1(c.in); got != c.want {
			t.Errorf("Round1(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestRound1_SpecialValues(t *testing.T) {
	if got := Round1(math.Inf(1)); !math.IsInf(got, 1) {
		t.Errorf("Round1(+Inf) = %v, want +Inf", got)
	}
	if got := Round1(math.Inf(-1)); !math.IsInf(got, -1) {
		t.Errorf("Round1(-Inf) = %v, want -Inf", got)
	}
	if got := Round1(math.NaN()); !math.IsNaN(got) {
		t.Errorf("Round1(NaN) = %v, want NaN", got)
	}
}

func TestRound1Ptr(t *testing.T) {
	if got := Round1Ptr(nil); got != nil {
		t.Errorf("Round1Ptr(nil) = %v, want nil", got)
	}

	in := 2.46
	got := Round1Ptr(&in)
	if got == nil {
		t.Fatal("Round1Ptr(non-nil) returned nil")
	}
	if *got != 2.5 {
		t.Errorf("*Round1Ptr(&2.46) = %v, want 2.5", *got)
	}
	if got == &in {
		t.Error("Round1Ptr must return a fresh pointer, not alias its input")
	}
	if in != 2.46 {
		t.Errorf("Round1Ptr mutated its input to %v", in)
	}
}

func TestRound2(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{0, 0},
		{0.125, 0.13}, // half away from zero (12.5 → 13)
		{70.4497, 70.45},
		{1.234, 1.23},
		{-0.125, -0.13},
	}
	for _, c := range cases {
		if got := Round2(c.in); got != c.want {
			t.Errorf("Round2(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
