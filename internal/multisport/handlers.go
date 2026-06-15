package multisport

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/kazper/internal/workouttemplates"
)

// Handlers wires the multisport-template endpoints.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/multisport-templates", h.create)
	rg.GET("/multisport-templates", h.list)
	rg.GET("/multisport-templates/:id", h.get)
	rg.DELETE("/multisport-templates/:id", h.delete)
}

// createRequest is the create body. Segments is required and non-empty.
type createRequest struct {
	Name     string    `json:"name"`
	Segments []Segment `json:"segments"`
}

// create godoc
// @Summary      Create a multisport workout template
// @Description  Create a multisport session (triathlon/brick) as an ordered list of per-sport segments plus transitions. Needs ≥2 non-transition segments; each segment's steps are validated under that segment's sport. `Idempotency-Key` supported.
// @Tags         multisport-templates
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string         false  "Optional client-supplied idempotency key"
// @Param        body             body    createRequest  true   "Multisport template"
// @Success      201  {object}  Template
// @Failure      400  {object}  map[string]string  "invalid_json | name_required | segments_empty | too_few_sport_segments | segment_sport_invalid | transition_segment_invalid | (per-segment step errors: intent_invalid | duration_invalid | target_invalid | target_range_invalid | target_sport_mismatch | secondary_target_invalid | repeat_invalid | repeat_nested | steps_empty)"
// @Security     BearerAuth
// @Router       /multisport-templates [post]
func (h *Handlers) create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	t := &Template{Name: req.Name, Segments: req.Segments}
	out, err := h.svc.Create(c.Request.Context(), t)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

// list godoc
// @Summary      List multisport workout templates
// @Tags         multisport-templates
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "{ multisport_templates: [...] }"
// @Security     BearerAuth
// @Router       /multisport-templates [get]
func (h *Handlers) list(c *gin.Context) {
	out, err := h.svc.List(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_failed")
		return
	}
	templates := make([]*Template, 0, len(out))
	templates = append(templates, out...)
	c.JSON(http.StatusOK, gin.H{"multisport_templates": templates})
}

// get godoc
// @Summary      Get a multisport workout template by id
// @Tags         multisport-templates
// @Produce      json
// @Param        id  path  string  true  "Template UUID"
// @Success      200  {object}  Template
// @Failure      404  {object}  map[string]string  "multisport_template_not_found"
// @Security     BearerAuth
// @Router       /multisport-templates/{id} [get]
func (h *Handlers) get(c *gin.Context) {
	out, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "multisport_template_not_found")
			return
		}
		respondError(c, http.StatusInternalServerError, "get_failed")
		return
	}
	c.JSON(http.StatusOK, out)
}

// delete godoc
// @Summary      Delete a multisport workout template
// @Tags         multisport-templates
// @Param        id  path  string  true  "Template UUID"
// @Success      204  "no content"
// @Failure      404  {object}  map[string]string  "multisport_template_not_found"
// @Security     BearerAuth
// @Router       /multisport-templates/{id} [delete]
func (h *Handlers) delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "multisport_template_not_found")
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

// respondServiceError maps both this package's sentinels and the reused
// workouttemplates step-validation sentinels (returned verbatim by per-segment
// validation) to their 1:1 error codes.
func respondServiceError(c *gin.Context, err error) {
	for _, e := range []error{
		ErrNameRequired, ErrSegmentsEmpty, ErrTooFewSports, ErrSegmentSport, ErrTransitionShape,
		workouttemplates.ErrSportInvalid, workouttemplates.ErrStepsEmpty, workouttemplates.ErrStepTypeInvalid,
		workouttemplates.ErrIntentInvalid, workouttemplates.ErrDurationInvalid, workouttemplates.ErrTargetInvalid,
		workouttemplates.ErrTargetRangeInvalid, workouttemplates.ErrTargetSportMismatch, workouttemplates.ErrSecondaryTarget,
		workouttemplates.ErrRepeatInvalid, workouttemplates.ErrRepeatNested,
	} {
		if errors.Is(err, e) {
			respondError(c, http.StatusBadRequest, e.Error())
			return
		}
	}
	respondError(c, http.StatusInternalServerError, "write_failed")
}
