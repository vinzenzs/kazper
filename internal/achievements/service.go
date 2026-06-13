package achievements

import (
	"context"
	"errors"
	"strings"
)

// Validation errors map 1:1 to API error codes.
var (
	ErrExternalIDRequired = errors.New("external_id_required")
	ErrKindInvalid        = errors.New("kind_invalid")
	ErrNameRequired       = errors.New("name_required")
	ErrProgressPctInvalid = errors.New("progress_pct_invalid")
)

// Service orchestrates achievement CRUD over the repo.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// Upsert validates and upserts an achievement by external_id. created on INSERT.
func (s *Service) Upsert(ctx context.Context, a *Achievement) (*Achievement, bool, error) {
	a.ExternalID = strings.TrimSpace(a.ExternalID)
	a.Name = strings.TrimSpace(a.Name)
	if a.ExternalID == "" {
		return nil, false, ErrExternalIDRequired
	}
	if !ValidKind(string(a.Kind)) {
		return nil, false, ErrKindInvalid
	}
	if a.Name == "" {
		return nil, false, ErrNameRequired
	}
	if a.ProgressPct != nil && (*a.ProgressPct < 0 || *a.ProgressPct > 100) {
		return nil, false, ErrProgressPctInvalid
	}
	created, err := s.repo.Upsert(ctx, a)
	if err != nil {
		return nil, false, err
	}
	return a, created, nil
}

// List returns achievements, optionally filtered by kind.
func (s *Service) List(ctx context.Context, kind *string) ([]*Achievement, error) {
	return s.repo.List(ctx, kind)
}
