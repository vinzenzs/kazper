package agenttools

import (
	"encoding/json"
	"net/url"
)

// Macrocycle tools — the season-level periodization layer the coach reads and
// edits (add-macrocycle-planning). A macrocycle is a named, dated season that
// orders training-phases into a yearly progression toward a goal race. It is
// planning/visualization + coach-context only: it never enters the goals
// resolver or plan materialization. Membership lives on the phase (set via the
// create_phase / update_phase macrocycle_id field); get_macrocycle returns the
// season with its ordered member phases.

func init() { registerMCPDomain(macrocycleSpecs()) }

// CreateMacrocycleArgs is the input to create_macrocycle.
type CreateMacrocycleArgs struct {
	Name           string  `json:"name" jsonschema:"season name (user-chosen, e.g. '2026 road season')"`
	StartDate      string  `json:"start_date" jsonschema:"inclusive season start date YYYY-MM-DD"`
	EndDate        string  `json:"end_date" jsonschema:"inclusive season end date YYYY-MM-DD (must be >= start_date)"`
	RaceID         *string `json:"race_id,omitempty" jsonschema:"optional UUID of the goal race the season peaks for (the A-race); resolves race_name on read"`
	Methodology    *string `json:"methodology,omitempty" jsonschema:"optional curated Markdown 'why this whole arc' prose the coach reads; distinct from operational notes"`
	Notes          *string `json:"notes,omitempty" jsonschema:"optional free-text operational notes"`
	IdempotencyKey string  `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

// ListMacrocyclesArgs is the (empty) input to list_macrocycles.
type ListMacrocyclesArgs struct{}

// GetMacrocycleArgs is the input to get_macrocycle.
type GetMacrocycleArgs struct {
	MacrocycleID string `json:"macrocycle_id" jsonschema:"macrocycle UUID"`
}

// UpdateMacrocycleArgs is the input to update_macrocycle.
type UpdateMacrocycleArgs struct {
	MacrocycleID   string  `json:"macrocycle_id" jsonschema:"macrocycle UUID"`
	Name           *string `json:"name,omitempty"`
	StartDate      *string `json:"start_date,omitempty"`
	EndDate        *string `json:"end_date,omitempty"`
	RaceID         *string `json:"race_id,omitempty" jsonschema:"empty string clears the race anchor, UUID string sets it, missing leaves unchanged"`
	Methodology    *string `json:"methodology,omitempty" jsonschema:"replaces wholesale when supplied, leaves unchanged when omitted"`
	Notes          *string `json:"notes,omitempty"`
	IdempotencyKey string  `json:"idempotency_key,omitempty"`
}

// DeleteMacrocycleArgs is the input to delete_macrocycle.
type DeleteMacrocycleArgs struct {
	MacrocycleID   string `json:"macrocycle_id" jsonschema:"macrocycle UUID"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

func macrocycleSpecs() []Spec {
	return []Spec{
		{
			Name: "create_macrocycle",
			Description: "Create a macrocycle: a named, dated training season that orders training-phases into a yearly progression. " +
				"Optionally anchored to a goal race via race_id (the A-race the season peaks for). " +
				"Planning/visualization only — a macrocycle does NOT change daily nutrition goals or materialize workouts. " +
				"Link phases into the season afterward with create_phase / update_phase's macrocycle_id + macrocycle_ordinal.",
			SchemaType: CreateMacrocycleArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a CreateMacrocycleArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				payload := struct {
					Name        string  `json:"name"`
					StartDate   string  `json:"start_date"`
					EndDate     string  `json:"end_date"`
					RaceID      *string `json:"race_id,omitempty"`
					Methodology *string `json:"methodology,omitempty"`
					Notes       *string `json:"notes,omitempty"`
				}{
					Name:        a.Name,
					StartDate:   a.StartDate,
					EndDate:     a.EndDate,
					RaceID:      a.RaceID,
					Methodology: a.Methodology,
					Notes:       a.Notes,
				}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/macrocycles", Body: body}, nil
			},
		},
		{
			Name:        "list_macrocycles",
			Description: "List every macrocycle (season) ordered by start_date descending. Entries carry race_name (resolved, or null) and omit the nested member phases — fetch a macrocycle by id for its progression.",
			SchemaType:  ListMacrocyclesArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				return HTTPCall{Method: "GET", Path: "/macrocycles"}, nil
			},
		},
		{
			Name:        "get_macrocycle",
			Description: "Fetch a macrocycle by UUID, including its ordered member phases (the periods) — each with its macrocycle_ordinal and per-period progression targets (target_weekly_tss, target_weekly_hours). This is the yearly-progression view.",
			SchemaType:  GetMacrocycleArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GetMacrocycleArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "GET", Path: "/macrocycles/" + url.PathEscape(a.MacrocycleID)}, nil
			},
		},
		{
			Name: "update_macrocycle",
			Description: "Partially update a macrocycle. Tri-state on race_id: empty string clears the race anchor, a UUID string sets a new one, omitting the field leaves it unchanged. " +
				"Other supplied fields (name, start_date, end_date, methodology, notes) replace wholesale; omitted fields are unchanged.",
			SchemaType: UpdateMacrocycleArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a UpdateMacrocycleArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				payload := struct {
					Name        *string `json:"name,omitempty"`
					StartDate   *string `json:"start_date,omitempty"`
					EndDate     *string `json:"end_date,omitempty"`
					RaceID      *string `json:"race_id,omitempty"`
					Methodology *string `json:"methodology,omitempty"`
					Notes       *string `json:"notes,omitempty"`
				}{
					Name:        a.Name,
					StartDate:   a.StartDate,
					EndDate:     a.EndDate,
					RaceID:      a.RaceID,
					Methodology: a.Methodology,
					Notes:       a.Notes,
				}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "PATCH", Path: "/macrocycles/" + url.PathEscape(a.MacrocycleID), Body: body}, nil
			},
		},
		{
			Name:        "delete_macrocycle",
			Description: "Delete a macrocycle. Its member phases survive — they are unlinked from the season (macrocycle_id set null) and continue to drive adherence unchanged.",
			SchemaType:  DeleteMacrocycleArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DeleteMacrocycleArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "DELETE", Path: "/macrocycles/" + url.PathEscape(a.MacrocycleID)}, nil
			},
		},
	}
}
