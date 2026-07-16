package agenttools

import (
	"encoding/json"
	"net/url"
)

// Heat read — a planned session's heat load and suggested adjustment.

func init() { registerMCPDomain(heatSpecs()) }

// WorkoutHeatArgs is the input to workout_heat.
type WorkoutHeatArgs struct {
	WorkoutID string `json:"workout_id" jsonschema:"the PLANNED workout id to compute the heat picture for"`
}

func heatSpecs() []Spec {
	return []Spec{
		{
			Name: "workout_heat",
			Description: "Get the heat picture for a PLANNED session: how hot it will effectively be, how " +
				"heat-adapted the athlete currently is, and how much to back off. " +
				"Returns `heat_load_c` (a °C-equivalent composite: heat index from temperature and humidity, " +
				"plus a solar penalty scaled down by cloud cover, minus bounded wind cooling — each term " +
				"echoed), an `acclimatization` level DERIVED from the athlete's own recent outdoor sessions " +
				"(low / medium / good, with the qualifying workout ids so the level traces back to real " +
				"rides), a suggested percentage reduction off the effective baseline (FTP for bike, " +
				"threshold pace for run — note a run's suggested pace is a LARGER sec/km number, i.e. " +
				"slower), and a fluid note. " +
				"The forecast is taken at the location that date resolves to (travel period, else the " +
				"configured home) and the resolved name is echoed — if it names the wrong city, the athlete " +
				"has unlogged travel: log it with `log_location_period`. " +
				"THIS IS A HEURISTIC, not WBGT and not physiology: there is no solar sensor (cloud cover is " +
				"the proxy) and the constants are v1. Present it as a starting point for a conversation, " +
				"never as a measurement. " +
				"STRICTLY ADVISORY — this endpoint writes nothing. To act on a suggestion, propose the " +
				"specific edits to the scheduled workout and apply them through the normal workout/template " +
				"tools once the athlete confirms. Never silently rewrite a session's targets. " +
				"Honest degradations, all 200s: `not_applicable: true` (the session is indoor — no weather " +
				"applies); `assumed_outdoor: true` (the session's environment is unstated, so outdoor was " +
				"assumed — say so); `reason: \"location_unconfigured\"` (no travel period and no configured " +
				"home); `reason: \"weather_unavailable\"` (the forecast could not be fetched — say the " +
				"forecast is unavailable, do NOT guess the weather). A completed workout returns 409: this " +
				"is a pre-session question. Read-only; no idempotency-key is sent.",
			SchemaType: WorkoutHeatArgs{},
			Tier:       TierRead,
			Build: func(in json.RawMessage) (HTTPCall, error) {
				var a WorkoutHeatArgs
				if err := DecodeInto(in, &a); err != nil {
					return HTTPCall{}, err
				}
				return HTTPCall{
					Method: "GET",
					Path:   "/workouts/" + url.PathEscape(a.WorkoutID) + "/heat",
				}, nil
			},
		},
	}
}
