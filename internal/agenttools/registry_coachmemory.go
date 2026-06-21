package agenttools

import (
	"encoding/json"
	"net/url"
)

// Coach memory — a durable, athlete-scoped store the coach writes facts /
// preferences / constraints / observations / recommendations into and both
// surfaces read at grounding time (widen-coach-recs-to-memory; supersedes the
// coach-recommendations tools). A storage primitive: the agent authors the
// text; the API records and returns it verbatim and never synthesizes one.
// Writing memory does NOT change an enforced target — use the goal-override
// tools to change a number. Writes are explicit: only on a user "remember…".
//
// Memory also folds into get_daily_context / get_training_context, so the agent
// usually GROUNDS on memory there and only calls list/get for a targeted lookup.

func init() { registerMCPDomain(coachMemorySpecs()) }

// RememberArgs is the input to remember.
type RememberArgs struct {
	Kind      string `json:"kind" jsonschema:"one of: fact, preference, constraint, observation, recommendation"`
	Text      string `json:"text" jsonschema:"the memory text; stored verbatim, required, non-empty"`
	Reason    string `json:"reason,omitempty" jsonschema:"optional rationale behind the item"`
	Scope     string `json:"scope,omitempty" jsonschema:"optional tag: fueling, training, recovery, race, general"`
	Date      string `json:"date,omitempty" jsonschema:"YYYY-MM-DD; REQUIRED when kind=recommendation, omit for standing items"`
	ExpiresAt string `json:"expires_at,omitempty" jsonschema:"YYYY-MM-DD; hard cutoff after which the item drops out of grounding"`
	ReviewAt  string `json:"review_at,omitempty" jsonschema:"YYYY-MM-DD; soft review date — past it, the item is flagged needs_review"`

	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args"`
}

// ListCoachMemoryArgs is the input to list_coach_memory.
type ListCoachMemoryArgs struct {
	From            string `json:"from" jsonschema:"inclusive start date YYYY-MM-DD (recommendation window)"`
	To              string `json:"to" jsonschema:"inclusive end date YYYY-MM-DD (recommendation window)"`
	TZ              string `json:"tz,omitempty" jsonschema:"IANA timezone; if omitted, the REST server uses DEFAULT_USER_TZ"`
	Kind            string `json:"kind,omitempty" jsonschema:"optional filter: fact, preference, constraint, observation, recommendation"`
	Scope           string `json:"scope,omitempty" jsonschema:"optional filter: fueling, training, recovery, race, general"`
	IncludeArchived bool   `json:"include_archived,omitempty" jsonschema:"include archived items (default false)"`
}

// GetCoachMemoryArgs is the input to get_coach_memory.
type GetCoachMemoryArgs struct {
	ID string `json:"id" jsonschema:"the id of the memory item to fetch"`
}

// UpdateCoachMemoryArgs is the input to update_coach_memory (lifecycle only).
type UpdateCoachMemoryArgs struct {
	ID             string `json:"id" jsonschema:"the id of the memory item to update"`
	ReviewAt       string `json:"review_at,omitempty" jsonschema:"new YYYY-MM-DD review date (e.g. to confirm a fact is still true)"`
	ExpiresAt      string `json:"expires_at,omitempty" jsonschema:"new YYYY-MM-DD hard-expiry date"`
	Status         string `json:"status,omitempty" jsonschema:"active or archived"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

// DeleteCoachMemoryArgs is the input to delete_coach_memory.
type DeleteCoachMemoryArgs struct {
	ID             string `json:"id" jsonschema:"the id of the memory item to delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

func coachMemorySpecs() []Spec {
	return []Spec{
		{
			Name: "remember",
			Description: "Record a durable coach-memory item the user asked you to remember — a fact, " +
				"preference, constraint, observation, or recommendation. Text is stored verbatim; this is a " +
				"note, NOT an enforced target (use the goal-override tools to change a number). A recommendation " +
				"requires a date; standing items (fact/preference/constraint/observation) may be dateless. Only " +
				"record memory when the user explicitly asks you to.",
			SchemaType: RememberArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var args RememberArgs
				if err := DecodeInto(in, &args); err != nil {
					return HTTPCall{}, err
				}
				payload := map[string]any{"kind": args.Kind, "text": args.Text}
				if args.Reason != "" {
					payload["reason"] = args.Reason
				}
				if args.Scope != "" {
					payload["scope"] = args.Scope
				}
				if args.Date != "" {
					payload["date"] = args.Date
				}
				if args.ExpiresAt != "" {
					payload["expires_at"] = args.ExpiresAt
				}
				if args.ReviewAt != "" {
					payload["review_at"] = args.ReviewAt
				}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/coach/memory", Body: body}, nil
			},
		},
		{
			Name: "list_coach_memory",
			Description: "List coach memory: standing items (fact/preference/constraint/observation) regardless " +
				"of the window, plus recommendations whose date falls in [from, to], newest-first. Archived and " +
				"expired items are excluded unless include_archived=true. Memory also rides get_daily_context / " +
				"get_training_context — prefer those for grounding; use this for a targeted lookup.",
			SchemaType: ListCoachMemoryArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var args ListCoachMemoryArgs
				if err := DecodeInto(in, &args); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", args.From)
				q.Set("to", args.To)
				if args.TZ != "" {
					q.Set("tz", args.TZ)
				}
				if args.Kind != "" {
					q.Set("kind", args.Kind)
				}
				if args.Scope != "" {
					q.Set("scope", args.Scope)
				}
				if args.IncludeArchived {
					q.Set("include_archived", "true")
				}
				return HTTPCall{Method: "GET", Path: "/coach/memory", Query: q}, nil
			},
		},
		{
			Name:        "get_coach_memory",
			Description: "Fetch a single coach-memory item by id.",
			SchemaType:  GetCoachMemoryArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var args GetCoachMemoryArgs
				if err := DecodeInto(in, &args); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "GET", Path: "/coach/memory/" + url.PathEscape(args.ID)}, nil
			},
		},
		{
			Name: "update_coach_memory",
			Description: "Update a memory item's lifecycle in place — review_at (e.g. to confirm a fact is still " +
				"true and push the next review out), expires_at, or status (active/archived). Preserves the " +
				"item's original created_at. Content (text/kind/scope/date) is immutable; correct it with a " +
				"delete + re-remember.",
			SchemaType: UpdateCoachMemoryArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var args UpdateCoachMemoryArgs
				if err := DecodeInto(in, &args); err != nil {
					return HTTPCall{}, err
				}
				payload := map[string]any{}
				if args.ReviewAt != "" {
					payload["review_at"] = args.ReviewAt
				}
				if args.ExpiresAt != "" {
					payload["expires_at"] = args.ExpiresAt
				}
				if args.Status != "" {
					payload["status"] = args.Status
				}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "PATCH", Path: "/coach/memory/" + url.PathEscape(args.ID), Body: body}, nil
			},
		},
		{
			Name: "delete_coach_memory",
			Description: "Delete a coach-memory item by id. A content correction is a delete followed by a " +
				"re-remember.",
			SchemaType: DeleteCoachMemoryArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var args DeleteCoachMemoryArgs
				if err := DecodeInto(in, &args); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "DELETE", Path: "/coach/memory/" + url.PathEscape(args.ID)}, nil
			},
		},
	}
}
