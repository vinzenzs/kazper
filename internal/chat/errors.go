package chat

import (
	"errors"
	"fmt"
)

// ErrAPIKeyMissing is returned by New when ANTHROPIC_API_KEY is empty, so the
// server can leave the chat service nil and POST /chat can return 503
// chat_unavailable rather than panicking. Mirrors internal/vision.
var ErrAPIKeyMissing = errors.New("anthropic api key missing")

// ErrUpstreamUnavailable wraps a transport failure or a retryable upstream
// status (429 / 5xx) from the Anthropic Messages API. Surfaced to the client as
// an `error` SSE event with code `upstream_unavailable`.
type ErrUpstreamUnavailable struct {
	StatusCode int // 0 for transport/timeout failures
	Err        error
}

func (e *ErrUpstreamUnavailable) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("anthropic upstream unavailable: HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("anthropic upstream unavailable: %v", e.Err)
}

func (e *ErrUpstreamUnavailable) Unwrap() error { return e.Err }

// ErrUpstreamProtocol is returned when the Anthropic stream is malformed in a
// way the parser cannot recover from (e.g. an `error` event, or an unparseable
// frame). Also surfaced as `upstream_unavailable` to the client.
type ErrUpstreamProtocol struct{ Msg string }

func (e *ErrUpstreamProtocol) Error() string { return "anthropic stream protocol error: " + e.Msg }
