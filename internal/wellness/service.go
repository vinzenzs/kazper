package wellness

import (
	"context"
	"errors"
	"time"

	"github.com/vinzenzs/kazper/internal/numfmt"
)

// Sentinel errors map 1:1 to API error codes.
var (
	ErrEmpty         = errors.New("wellness_empty")
	ErrNoteTooLong   = errors.New("note_too_long")
	ErrRangeInvalid  = errors.New("range_invalid")
	ErrRangeTooLarge = errors.New("range_too_large")
	ErrMetricInvalid = errors.New("metric_invalid")
)

// minCorrelationPairs is the hard floor below which a rho on 5-level ordinal data
// is noise (design D3). Gated fields report {n, reason} instead of a number.
const minCorrelationPairs = 14

// correlationFields is the fixed set of wellness fields correlated, in a stable
// order for a deterministic response.
var correlationFields = []string{"fatigue", "soreness", "stress", "mood", "motivation"}

// ScoreError names the offending score field for wellness_score_invalid.
type ScoreError struct{ Field string }

func (e *ScoreError) Error() string { return "wellness_score_invalid" }

const (
	maxNoteLen   = 2000
	maxRangeDays = 92
)

// Service validates and persists wellness entries.
type Service struct {
	repo *Repo
	pmc  PMCProvider // set via SetPMCProvider for the correlation read
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// SetPMCProvider cross-injects the PMC series source used by CorrelationFor
// (wired in httpserver.Run() so wellness carries no pmc import).
func (s *Service) SetPMCProvider(p PMCProvider) { s.pmc = p }

// fieldScore extracts a wellness field's *int by name.
func fieldScore(e *Entry, field string) *int {
	switch field {
	case "fatigue":
		return e.Fatigue
	case "soreness":
		return e.Soreness
	case "stress":
		return e.Stress
	case "mood":
		return e.Mood
	case "motivation":
		return e.Motivation
	}
	return nil
}

// metricValue extracts a PMC day's chosen metric.
func metricValue(d PMCDayValue, metric string) float64 {
	switch metric {
	case "ctl":
		return d.CTL
	case "ramp_rate":
		return d.RampRate
	default: // tsb
		return d.TSB
	}
}

// CorrelationFor pairs each wellness field's daily entries with the same-day PMC
// metric and returns the per-field Spearman rank correlation. Fields with fewer
// than minCorrelationPairs paired days report {n, insufficient_pairs}. Metric is
// tsb (default) | ctl | ramp_rate. Compute-on-read; nothing persisted.
func (s *Service) CorrelationFor(ctx context.Context, from, to time.Time, loc *time.Location, metric string) (*CorrelationResult, error) {
	switch metric {
	case "", "tsb", "ctl", "ramp_rate":
		if metric == "" {
			metric = "tsb"
		}
	default:
		return nil, ErrMetricInvalid
	}

	entries, err := s.repo.List(ctx, from, to)
	if err != nil {
		return nil, err
	}
	pmcDays, err := s.pmc.PMCValues(ctx, from, to, loc)
	if err != nil {
		return nil, err
	}
	pmcByDate := make(map[string]float64, len(pmcDays))
	for _, d := range pmcDays {
		pmcByDate[d.Date] = metricValue(d, metric)
	}

	res := &CorrelationResult{Metric: metric, Fields: map[string]FieldCorrelation{}}
	for _, field := range correlationFields {
		var scores, metrics []float64
		for _, e := range entries {
			score := fieldScore(e, field)
			if score == nil {
				continue
			}
			mv, ok := pmcByDate[e.Date]
			if !ok {
				continue // no PMC value that day → no pair (no interpolation)
			}
			scores = append(scores, float64(*score))
			metrics = append(metrics, mv)
		}
		n := len(scores)
		fc := FieldCorrelation{N: n}
		if n < minCorrelationPairs {
			fc.Reason = "insufficient_pairs"
		} else {
			rho := numfmt.Round2(spearman(scores, metrics))
			fc.Rho = &rho
		}
		res.Fields[field] = fc
	}
	return res, nil
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
