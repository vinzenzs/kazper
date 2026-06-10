package mcpserver

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type LogRecoveryMetricsArgs struct {
	Date               string   `json:"date" jsonschema:"the calendar day YYYY-MM-DD (the snapshot identity; re-posting the same date updates in place)"`
	SleepSeconds       *int     `json:"sleep_seconds,omitempty" jsonschema:"total sleep duration in seconds (> 0)"`
	SleepScore         *int     `json:"sleep_score,omitempty" jsonschema:"sleep score 0..100"`
	HRVMs              *float64 `json:"hrv_ms,omitempty" jsonschema:"overnight heart-rate variability in milliseconds (> 0)"`
	RestingHR          *int     `json:"resting_hr,omitempty" jsonschema:"resting heart rate in bpm (> 0)"`
	StressAvg          *int     `json:"stress_avg,omitempty" jsonschema:"average daily stress 0..100"`
	BodyBatteryCharged *int     `json:"body_battery_charged,omitempty" jsonschema:"body battery charged over the day 0..100"`
	BodyBatteryDrained *int     `json:"body_battery_drained,omitempty" jsonschema:"body battery drained over the day 0..100"`
	TrainingReadiness  *int     `json:"training_readiness,omitempty" jsonschema:"training readiness score 0..100"`
	IdempotencyKey     string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args"`
}

type ListRecoveryMetricsArgs struct {
	From string `json:"from" jsonschema:"inclusive lower bound date YYYY-MM-DD"`
	To   string `json:"to" jsonschema:"inclusive upper bound date YYYY-MM-DD; max 92 days from 'from'"`
}

type GetRecoveryMetricsArgs struct {
	Date string `json:"date" jsonschema:"the snapshot date YYYY-MM-DD"`
}

type DeleteRecoveryMetricsArgs struct {
	Date           string `json:"date" jsonschema:"the snapshot date YYYY-MM-DD to delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

func handleLogRecoveryMetrics(ctx context.Context, c *apiClient, args LogRecoveryMetricsArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		Date               string   `json:"date"`
		SleepSeconds       *int     `json:"sleep_seconds,omitempty"`
		SleepScore         *int     `json:"sleep_score,omitempty"`
		HRVMs              *float64 `json:"hrv_ms,omitempty"`
		RestingHR          *int     `json:"resting_hr,omitempty"`
		StressAvg          *int     `json:"stress_avg,omitempty"`
		BodyBatteryCharged *int     `json:"body_battery_charged,omitempty"`
		BodyBatteryDrained *int     `json:"body_battery_drained,omitempty"`
		TrainingReadiness  *int     `json:"training_readiness,omitempty"`
	}{
		Date:               args.Date,
		SleepSeconds:       args.SleepSeconds,
		SleepScore:         args.SleepScore,
		HRVMs:              args.HRVMs,
		RestingHR:          args.RestingHR,
		StressAvg:          args.StressAvg,
		BodyBatteryCharged: args.BodyBatteryCharged,
		BodyBatteryDrained: args.BodyBatteryDrained,
		TrainingReadiness:  args.TrainingReadiness,
	})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "log_recovery_metrics", args)
	status, respBody, err := c.Post(ctx, "/recovery-metrics", nil, body, key)
	return toToolResult(status, respBody, err)
}

func handleListRecoveryMetrics(ctx context.Context, c *apiClient, args ListRecoveryMetricsArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("from", args.From)
	q.Set("to", args.To)
	status, body, err := c.Get(ctx, "/recovery-metrics", q)
	return toToolResult(status, body, err)
}

func handleGetRecoveryMetrics(ctx context.Context, c *apiClient, args GetRecoveryMetricsArgs) *mcp.CallToolResult {
	status, body, err := c.Get(ctx, "/recovery-metrics/"+url.PathEscape(args.Date), nil)
	return toToolResult(status, body, err)
}

func handleDeleteRecoveryMetrics(ctx context.Context, c *apiClient, args DeleteRecoveryMetricsArgs) *mcp.CallToolResult {
	key := effectiveIdempotencyKey(args.IdempotencyKey, "delete_recovery_metrics", args)
	status, respBody, err := c.Delete(ctx, "/recovery-metrics/"+url.PathEscape(args.Date), key)
	if err == nil && status == 204 {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: ""}}}
	}
	return toToolResult(status, respBody, err)
}

func registerRecoveryMetricsTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "log_recovery_metrics",
		Description: "Record (or replace) the daily recovery snapshot for a date — sleep, HRV, resting HR, " +
			"stress, body battery, training readiness. One snapshot per calendar day, keyed by `date`; " +
			"re-posting the same date overwrites it. Every metric is optional. This is the recovery context " +
			"for deciding whether today's deficit / training load is tolerable.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args LogRecoveryMetricsArgs) (*mcp.CallToolResult, any, error) {
		return handleLogRecoveryMetrics(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_recovery_metrics",
		Description: "List daily recovery snapshots whose date falls in the inclusive [from, to] window " +
			"(YYYY-MM-DD, max 92 days). Use for trend questions ('how has my HRV trended this week?').",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args ListRecoveryMetricsArgs) (*mcp.CallToolResult, any, error) {
		return handleListRecoveryMetrics(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_recovery_metrics",
		Description: "Fetch the recovery snapshot for a single date (YYYY-MM-DD).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GetRecoveryMetricsArgs) (*mcp.CallToolResult, any, error) {
		return handleGetRecoveryMetrics(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_recovery_metrics",
		Description: "Delete the recovery snapshot for a date. Returns an empty result on success.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args DeleteRecoveryMetricsArgs) (*mcp.CallToolResult, any, error) {
		return handleDeleteRecoveryMetrics(ctx, c, args), nil, nil
	})
}
