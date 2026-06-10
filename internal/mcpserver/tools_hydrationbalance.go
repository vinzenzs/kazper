package mcpserver

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type LogHydrationBalanceArgs struct {
	Date             string   `json:"date" jsonschema:"the calendar day YYYY-MM-DD (the snapshot identity; re-posting the same date updates in place)"`
	SweatLossML      *float64 `json:"sweat_loss_ml,omitempty" jsonschema:"estimated daily sweat loss in millilitres (> 0)"`
	ActivityIntakeML *float64 `json:"activity_intake_ml,omitempty" jsonschema:"fluid taken during activities that day in millilitres (>= 0; a real 0 means sweated but drank nothing)"`
	GoalML           *float64 `json:"goal_ml,omitempty" jsonschema:"daily hydration goal in millilitres (> 0)"`
	IdempotencyKey   string   `json:"idempotency_key,omitempty" jsonschema:"optional retry key; if omitted, a stable key is derived from the other args"`
}

type ListHydrationBalanceArgs struct {
	From string `json:"from" jsonschema:"inclusive lower bound date YYYY-MM-DD"`
	To   string `json:"to" jsonschema:"inclusive upper bound date YYYY-MM-DD; max 92 days from 'from'"`
}

type GetHydrationBalanceArgs struct {
	Date string `json:"date" jsonschema:"the snapshot date YYYY-MM-DD"`
}

type DeleteHydrationBalanceArgs struct {
	Date           string `json:"date" jsonschema:"the snapshot date YYYY-MM-DD to delete"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

func handleLogHydrationBalance(ctx context.Context, c *apiClient, args LogHydrationBalanceArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		Date             string   `json:"date"`
		SweatLossML      *float64 `json:"sweat_loss_ml,omitempty"`
		ActivityIntakeML *float64 `json:"activity_intake_ml,omitempty"`
		GoalML           *float64 `json:"goal_ml,omitempty"`
	}{
		Date:             args.Date,
		SweatLossML:      args.SweatLossML,
		ActivityIntakeML: args.ActivityIntakeML,
		GoalML:           args.GoalML,
	})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "log_hydration_balance", args)
	status, respBody, err := c.Post(ctx, "/hydration-balance", nil, body, key)
	return toToolResult(status, respBody, err)
}

func handleListHydrationBalance(ctx context.Context, c *apiClient, args ListHydrationBalanceArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("from", args.From)
	q.Set("to", args.To)
	status, body, err := c.Get(ctx, "/hydration-balance", q)
	return toToolResult(status, body, err)
}

func handleGetHydrationBalance(ctx context.Context, c *apiClient, args GetHydrationBalanceArgs) *mcp.CallToolResult {
	status, body, err := c.Get(ctx, "/hydration-balance/"+url.PathEscape(args.Date), nil)
	return toToolResult(status, body, err)
}

func handleDeleteHydrationBalance(ctx context.Context, c *apiClient, args DeleteHydrationBalanceArgs) *mcp.CallToolResult {
	key := effectiveIdempotencyKey(args.IdempotencyKey, "delete_hydration_balance", args)
	status, respBody, err := c.Delete(ctx, "/hydration-balance/"+url.PathEscape(args.Date), key)
	if err == nil && status == 204 {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: ""}}}
	}
	return toToolResult(status, respBody, err)
}

func registerHydrationBalanceTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "log_hydration_balance",
		Description: "Record (or replace) the daily water-balance snapshot for a date — estimated sweat " +
			"loss, fluid taken during activity, daily hydration goal (all millilitres). One snapshot per " +
			"calendar day, keyed by `date`; re-posting overwrites it. DISTINCT from log_hydration (per-entry " +
			"logged intake): this is a device's daily estimate. Compute the balance against logged intake " +
			"from the daily hydration summary.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args LogHydrationBalanceArgs) (*mcp.CallToolResult, any, error) {
		return handleLogHydrationBalance(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_hydration_balance",
		Description: "List daily hydration-balance snapshots whose date falls in the inclusive [from, to] " +
			"window (YYYY-MM-DD, max 92 days). Use for 'did my fluid intake keep up with sweat loss this week?'.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args ListHydrationBalanceArgs) (*mcp.CallToolResult, any, error) {
		return handleListHydrationBalance(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_hydration_balance",
		Description: "Fetch the hydration-balance snapshot for a single date (YYYY-MM-DD).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args GetHydrationBalanceArgs) (*mcp.CallToolResult, any, error) {
		return handleGetHydrationBalance(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_hydration_balance",
		Description: "Delete the hydration-balance snapshot for a date. Returns an empty result on success.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args DeleteHydrationBalanceArgs) (*mcp.CallToolResult, any, error) {
		return handleDeleteHydrationBalance(ctx, c, args), nil, nil
	})
}
