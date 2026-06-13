package devices

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

// Validation errors map 1:1 to API error codes.
var (
	ErrExternalIDRequired  = errors.New("external_id_required")
	ErrDisplayNameRequired = errors.New("display_name_required")
	ErrBatteryPctInvalid   = errors.New("battery_pct_invalid")
)

// Service orchestrates device CRUD over the repo.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// Upsert validates and upserts a device by external_id. created=true on INSERT.
func (s *Service) Upsert(ctx context.Context, d *Device) (*Device, bool, error) {
	d.ExternalID = strings.TrimSpace(d.ExternalID)
	d.DisplayName = strings.TrimSpace(d.DisplayName)
	if d.ExternalID == "" {
		return nil, false, ErrExternalIDRequired
	}
	if d.DisplayName == "" {
		return nil, false, ErrDisplayNameRequired
	}
	if d.BatteryPct != nil && (*d.BatteryPct < 0 || *d.BatteryPct > 100) {
		return nil, false, ErrBatteryPctInvalid
	}
	created, err := s.repo.Upsert(ctx, d)
	if err != nil {
		return nil, false, err
	}
	out, err := s.repo.GetByID(ctx, d.ID)
	if err != nil {
		return nil, false, err
	}
	return out, created, nil
}

// Get returns the device by backend id.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Device, error) {
	return s.repo.GetByID(ctx, id)
}

// List returns all device rows.
func (s *Service) List(ctx context.Context) ([]*Device, error) {
	return s.repo.List(ctx)
}
