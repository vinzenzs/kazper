package push

import (
	"context"
	"errors"
	"log/slog"
)

// ErrPushDisabled is returned by operations that require a configured sender
// when push is unconfigured. Registration does not require the sender and so
// never returns this; the relogin notification simply no-ops.
var ErrPushDisabled = errors.New("push_disabled")

// Service registers device tokens and drives the Garmin relogin notification.
// A nil sender means push is unconfigured: registration still works, delivery is
// a silent no-op.
type Service struct {
	repo   *Repo
	sender Sender
	logger *slog.Logger
}

// NewService builds the service. Pass a nil sender to disable delivery; pass a
// nil logger to silence it.
func NewService(repo *Repo, sender Sender, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Service{repo: repo, sender: sender, logger: logger}
}

// Enabled reports whether a sender is configured.
func (s *Service) Enabled() bool { return s.sender != nil }

// RegisterToken upserts a device token. It works regardless of whether the
// sender is configured, so enabling push later delivers without re-pairing.
func (s *Service) RegisterToken(ctx context.Context, token, platform string) (*PushToken, error) {
	return s.repo.UpsertToken(ctx, token, platform)
}

// RemoveToken drops a device token (no-op when absent).
func (s *Service) RemoveToken(ctx context.Context, token string) error {
	return s.repo.DeleteToken(ctx, token)
}

// NotifyReloginNeeded sends the Garmin relogin push to every registered device,
// once per outage. It is idempotent while the latch is set and a silent no-op
// when push is unconfigured or no device is registered. Per-token send failures
// are swallowed (logged): a dead token is pruned, the rest are still attempted,
// and the latch is set after a delivery round so repeated failing syncs do not
// re-notify. The error return is reserved for latch/store failures, not delivery.
func (s *Service) NotifyReloginNeeded(ctx context.Context) error {
	if s.sender == nil {
		return nil // push disabled
	}
	latch, err := s.repo.Latch(ctx)
	if err != nil {
		return err
	}
	if latch.Notified {
		return nil // already notified this outage
	}
	tokens, err := s.repo.ListTokens(ctx)
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		return nil // nobody to notify; leave the latch unset for when a device registers
	}

	msg := reloginMessage()
	for _, tok := range tokens {
		switch err := s.sender.Send(ctx, tok, msg); {
		case err == nil:
			// delivered
		case errors.Is(err, ErrTokenUnregistered):
			if derr := s.repo.DeleteToken(ctx, tok); derr != nil {
				s.logger.WarnContext(ctx, "push: prune unregistered token failed", "error", derr)
			}
		default:
			s.logger.WarnContext(ctx, "push: relogin send failed", "error", err)
		}
	}
	return s.repo.SetNotified(ctx)
}

// ClearReloginLatch resets the latch so a future outage notifies again. Safe to
// call when push is disabled or the latch is already clear.
func (s *Service) ClearReloginLatch(ctx context.Context) error {
	return s.repo.ClearLatch(ctx)
}
