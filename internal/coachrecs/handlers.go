package coachrecs

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handlers wires the coach-recommendation CRUD endpoints.
type Handlers struct {
	svc           *Service
	defaultTZName string
	logger        *slog.Logger
}

func NewHandlers(svc *Service, defaultTZ string, logger *slog.Logger) *Handlers {
	return &Handlers{svc: svc, defaultTZName: defaultTZ, logger: logger}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/coach/recommendations", h.create)
	rg.GET("/coach/recommendations", h.list)
	rg.GET("/coach/recommendations/:id", h.get)
	rg.DELETE("/coach/recommendations/:id", h.delete)
}

type createRequest struct {
	Date           string  `json:"date"`
	Scope          string  `json:"scope"`
	Recommendation string  `json:"recommendation"`
	Reason         *string `json:"reason,omitempty"`
}

// create godoc
// @Summary      Record a coach recommendation
// @Description  Appends one coach-authored recommendation to the dated log. A storage primitive only — the server stores the supplied text verbatim and never generates or interprets a recommendation, and recording one does NOT change any enforced goal/override target. Standard `Idempotency-Key` header supported.
// @Tags         coach-recommendations
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string         false  "Optional client-supplied idempotency key"
// @Param        body             body    createRequest  true   "Recommendation"
// @Success      201  {object}  Recommendation
// @Failure      400  {object}  map[string]string  "invalid_json | date_invalid | scope_invalid | recommendation_required"
// @Security     BearerAuth
// @Router       /coach/recommendations [post]
func (h *Handlers) create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	if _, err := time.Parse("2006-01-02", req.Date); err != nil {
		respondError(c, http.StatusBadRequest, "date_invalid")
		return
	}
	rec, err := h.svc.Create(c.Request.Context(), CreateInput{
		Date:           req.Date,
		Scope:          req.Scope,
		Recommendation: req.Recommendation,
		Reason:         req.Reason,
	})
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, rec)
}

// list godoc
// @Summary      List coach recommendations in a date window
// @Description  Returns recommendations whose date falls in [from, to] (inclusive local dates), newest-first, optionally narrowed to one scope.
// @Tags         coach-recommendations
// @Produce      json
// @Param        from   query  string  true   "Inclusive start date YYYY-MM-DD"
// @Param        to     query  string  true   "Inclusive end date YYYY-MM-DD"
// @Param        tz     query  string  false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Param        scope  query  string  false  "Filter by scope: fueling | training | recovery | race | general"
// @Success      200  {object}  map[string]interface{}  "{ recommendations: [...] }"
// @Failure      400  {object}  map[string]string  "window_required | date_invalid | window_invalid | tz_invalid | scope_invalid"
// @Security     BearerAuth
// @Router       /coach/recommendations [get]
func (h *Handlers) list(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		respondError(c, http.StatusBadRequest, "window_required")
		return
	}
	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "date_invalid")
		return
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "date_invalid")
		return
	}
	if to.Before(from) {
		respondError(c, http.StatusBadRequest, "window_invalid")
		return
	}
	// tz is validated for API consistency (dates compare directly, tz-free).
	if _, err := h.resolveTZ(c); err != nil {
		respondError(c, http.StatusBadRequest, "tz_invalid")
		return
	}
	var scope *string
	if s := c.Query("scope"); s != "" {
		if !ValidScope(s) {
			respondError(c, http.StatusBadRequest, "scope_invalid")
			return
		}
		scope = &s
	}
	recs, err := h.svc.List(c.Request.Context(), fromStr, toStr, scope)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_failed")
		return
	}
	out := make([]*Recommendation, 0, len(recs))
	out = append(out, recs...)
	c.JSON(http.StatusOK, gin.H{"recommendations": out})
}

// get godoc
// @Summary      Get a coach recommendation by id
// @Tags         coach-recommendations
// @Produce      json
// @Param        id   path  string  true  "Recommendation UUID"
// @Success      200  {object}  Recommendation
// @Failure      404  {object}  map[string]string  "recommendation_not_found"
// @Security     BearerAuth
// @Router       /coach/recommendations/{id} [get]
func (h *Handlers) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "recommendation_not_found")
		return
	}
	rec, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "recommendation_not_found")
			return
		}
		respondError(c, http.StatusInternalServerError, "get_failed")
		return
	}
	c.JSON(http.StatusOK, rec)
}

// delete godoc
// @Summary      Delete a coach recommendation
// @Description  Removes a superseded/incorrect recommendation. Corrections are a delete followed by a re-log (there is no PATCH).
// @Tags         coach-recommendations
// @Param        id   path  string  true  "Recommendation UUID"
// @Success      204  "No Content"
// @Failure      404  {object}  map[string]string  "recommendation_not_found"
// @Security     BearerAuth
// @Router       /coach/recommendations/{id} [delete]
func (h *Handlers) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "recommendation_not_found")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "recommendation_not_found")
			return
		}
		respondError(c, http.StatusInternalServerError, "delete_failed")
		return
	}
	c.Status(http.StatusNoContent)
}

// ----- helpers -----

func respondError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

func respondServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrRecommendationRequired):
		respondError(c, http.StatusBadRequest, "recommendation_required")
	case errors.Is(err, ErrScopeInvalid):
		respondError(c, http.StatusBadRequest, "scope_invalid")
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
