package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// ErrTokenUnregistered signals that FCM rejected a token as no longer valid, so
// the caller should prune it. Returned by Sender.Send.
var ErrTokenUnregistered = errors.New("push token unregistered")

// fcmScope is the OAuth2 scope FCM HTTP v1 requires.
const fcmScope = "https://www.googleapis.com/auth/firebase.messaging"

// Sender delivers a Message to a single device token. The FCM client implements
// it; a nil Sender means push is disabled and the service no-ops every delivery.
type Sender interface {
	Send(ctx context.Context, token string, msg Message) error
}

// FCMClient sends via the FCM HTTP v1 API, authenticating with a Google
// service-account credential. The credential's TokenSource mints and caches the
// OAuth2 access token; we only issue messages:send per token.
type FCMClient struct {
	projectID string
	http      *http.Client
	baseURL   string // overridable in tests
}

// NewFCMClient builds an FCM client from raw service-account JSON. Returns an
// error when the credential is unparseable. The token source refreshes itself,
// so the returned client is safe to keep for the process lifetime.
func NewFCMClient(projectID string, saJSON []byte, timeout time.Duration) (*FCMClient, error) {
	if projectID == "" {
		return nil, errors.New("fcm: project id is required")
	}
	conf, err := google.JWTConfigFromJSON(saJSON, fcmScope)
	if err != nil {
		return nil, fmt.Errorf("fcm: parse service account: %w", err)
	}
	hc := oauth2.NewClient(context.Background(), conf.TokenSource(context.Background()))
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	hc.Timeout = timeout
	return &FCMClient{
		projectID: projectID,
		http:      hc,
		baseURL:   "https://fcm.googleapis.com/v1",
	}, nil
}

// fcmRequest is the messages:send envelope.
type fcmRequest struct {
	Message fcmMessage `json:"message"`
}

type fcmMessage struct {
	Token        string            `json:"token"`
	Notification fcmNotification   `json:"notification"`
	Data         map[string]string `json:"data,omitempty"`
}

type fcmNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// Send issues one messages:send for token. On a 200 it returns nil; on FCM
// reporting the token unregistered it returns ErrTokenUnregistered so the caller
// prunes the row; any other non-2xx is a generic error.
func (c *FCMClient) Send(ctx context.Context, token string, msg Message) error {
	payload, err := json.Marshal(fcmRequest{Message: fcmMessage{
		Token:        token,
		Notification: fcmNotification{Title: msg.Title, Body: msg.Body},
		Data:         msg.Data,
	}})
	if err != nil {
		return fmt.Errorf("fcm: marshal: %w", err)
	}
	url := fmt.Sprintf("%s/projects/%s/messages:send", c.baseURL, c.projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("fcm: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("fcm: send: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	// A 404 or an UNREGISTERED error code means the token is dead — prune it.
	if resp.StatusCode == http.StatusNotFound || strings.Contains(string(body), "UNREGISTERED") {
		return ErrTokenUnregistered
	}
	return fmt.Errorf("fcm: send status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}
