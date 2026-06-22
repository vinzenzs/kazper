package garminsyncstatus

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrStatusInvalid is returned when a close is attempted with a status outside
// the terminal set (success|error).
var ErrStatusInvalid = errors.New("status_invalid")

// stalenessThreshold bounds how long after the last successful sync the data is
// still considered fresh. A daily sync plus slack: 26h covers a once-a-day cron
// without flapping. (design open question — revisit if cadence changes.)
const stalenessThreshold = 26 * time.Hour

// Service records sync runs and composes sync-status reads over the repo.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// Open records a new running sync run with the rolling window (either bound may
// be nil for "no window supplied").
func (s *Service) Open(ctx context.Context, windowFrom, windowTo *string) (*SyncRun, error) {
	return s.repo.Open(ctx, windowFrom, windowTo)
}

// Close terminates a run with status success|error, recording an optional error
// message. Rejects a non-terminal status with ErrStatusInvalid and a missing run
// with ErrNotFound.
func (s *Service) Close(ctx context.Context, id uuid.UUID, status string, errMsg *string) (*SyncRun, error) {
	if !ValidCloseStatus(status) {
		return nil, ErrStatusInvalid
	}
	return s.repo.Close(ctx, id, status, errMsg)
}

// Status returns the latest run, the last-successful timestamp, and a derived
// staleness flag. Pure composition — no synthesis over the stored runs.
func (s *Service) Status(ctx context.Context) (*SyncStatus, error) {
	out := &SyncStatus{}

	latest, err := s.repo.Latest(ctx)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if latest != nil {
		out.Latest = latest
	}

	lastSucc, err := s.repo.LastSuccessfulAt(ctx)
	if err != nil {
		return nil, err
	}
	out.LastSuccessfulAt = lastSucc
	out.IsStale = lastSucc == nil || time.Since(*lastSucc) > stalenessThreshold
	return out, nil
}
