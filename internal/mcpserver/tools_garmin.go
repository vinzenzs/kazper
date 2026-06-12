package mcpserver

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GarminLoginArgs is intentionally empty: the bridge holds the credentials, so
// starting the login takes no arguments (design D3 of add-garmin-mcp-login).
type GarminLoginArgs struct{}

// GarminSubmitMFAArgs carries only the ephemeral 6-digit code — the single
// secret that ever transits the agent on this path (never the password/token).
type GarminSubmitMFAArgs struct {
	Code string `json:"code" jsonschema:"the 6-digit MFA code from the user's authenticator app or email"`
}

func handleGarminLogin(ctx context.Context, c *apiClient, _ GarminLoginArgs) *mcp.CallToolResult {
	// One HTTP call, no body, no idempotency key: starting an interactive login
	// is not a replayable write.
	status, body, err := c.Post(ctx, "/garmin/login", nil, nil, "")
	return toToolResult(status, body, err)
}

func handleGarminSubmitMFA(ctx context.Context, c *apiClient, args GarminSubmitMFAArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		Code string `json:"code"`
	}{Code: args.Code})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	status, respBody, err := c.Post(ctx, "/garmin/login/mfa", nil, body, "")
	return toToolResult(status, respBody, err)
}

// ----- scheduling (push the plan to the watch) -----

type GarminScheduleWorkoutArgs struct {
	WorkoutID      string `json:"workout_id" jsonschema:"a planned workout's UUID (must have a template)"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key; derived when omitted"`
}

type GarminUnscheduleWorkoutArgs struct {
	WorkoutID      string `json:"workout_id" jsonschema:"the workout UUID to remove from the Garmin calendar"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type GarminSchedulePlanArgs struct {
	PlanID         string  `json:"plan_id" jsonschema:"the training-plan UUID"`
	Scope          string  `json:"scope" jsonschema:"all, week, or range"`
	Week           *int    `json:"week,omitempty" jsonschema:"required when scope is week: the week ordinal"`
	From           *string `json:"from,omitempty" jsonschema:"required when scope is range: inclusive start YYYY-MM-DD"`
	To             *string `json:"to,omitempty" jsonschema:"required when scope is range: inclusive end YYYY-MM-DD"`
	IdempotencyKey string  `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type GarminListScheduledArgs struct {
	From string `json:"from" jsonschema:"inclusive lower bound YYYY-MM-DD"`
	To   string `json:"to" jsonschema:"inclusive upper bound YYYY-MM-DD"`
}

func handleGarminScheduleWorkout(ctx context.Context, c *apiClient, args GarminScheduleWorkoutArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		WorkoutID string `json:"workout_id"`
	}{args.WorkoutID})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "garmin_schedule_workout", args)
	status, resp, err := c.Post(ctx, "/garmin/schedule/workout", nil, body, key)
	return toToolResult(status, resp, err)
}

func handleGarminUnscheduleWorkout(ctx context.Context, c *apiClient, args GarminUnscheduleWorkoutArgs) *mcp.CallToolResult {
	key := effectiveIdempotencyKey(args.IdempotencyKey, "garmin_unschedule_workout", args)
	status, resp, err := c.Delete(ctx, "/garmin/schedule/workout/"+url.PathEscape(args.WorkoutID), key)
	return toToolResult(status, resp, err)
}

func handleGarminSchedulePlan(ctx context.Context, c *apiClient, args GarminSchedulePlanArgs) *mcp.CallToolResult {
	body, err := json.Marshal(map[string]any{"plan_id": args.PlanID, "scope": args.Scope, "week": args.Week, "from": args.From, "to": args.To})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "garmin_schedule_plan", args)
	status, resp, err := c.Post(ctx, "/garmin/schedule/plan", nil, body, key)
	return toToolResult(status, resp, err)
}

func handleGarminListScheduled(ctx context.Context, c *apiClient, args GarminListScheduledArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("from", args.From)
	q.Set("to", args.To)
	status, resp, err := c.Get(ctx, "/garmin/calendar", q)
	return toToolResult(status, resp, err)
}

func registerGarminTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "garmin_login",
		Description: "Start re-linking the user's Garmin account (renews the ~yearly-expiring Garmin token). " +
			"Takes no arguments — the bridge holds the credentials. If the result is `{\"needs_mfa\": true}`, " +
			"ask the user for the 6-digit code from their authenticator app, then call `garmin_submit_mfa` with it. " +
			"A `{\"logged_in\": true}` result means no code was needed and re-linking is already complete. " +
			"A `503 garmin_disabled` result means the Garmin integration is not configured on this server.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminLoginArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminLogin(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "garmin_submit_mfa",
		Description: "Complete a Garmin re-link by submitting the 6-digit MFA code the user read from their " +
			"authenticator. Call this only after `garmin_login` returned `{\"needs_mfa\": true}`. A " +
			"`{\"logged_in\": true}` result means the token was renewed; an error (e.g. `mfa_invalid`) means the " +
			"code was wrong or expired — call `garmin_login` again to restart.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminSubmitMFAArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminSubmitMFA(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "garmin_schedule_workout",
		Description: "Push one planned workout to the Garmin watch: compiles its template into a structured Garmin " +
			"workout, schedules it on the workout's date, and stores the Garmin ids. Re-pushing replaces the prior " +
			"calendar entry. The workout must be planned and have a template. 503 garmin_disabled when the bridge is off.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminScheduleWorkoutArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminScheduleWorkout(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "garmin_unschedule_workout",
		Description: "Remove a workout from the Garmin calendar and clear its stored Garmin ids. No-op success if it " +
			"was never scheduled.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminUnscheduleWorkoutArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminUnscheduleWorkout(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "garmin_schedule_plan",
		Description: "Push every planned workout in a plan scope (all, week, or range) to the watch in one call. " +
			"Per-workout failures are reported alongside successes, not fatal.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminSchedulePlanArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminSchedulePlan(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "garmin_list_scheduled",
		Description: "List the workouts scheduled on the Garmin calendar in a date range, for reconciliation.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GarminListScheduledArgs) (*mcp.CallToolResult, any, error) {
		return handleGarminListScheduled(ctx, c, args), nil, nil
	})
}
