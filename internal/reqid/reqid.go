// Package reqid carries a per-request correlation id across the request context
// so the HTTP middleware, the request logger, and the chat loopback dispatcher
// all agree on one id per turn. Kept in its own tiny package so httpserver and
// chat can share it without an import cycle.
package reqid

import (
	"context"

	"github.com/google/uuid"
)

// HeaderName is the HTTP header the id is honored from and echoed on.
const HeaderName = "X-Request-ID"

type ctxKey struct{}

// NewContext returns ctx carrying id.
func NewContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// FromContext returns the request id carried by ctx, or "" if none.
func FromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

// Generate returns a fresh random request id.
func Generate() string {
	return uuid.NewString()
}
