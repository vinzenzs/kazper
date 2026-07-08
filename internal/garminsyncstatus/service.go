package garminsyncstatus

import (
	"context"
	"errors"
	"log/slog"
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

// ReloginNotifier is the push side-effect port (satisfied by push.Service):
// notify that Garmin re-authentication is needed, and clear that state on
// recovery. Both are latched/no-op on the push side, so this service may call
// them freely. Nil when push is not wired.
type ReloginNotifier interface {
	NotifyReloginNeeded(ctx context.Context) error
	ClearReloginLatch(ctx context.Context) error
}

// GarminTokenPresence reports whether a Garmin token is currently stored
// (satisfied by garminauth.Service). Used to distinguish a relogin-needed
// failure (token cleared by the bridge) from a transient one. Nil when unwired.
type GarminTokenPresence interface {
	HasToken(ctx context.Context) (bool, error)
}

// Service records sync runs and composes sync-status reads over the repo.
type Service struct {
	repo     *Repo
	notifier ReloginNotifier
	tokens   GarminTokenPresence
	logger   *slog.Logger
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo, logger: slog.New(slog.DiscardHandler)}
}

// SetReloginNotifier wires the push side-effect (optional; cross-injected in the
// server). When nil, closing a run has no notification side-effect.
func (s *Service) SetReloginNotifier(n ReloginNotifier) { s.notifier = n }

// SetGarminTokenPresence wires the token-presence check used to gate the relogin
// notification on an actually-absent token (optional).
func (s *Service) SetGarminTokenPresence(p GarminTokenPresence) { s.tokens = p }

// SetLogger wires a logger for the swallowed side-effect errors (optional).
func (s *Service) SetLogger(l *slog.Logger) {
	if l != nil {
		s.logger = l
	}
}

// Open records a new running sync run with the rolling window (either bound may
// be nil for "no window supplied").
func (s *Service) Open(ctx context.Context, windowFrom, windowTo *string) (*SyncRun, error) {
	return s.repo.Open(ctx, windowFrom, windowTo)
}

// Close terminates a run with status success|error|partial, recording an
// optional error message and roll-up summary. Rejects a non-terminal status with
// ErrStatusInvalid and a missing run with ErrNotFound.
func (s *Service) Close(ctx context.Context, id uuid.UUID, status string, errMsg *string, summary []byte) (*SyncRun, error) {
	if !ValidCloseStatus(status) {
		return nil, ErrStatusInvalid
	}
	run, err := s.repo.Close(ctx, id, status, errMsg, summary)
	if err != nil {
		return nil, err
	}
	// Drive the relogin push side-effect after the run is persisted. Failures
	// here never fail the close — the bridge's close contract is unchanged.
	s.afterClose(ctx, Status(status))
	return run, nil
}

// afterClose runs the post-commit push side-effects: an error-close with the
// Garmin token absent means relogin is needed (notify, latched on the push
// side); a success-close clears the latch so a future outage notifies again.
// All errors are logged and swallowed.
func (s *Service) afterClose(ctx context.Context, status Status) {
	if s.notifier == nil {
		return
	}
	switch status {
	case StatusError:
		if s.tokens == nil {
			return
		}
		has, err := s.tokens.HasToken(ctx)
		if err != nil {
			s.logger.WarnContext(ctx, "sync close: token presence check failed", "error", err)
			return
		}
		if has {
			return // token still present ⇒ transient error, not a relogin
		}
		if err := s.notifier.NotifyReloginNeeded(ctx); err != nil {
			s.logger.WarnContext(ctx, "sync close: relogin notify failed", "error", err)
		}
	case StatusSuccess:
		if err := s.notifier.ClearReloginLatch(ctx); err != nil {
			s.logger.WarnContext(ctx, "sync close: relogin latch clear failed", "error", err)
		}
	}
}

// Status returns a run, the last-successful timestamp, and a derived staleness
// flag. When runID is non-nil the named run is returned as `latest` (ErrNotFound
// when it does not exist), so a caller holding a 202 backfill's run_id can poll
// that specific run even while a concurrent daily sync opens newer runs;
// otherwise the most recent run is used. `last_successful_at`/`is_stale` are
// always derived globally (only `success` runs count). Pure composition — no
// synthesis over the stored runs.
func (s *Service) Status(ctx context.Context, runID *uuid.UUID) (*SyncStatus, error) {
	out := &SyncStatus{}

	var (
		latest *SyncRun
		err    error
	)
	if runID != nil {
		latest, err = s.repo.GetByID(ctx, *runID)
		if err != nil {
			return nil, err // ErrNotFound surfaces so the handler 404s
		}
	} else {
		latest, err = s.repo.Latest(ctx)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
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
