package healthvitals

import (
	"context"
	"errors"
	"time"
)

// Validation errors map 1:1 to API error codes.
var (
	ErrDateInvalid        = errors.New("date_invalid")
	ErrBPSystolicInvalid  = errors.New("bp_systolic_invalid")
	ErrBPDiastolicInvalid = errors.New("bp_diastolic_invalid")
	ErrBPPulseInvalid     = errors.New("bp_pulse_invalid")
	ErrRestingHRInvalid   = errors.New("resting_hr_invalid")
	ErrMinHRInvalid       = errors.New("min_hr_invalid")
	ErrMaxHRInvalid       = errors.New("max_hr_invalid")
	ErrStressAvgInvalid   = errors.New("stress_avg_invalid")
	ErrStressMaxInvalid   = errors.New("stress_max_invalid")
)

const dateLayout = "2006-01-02"

// Service orchestrates health-vitals CRUD over the repo.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// Upsert validates and date-keyed-upserts a snapshot. created=true on INSERT.
func (s *Service) Upsert(ctx context.Context, in *Snapshot) (*Snapshot, bool, error) {
	if !validDate(in.Date) {
		return nil, false, ErrDateInvalid
	}
	if err := validate(in); err != nil {
		return nil, false, err
	}
	created, err := s.repo.Upsert(ctx, in)
	if err != nil {
		return nil, false, err
	}
	out, err := s.repo.GetByDate(ctx, in.Date)
	if err != nil {
		return nil, false, err
	}
	return out, created, nil
}

// Get returns the snapshot for a date.
func (s *Service) Get(ctx context.Context, date string) (*Snapshot, error) {
	if !validDate(date) {
		return nil, ErrDateInvalid
	}
	return s.repo.GetByDate(ctx, date)
}

// ListWindow returns snapshots in [from, to].
func (s *Service) ListWindow(ctx context.Context, from, to string) ([]*Snapshot, error) {
	return s.repo.List(ctx, from, to)
}

func validDate(d string) bool {
	if d == "" {
		return false
	}
	_, err := time.Parse(dateLayout, d)
	return err == nil
}

// validate rejects out-of-range metrics: BP/HR must be > 0, stress 0–100.
func validate(s *Snapshot) error {
	if err := posInt(s.BPSystolic, ErrBPSystolicInvalid); err != nil {
		return err
	}
	if err := posInt(s.BPDiastolic, ErrBPDiastolicInvalid); err != nil {
		return err
	}
	if err := posInt(s.BPPulse, ErrBPPulseInvalid); err != nil {
		return err
	}
	if err := posInt(s.RestingHR, ErrRestingHRInvalid); err != nil {
		return err
	}
	if err := posInt(s.MinHR, ErrMinHRInvalid); err != nil {
		return err
	}
	if err := posInt(s.MaxHR, ErrMaxHRInvalid); err != nil {
		return err
	}
	if err := rangeInt(s.StressAvg, 0, 100, ErrStressAvgInvalid); err != nil {
		return err
	}
	if err := rangeInt(s.StressMax, 0, 100, ErrStressMaxInvalid); err != nil {
		return err
	}
	return nil
}

func posInt(v *int, e error) error {
	if v != nil && *v <= 0 {
		return e
	}
	return nil
}

func rangeInt(v *int, lo, hi int, e error) error {
	if v != nil && (*v < lo || *v > hi) {
		return e
	}
	return nil
}
