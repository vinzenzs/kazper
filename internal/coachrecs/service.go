package coachrecs

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

// Validation errors map 1:1 to API error codes.
var (
	ErrRecommendationRequired = errors.New("recommendation_required")
	ErrScopeInvalid           = errors.New("scope_invalid")
)

// Service validates and orchestrates recommendation CRUD over the repo.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// CreateInput is the validated payload for recording a recommendation. Date is a
// YYYY-MM-DD string already parsed/validated by the handler.
type CreateInput struct {
	Date           string
	Scope          string
	Recommendation string
	Reason         *string
}

// Create validates and persists a recommendation.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Recommendation, error) {
	if strings.TrimSpace(in.Recommendation) == "" {
		return nil, ErrRecommendationRequired
	}
	if !ValidScope(in.Scope) {
		return nil, ErrScopeInvalid
	}
	rec := &Recommendation{
		Date:           in.Date,
		Scope:          Scope(in.Scope),
		Recommendation: in.Recommendation,
		Reason:         in.Reason,
	}
	if err := s.repo.Insert(ctx, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// List returns the recommendations in [from, to] (inclusive local dates),
// optionally narrowed to one scope, newest-first.
func (s *Service) List(ctx context.Context, from, to string, scope *string) ([]*Recommendation, error) {
	return s.repo.ListWindow(ctx, from, to, scope)
}

// Get returns one recommendation or ErrNotFound.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Recommendation, error) {
	return s.repo.GetByID(ctx, id)
}

// Delete removes one recommendation or returns ErrNotFound.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
