package shoppinglist

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handlers wires shopping-list endpoints onto a Gin router group.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/shopping/items", h.bulkCreate)
	rg.GET("/shopping/items", h.list)
	rg.PATCH("/shopping/items/:id", h.patch)
	rg.DELETE("/shopping/items/:id", h.delete)
	rg.DELETE("/shopping/items", h.deleteBulk)
}

type itemRequest struct {
	Name            string  `json:"name"`
	QuantityText    *string `json:"quantity_text,omitempty"`
	RecipeProductID *string `json:"recipe_product_id,omitempty"`
	PlanDate        *string `json:"plan_date,omitempty"`
}

type bulkCreateRequest struct {
	Items []itemRequest `json:"items"`
}

// bulkCreate godoc
// @Summary      Create shopping items in bulk
// @Description  Inserts 1–200 items atomically (all or none) in input order — the agent's consolidated, already-merged list in one call. `name` is required (≤300 chars); `quantity_text` is opaque ("3 large"), never parsed; `recipe_product_id` (if set) must reference an existing product; `plan_date` is bare provenance. A single invalid item fails the batch with its index.
// @Tags         shopping-list
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string             false  "Optional client-supplied idempotency key"
// @Param        body             body    bulkCreateRequest  true   "Items"
// @Success      201  {object}  map[string]interface{}  "{ items: [...] }"
// @Failure      400  {object}  map[string]interface{}  "items_required | batch_too_large | name_invalid | plan_date_invalid | recipe_product_id_invalid | product_not_found (with index)"
// @Security     BearerAuth
// @Router       /shopping/items [post]
func (h *Handlers) bulkCreate(c *gin.Context) {
	var req bulkCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	inputs := make([]CreateItemInput, 0, len(req.Items))
	for i, it := range req.Items {
		in := CreateItemInput{Name: it.Name, QuantityText: it.QuantityText, PlanDate: it.PlanDate}
		if it.RecipeProductID != nil && *it.RecipeProductID != "" {
			pid, err := uuid.Parse(*it.RecipeProductID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "recipe_product_id_invalid", "index": i})
				return
			}
			in.RecipeProductID = &pid
		}
		inputs = append(inputs, in)
	}
	items, err := h.svc.BulkCreate(c.Request.Context(), inputs)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"items": items})
}

// list godoc
// @Summary      List shopping items
// @Description  Returns unchecked items in creation order by default. With include_checked=true, checked items are appended after the unchecked ones.
// @Tags         shopping-list
// @Produce      json
// @Param        include_checked  query  bool  false  "Include checked items (last)"
// @Success      200  {object}  map[string]interface{}  "{ items: [...] }"
// @Security     BearerAuth
// @Router       /shopping/items [get]
func (h *Handlers) list(c *gin.Context) {
	includeChecked := c.Query("include_checked") == "true"
	items, err := h.svc.List(c.Request.Context(), includeChecked)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_failed")
		return
	}
	if items == nil {
		items = []*Item{}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// patch godoc
// @Summary      Update a shopping item (rename / check / uncheck)
// @Description  Partial update of `name`, `quantity_text` (null clears) and `checked`. Setting checked=true stamps checked_at; checked=false clears it.
// @Tags         shopping-list
// @Accept       json
// @Produce      json
// @Param        id    path  string                  true  "Item UUID"
// @Param        body  body  map[string]interface{}  true  "Fields to update"
// @Success      200  {object}  Item
// @Failure      400  {object}  map[string]string  "name_invalid"
// @Failure      404  {object}  map[string]string  "shopping_item_not_found"
// @Security     BearerAuth
// @Router       /shopping/items/{id} [patch]
func (h *Handlers) patch(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "shopping_item_not_found")
		return
	}
	var fields map[string]json.RawMessage
	if err := c.ShouldBindJSON(&fields); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	in, code := buildUpdateInput(fields)
	if code != "" {
		respondError(c, http.StatusBadRequest, code)
		return
	}
	item, err := h.svc.Update(c.Request.Context(), id, in)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

// delete godoc
// @Summary      Delete a shopping item
// @Tags         shopping-list
// @Param        id  path  string  true  "Item UUID"
// @Success      204  "no content"
// @Failure      404  {object}  map[string]string  "shopping_item_not_found"
// @Security     BearerAuth
// @Router       /shopping/items/{id} [delete]
func (h *Handlers) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "shopping_item_not_found")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// deleteBulk godoc
// @Summary      Clear checked shopping items
// @Description  `DELETE /shopping/items?checked=true` removes all checked items and reports the count. An unqualified bulk delete is intentionally rejected — there is no "wipe the whole list" call.
// @Tags         shopping-list
// @Produce      json
// @Param        checked  query  bool  true  "Must be true"
// @Success      200  {object}  map[string]interface{}  "{ deleted: n }"
// @Failure      400  {object}  map[string]string  "checked_qualifier_required"
// @Security     BearerAuth
// @Router       /shopping/items [delete]
func (h *Handlers) deleteBulk(c *gin.Context) {
	if c.Query("checked") != "true" {
		respondError(c, http.StatusBadRequest, "checked_qualifier_required")
		return
	}
	n, err := h.svc.ClearChecked(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "clear_failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": n})
}

// ----- helpers -----

func buildUpdateInput(fields map[string]json.RawMessage) (UpdateInput, string) {
	var in UpdateInput
	isNull := func(raw json.RawMessage) bool { return string(bytes.TrimSpace(raw)) == "null" }
	if v, ok := fields["name"]; ok && !isNull(v) {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return in, "name_invalid"
		}
		in.Name = &s
	}
	if v, ok := fields["quantity_text"]; ok {
		if isNull(v) {
			in.ClearQuantityText = true
		} else {
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				return in, "invalid_json"
			}
			in.QuantityText = &s
		}
	}
	if v, ok := fields["checked"]; ok && !isNull(v) {
		var b bool
		if err := json.Unmarshal(v, &b); err != nil {
			return in, "checked_invalid"
		}
		in.Checked = &b
	}
	return in, ""
}

func respondError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

func respondServiceError(c *gin.Context, err error) {
	var ie *ItemError
	if errors.As(err, &ie) {
		c.JSON(http.StatusBadRequest, gin.H{"error": ie.Code, "index": ie.Index})
		return
	}
	switch {
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "shopping_item_not_found")
	case errors.Is(err, ErrProductNotFound):
		respondError(c, http.StatusNotFound, "product_not_found")
	case errors.Is(err, ErrBatchEmpty):
		respondError(c, http.StatusBadRequest, "items_required")
	case errors.Is(err, ErrBatchTooLarge):
		respondError(c, http.StatusBadRequest, "batch_too_large")
	case errors.Is(err, ErrNameInvalid):
		respondError(c, http.StatusBadRequest, "name_invalid")
	default:
		respondError(c, http.StatusInternalServerError, "write_failed")
	}
}
