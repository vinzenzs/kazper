package mcpserver

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type LogFitnessMetricsArgs struct {
	Date                     string   `json:"date" jsonschema:"the calendar day YYYY-MM-DD (the snapshot identity; re-posting the same date updates in place)"`
	VO2MaxRunning            *float64 `json:"vo2max_running,omitempty" jsonschema:"running VO2max (> 0)"`
	VO2MaxCycling            *float64 `json:"vo2max_cycling,omitempty" jsonschema:"cycling VO2max (> 0)"`
	RacePredictor5kSeconds   *int     `json:"race_predictor_5k_seconds,omitempty" jsonschema:"predicted 5k time in SECONDS (> 0)"`
	RacePredictor10kSeconds  *int     `json:"race_predictor_10k_seconds,omitempty" jsonschema:"predicted 10k time in SECONDS (> 0)"`
	RacePredictorHalfSeconds *int     `json:"race_predictor_half_seconds,omitempty" jsonschema:"predicted half-marathon time in SECONDS (> 0)"`
	RacePredictorFullSeconds *int     `json:"race_predictor_full_seconds,omitempty" jsonschema:"predicted marathon time in SECONDS (> 0)"`
	AcuteLoad                *float64 `json:"acute_load,omitempty" jsonschema:"acute (7-day) training load (>= 0)"`
	ChronicLoad              *float64 `json:"chronic_load,omitempty" jsonschema:"chronic (28-day) training load (>= 0)"`
	IdempotencyKey           string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args"`
}

type ListFitnessMetricsArgs struct {
	From string `json:"from" jsonschema:"inclusive lower bound date YYYY-MM-DD"`
	To   string `json:"to" jsonschema:"inclusive upper bound date YYYY-MM-DD; max 92 days from 'from'"`
}

type GetFitnessMetricsArgs struct {
	Date string `json:"date" jsonschema:"the snapshot date YYYY-MM-DD"`
}

type DeleteFitnessMetricsArgs struct {
	Date           string `json:"date" jsonschema:"the snapshot date YYYY-MM-DD to delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

func handleLogFitnessMetrics(ctx context.Context, c *apiClient, args LogFitnessMetricsArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		Date                     string   `json:"date"`
		VO2MaxRunning            *float64 `json:"vo2max_running,omitempty"`
		VO2MaxCycling            *float64 `json:"vo2max_cycling,omitempty"`
		RacePredictor5kSeconds   *int     `json:"race_predictor_5k_seconds,omitempty"`
		RacePredictor10kSeconds  *int     `json:"race_predictor_10k_seconds,omitempty"`
		RacePredictorHalfSeconds *int     `json:"race_predictor_half_seconds,omitempty"`
		RacePredictorFullSeconds *int     `json:"race_predictor_full_seconds,omitempty"`
		AcuteLoad                *float64 `json:"acute_load,omitempty"`
		ChronicLoad              *float64 `json:"chronic_load,omitempty"`
	}{
		Date:                     args.Date,
		VO2MaxRunning:            args.VO2MaxRunning,
		VO2MaxCycling:            args.VO2MaxCycling,
		RacePredictor5kSeconds:   args.RacePredictor5kSeconds,
		RacePredictor10kSeconds:  args.RacePredictor10kSeconds,
		RacePredictorHalfSeconds: args.RacePredictorHalfSeconds,
		RacePredictorFullSeconds: args.RacePredictorFullSeconds,
		AcuteLoad:                args.AcuteLoad,
		ChronicLoad:              args.ChronicLoad,
	})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "log_fitness_metrics", args)
	status, respBody, err := c.Post(ctx, "/fitness-metrics", nil, body, key)
	return toToolResult(status, respBody, err)
}

func handleListFitnessMetrics(ctx context.Context, c *apiClient, args ListFitnessMetricsArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("from", args.From)
	q.Set("to", args.To)
	status, body, err := c.Get(ctx, "/fitness-metrics", q)
	return toToolResult(status, body, err)
}

func handleGetFitnessMetrics(ctx context.Context, c *apiClient, args GetFitnessMetricsArgs) *mcp.CallToolResult {
	status, body, err := c.Get(ctx, "/fitness-metrics/"+url.PathEscape(args.Date), nil)
	return toToolResult(status, body, err)
}

func handleDeleteFitnessMetrics(ctx context.Context, c *apiClient, args DeleteFitnessMetricsArgs) *mcp.CallToolResult {
	key := effectiveIdempotencyKey(args.IdempotencyKey, "delete_fitness_metrics", args)
	status, respBody, err := c.Delete(ctx, "/fitness-metrics/"+url.PathEscape(args.Date), key)
	if err == nil && status == 204 {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: ""}}}
	}
	return toToolResult(status, respBody, err)
}

func registerFitnessMetricsTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "log_fitness_metrics",
		Description: "Record (or replace) the daily fitness snapshot for a date — VO2max (run/bike), race " +
			"predictions, acute/chronic training load. One snapshot per calendar day, keyed by `date`. " +
			"Race predictions are SECONDS (format h:mm:ss yourself). Acute:chronic ratio = acute_load / " +
			"chronic_load (compute it; it isn't stored).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args LogFitnessMetricsArgs) (*mcp.CallToolResult, any, error) {
		return handleLogFitnessMetrics(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_fitness_metrics",
		Description: "List daily fitness snapshots whose date falls in the inclusive [from, to] window " +
			"(YYYY-MM-DD, max 92 days). Use for fitness-trend questions (VO2max progression, load ramp).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args ListFitnessMetricsArgs) (*mcp.CallToolResult, any, error) {
		return handleListFitnessMetrics(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_fitness_metrics",
		Description: "Fetch the fitness snapshot for a single date (YYYY-MM-DD).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GetFitnessMetricsArgs) (*mcp.CallToolResult, any, error) {
		return handleGetFitnessMetrics(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_fitness_metrics",
		Description: "Delete the fitness snapshot for a date. Returns an empty result on success.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args DeleteFitnessMetricsArgs) (*mcp.CallToolResult, any, error) {
		return handleDeleteFitnessMetrics(ctx, c, args), nil, nil
	})
}
