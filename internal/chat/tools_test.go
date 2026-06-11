package chat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The registry exposes EXACTLY the curated allowlist — no goal, delete, or
// meal/hydration-logging tools — and the upstream tool defs add web_search.
func TestRegistry_ExactAllowlist(t *testing.T) {
	specs := registry()
	got := make([]string, 0, len(specs))
	for _, s := range specs {
		got = append(got, s.name)
	}
	want := []string{
		"get_daily_context", "get_race_fueling", "list_planned_meals",
		"list_shopping_items", "search_products", "get_product",
		"import_cookidoo_recipe", "update_product", "create_planned_meal",
		"update_planned_meal", "mark_planned_meal_eaten", "add_shopping_items",
		"update_shopping_item", "clear_checked_shopping_items",
	}
	assert.ElementsMatch(t, want, got)
	assert.Len(t, got, 14)

	// Forbidden surfaces must be absent.
	forbidden := []string{
		"log_meal", "log_meal_freeform", "log_hydration", "delete_meal",
		"delete_product", "delete_planned_meal", "set_daily_goal_override",
		"delete_shopping_item", "log_workout",
	}
	names := map[string]bool{}
	for _, n := range got {
		names[n] = true
	}
	for _, f := range forbidden {
		assert.Falsef(t, names[f], "forbidden tool present: %s", f)
	}

	// Tool defs include web_search, domain-restricted to Cookidoo.
	defs := anthropicToolDefs(specs)
	require.Len(t, defs, 15) // 14 custom + web_search
	last := string(defs[len(defs)-1])
	assert.Contains(t, last, "web_search")
	assert.Contains(t, last, "cookidoo.de")
	assert.Contains(t, last, "allowed_domains")
}

// Every tool's schema is valid JSON and every write tool is flagged.
func TestRegistry_SchemasValidAndWriteFlags(t *testing.T) {
	writes := map[string]bool{
		"import_cookidoo_recipe": true, "update_product": true,
		"create_planned_meal": true, "update_planned_meal": true,
		"mark_planned_meal_eaten": true, "add_shopping_items": true,
		"update_shopping_item": true, "clear_checked_shopping_items": true,
	}
	for _, s := range registry() {
		var schema any
		require.NoErrorf(t, json.Unmarshal([]byte(s.schema), &schema), "tool %s schema invalid", s.name)
		assert.Equalf(t, writes[s.name], s.write, "write flag wrong for %s", s.name)
	}
}

func TestBuild_PathParamAndQueryTools(t *testing.T) {
	specs := registryByName(registry())

	// get_product → GET /products/{id}
	call, err := specs["get_product"].build(json.RawMessage(`{"product_id":"abc"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.method)
	assert.Equal(t, "/products/abc", call.path)

	// get_race_fueling, no id → list; with id → plan
	c1, _ := specs["get_race_fueling"].build(json.RawMessage(`{}`))
	assert.Equal(t, "/races", c1.path)
	c2, _ := specs["get_race_fueling"].build(json.RawMessage(`{"race_id":"r1"}`))
	assert.Equal(t, "/races/r1/fueling-plan", c2.path)

	// mark_planned_meal_eaten → POST /plan/{id}/eaten, plan_id stripped from body
	c3, err := specs["mark_planned_meal_eaten"].build(json.RawMessage(`{"plan_id":"p1","quantity_g":450}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", c3.method)
	assert.Equal(t, "/plan/p1/eaten", c3.path)
	assert.Contains(t, string(c3.body), "quantity_g")
	assert.NotContains(t, string(c3.body), "plan_id")

	// update_shopping_item → PATCH /shopping/items/{id}, item_id stripped
	c4, err := specs["update_shopping_item"].build(json.RawMessage(`{"item_id":"i1","checked":true}`))
	require.NoError(t, err)
	assert.Equal(t, "PATCH", c4.method)
	assert.Equal(t, "/shopping/items/i1", c4.path)
	assert.Contains(t, string(c4.body), "checked")
	assert.NotContains(t, string(c4.body), "item_id")

	// clear_checked → DELETE /shopping/items?checked=true
	c5, _ := specs["clear_checked_shopping_items"].build(json.RawMessage(`{}`))
	assert.Equal(t, "DELETE", c5.method)
	assert.Equal(t, "true", c5.query.Get("checked"))

	// missing required path param errors out (no call made)
	_, err = specs["mark_planned_meal_eaten"].build(json.RawMessage(`{}`))
	assert.Error(t, err)
}

// Idempotency keys are deterministic across key reordering / whitespace and
// differ for different inputs — the property the retry-safety guarantee rests on.
func TestDeriveIdempotencyKey_Deterministic(t *testing.T) {
	a := deriveIdempotencyKey("add_shopping_items", json.RawMessage(`{"items":[{"name":"onion"}]}`))
	b := deriveIdempotencyKey("add_shopping_items", json.RawMessage(`{"items":[{"name":"onion"}]}`))
	assert.Equal(t, a, b, "identical calls must hash identically")

	// Reordered keys / whitespace must not change the key.
	c := deriveIdempotencyKey("create_planned_meal", json.RawMessage(`{"slot":"dinner","plan_date":"2026-06-12"}`))
	d := deriveIdempotencyKey("create_planned_meal", json.RawMessage("{ \"plan_date\":\"2026-06-12\" , \"slot\":\"dinner\" }"))
	assert.Equal(t, c, d, "key/whitespace reordering must be invariant")

	// Different input → different key.
	e := deriveIdempotencyKey("add_shopping_items", json.RawMessage(`{"items":[{"name":"garlic"}]}`))
	assert.NotEqual(t, a, e)

	// Hex-encoded sha256.
	assert.Len(t, a, 64)
	assert.Empty(t, strings.Trim(a, "0123456789abcdef"))
}
