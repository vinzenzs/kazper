// Package devices mirrors Garmin's device inventory (watches, bike computers,
// scales) as slowly-changing, upsert-by-external-id rows — not date-keyed. It is
// reference/coaching context (battery + firmware nudges) and is unit-isolated:
// device fields never feed any nutrition/hydration/energy total.
package devices

import (
	"time"

	"github.com/google/uuid"
)

// Device mirrors a devices row. Identity is the backend `id`; `external_id` is
// the stable Garmin device id the upsert dedups on. Nullable fields are
// pointers with omitempty so absent stays distinct from a real zero.
type Device struct {
	ID              uuid.UUID  `json:"id"`
	ExternalID      string     `json:"external_id"`
	DisplayName     string     `json:"display_name"`
	Model           *string    `json:"model,omitempty"`
	LastSyncAt      *time.Time `json:"last_sync_at,omitempty"`
	BatteryPct      *float64   `json:"battery_pct,omitempty"`
	FirmwareVersion *string    `json:"firmware_version,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
