package chat

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// httpCall is the REST request a tool maps to. Exactly one per tool execution.
type httpCall struct {
	method string
	path   string
	query  url.Values
	body   []byte
}

// toolSpec defines one custom (client-executed) tool: its Anthropic schema and
// how its input maps to a single loopback REST call.
type toolSpec struct {
	name        string
	description string
	schema      string // JSON Schema (object) for the tool input
	write       bool   // write tools get an auto-derived Idempotency-Key
	build       func(input json.RawMessage) (httpCall, error)
}

// cookidooSearchDomains restricts the web_search server tool to Cookidoo hosts
// so the agent can only discover importable recipes, not arbitrary pages.
var cookidooSearchDomains = []string{
	"cookidoo.de", "cookidoo.com", "cookidoo.at", "cookidoo.ch",
	"cookidoo.co.uk", "cookidoo.fr", "cookidoo.es", "cookidoo.it",
	"cookidoo.nl", "cookidoo.be", "cookidoo.com.au", "cookidoo.pl",
}

// decodeInto unmarshals a tool's input JSON into dst, returning a friendly
// error the loop surfaces as a tool error rather than crashing the stream.
func decodeInto(input json.RawMessage, dst any) error {
	if len(input) == 0 {
		return nil
	}
	if err := json.Unmarshal(input, dst); err != nil {
		return fmt.Errorf("invalid tool input: %w", err)
	}
	return nil
}

