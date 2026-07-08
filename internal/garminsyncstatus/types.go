// Package garminsyncstatus records Garmin sync runs and exposes sync freshness
// (per add-garmin-connect-and-sync-status). The garmin-bridge opens a run before
// a `/sync` and closes it (success|error) after; the app and coach read
// GET /garmin/sync-status to answer "is my Garmin data current?". This is the
// backend's authoritative sync history — distinct from devices.last_sync_at,
// which is the watch's own field. A storage + read primitive: no synthesis.
package garminsyncstatus

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Status is the lifecycle of a sync run. A run starts `running` and is closed to
// `success`, `error`, or `partial`. `partial` denotes a multi-day run (a
// backfill) that completed with one or more isolated per-day failures — distinct
// from `error`, a run-level failure. Kept in sync with the sync_runs.status
// CHECK constraint.
type Status string

const (
	StatusRunning Status = "running"
	StatusSuccess Status = "success"
	StatusError   Status = "error"
	StatusPartial Status = "partial"
)

// ValidCloseStatus reports whether s is a terminal status a PATCH may set
// (`running` is the open-state default and cannot be PATCHed in).
func ValidCloseStatus(s string) bool {
	switch Status(s) {
	case StatusSuccess, StatusError, StatusPartial:
		return true
	default:
		return false
	}
}

// SyncRun mirrors a sync_runs row. Window dates serialize as YYYY-MM-DD strings
// (nullable); FinishedAt/Error are nil while running or on success respectively.
type SyncRun struct {
	ID         uuid.UUID       `json:"id"`
	StartedAt  time.Time       `json:"started_at"`
	FinishedAt *time.Time      `json:"finished_at,omitempty"`
	Status     Status          `json:"status"`
	WindowFrom *string         `json:"window_from,omitempty"`
	WindowTo   *string         `json:"window_to,omitempty"`
	Error      *string         `json:"error,omitempty"`
	// Summary is the roll-up of a multi-day/long job (a backfill's
	// days_total/days_ok/days_failed plus per-day results), recorded on close so
	// a non-blocking job's outcome is readable via sync-status. Null otherwise.
	Summary   json.RawMessage `json:"summary,omitempty" swaggertype:"object"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// SyncStatus is the GET /garmin/sync-status response: the most recent run, the
// timestamp of the newest successful run (independent of `latest` so a failed
// latest still shows when data was last good), and a derived staleness flag.
type SyncStatus struct {
	Latest           *SyncRun   `json:"latest"`
	LastSuccessfulAt *time.Time `json:"last_successful_at"`
	IsStale          bool       `json:"is_stale"`
}
