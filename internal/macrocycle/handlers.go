package macrocycle

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/numfmt"
)

const dateFormat = "2006-01-02"

// Handlers wires POST/GET/PATCH/DELETE for /macrocycles.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/macrocycles", h.create)
	rg.GET("/macrocycles", h.list)
	rg.GET("/macrocycles/:id", h.get)
	rg.PATCH("/macrocycles/:id", h.patch)
	rg.DELETE("/macrocycles/:id", h.delete)
}

// createRequest is the JSON shape POST /macrocycles accepts. Date fields are
// YYYY-MM-DD strings.
type createRequest struct {
	Name        string  `json:"name"`
	StartDate   string  `json:"start_date"`
	EndDate     string  `json:"end_date"`
	RaceID      *string `json:"race_id,omitempty"`
	Methodology *string `json:"methodology,omitempty"`
	Notes       *string `json:"notes,omitempty"`
}

// create godoc
// @Summary      Create a macrocycle (training season)
// @Description  A macrocycle is a named, dated season container that orders training-phases into a yearly progression. Optionally anchored to a goal race via race_id (the A-race the season peaks for). Planning/visualization only — it does not affect adherence or plan materialization.
// @Tags         macrocycles
// @Accept       json
// @Produce      json
// @Param        body  body  createRequest  true  "Macrocycle fields"
// @Success      201   {object}  map[string]interface{}  "{\"macrocycle\": Macrocycle}"
// @Failure      400   {object}  map[string]interface{}  "macrocycle_name_invalid | macrocycle_name_too_long | date_range_invalid | date_invalid | race_id_invalid | race_not_found | invalid_json"
// @Security     BearerAuth
// @Router       /macrocycles [post]
func (h *Handlers) create(c *gin.Context) {
	var req createRequest
	if !decodeStrict(c, &req) {
		return
	}
	startDate, ok := parseDate(c, req.StartDate, "start_date")
	if !ok {
		return
	}
	endDate, ok := parseDate(c, req.EndDate, "end_date")
	if !ok {
		return
	}
	in := CreateInput{
		Name:        req.Name,
		StartDate:   startDate,
		EndDate:     endDate,
		Methodology: req.Methodology,
		Notes:       req.Notes,
	}
	if req.RaceID != nil && *req.RaceID != "" {
		rid, err := uuid.Parse(*req.RaceID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "race_id_invalid"})
			return
		}
		in.RaceID = &rid
	}
	m, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"macrocycle": roundMacrocycle(m)})
}

// get godoc
// @Summary      Get a macrocycle with its ordered member phases
// @Tags         macrocycles
// @Produce      json
// @Param        id  path  string  true  "Macrocycle id (UUID)"
// @Success      200   {object}  map[string]interface{}  "{\"macrocycle\": Macrocycle}"
// @Failure      400   {object}  map[string]string  "macrocycle_id_invalid"
// @Failure      404   {object}  map[string]string  "macrocycle_not_found"
// @Security     BearerAuth
// @Router       /macrocycles/{id} [get]
func (h *Handlers) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "macrocycle_id_invalid"})
		return
	}
	m, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"macrocycle": roundMacrocycle(m)})
}

// list godoc
// @Summary      List all macrocycles (seasons)
// @Description  Returns every macrocycle ordered by start_date descending. The nested member phases are omitted; fetch a macrocycle by id for its progression.
// @Tags         macrocycles
// @Produce      json
// @Success      200   {object}  map[string]interface{}  "{\"macrocycles\": [Macrocycle]}"
// @Security     BearerAuth
// @Router       /macrocycles [get]
func (h *Handlers) list(c *gin.Context) {
	ms, err := h.svc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "macrocycle_list_failed"})
		return
	}
	if ms == nil {
		ms = []*Macrocycle{}
	}
	for _, m := range ms {
		roundMacrocycle(m)
	}
	c.JSON(http.StatusOK, gin.H{"macrocycles": ms})
}

// patchRequest is the JSON shape PATCH /macrocycles/{id} accepts. Tri-state for
// race_id: missing = leave unchanged, empty string = clear, UUID string = set.
type patchRequest struct {
	Name        *string `json:"name,omitempty"`
	StartDate   *string `json:"start_date,omitempty"`
	EndDate     *string `json:"end_date,omitempty"`
	RaceID      *string `json:"race_id,omitempty"`
	Methodology *string `json:"methodology,omitempty"`
	Notes       *string `json:"notes,omitempty"`
}

