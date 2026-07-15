// Package supplements stores a dated log of supplement intakes (creatine, iron,
// vitamin D, magnesium, out-of-session caffeine). Distinct from meals (no macros),
// workout-fuel (not in-session) and coach-memory (not date-queryable). Unit-
// isolated: supplements feed no nutrition/hydration/energy total.
package supplements

import (
	"time"

	"github.com/google/uuid"
)

// Entry mirrors a supplement_entries row. Dose and DoseUnit are paired (both set
// or both nil); Note is optional. Timestamps are stored at full precision.
type Entry struct {
	ID        uuid.UUID `json:"id"`
	LoggedAt  time.Time `json:"logged_at"`
	Name      string    `json:"name"`
	Dose      *float64  `json:"dose,omitempty"`
	DoseUnit  *string   `json:"dose_unit,omitempty"`
	Note      *string   `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
