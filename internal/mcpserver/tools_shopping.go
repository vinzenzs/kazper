package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ShoppingItemArg is one item in a bulk shopping add.
type ShoppingItemArg struct {
	Name            string  `json:"name" jsonschema:"item name, e.g. 'Zwiebeln'; required, ≤300 chars"`
	QuantityText    *string `json:"quantity_text,omitempty" jsonschema:"opaque quantity text, e.g. '3 large' or '500 g' — already merged across recipes by you; the API never parses it"`
	RecipeProductID *string `json:"recipe_product_id,omitempty" jsonschema:"optional UUID of the recipe product this came from (soft provenance)"`
	PlanDate        *string `json:"plan_date,omitempty" jsonschema:"optional plan date YYYY-MM-DD this came from (provenance)"`
}

// AddShoppingItemsArgs is the input for add_shopping_items.
type AddShoppingItemsArgs struct {
	Items          []ShoppingItemArg `json:"items" jsonschema:"the consolidated shopping list, 1–200 items, in display order"`
	IdempotencyKey string            `json:"idempotency_key,omitempty" jsonschema:"optional retry key; derived from the items if omitted"`
}

// ListShoppingItemsArgs is the input for list_shopping_items.
type ListShoppingItemsArgs struct {
	IncludeChecked *bool `json:"include_checked,omitempty" jsonschema:"when true, also return checked (bought) items, listed after the unchecked ones"`
}

// UpdateShoppingItemArgs is the input for update_shopping_item.
type UpdateShoppingItemArgs struct {
	ID             string  `json:"id" jsonschema:"shopping item UUID"`
	Name           *string `json:"name,omitempty" jsonschema:"new name"`
	QuantityText   *string `json:"quantity_text,omitempty" jsonschema:"new opaque quantity text"`
	Checked        *bool   `json:"checked,omitempty" jsonschema:"true to mark bought (stamps checked_at), false to un-check"`
	IdempotencyKey string  `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

// DeleteShoppingItemArgs / ClearCheckedShoppingItemsArgs.
type DeleteShoppingItemArgs struct {
	ID             string `json:"id" jsonschema:"shopping item UUID"`
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

type ClearCheckedShoppingItemsArgs struct {
	IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"optional retry key"`
}

func handleAddShoppingItems(ctx context.Context, c *apiClient, args AddShoppingItemsArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		Items []ShoppingItemArg `json:"items"`
	}{args.Items})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "add_shopping_items", args)
	status, respBody, err := c.Post(ctx, "/shopping/items", nil, body, key)
	return toToolResult(status, respBody, err)
}

func handleUpdateShoppingItem(ctx context.Context, c *apiClient, args UpdateShoppingItemArgs) *mcp.CallToolResult {
	body, err := json.Marshal(struct {
		Name         *string `json:"name,omitempty"`
		QuantityText *string `json:"quantity_text,omitempty"`
		Checked      *bool   `json:"checked,omitempty"`
	}{args.Name, args.QuantityText, args.Checked})
	if err != nil {
		return toToolResult(0, nil, &transportError{inner: err})
	}
	key := effectiveIdempotencyKey(args.IdempotencyKey, "update_shopping_item", args)
	status, respBody, err := c.Patch(ctx, "/shopping/items/"+url.PathEscape(args.ID), body, key)
	return toToolResult(status, respBody, err)
}

func registerShoppingTools(server *mcp.Server, c *apiClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "add_shopping_items",
		Description: "Add items to the shopping list in one call (1–200, atomic). IMPORTANT: merge and " +
			"dedupe quantities across the planned recipes YOURSELF before calling — combine '1 Zwiebel' + " +
			"'2 Zwiebeln' into one '3 Zwiebeln' item. The API stores items verbatim and never aggregates or " +
			"parses quantity_text. Set recipe_product_id / plan_date as provenance when known. A single " +
			"invalid item fails the whole batch (the error names the offending index).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args AddShoppingItemsArgs) (*mcp.CallToolResult, any, error) {
		return handleAddShoppingItems(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_shopping_items",
		Description: "List shopping items — unchecked (still-to-buy) in order by default; pass include_checked=true to also see bought items (listed last). Read-only.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args ListShoppingItemsArgs) (*mcp.CallToolResult, any, error) {
		q := url.Values{}
		if args.IncludeChecked != nil {
			q.Set("include_checked", strconv.FormatBool(*args.IncludeChecked))
		}
		status, body, err := c.Get(ctx, "/shopping/items", q)
		return toToolResult(status, body, err), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_shopping_item",
		Description: "Rename a shopping item or check/uncheck it (checked=true stamps it bought, false un-checks).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args UpdateShoppingItemArgs) (*mcp.CallToolResult, any, error) {
		return handleUpdateShoppingItem(ctx, c, args), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_shopping_item",
		Description: "Delete a single shopping item by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args DeleteShoppingItemArgs) (*mcp.CallToolResult, any, error) {
		key := effectiveIdempotencyKey(args.IdempotencyKey, "delete_shopping_item", args)
		status, body, err := c.Delete(ctx, "/shopping/items/"+url.PathEscape(args.ID), key)
		return toToolResult(status, body, err), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "clear_checked_shopping_items",
		Description: "Delete all checked (bought) items in one call and report how many were removed — the routine post-shop cleanup. There is intentionally no 'clear the whole list' call.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args ClearCheckedShoppingItemsArgs) (*mcp.CallToolResult, any, error) {
		q := url.Values{}
		q.Set("checked", "true")
		key := effectiveIdempotencyKey(args.IdempotencyKey, "clear_checked_shopping_items", args)
		status, body, err := c.do(ctx, http.MethodDelete, "/shopping/items", q, nil, key)
		return toToolResult(status, body, err), nil, nil
	})
}
