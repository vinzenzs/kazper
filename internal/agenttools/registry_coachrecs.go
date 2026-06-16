package agenttools

import (
	"encoding/json"
	"net/url"
)

// Coach recommendations — a thin dated log the coach writes as it advises and
// reads back for cross-session continuity (persist-coach-recommendations,
// priorities #6F). A storage primitive: the agent authors the recommendation;
// the API records and returns it verbatim and never generates one. Recording a
// recommendation does NOT change an enforced target — use the goal-override
// tools to change a number.

func init() { registerMCPDomain(coachRecsSpecs()) }

// LogCoachRecommendationArgs is the input to log_coach_recommendation.
type LogCoachRecommendationArgs struct {
	Date           string `json:"date" jsonschema:"the local date the advice applies to, YYYY-MM-DD"`
	Scope          string `json:"scope" jsonschema:"one of: fueling, training, recovery, race, general"`
	Recommendation string `json:"recommendation" jsonschema:"the recommendation text you synthesized; stored verbatim, required, non-empty"`
	Reason         string `json:"reason,omitempty" jsonschema:"optional rationale behind the recommendation"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args"`
}

// ListCoachRecommendationsArgs is the input to list_coach_recommendations.
type ListCoachRecommendationsArgs struct {
	From  string `json:"from" jsonschema:"inclusive start date YYYY-MM-DD"`
	To    string `json:"to" jsonschema:"inclusive end date YYYY-MM-DD"`
	TZ    string `json:"tz,omitempty" jsonschema:"IANA timezone (e.g. Europe/Berlin); if omitted, the REST server uses DEFAULT_USER_TZ"`
	Scope string `json:"scope,omitempty" jsonschema:"optional filter: fueling, training, recovery, race, or general"`
}

// GetCoachRecommendationArgs is the input to get_coach_recommendation.
type GetCoachRecommendationArgs struct {
	ID string `json:"id" jsonschema:"the id of the recommendation to fetch"`
}

// DeleteCoachRecommendationArgs is the input to delete_coach_recommendation.
type DeleteCoachRecommendationArgs struct {
	ID             string `json:"id" jsonschema:"the id of the recommendation to delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

func coachRecsSpecs() []Spec {
	return []Spec{
		{
			Name: "log_coach_recommendation",
			Description: "Record a coach recommendation you just synthesized to the dated log so it persists " +
				"across sessions (e.g. 'today's carb target is 220g because of tomorrow's long ride'). The text " +
				"is stored verbatim — this is a note, NOT an enforced target: to change an actual goal number, " +
				"use the goal-override tools. Scope is one of fueling, training, recovery, race, general.",
			SchemaType: LogCoachRecommendationArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var args LogCoachRecommendationArgs
				if err := DecodeInto(in, &args); err != nil {
					return HTTPCall{}, err
				}
				payload := map[string]any{
					"date":           args.Date,
					"scope":          args.Scope,
					"recommendation": args.Recommendation,
				}
				if args.Reason != "" {
					payload["reason"] = args.Reason
				}
				body, err := json.Marshal(payload)
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/coach/recommendations", Body: body}, nil
			},
		},
		{
			Name: "list_coach_recommendations",
			Description: "List coach recommendations whose date falls in [from, to] (inclusive local dates), " +
				"newest-first, optionally narrowed to one scope. Use this to ground on what you previously " +
				"advised before giving new advice.",
			SchemaType: ListCoachRecommendationsArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var args ListCoachRecommendationsArgs
				if err := DecodeInto(in, &args); err != nil {
					return HTTPCall{}, err
				}
				q := url.Values{}
				q.Set("from", args.From)
				q.Set("to", args.To)
				if args.TZ != "" {
					q.Set("tz", args.TZ)
				}
				if args.Scope != "" {
					q.Set("scope", args.Scope)
				}
				return HTTPCall{Method: "GET", Path: "/coach/recommendations", Query: q}, nil
			},
		},
		{
			Name:        "get_coach_recommendation",
			Description: "Fetch a single coach recommendation by id.",
			SchemaType:  GetCoachRecommendationArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var args GetCoachRecommendationArgs
				if err := DecodeInto(in, &args); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "GET", Path: "/coach/recommendations/" + url.PathEscape(args.ID)}, nil
			},
		},
		{
			Name: "delete_coach_recommendation",
			Description: "Delete a superseded or incorrect coach recommendation. Corrections are a delete " +
				"followed by a re-log (there is no edit).",
			SchemaType: DeleteCoachRecommendationArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var args DeleteCoachRecommendationArgs
				if err := DecodeInto(in, &args); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "DELETE", Path: "/coach/recommendations/" + url.PathEscape(args.ID)}, nil
			},
		},
	}
}
