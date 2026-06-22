package garminauth

import (
	"context"
	"errors"
	"log/slog"
)

// ErrEncKeyUnconfigured is returned when an operation is attempted but no
// encryption key was configured. In practice the handler short-circuits with
// 503 garmin_disabled before reaching the service, but the service refuses
// defensively so the blob is never stored or read unencrypted.
var ErrEncKeyUnconfigured = errors.New("garmin token encryption key not configured")

// ReloginLatchClearer is the push side-effect port (satisfied by push.Service):
// clear the relogin latch when a fresh token is stored, so re-authenticating
// after an outage stops the reminder. Nil when push is not wired.
type ReloginLatchClearer interface {
	ClearReloginLatch(ctx context.Context) error
}

// Service seals blobs on store and opens them on read. The blob is opaque:
// the service never parses or interprets it.
type Service struct {
	repo         *Repo
	crypto       *crypto
	latchClearer ReloginLatchClearer
	logger       *slog.Logger
}

// NewService builds the service over a repo and the 32-byte encryption key.
// A nil/empty key yields a service whose operations return
// ErrEncKeyUnconfigured; callers gate on garmin being enabled before wiring a
// real key.
func NewService(repo *Repo, encKey []byte) (*Service, error) {
	s := &Service{repo: repo, logger: slog.New(slog.DiscardHandler)}
	if len(encKey) == 0 {
		return s, nil
	}
	c, err := newCrypto(encKey)
	if err != nil {
		return nil, err
	}
	s.crypto = c
	return s, nil
}

// SetReloginLatchClearer wires the push latch-clear side-effect (optional;
// cross-injected in the server). When nil, storing a token has no side-effect.
func (s *Service) SetReloginLatchClearer(c ReloginLatchClearer) { s.latchClearer = c }

// SetLogger wires a logger for the swallowed side-effect error (optional).
func (s *Service) SetLogger(l *slog.Logger) {
	if l != nil {
		s.logger = l
	}
}

// Store encrypts and persists the opaque blob, replacing any prior value. On a
// successful store it clears the relogin latch (a fresh token means the user
// re-authenticated); a latch-clear failure is logged, never returned, so it
// cannot fail the store.
func (s *Service) Store(ctx context.Context, blob []byte) error {
	if s.crypto == nil {
		return ErrEncKeyUnconfigured
	}
	ciphertext, nonce, err := s.crypto.seal(blob)
	if err != nil {
		return err
	}
	if err := s.repo.Upsert(ctx, record{Ciphertext: ciphertext, Nonce: nonce}); err != nil {
		return err
	}
	if s.latchClearer != nil {
		if err := s.latchClearer.ClearReloginLatch(ctx); err != nil {
			s.logger.WarnContext(ctx, "garmin token store: relogin latch clear failed", "error", err)
		}
	}
	return nil
}

// HasToken reports whether a Garmin token blob is currently stored. Existence
// only — no decryption — so it works regardless of the encryption key. Satisfies
// garminsyncstatus.GarminTokenPresence.
func (s *Service) HasToken(ctx context.Context) (bool, error) {
	_, err := s.repo.Get(ctx)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Get returns the decrypted blob byte-identical to what was stored, or
// ErrNotFound when nothing has been stored.
func (s *Service) Get(ctx context.Context) ([]byte, error) {
	if s.crypto == nil {
		return nil, ErrEncKeyUnconfigured
	}
	rec, err := s.repo.Get(ctx)
	if err != nil {
		return nil, err
	}
	return s.crypto.open(rec.Ciphertext, rec.Nonce)
}

// Delete removes the stored blob, returning ErrNotFound when none existed.
func (s *Service) Delete(ctx context.Context) error {
	if s.crypto == nil {
		return ErrEncKeyUnconfigured
	}
	return s.repo.Delete(ctx)
}