// registry returns the curated allowlist in a stable order. This is the single
// source of truth for which tools the chat loop exposes.
func registry() []toolSpec {
	return []toolSpec{
		// ---------- reads ----------
		{
			name:        "get_daily_context",
			description: "Read the day's full nutrition state in one call: adherence vs goals, nutrition totals, hydration, workouts, workout-fuel, body weight, training phase, and any goal override. Call this FIRST before recommending meals so suggestions fit the remaining macro budget.",
			schema:      `{"type":"object","properties":{"date":{"type":"string","description":"YYYY-MM-DD"},"tz":{"type":"string","description":"IANA timezone; optional, defaults to the server's configured zone"}},"required":["date"]}`,
			build: func(in json.RawMessage) (httpCall, error) {
				var a struct {
					Date string `json:"date"`
					TZ   string `json:"tz"`
				}
				if err := decodeInto(in, &a); err != nil {
					return httpCall{}, err
				}
				q := url.Values{}
				q.Set("date", a.Date)
				if a.TZ != "" {
					q.Set("tz", a.TZ)
				}
				return httpCall{method: "GET", path: "/context/daily", query: q}, nil
			},
		},
		{
			name:        "get_race_fueling",
			description: "Race-day fuelling context. Call with no arguments to list the user's races and their dates (so you know how near a race is); call with race_id to get that race's per-leg fuelling plan (carbs/sodium/fluid). Use when a race is near and it should shape today's eating.",
			schema:      `{"type":"object","properties":{"race_id":{"type":"string","description":"optional; omit to list races, supply to get one race's per-leg fuelling plan"}}}`,
			build: func(in json.RawMessage) (httpCall, error) {
				var a struct {
					RaceID string `json:"race_id"`
				}
				if err := decodeInto(in, &a); err != nil {
					return httpCall{}, err
				}
				if a.RaceID == "" {
					return httpCall{method: "GET", path: "/races"}, nil
				}
				return httpCall{method: "GET", path: "/races/" + url.PathEscape(a.RaceID) + "/fueling-plan"}, nil
			},
		},
		{
			name:        "list_planned_meals",
			description: "List planned meals (the user's selected-but-not-yet-eaten dishes) over a date range, inclusive. Use to see what's already planned before adding more.",
			schema:      `{"type":"object","properties":{"from":{"type":"string","description":"YYYY-MM-DD"},"to":{"type":"string","description":"YYYY-MM-DD"}},"required":["from","to"]}`,
			build: func(in json.RawMessage) (httpCall, error) {
				var a struct {
					From string `json:"from"`
					To   string `json:"to"`
				}
				if err := decodeInto(in, &a); err != nil {
					return httpCall{}, err
				}
				q := url.Values{}
				q.Set("from", a.From)
				q.Set("to", a.To)
				return httpCall{method: "GET", path: "/plan", query: q}, nil
			},
		},
		{
			name:        "list_shopping_items",
			description: "List the shopping list. By default returns only unchecked (still-to-buy) items; pass include_checked=true to also see bought ones.",
			schema:      `{"type":"object","properties":{"include_checked":{"type":"boolean"}}}`,
			build: func(in json.RawMessage) (httpCall, error) {
				var a struct {
					IncludeChecked bool `json:"include_checked"`
				}
				if err := decodeInto(in, &a); err != nil {
					return httpCall{}, err
				}
				q := url.Values{}
				if a.IncludeChecked {
					q.Set("include_checked", "true")
				}
				return httpCall{method: "GET", path: "/shopping/items", query: q}, nil
			},
		},
		{
			name:        "search_products",
			description: "Search the user's product/recipe library by name or brand (recently-logged first). Check here for an existing recipe before web-searching Cookidoo for a new one.",
			schema:      `{"type":"object","properties":{"q":{"type":"string"}},"required":["q"]}`,
			build: func(in json.RawMessage) (httpCall, error) {
				var a struct {
					Q string `json:"q"`
				}
				if err := decodeInto(in, &a); err != nil {
					return httpCall{}, err
				}
				q := url.Values{}
				q.Set("q", a.Q)
				return httpCall{method: "GET", path: "/products/search", query: q}, nil
			},
		},
		{
			name:        "get_product",
			description: "Fetch a single product/recipe by id, including its nutriments and (for recipes) its ingredient list.",
			schema:      `{"type":"object","properties":{"product_id":{"type":"string"}},"required":["product_id"]}`,
			build: func(in json.RawMessage) (httpCall, error) {
				var a struct {
					ProductID string `json:"product_id"`
				}
				if err := decodeInto(in, &a); err != nil {
					return httpCall{}, err
				}
				return httpCall{method: "GET", path: "/products/" + url.PathEscape(a.ProductID)}, nil
			},
		},

		// ---------- writes ----------
		{
			name:        "import_cookidoo_recipe",
			description: "Import a Cookidoo recipe into the library by URL. ALWAYS estimate the serving mass from the ingredients and pass serving_size_g so the nutriments are computed at import — omitting it leaves the recipe without nutriments. Re-importing the same URL is safe (returns the existing product).",
			schema:      `{"type":"object","properties":{"url":{"type":"string"},"serving_size_g":{"type":"number","description":"grams per serving; strongly recommended so per-100g nutriments are computed"}},"required":["url"]}`,
			write:       true,
			build: func(in json.RawMessage) (httpCall, error) {
				var a struct {
					URL          string   `json:"url"`
					ServingSizeG *float64 `json:"serving_size_g,omitempty"`
				}
				if err := decodeInto(in, &a); err != nil {
					return httpCall{}, err
				}
				body, _ := json.Marshal(a)
				return httpCall{method: "POST", path: "/products/import/cookidoo", body: body}, nil
			},
		},
		{
			name:        "update_product",
			description: "Set a product's editable fields (name, serving_size_g, nutriments_per_100g). Use to fill in nutriments after importing a Cookidoo recipe without a serving size. Only supplied fields change.",
			schema:      `{"type":"object","properties":{"product_id":{"type":"string"},"name":{"type":"string"},"serving_size_g":{"type":"number"},"nutriments_per_100g":{"type":"object","properties":{"kcal":{"type":"number"},"protein_g":{"type":"number"},"carbs_g":{"type":"number"},"fat_g":{"type":"number"},"fiber_g":{"type":"number"},"sugar_g":{"type":"number"},"salt_g":{"type":"number"}}}},"required":["product_id"]}`,
			write:       true,
			build: func(in json.RawMessage) (httpCall, error) {
				var a struct {
					ProductID         string          `json:"product_id"`
					Name              *string         `json:"name,omitempty"`
					ServingSizeG      *float64        `json:"serving_size_g,omitempty"`
					NutrimentsPer100g json.RawMessage `json:"nutriments_per_100g,omitempty"`
				}
				if err := decodeInto(in, &a); err != nil {
					return httpCall{}, err
				}
				body := map[string]any{}
				if a.Name != nil {
					body["name"] = *a.Name
				}
				if a.ServingSizeG != nil {
					body["serving_size_g"] = *a.ServingSizeG
				}
				if len(a.NutrimentsPer100g) > 0 {
					body["nutriments_per_100g"] = a.NutrimentsPer100g
				}
				b, _ := json.Marshal(body)
				return httpCall{method: "PATCH", path: "/products/" + url.PathEscape(a.ProductID), body: b}, nil
			},
		},
		{
			name:        "create_planned_meal",
			description: "Plan a meal: a selected dish for a date and slot. slot is breakfast|lunch|dinner|snack. Use after the user picks an option; product_id must reference an existing product/recipe.",
			schema:      `{"type":"object","properties":{"plan_date":{"type":"string","description":"YYYY-MM-DD"},"slot":{"type":"string","enum":["breakfast","lunch","dinner","snack"]},"product_id":{"type":"string"},"quantity_g":{"type":"number"},"notes":{"type":"string"}},"required":["plan_date","slot","product_id"]}`,
			write:       true,
			build: func(in json.RawMessage) (httpCall, error) {
				return passthrough("POST", "/plan", in)
			},
		},
		{
			name:        "update_planned_meal",
			description: "Update a planned meal: change status (planned↔skipped only; use mark_planned_meal_eaten to log), quantity_g, slot, plan_date, or notes.",
			schema:      `{"type":"object","properties":{"plan_id":{"type":"string"},"status":{"type":"string","enum":["planned","skipped"]},"quantity_g":{"type":"number"},"slot":{"type":"string"},"plan_date":{"type":"string"},"notes":{"type":"string"}},"required":["plan_id"]}`,
			write:       true,
			build: func(in json.RawMessage) (httpCall, error) {
				return pathParamPassthrough(in, "plan_id", "PATCH", "/plan/")
			},
		},
		{
			name:        "mark_planned_meal_eaten",
			description: "Mark a planned meal as eaten NOW — this logs a real meal entry. The only correct way to record that a planned meal was actually eaten. Optionally override quantity_g.",
			schema:      `{"type":"object","properties":{"plan_id":{"type":"string"},"quantity_g":{"type":"number"},"logged_at":{"type":"string","description":"RFC3339; defaults to now, must not be in the future"}},"required":["plan_id"]}`,
			write:       true,
			build: func(in json.RawMessage) (httpCall, error) {
				var a struct {
					PlanID    string          `json:"plan_id"`
					QuantityG json.RawMessage `json:"quantity_g,omitempty"`
					LoggedAt  json.RawMessage `json:"logged_at,omitempty"`
				}
				if err := decodeInto(in, &a); err != nil {
					return httpCall{}, err
				}
				if a.PlanID == "" {
					return httpCall{}, fmt.Errorf("plan_id is required")
				}
				body := map[string]any{}
				if len(a.QuantityG) > 0 {
					body["quantity_g"] = a.QuantityG
				}
				if len(a.LoggedAt) > 0 {
					body["logged_at"] = a.LoggedAt
				}
				b, _ := json.Marshal(body)
				return httpCall{method: "POST", path: "/plan/" + url.PathEscape(a.PlanID) + "/eaten", body: b}, nil
			},
		},
		{
			name:        "add_shopping_items",
			description: "Add items to the shopping list in one call. MERGE and DEDUPE across recipes BEFORE calling — the list stores items verbatim and never aggregates (combine '1 onion' + '2 onions' into '3 onions' yourself). quantity_text is free text.",
			schema:      `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"name":{"type":"string"},"quantity_text":{"type":"string"},"recipe_product_id":{"type":"string"},"plan_date":{"type":"string"}},"required":["name"]}}},"required":["items"]}`,
			write:       true,
			build: func(in json.RawMessage) (httpCall, error) {
				return passthrough("POST", "/shopping/items", in)
			},
		},
		{
			name:        "update_shopping_item",
			description: "Update a shopping item: check/uncheck it (checked), or edit name/quantity_text.",
			schema:      `{"type":"object","properties":{"item_id":{"type":"string"},"checked":{"type":"boolean"},"name":{"type":"string"},"quantity_text":{"type":"string"}},"required":["item_id"]}`,
			write:       true,
			build: func(in json.RawMessage) (httpCall, error) {
				return pathParamPassthrough(in, "item_id", "PATCH", "/shopping/items/")
			},
		},
		{
			name:        "clear_checked_shopping_items",
			description: "Remove all checked (bought) items from the shopping list. Reports how many were cleared.",
			schema:      `{"type":"object","properties":{}}`,
			write:       true,
			build: func(in json.RawMessage) (httpCall, error) {
				q := url.Values{}
				q.Set("checked", "true")
				return httpCall{method: "DELETE", path: "/shopping/items", query: q}, nil
			},
		},
	}
}

