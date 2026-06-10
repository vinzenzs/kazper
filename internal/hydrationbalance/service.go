package hydrationbalance

import (
	"context"
	"errors"
	"math"
	"time"
)

// Validation errors map 1:1 to API error codes.
var (
	ErrDateInvalid             = errors.New("date_invalid")
	ErrSweatLossMLInvalid      = errors.New("sweat_loss_ml_invalid")
	ErrActivityIntakeMLInvalid = errors.New("activity_intake_ml_invalid")
	ErrGoalMLInvalid           = errors.New("goal_ml_invalid")
)

const dateLayout = "2006-01-02"

// Service orchestrates hydration-balance CRUD over the repo.
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

// Delete removes the snapshot for a date.
func (s *Service) Delete(ctx context.Context, date string) error {
	if !validDate(date) {
		return ErrDateInvalid
	}
	return s.repo.DeleteByDate(ctx, date)
}

// ----- validators -----

func validDate(d string) bool {
	if d == "" {
		return false
	}
	_, err := time.Parse(dateLayout, d)
	return err == nil
}

func validate(s *Snapshot) error {
	// sweat_loss_ml and goal_ml must be > 0 (a zero there means "not measured",
	// better expressed as NULL); activity_intake_ml allows a real 0 (sweated,
	// drank nothing during the session).
	if err := posFloat(s.SweatLossML, ErrSweatLossMLInvalid); err != nil {
		return err
	}
	if err := nonNegFloat(s.ActivityIntakeML, ErrActivityIntakeMLInvalid); err != nil {
		return err
	}
	if err := posFloat(s.GoalML, ErrGoalMLInvalid); err != nil {
		return err
	}
	return nil
}

func posFloat(v *float64, e error) error {
	if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0) || *v <= 0) {
		return e
	}
	return nil
}

func nonNegFloat(v *float64, e error) error {
	if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0) {
		return e
	}
	return nil
}
