// Package shoppinglist persists a single global shopping checklist. It is
// deliberately the dumbest store that works: ordered text items with a checked
// flag and optional provenance (the recipe and plan date they came from). All
// merging, deduping and quantity arithmetic is the agent's job — the API never
// parses quantity_text or aggregates.
package shoppinglist

import (
	"time"

	"github.com/google/uuid"
)

// Item mirrors a shopping_items row. QuantityText is opaque free text;
// RecipeProductID is soft provenance (set NULL if the product is deleted);
// PlanDate is a bare date with no FK semantics.
type Item struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"name"`
	QuantityText    *string    `json:"quantity_text,omitempty"`
	RecipeProductID *uuid.UUID `json:"recipe_product_id,omitempty"`
	PlanDate        *string    `json:"plan_date,omitempty"` // YYYY-MM-DD
	Checked         bool       `json:"checked"`
	CheckedAt       *time.Time `json:"checked_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

const dateLayout = "2006-01-02"

const (
	maxNameLen   = 300
	maxBatchSize = 200
)