// passthrough forwards the tool input verbatim as the request body.
func passthrough(method, path string, in json.RawMessage) (httpCall, error) {
	body := in
	if len(body) == 0 {
		body = json.RawMessage(`{}`)
	}
	return httpCall{method: method, path: path, body: body}, nil
}

// pathParamPassthrough pulls a single id field out of the input, uses it as a
// path segment, and forwards the remaining fields as the request body.
func pathParamPassthrough(in json.RawMessage, idField, method, pathPrefix string) (httpCall, error) {
	var generic map[string]json.RawMessage
	if err := decodeInto(in, &generic); err != nil {
		return httpCall{}, err
	}
	idRaw, ok := generic[idField]
	if !ok {
		return httpCall{}, fmt.Errorf("%s is required", idField)
	}
	var id string
	if err := json.Unmarshal(idRaw, &id); err != nil || id == "" {
		return httpCall{}, fmt.Errorf("%s must be a non-empty string", idField)
	}
	delete(generic, idField)
	body, _ := json.Marshal(generic)
	return httpCall{method: method, path: pathPrefix + url.PathEscape(id), body: body}, nil
}

// anthropicTools renders the custom registry plus the web_search server tool
// into the Messages API `tools` array. When withTools is false (the forced
// final turn after the round cap) it returns nil so no tools are offered.
func anthropicToolDefs(specs []toolSpec) []json.RawMessage {
	out := make([]json.RawMessage, 0, len(specs)+1)
	for _, s := range specs {
		def := map[string]any{
			"name":         s.name,
			"description":  s.description,
			"input_schema": json.RawMessage(s.schema),
		}
		raw, _ := json.Marshal(def)
		out = append(out, raw)
	}
	// web_search server tool, domain-restricted to Cookidoo.
	ws := map[string]any{
		"type":            "web_search_20250305",
		"name":            "web_search",
		"allowed_domains": cookidooSearchDomains,
		"max_uses":        5,
	}
	raw, _ := json.Marshal(ws)
	out = append(out, raw)
	return out
}

// registryByName indexes the registry for dispatch.
func registryByName(specs []toolSpec) map[string]toolSpec {
	m := make(map[string]toolSpec, len(specs))
	for _, s := range specs {
		m[s.name] = s
	}
	return m
}
