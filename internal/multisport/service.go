package multisport

import (
	"context"
	"errors"
	"strings"

	"github.com/vinzenzs/kazper/internal/workouttemplates"
)

// Validation errors map 1:1 to API error codes. Per-segment step validation
// returns the underlying workouttemplates error verbatim (e.g.
// target_sport_mismatch), so a swim segment with a /km pace surfaces the same
// code it would on a single-sport template.
var (
	ErrNameRequired    = errors.New("name_required")
	ErrSegmentsEmpty   = errors.New("segments_empty")
	ErrTooFewSports    = errors.New("too_few_sport_segments")
	ErrSegmentSport    = errors.New("segment_sport_invalid")
	ErrTransitionShape = errors.New("transition_segment_invalid")
)

// Service orchestrates multisport-template CRUD with per-segment validation.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// Create validates and persists a new multisport template.
func (s *Service) Create(ctx context.Context, t *Template) (*Template, error) {
	if err := validateTemplate(t); err != nil {
		return nil, err
	}
	t.Name = strings.TrimSpace(t.Name)
	created, err := s.repo.Create(ctx, t)
	if err != nil {
		return nil, err
	}
	return stampDuration(created), nil
}

// Get returns one template by id.
func (s *Service) Get(ctx context.Context, id string) (*Template, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return stampDuration(t), nil
}

// List returns all templates, newest first.
func (s *Service) List(ctx context.Context) ([]*Template, error) {
	ts, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, t := range ts {
		stampDuration(t)
	}
	return ts, nil
}

// stampDuration sets the read-only derived EstimatedDurationSec from the
// template's segments and returns the same pointer for chaining.
func stampDuration(t *Template) *Template {
	if t != nil {
		t.EstimatedDurationSec = estimatedDurationSec(t.Segments)
		for i := range t.Segments {
			t.Segments[i].EstimatedDurationSec = segmentEstimatedDurationSec(t.Segments[i])
		}
	}
	return t
}

// Delete removes a template by id.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// ----- validation -----

// validateTemplate enforces: a name, a non-empty segment list, at least two
// non-transition (sport) segments, each sport segment's steps validated under
// its own sport, and each transition segment carrying only a valid duration.
func validateTemplate(t *Template) error {
	if strings.TrimSpace(t.Name) == "" {
		return ErrNameRequired
	}
	if len(t.Segments) == 0 {
		return ErrSegmentsEmpty
	}
	sportSegments := 0
	for _, seg := range t.Segments {
		if seg.IsTransition() {
			if err := validateTransition(seg); err != nil {
				return err
			}
			continue
		}
		if err := validateSportSegment(seg); err != nil {
			return err
		}
		sportSegments++
	}
	if sportSegments < 2 {
		return ErrTooFewSports
	}
	return nil
}

// validateSportSegment validates a non-transition segment: a real sport and a
// step program valid under that sport (the workouttemplates validator, reused).
func validateSportSegment(seg Segment) error {
	switch seg.Sport {
	case workouttemplates.SportRun, workouttemplates.SportBike, workouttemplates.SportSwim,
		workouttemplates.SportStrength, workouttemplates.SportYoga, workouttemplates.SportMobility,
		workouttemplates.SportOther:
	default:
		return ErrSegmentSport
	}
	if seg.Duration != nil {
		// Durations belong to transition segments only; a sport segment's length
		// comes from its steps.
		return ErrTransitionShape
	}
	return workouttemplates.ValidateSteps(seg.Steps, seg.Sport)
}

// validateTransition validates a T1/T2 segment: a single valid duration and no
// steps.
func validateTransition(seg Segment) error {
	if len(seg.Steps) > 0 {
		return ErrTransitionShape
	}
	if err := workouttemplates.ValidateDuration(seg.Duration); err != nil {
		return ErrTransitionShape
	}
	return nil
}
