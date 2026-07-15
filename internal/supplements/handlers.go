package supplements

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handlers wires the supplement CRUD endpoints.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers { return &Handlers{svc: svc} }

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/supplements", h.create)
	rg.GET("/supplements", h.list)
	rg.GET("/supplements/:id", h.get)
	rg.DELETE("/supplements/:id", h.delete)
}

type createBody struct {
	LoggedAt time.Time `json:"logged_at"`
	Name     string    `json:"name"`
	Dose     *float64  `json:"dose"`
	DoseUnit *string   `json:"dose_unit"`
	Note     *string   `json:"note"`
}

// create godoc
// @Summary      Log a supplement intake
// @Description  Records a timestamped supplement event. `name` is required; `dose` + `dose_unit` are paired (both or neither); `note` optional. Multiple entries per day are allowed. `Idempotency-Key` is supported. Supplements feed no nutrition/hydration/energy total. No PATCH — corrections are delete + re-log.
// @Tags         supplements
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string      false  "Optional client-supplied idempotency key"
// @Param        body  body  createBody  true  "Supplement event"
// @Success      201   {object}  map[string]interface{}  "{\"supplement\": Entry}"
// @Failure      400   {object}  map[string]string  "name_required | dose_pair_required | dose_invalid | note_too_long | logged_at_required | invalid_json"
// @Security     BearerAuth
// @Router       /supplements [post]
func (h *Handlers) create(c *gin.Context) {
	var b createBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	if b.LoggedAt.IsZero() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "logged_at_required"})
		return
	}
	e, err := h.svc.Create(c.Request.Context(), CreateInput{
		LoggedAt: b.LoggedAt, Name: b.Name, Dose: b.Dose, DoseUnit: b.DoseUnit, Note: b.Note,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"supplement": e})
}

// list godoc
// @Summary      List supplement intakes in a window
// @Description  Returns entries whose `logged_at` falls within `[from, to]` (inclusive) ascending. `200` with an empty array when none. Range capped at 92 days.
// @Tags         supplements
// @Produce      json
// @Param        from  query  string  true   "Inclusive RFC3339 lower bound"
// @Param        to    query  string  true   "Inclusive RFC3339 upper bound; max 92 days from from"
// @Success      200  {object}  map[string]interface{}  "{\"entries\": [Entry]}"
// @Failure      400  {object}  map[string]interface{}  "range_required | date_invalid | range_invalid | range_too_large"
// @Security     BearerAuth
// @Router       /supplements [get]
func (h *Handlers) list(c *gin.Context) {
	fromStr, toStr := c.Query("from"), c.Query("to")
	if fromStr == "" || toStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_required"})
		return
	}
	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid", "field": "from"})
		return
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid", "field": "to"})
		return
	}
	// Inclusive upper bound → half-open [from, to] by nudging to the next instant.
	entries, err := h.svc.List(c.Request.Context(), from, to.Add(time.Nanosecond))
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"entries": entries})
}

// get godoc
// @Summary      Get a supplement entry
// @Tags         supplements
// @Produce      json
// @Param        id  path  string  true  "Entry UUID"
// @Success      200  {object}  map[string]interface{}  "{\"supplement\": Entry}"
// @Failure      404  {object}  map[string]string  "not_found"
// @Security     BearerAuth
// @Router       /supplements/{id} [get]
func (h *Handlers) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	e, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"supplement": e})
}

// delete godoc
// @Summary      Delete a supplement entry
// @Tags         supplements
// @Param        id  path  string  true  "Entry UUID"
// @Success      204  "no content"
// @Failure      404  {object}  map[string]string  "not_found"
// @Security     BearerAuth
// @Router       /supplements/{id} [delete]
func (h *Handlers) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		writeErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func writeErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
	case errors.Is(err, ErrNameRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": "name_required"})
	case errors.Is(err, ErrDosePairRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": "dose_pair_required"})
	case errors.Is(err, ErrDoseInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "dose_invalid"})
	case errors.Is(err, ErrNoteTooLong):
		c.JSON(http.StatusBadRequest, gin.H{"error": "note_too_long"})
	case errors.Is(err, ErrRangeInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_invalid"})
	case errors.Is(err, ErrRangeTooLarge):
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_too_large", "max_days": MaxRangeDays})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
	}
}