// patch godoc
// @Summary      Partially update a macrocycle
// @Description  Tri-state on race_id: empty string clears the anchor, UUID sets, missing leaves unchanged.
// @Tags         macrocycles
// @Accept       json
// @Produce      json
// @Param        id    path  string  true  "Macrocycle id"
// @Param        body  body  patchRequest  true  "Fields to update"
// @Success      200   {object}  map[string]interface{}  "{\"macrocycle\": Macrocycle}"
// @Failure      400   {object}  map[string]interface{}  "macrocycle_id_invalid | invalid_json | patch_empty | macrocycle_name_invalid | macrocycle_name_too_long | date_invalid | date_range_invalid | race_id_invalid | race_not_found"
// @Failure      404   {object}  map[string]string  "macrocycle_not_found"
// @Security     BearerAuth
// @Router       /macrocycles/{id} [patch]
func (h *Handlers) patch(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "macrocycle_id_invalid"})
		return
	}
	var req patchRequest
	if !decodeStrict(c, &req) {
		return
	}
	p := PatchInput{
		Name:        req.Name,
		Methodology: req.Methodology,
		Notes:       req.Notes,
	}
	if req.StartDate != nil {
		d, ok := parseDate(c, *req.StartDate, "start_date")
		if !ok {
			return
		}
		p.StartDate = &d
	}
	if req.EndDate != nil {
		d, ok := parseDate(c, *req.EndDate, "end_date")
		if !ok {
			return
		}
		p.EndDate = &d
	}
	if req.RaceID != nil {
		if *req.RaceID == "" {
			p.ClearRaceID = true
		} else {
			rid, err := uuid.Parse(*req.RaceID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "race_id_invalid"})
				return
			}
			p.RaceID = &rid
		}
	}
	m, err := h.svc.Patch(c.Request.Context(), id, p)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"macrocycle": roundMacrocycle(m)})
}

// delete godoc
// @Summary      Delete a macrocycle
// @Description  Member phases survive — their macrocycle_id is set NULL (ON DELETE SET NULL) and they continue to drive adherence unchanged.
// @Tags         macrocycles
// @Param        id  path  string  true  "Macrocycle id"
// @Success      204   "no content"
// @Failure      400   {object}  map[string]string  "macrocycle_id_invalid"
// @Failure      404   {object}  map[string]string  "macrocycle_not_found"
// @Security     BearerAuth
// @Router       /macrocycles/{id} [delete]
func (h *Handlers) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "macrocycle_id_invalid"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// decodeStrict reads the raw body and decodes it with unknown-field rejection,
// responding 400 invalid_json on any failure. Returns false when it has already
// written an error response.
func decodeStrict(c *gin.Context, dst any) bool {
	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return false
	}
	return true
}

// parseDate parses a YYYY-MM-DD string, responding 400 date_invalid (with the
// field name) on failure. Returns false when it has written an error response.
func parseDate(c *gin.Context, s, field string) (time.Time, bool) {
	d, err := time.Parse(dateFormat, s)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid", "field": field})
		return time.Time{}, false
	}
	return d, true
}

// roundMacrocycle rounds the member phases' progression targets to 1dp at the
// response boundary (mutates in place and returns the same pointer).
func roundMacrocycle(m *Macrocycle) *Macrocycle {
	for _, p := range m.Phases {
		p.TargetWeeklyTSS = numfmt.Round1Ptr(p.TargetWeeklyTSS)
		p.TargetWeeklyHours = numfmt.Round1Ptr(p.TargetWeeklyHours)
	}
	return m
}

func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrMacrocycleNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "macrocycle_not_found"})
	case errors.Is(err, ErrNameInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "macrocycle_name_invalid"})
	case errors.Is(err, ErrNameTooLong):
		c.JSON(http.StatusBadRequest, gin.H{"error": "macrocycle_name_too_long", "max_length": MaxNameLength})
	case errors.Is(err, ErrDateRangeInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_range_invalid"})
	case errors.Is(err, ErrRaceNotFound):
		c.JSON(http.StatusBadRequest, gin.H{"error": "race_not_found"})
	case errors.Is(err, ErrPatchEmpty):
		c.JSON(http.StatusBadRequest, gin.H{"error": "patch_empty"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "macrocycle_write_failed"})
	}
}
