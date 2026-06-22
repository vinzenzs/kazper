// Package push registers Android device push tokens and delivers notifications
// via FCM (per add-garmin-relogin-push). Its one trigger today is Garmin
// relogin: when a sync run is closed `error` and the Garmin token is absent, the
// backend infers that re-authentication is needed and sends a single push to
// every registered device. A one-row latch keeps that to one notification per
// outage; it is cleared when a fresh token is stored or a sync run succeeds.
//
// Push is opt-in: with no FCM credential configured the sender is nil and every
// delivery is a silent no-op, while token registration still persists so a later
// configuration change can deliver without re-pairing.
package push

import "time"

// PushToken mirrors a push_tokens row: an opaque FCM registration token for one
// device. Tokens rotate; registration upserts by the token string.
type PushToken struct {
	ID        string    `json:"id"`
	Token     string    `json:"token"`
	Platform  string    `json:"platform"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ReloginLatch is the single-row guard against re-notifying within one outage.
type ReloginLatch struct {
	Notified   bool       `json:"notified"`
	NotifiedAt *time.Time `json:"notified_at,omitempty"`
}

// Message is a delivered notification: a tray notification (title/body) plus a
// data payload the app uses to route (e.g. action="garmin_relogin").
type Message struct {
	Title string
	Body  string
	Data  map[string]string
}

// reloginMessage is the notification sent when Garmin re-authentication is
// needed. The data action lets the companion deep-link straight into the login
// flow rather than a generic screen.
func reloginMessage() Message {
	return Message{
		Title: "Garmin re-login needed",
		Body:  "Kazper can't sync your Garmin data until you sign in again.",
		Data:  map[string]string{"action": "garmin_relogin"},
	}
}
