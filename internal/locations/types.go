// Package locations answers "where is the athlete on date X" for the weather
// arc: home by default, elsewhere while travelling.
//
// Deliberately city-grade, not GPS: heat planning needs a coordinate to ask a
// forecast about, not a track (GPS remains a standing non-goal). Home lives in
// config (HOME_LAT/HOME_LON) because it is quasi-static infrastructure; this
// package carries only the travel layer, as dated ranges.
//
// Nothing downstream is precomputed, so a session scheduled months ago follows
// a trip the moment that trip is logged.
package locations

import (
	"time"

	"github.com/google/uuid"
)

// Period mirrors a location_periods row: an inclusive date range at a place.
type Period struct {
	ID        uuid.UUID `json:"id"`
	StartDate string    `json:"start_date"` // YYYY-MM-DD, inclusive
	EndDate   string    `json:"end_date"`   // YYYY-MM-DD, inclusive
	Name      string    `json:"name"`
	Lat       float64   `json:"lat"`
	Lon       float64   `json:"lon"`
	Note      *string   `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Source states which rule produced a resolution — the difference between "the
// forecast used your training camp" and "the forecast fell back to home".
type Source string

const (
	SourceTravel Source = "travel"
	SourceHome   Source = "home"
)

// HomeName is the Name a home-fallback resolution carries. Home has no row, so
// it has no user-supplied name.
const HomeName = "home"

// Resolved is one date's effective location: the response shape of
// GET /locations/resolve and the value every weather consumer reads.
type Resolved struct {
	Date   string  `json:"date"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	Name   string  `json:"name"`
	Source Source  `json:"source"`
}
