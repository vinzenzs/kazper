// Package publicfeed implements the single secret-gated, unauthenticated read
// endpoint Kazper exposes: GET /public/race-feed. It projects only a curated,
// non-PII slice of the active macrocycle's A-race (name, date, countdown) for an
// external Strapi shield to cache and re-serve publicly (public-race-feed). It
// reads existing macrocycle/races data read-only and holds no PII.
package publicfeed

// Race is the non-PII race projection — name and date only.
type Race struct {
	Name     string `json:"name"`
	RaceDate string `json:"race_date"` // YYYY-MM-DD
}

// Feed is the curated public response. Both fields are null when there is no
// active anchored race, so a consuming page degrades gracefully rather than
// erroring.
type Feed struct {
	Race          *Race `json:"race"`
	DaysRemaining *int  `json:"days_remaining"`
}
