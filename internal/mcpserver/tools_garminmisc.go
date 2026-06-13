package mcpserver

import (
	"context"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Read tools for the catch-all mirror capabilities (add-garmin-misc-mirror):
// devices, health-vitals, achievements. Each mirrors a REST list endpoint 1:1
// and forwards the body verbatim — read-only, no idempotency key.

type ListDevicesArgs struct{}

type ListHealthVitalsArgs struct {
	From string `json:"from" jsonschema:"inclusive lower bound date YYYY-MM-DD"`
	To   string `json:"to" jsonschema:"inclusive upper bound date YYYY-MM-DD; max 92 days from 'from'"`
}

type ListAchievementsArgs struct {
	Kind string `json:"kind,omitempty" jsonschema:"optional filter: badge | challenge"`
}

func handleListDevices(ctx context.Context, c *apiClient, _ ListDevicesArgs) *mcp.CallToolResult {
	status, body, err := c.Get(ctx, "/devices", nil)
	return toToolResult(status, body, err)
}

func handleListHealthVitals(ctx context.Context, c *apiClient, args ListHealthVitalsArgs) *mcp.CallToolResult {
	q := url.Values{}
	q.Set("from", args.From)
	q.Set("to", args.To)
	status, body, err := c.Get(ctx, "/health-vitals", q)
	return toToolResult(status, body, err)
}

func handleListAchievements(ctx context.Context, c *apiClient, args ListAchievementsArgs) *mcp.CallToolResult {
	q := url.Values{}
	if args.Kind != "" {
		q.Set("kind", args.Kind)
	}
	status, body, err := c.Get(ctx, "/achievements", q)
	return toToolResult(status, body, err)
}

func registerGarminMiscTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "devices_list",
		Description: "List the athlete's paired Garmin devices (watches, bike computers, scales) with model, last " +
			"sync time, battery, and firmware. Reference context — e.g. flagging a low-battery or stale-sync device.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args ListDevicesArgs) (*mcp.CallToolResult, any, error) {
		return handleListDevices(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "health_vitals_list",
		Description: "List daily health vitals (blood pressure, all-day resting/min/max heart rate, all-day stress) " +
			"in an inclusive [from, to] date window (YYYY-MM-DD, max 92 days). Distinct from recovery metrics.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args ListHealthVitalsArgs) (*mcp.CallToolResult, any, error) {
		return handleListHealthVitals(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "achievements_list",
		Description: "List the athlete's earned Garmin badges and ad-hoc challenges (most recent first), optionally " +
			"filtered by kind (badge|challenge). Coaching context — e.g. \"you just earned the 100-rides badge\".",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args ListAchievementsArgs) (*mcp.CallToolResult, any, error) {
		return handleListAchievements(ctx, c, args), nil, nil
	})
}
