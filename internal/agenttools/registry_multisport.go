package agenttools

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// Multisport workout-template tools (add-multisport-structured-workouts): the
// triathlon/brick library — an ordered list of per-sport segments + transitions
// — plus the action that compiles one to a single multisport Garmin workout and
// schedules it. Reuses the workout-template step schema (wtStep / wtDuration /
// wtTarget) for each segment's program.

func init() { registerMCPDomain(multisportSpecs()) }

// msSegment is one leg of a multisport session. A sport segment carries a sport
// and a step program; a transition (T1/T2) segment carries sport "transition"
// and only a duration.
type msSegment struct {
	Sport    string      `json:"sport" jsonschema:"segment sport: run | bike | swim | strength | yoga | mobility | other, or transition for a T1/T2 changeover"`
	Steps    []wtStep    `json:"steps,omitempty" jsonschema:"for a sport segment: its ordered step program (same model as a workout template), validated under this segment's sport"`
	Duration *wtDuration `json:"duration,omitempty" jsonschema:"for a transition segment only: the changeover end condition (typically kind lap_button)"`
}

type CreateMultisportTemplateArgs struct {
	Name           string      `json:"name" jsonschema:"human-readable session name, e.g. 'Olympic Tri Race Sim'"`
	Segments       []msSegment `json:"segments" jsonschema:"ordered segments: at least two sport segments, with optional transition segments between them (e.g. swim, T1, bike, T2, run)"`
	IdempotencyKey string      `json:"idempotency_key,omitempty" jsonschema:"optional retry key; derived from args when omitted"`
}

type ListMultisportTemplatesArgs struct{}

type GetMultisportTemplateArgs struct {
	ID string `json:"id" jsonschema:"the multisport template UUID"`
}

type DeleteMultisportTemplateArgs struct {
	ID             string `json:"id" jsonschema:"the multisport template UUID to delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type ScheduleMultisportArgs struct {
	MultisportTemplateID string `json:"multisport_template_id" jsonschema:"the multisport template UUID to compile and schedule"`
	Date                 string `json:"date" jsonschema:"the calendar day YYYY-MM-DD to schedule it on"`
	IdempotencyKey       string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; derived when omitted"`
}

func multisportSpecs() []Spec {
	return []Spec{
		{
			Name: "create_multisport_template",
			Description: "Create a multisport workout template — a triathlon or brick session modeled as an ordered " +
				"list of per-sport segments (swim → T1 → bike → T2 → run), plus transition segments. Needs at least two " +
				"sport segments; each segment's steps are validated under that segment's own sport (so a swim segment uses " +
				"swim_pace, a bike segment may carry a secondary_target). Compiled to a single auto-advancing Garmin " +
				"multisport workout by schedule_multisport.",
			SchemaType: CreateMultisportTemplateArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a CreateMultisportTemplateArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				body, err := json.Marshal(struct {
					Name     string      `json:"name"`
					Segments []msSegment `json:"segments"`
				}{a.Name, a.Segments})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/multisport-templates", Body: body}, nil
			},
		},
		{
			Name:        "list_multisport_templates",
			Description: "List all multisport workout templates (triathlon/brick sessions), newest first.",
			SchemaType:  ListMultisportTemplatesArgs{},
			Tier:        TierRead,
			Build: func(_ json.RawMessage) (HTTPCall, error) {
				return HTTPCall{Method: "GET", Path: "/multisport-templates"}, nil
			},
		},
		{
			Name:        "get_multisport_template",
			Description: "Fetch a single multisport template by id, including its full ordered segments and per-segment steps.",
			SchemaType:  GetMultisportTemplateArgs{},
			Tier:        TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a GetMultisportTemplateArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				if a.ID == "" {
					return HTTPCall{}, fmt.Errorf("id is required")
				}
				return HTTPCall{Method: "GET", Path: "/multisport-templates/" + url.PathEscape(a.ID)}, nil
			},
		},
		{
			Name:        "delete_multisport_template",
			Description: "Delete a multisport template by id. Returns an empty result on success.",
			SchemaType:  DeleteMultisportTemplateArgs{},
			Tier:        TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a DeleteMultisportTemplateArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				if a.ID == "" {
					return HTTPCall{}, fmt.Errorf("id is required")
				}
				return HTTPCall{Method: "DELETE", Path: "/multisport-templates/" + url.PathEscape(a.ID)}, nil
			},
		},
		{
			Name: "schedule_multisport",
			Description: "Compile a multisport template into one auto-advancing Garmin multisport workout (its segments " +
				"+ transitions, each sport segment's zone targets resolved by that segment's sport) and schedule it on a date. " +
				"Returns the Garmin workout and schedule ids.",
			SchemaType: ScheduleMultisportArgs{},
			Tier:       TierWriteAuto,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a ScheduleMultisportArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				if a.MultisportTemplateID == "" || a.Date == "" {
					return HTTPCall{}, fmt.Errorf("multisport_template_id and date are required")
				}
				body, err := json.Marshal(struct {
					MultisportTemplateID string `json:"multisport_template_id"`
					Date                 string `json:"date"`
				}{a.MultisportTemplateID, a.Date})
				if err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{Method: "POST", Path: "/garmin/schedule/multisport", Body: body}, nil
			},
		},
	}
}
