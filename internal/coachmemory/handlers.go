package coachmemory

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const dateFormat = "2006-01-02"

// Handlers wires the coach-memory CRUD endpoints.
type Handlers struct {
	svc           *Service
	defaultTZName string
	logger        *slog.Logger
}

func NewHandlers(svc *Service, defaultTZ string, logger *slog.Logger) *Handlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handlers{svc: svc, defaultTZName: defaultTZ, logger: logger}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/coach/memory", h.create)
	rg.GET("/coach/memory", h.list)
	rg.GET("/coach/memory/:id", h.get)
	rg.PATCH("/coach/memory/:id", h.patch)
	rg.DELETE("/coach/memory/:id", h.delete)
}

type createRequest struct {
	Kind      string  `json:"kind"`
	Text      string  `json:"text"`
	Reason    *string `json:"reason,omitempty"`
	Scope     *string `json:"scope,omitempty"`
	Date      *string `json:"date,omitempty"`
	ExpiresAt *string `json:"expires_at,omitempty"`
	ReviewAt  *string `json:"review_at,omitempty"`
}

// create godoc
// @Summary      Record a coach-memory item
// @Description  Appends one coach-authored memory item — a `kind` of fact, preference, constraint, observation, or recommendation, with required `text`. A recommendation requires a `date`; standing kinds may be dateless. A storage primitive only — text is stored verbatim and recording one does NOT change any enforced goal/override target. Standard `Idempotency-Key` header supported.
// @Tags         coach-memory
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string         false  "Optional client-supplied idempotency key"
// @Param        body             body    createRequest  true   "Memory item"
// @Success      201  {object}  Memory
// @Failure      400  {object}  map[string]string  "invalid_json | kind_invalid | text_required | scope_invalid | date_invalid | date_required"
// @Security     BearerAuth
// @Router       /coach/memory [post]
func (h *Handlers) create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	// Validate any supplied date fields up front (each YYYY-MM-DD).
	for _, d := range []*string{req.Date, req.ExpiresAt, req.ReviewAt} {
		if d != nil && *d != "" {
			if _, err := time.Parse(dateFormat, *d); err != nil {
				respondError(c, http.StatusBadRequest, "date_invalid")
				return
			}
		}
	}
	m, err := h.svc.Create(c.Request.Context(), CreateInput{
		Kind:      req.Kind,
		Text:      req.Text,
		Reason:    req.Reason,
		Scope:     req.Scope,
		Date:      emptyToNil(req.Date),
		ExpiresAt: emptyToNil(req.ExpiresAt),
		ReviewAt:  emptyToNil(req.ReviewAt),
	})
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, m)
}

// list godoc
// @Summary      List coach memory in a window
// @Description  Standing items (fact/preference/constraint/observation) are returned regardless of the window; recommendations are filtered to [from, to] (inclusive local dates). Newest-first. Archived and expired items are excluded unless include_archived=true. Optional kind/scope filters.
// @Tags         coach-memory
// @Produce      json
// @Param        from              query  string  true   "Inclusive start date YYYY-MM-DD (recommendation window)"
// @Param        to                query  string  true   "Inclusive end date YYYY-MM-DD (recommendation window)"
// @Param        tz                query  string  false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Param        kind              query  string  false  "Filter: fact | preference | constraint | observation | recommendation"
// @Param        scope             query  string  false  "Filter: fueling | training | recovery | race | general"
// @Param        include_archived  query  bool    false  "Include archived rows (default false)"
// @Success      200  {object}  map[string]interface{}  "{ memory: [...] }"
// @Failure      400  {object}  map[string]string  "window_required | date_invalid | window_invalid | tz_invalid | kind_invalid | scope_invalid"
// @Security     BearerAuth
// @Router       /coach/memory [get]
func (h *Handlers) list(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		respondError(c, http.StatusBadRequest, "window_required")
		return
	}
	from, err := time.Parse(dateFormat, fromStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "date_invalid")
		return
	}
	to, err := time.Parse(dateFormat, toStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "date_invalid")
		return
	}
	if to.Before(from) {
		respondError(c, http.StatusBadRequest, "window_invalid")
		return
	}
	loc, err := h.resolveTZ(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, "tz_invalid")
		return
	}
	p := ListParams{
		From:            fromStr,
		To:              toStr,
		IncludeArchived: c.Query("include_archived") == "true",
		AsOf:            time.Now().In(loc).Format(dateFormat),
	}
	if k := c.Query("kind"); k != "" {
		if !ValidKind(k) {
			respondError(c, http.StatusBadRequest, "kind_invalid")
			return
		}
		p.Kind = &k
	}
	if s := c.Query("scope"); s != "" {
		if !ValidScope(s) {
			respondError(c, http.StatusBadRequest, "scope_invalid")
			return
		}
		p.Scope = &s
	}
	items, err := h.svc.List(c.Request.Context(), p)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_failed")
		return
	}
	out := make([]*Memory, 0, len(items))
	out = append(out, items...)
	c.JSON(http.StatusOK, gin.H{"memory": out})
}

