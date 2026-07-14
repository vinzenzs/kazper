package wellness

import (
	"context"
	"errors"
	"time"
)

// Sentinel errors map 1:1 to API error codes.
var (
	ErrEmpty       = errors.New("wellness_empty")
	ErrNoteTooLong = errors.New("note_too_long")
	ErrRangeInvalid = errors.New("range_invalid")
	ErrRangeTooLarge = errors.New("range_too_large")
)

// ScoreError names the offending score field for wellness_score_invalid.
type ScoreError struct{ Field string }

func (e *ScoreError) Error() string { return "wellness_score_invalid" }

const (
	maxNoteLen  = 2000
	maxRangeDays = 92
)

// Service validates and persists wellness entries.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// PutInput carries the decoded PUT body. Nil pointers mean the field is absent;
// full-replace semantics store them as NULL.
type PutInput struct {
	Fatigue    *int
	Soreness   *int
	Stress     *int
	Mood       *int
	Motivation *int
	Note       *string
}

// Put validates the entry (at least one field present, scores in 1–5, note
// within cap) and upserts it for `date`, returning the stored row.
func (s *Service) Put(ctx context.Context, date time.Time, in PutInput) (*Entry, error) {
	if in.Fatigue == nil && in.Soreness == nil && in.Stress == nil &&
		in.Mood == nil && in.Motivation == nil && in.Note == nil {
		return nil, ErrEmpty
	}
	for _, sf := range []struct {
		field string
		val   *int
	}{
		{"fatigue", in.Fatigue}, {"soreness", in.Soreness}, {"stress", in.Stress},
		{"mood", in.Mood}, {"motivation", in.Motivation},
	} {
		if sf.val != nil && (*sf.val < 1 || *sf.val > 5) {
			return nil, &ScoreError{Field: sf.field}
		}
	}
	// An all-scores-nil entry with only a note is valid; a whitespace/empty
	// note alongside no scores is still "at least one field present".
	if in.Note != nil && len(*in.Note) > maxNoteLen {
		return nil, ErrNoteTooLong
	}

	e := &Entry{
		Fatigue:    in.Fatigue,
		Soreness:   in.Soreness,
		Stress:     in.Stress,
		Mood:       in.Mood,
		Motivation: in.Motivation,
		Note:       in.Note,
	}
	if err := s.repo.Upsert(ctx, date, e); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, date)
}

// Get returns the entry for `date`, or ErrNotFound.
func (s *Service) Get(ctx context.Context, date time.Time) (*Entry, error) {
	return s.repo.Get(ctx, date)
}

// Delete removes the entry for `date`, returning ErrNotFound when absent.
func (s *Service) Delete(ctx context.Context, date time.Time) error {
	return s.repo.Delete(ctx, date)
}

// List returns the entries in [from, to] ascending, enforcing the 92-day cap.
func (s *Service) List(ctx context.Context, from, to time.Time) ([]*Entry, error) {
	if from.After(to) {
		return nil, ErrRangeInvalid
	}
	if days := int(to.Sub(from).Hours()/24) + 1; days > maxRangeDays {
		return nil, ErrRangeTooLarge
	}
	return s.repo.List(ctx, from, to)
}

// MaxRangeDays exposes the window cap for the handler's error payload.
const MaxRangeDays = maxRangeDays
