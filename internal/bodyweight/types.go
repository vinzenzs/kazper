// Package bodyweight stores body-weight measurements and computes rolling-average
// trends over them. Sister to the meals / hydration / workouts capture surfaces;
// unit-isolated (kg, never mixed with grams or ml).
package bodyweight

import (
	"time"

	"github.com/google/uuid"
)

// Entry mirrors a body_weight_entries row.
type Entry struct {
	ID         uuid.UUID `json:"id"`
	LoggedAt   time.Time `json:"logged_at"`
	WeightKg   float64   `json:"weight_kg"`
	BodyFatPct *float64  `json:"body_fat_pct,omitempty"`
	// Smart-scale biometrics (Garmin full weigh-in). All nullable.
	MuscleMassKg *float64  `json:"muscle_mass_kg,omitempty"`
	BodyWaterPct *float64  `json:"body_water_pct,omitempty"`
	BoneMassKg   *float64  `json:"bone_mass_kg,omitempty"`
	BMI          *float64  `json:"bmi,omitempty"`
	Note         *string   `json:"note,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