// get godoc
// @Summary      Get a coach-memory item by id
// @Tags         coach-memory
// @Produce      json
// @Param        id   path  string  true  "Memory UUID"
// @Success      200  {object}  Memory
// @Failure      404  {object}  map[string]string  "memory_not_found"
// @Security     BearerAuth
// @Router       /coach/memory/{id} [get]
func (h *Handlers) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "memory_not_found")
		return
	}
	m, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "memory_not_found")
			return
		}
		respondError(c, http.StatusInternalServerError, "get_failed")
		return
	}
	c.JSON(http.StatusOK, m)
}

// patch godoc
// @Summary      Update a coach-memory item's lifecycle (review/expire/status)
// @Description  Lifecycle-only: `review_at`, `expires_at`, and `status` (active|archived). Content (text/kind/scope/date) is immutable here — correct it with a delete + re-log. Tri-state on the two date fields: omit to leave unchanged, value to set, JSON null to clear. `created_at` is preserved.
// @Tags         coach-memory
// @Accept       json
// @Produce      json
// @Param        id    path  string                  true  "Memory UUID"
// @Param        body  body  map[string]interface{}  true  "Lifecycle fields"
// @Success      200  {object}  Memory
// @Failure      400  {object}  map[string]string  "invalid_json | field_immutable | date_invalid | status_invalid"
// @Failure      404  {object}  map[string]string  "memory_not_found"
// @Security     BearerAuth
// @Router       /coach/memory/{id} [patch]
func (h *Handlers) patch(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "memory_not_found")
		return
	}
	raw, err := c.GetRawData()
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	// Reject content edits explicitly: only the lifecycle fields are mutable.
	if len(raw) > 0 {
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(raw, &probe); err != nil {
			respondError(c, http.StatusBadRequest, "invalid_json")
			return
		}
		mutable := map[string]bool{"review_at": true, "expires_at": true, "status": true}
		for field := range probe {
			if !mutable[field] {
				c.JSON(http.StatusBadRequest, gin.H{"error": "field_immutable", "field": field})
				return
			}
		}
	}
	var body struct {
		ReviewAt  json.RawMessage `json:"review_at,omitempty"`
		ExpiresAt json.RawMessage `json:"expires_at,omitempty"`
		Status    *string         `json:"status,omitempty"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &body); err != nil {
			respondError(c, http.StatusBadRequest, "invalid_json")
			return
		}
	}
	in := PatchInput{Status: body.Status}
	if rv, clear, ok, valid := decodeDateField(body.ReviewAt); ok {
		if !valid {
			respondError(c, http.StatusBadRequest, "date_invalid")
			return
		}
		in.ReviewAt, in.ClearReviewAt = rv, clear
	}
	if ev, clear, ok, valid := decodeDateField(body.ExpiresAt); ok {
		if !valid {
			respondError(c, http.StatusBadRequest, "date_invalid")
			return
		}
		in.ExpiresAt, in.ClearExpiresAt = ev, clear
	}
	m, err := h.svc.Patch(c.Request.Context(), id, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "memory_not_found")
			return
		}
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, m)
}

// delete godoc
// @Summary      Delete a coach-memory item
// @Description  Removes a memory item. A content correction is a delete followed by a re-log.
// @Tags         coach-memory
// @Param        id   path  string  true  "Memory UUID"
// @Success      204  "No Content"
// @Failure      404  {object}  map[string]string  "memory_not_found"
// @Security     BearerAuth
// @Router       /coach/memory/{id} [delete]
func (h *Handlers) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "memory_not_found")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "memory_not_found")
			return
		}
		respondError(c, http.StatusInternalServerError, "delete_failed")
		return
	}
	c.Status(http.StatusNoContent)
}

// ----- helpers -----

// decodeDateField decodes a tri-state JSON date field. Returns
// (value, clear, present, valid): present=false when the key was absent;
// clear=true when JSON null; value set for a parseable YYYY-MM-DD; valid=false
// for a malformed non-null value.
func decodeDateField(raw json.RawMessage) (value *string, clear, present, valid bool) {
	if raw == nil {
		return nil, false, false, true
	}
	if string(raw) == "null" {
		return nil, true, true, true
	}
	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, false, true, false
	}
	if _, err := time.Parse(dateFormat, v); err != nil {
		return nil, false, true, false
	}
	return &v, false, true, true
}

func emptyToNil(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}

func respondError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

func respondServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrTextRequired):
		respondError(c, http.StatusBadRequest, "text_required")
	case errors.Is(err, ErrKindInvalid):
		respondError(c, http.StatusBadRequest, "kind_invalid")
	case errors.Is(err, ErrScopeInvalid):
		respondError(c, http.StatusBadRequest, "scope_invalid")
	case errors.Is(err, ErrDateRequired):
		respondError(c, http.StatusBadRequest, "date_required")
	case errors.Is(err, ErrStatusInvalid):
		respondError(c, http.StatusBadRequest, "status_invalid")
	default:
		respondError(c, http.StatusInternalServerError, "write_failed")
	}
}

func (h *Handlers) resolveTZ(c *gin.Context) (*time.Location, error) {
	tz := c.Query("tz")
	if tz == "" {
		tz = h.defaultTZName
	}
	return time.LoadLocation(tz)
}
