package push

import (
	"context"
	"fmt"

	"github.com/vinzenzs/kazper/internal/store"
)

// Repo persists push tokens and the relogin latch against store.Querier.
type Repo struct {
	q store.Querier
}

func NewRepo(q store.Querier) *Repo {
	return &Repo{q: q}
}

// UpsertToken registers a device token, refreshing updated_at on re-register so
// the same token is idempotent (one row) and a rotated token is a new row.
func (r *Repo) UpsertToken(ctx context.Context, token, platform string) (*PushToken, error) {
	if platform == "" {
		platform = "android"
	}
	row := r.q.QueryRow(ctx, `
        INSERT INTO push_tokens (token, platform)
        VALUES ($1, $2)
        ON CONFLICT (token) DO UPDATE
            SET platform = EXCLUDED.platform,
                updated_at = now()
        RETURNING id, token, platform, created_at, updated_at`,
		token, platform,
	)
	var t PushToken
	if err := row.Scan(&t.ID, &t.Token, &t.Platform, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, fmt.Errorf("upsert push token: %w", err)
	}
	return &t, nil
}

// DeleteToken removes a token by its string value. A missing token is a no-op.
func (r *Repo) DeleteToken(ctx context.Context, token string) error {
	if _, err := r.q.Exec(ctx, `DELETE FROM push_tokens WHERE token = $1`, token); err != nil {
		return fmt.Errorf("delete push token: %w", err)
	}
	return nil
}

// ListTokens returns every registered token string.
func (r *Repo) ListTokens(ctx context.Context) ([]string, error) {
	rows, err := r.q.Query(ctx, `SELECT token FROM push_tokens ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list push tokens: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var tok string
		if err := rows.Scan(&tok); err != nil {
			return nil, fmt.Errorf("scan push token: %w", err)
		}
		out = append(out, tok)
	}
	return out, rows.Err()
}

// Latch reads the single relogin-latch row.
func (r *Repo) Latch(ctx context.Context) (ReloginLatch, error) {
	var l ReloginLatch
	err := r.q.QueryRow(ctx,
		`SELECT notified, notified_at FROM relogin_latch WHERE id = 1`,
	).Scan(&l.Notified, &l.NotifiedAt)
	if err != nil {
		return ReloginLatch{}, fmt.Errorf("read relogin latch: %w", err)
	}
	return l, nil
}

// SetNotified latches the relogin notification, stamping notified_at.
func (r *Repo) SetNotified(ctx context.Context) error {
	if _, err := r.q.Exec(ctx,
		`UPDATE relogin_latch SET notified = true, notified_at = now(), updated_at = now() WHERE id = 1`,
	); err != nil {
		return fmt.Errorf("set relogin latch: %w", err)
	}
	return nil
}

// ClearLatch resets the relogin latch so the next distinct outage can notify.
func (r *Repo) ClearLatch(ctx context.Context) error {
	if _, err := r.q.Exec(ctx,
		`UPDATE relogin_latch SET notified = false, notified_at = NULL, updated_at = now() WHERE id = 1`,
	); err != nil {
		return fmt.Errorf("clear relogin latch: %w", err)
	}
	return nil
}
