package mcpserver

import (
	"context"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TrainingContextArgs is the input for the get_training_context tool. Read-only.
type TrainingContextArgs struct {
	Date          string `json:"date,omitempty" jsonschema:"calendar date YYYY-MM-DD (defaults to today)"`
	TZ            string `json:"tz,omitempty" jsonschema:"IANA timezone (defaults to DEFAULT_USER_TZ)"`
	LookbackDays  int    `json:"lookback_days,omitempty" jsonschema:"completed-workout/fitness lookback window (default 14, max 90)"`
	LookaheadDays int    `json:"lookahead_days,omitempty" jsonschema:"planned-workout lookahead window (default 7, max 60)"`
}

// RecoveryContextArgs is the input for the get_recovery_context tool. Read-only.
type RecoveryContextArgs struct {
	Date string `json:"date,omitempty" jsonschema:"calendar date YYYY-MM-DD (defaults to today)"`
	TZ   string `json:"tz,omitempty" jsonschema:"IANA timezone (defaults to DEFAULT_USER_TZ)"`
	Days int    `json:"days,omitempty" jsonschema:"trend window in days (default 7, max 90)"`
}

func handleTrainingContext(ctx context.Context, c *apiClient, args TrainingContextArgs) *mcp.CallToolResult {
	q := url.Values{}
	if args.Date != "" {
		q.Set("date", args.Date)
	}
	if args.TZ != "" {
		q.Set("tz", args.TZ)
	}
	if args.LookbackDays > 0 {
		q.Set("lookback_days", strconv.Itoa(args.LookbackDays))
	}
	if args.LookaheadDays > 0 {
		q.Set("lookahead_days", strconv.Itoa(args.LookaheadDays))
	}
	status, body, err := c.Get(ctx, "/context/training", q)
	return toToolResult(status, body, err)
}

func handleRecoveryContext(ctx context.Context, c *apiClient, args RecoveryContextArgs) *mcp.CallToolResult {
	q := url.Values{}
	if args.Date != "" {
		q.Set("date", args.Date)
	}
	if args.TZ != "" {
		q.Set("tz", args.TZ)
	}
	if args.Days > 0 {
		q.Set("days", strconv.Itoa(args.Days))
	}
	status, body, err := c.Get(ctx, "/context/recovery", q)
	return toToolResult(status, body, err)
}

func registerCoachContextTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "get_training_context",
		Description: "Get the training context bundle in one call: the covering training phase, the latest fitness " +
			"snapshot (VO2max, acute/chronic load, training status, race predictors) with derived ACWR, a recent-load " +
			"summary plus recent completed workouts (lookback_days, default 14), and upcoming planned workouts " +
			"(lookahead_days, default 7). Recommended as the FIRST call before giving training advice — collapses many " +
			"granular reads (list_workouts, list_fitness_metrics, list_phases). For per-entry detail use the dedicated " +
			"tools. Read-only.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args TrainingContextArgs) (*mcp.CallToolResult, any, error) {
		return handleTrainingContext(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "get_recovery_context",
		Description: "Get the recovery context bundle in one call: the latest recovery snapshot on/before the date " +
			"(sleep, HRV, resting HR, body battery, training readiness, …) plus the recent trend over `days` " +
			"(default 7). Recommended before advising on a hard session or rest day. For per-day detail use " +
			"list_recovery_metrics. Read-only.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecoveryContextArgs) (*mcp.CallToolResult, any, error) {
		return handleRecoveryContext(ctx, c, args), nil, nil
	})
}
