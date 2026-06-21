package coachmemory

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

// Validation errors map 1:1 to API error codes.
var (
	ErrTextRequired          = errors.New("text_required")
	ErrKindInvalid           = errors.New("kind_invalid")
	ErrScopeInvalid          = errors.New("scope_invalid")
	ErrDateRequired  = errors.New("date_required")
	ErrStatusInvalid = errors.New("status_invalid")
)

// Service validates and orchestrates coach-memory CRUD over the repo.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// CreateInput is the validated payload for recording a memory item. Date/
// ExpiresAt/ReviewAt are YYYY-MM-DD strings already parsed by the handler (nil
// when absent).
type CreateInput struct {
	Kind      string
	Text      string
	Reason    *string
	Scope     *string
	Date      *string
	ExpiresAt *string
	ReviewAt  *string
}

// Create validates and persists a memory item.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Memory, error) {
	if strings.TrimSpace(in.Text) == "" {
		return nil, ErrTextRequired
	}
	if !ValidKind(in.Kind) {
		return nil, ErrKindInvalid
	}
	if in.Scope != nil && !ValidScope(*in.Scope) {
		return nil, ErrScopeInvalid
	}
	// A recommendation is advice for a day — date is required. Standing kinds
	// may be dateless.
	if Kind(in.Kind) == KindRecommendation && (in.Date == nil || *in.Date == "") {
		return nil, ErrDateRequired
	}
	m := &Memory{
		Kind:      Kind(in.Kind),
		Text:      in.Text,
		Reason:    in.Reason,
		Date:      in.Date,
		ExpiresAt: in.ExpiresAt,
		ReviewAt:  in.ReviewAt,
	}
	if in.Scope != nil {
		sc := Scope(*in.Scope)
		m.Scope = &sc
	}
	if err := s.repo.Insert(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// List returns memory items per the params (recommendations window-filtered,
// standing items always, archived/expired excluded by default).
func (s *Service) List(ctx context.Context, p ListParams) ([]*Memory, error) {
	return s.repo.ListWindow(ctx, p)
}

// Get returns one memory item or ErrNotFound.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Memory, error) {
	return s.repo.GetByID(ctx, id)
}

// PatchInput is the lifecycle-only editable subset. The Clear flags carry the
// JSON-null-clears tri-state on the two nullable date fields.
type PatchInput struct {
	ReviewAt       *string
	ClearReviewAt  bool
	ExpiresAt      *string
	ClearExpiresAt bool
	Status         *string
}

// Patch validates and applies the lifecycle update in place. Content is
// immutable here (correct via delete + re-log).
func (s *Service) Patch(ctx context.Context, id uuid.UUID, in PatchInput) (*Memory, error) {
	if in.Status != nil && !ValidStatus(*in.Status) {
		return nil, ErrStatusInvalid
	}
	if err := s.repo.Patch(ctx, id, PatchParams{
		ReviewAt:       in.ReviewAt,
		ClearReviewAt:  in.ClearReviewAt,
		ExpiresAt:      in.ExpiresAt,
		ClearExpiresAt: in.ClearExpiresAt,
		Status:         in.Status,
	}); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// Delete removes one memory item or returns ErrNotFound.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
