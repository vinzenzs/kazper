package workoutcompliance

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/numfmt"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// Handlers wires the /workouts/:id/compliance endpoint onto a Gin router group.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.GET("/workouts/:id/compliance", h.compliance)
}

// compliance godoc
// @Summary      Per-step execution compliance for a completed workout
// @Description  Scores how a completed, template-linked workout was executed against its effective program (template steps + slot overrides + athlete-config zone→absolute resolution), step by step. Expands repeat groups into a flat executed-step sequence and matches laps (splits) to steps positionally; each step reports its resolved target vs the lap's actual (in_band/under/over with a signed delta + deviation_pct), planned-vs-actual duration, and a 0–100 step score, aggregated into an overall planned-duration-weighted score. Returns `status:"unavailable"` (200) when the lap count does not equal the expanded step count. Compute-on-read — nothing is persisted.
// @Tags         workouts
// @Produce      json
// @Param        id  path  string  true  "Workout id (uuid)"
// @Success      200  {object}  Result
// @Failure      400  {object}  map[string]string  "workout_id_invalid"
// @Failure      404  {object}  map[string]string  "not_found"
// @Failure      409  {object}  map[string]string  "workout_not_completed | multisport_unsupported | no_template_link | splits_missing"
// @Security     BearerAuth
// @Router       /workouts/{id}/compliance [get]
func (h *Handlers) compliance(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workout_id_invalid"})
		return
	}
	res, err := h.svc.Compliance(c.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, workouts.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		case errors.Is(err, ErrNotCompleted):
			c.JSON(http.StatusConflict, gin.H{"error": "workout_not_completed"})
		case errors.Is(err, ErrMultisportUnsupported):
			c.JSON(http.StatusConflict, gin.H{"error": "multisport_unsupported"})
		case errors.Is(err, ErrNoTemplateLink):
			c.JSON(http.StatusConflict, gin.H{"error": "no_template_link"})
		case errors.Is(err, ErrSplitsMissing):
			c.JSON(http.StatusConflict, gin.H{"error": "splits_missing"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "compliance_failed"})
		}
		return
	}
	roundResult(res)
	c.JSON(http.StatusOK, res)
}

// roundResult applies numfmt.Round1 to every numeric response field at the
// boundary (full precision is kept through the scoring math).
func roundResult(res *Result) {
	res.Score = numfmt.Round1Ptr(res.Score)
	for i := range res.Steps {
		s := &res.Steps[i]
		s.Score = numfmt.Round1Ptr(s.Score)
		s.Actual.DurationS = numfmt.Round1Ptr(s.Actual.DurationS)
		s.Actual.DistanceM = numfmt.Round1Ptr(s.Actual.DistanceM)
		s.Actual.AvgSpeedMPS = numfmt.Round1Ptr(s.Actual.AvgSpeedMPS)
		roundTarget(s.Target)
		roundTarget(s.Secondary)
		if d := s.Duration; d != nil {
			d.Planned = numfmt.Round1Ptr(d.Planned)
			d.Actual = numfmt.Round1Ptr(d.Actual)
			d.Ratio = numfmt.Round1Ptr(d.Ratio)
			d.Score = numfmt.Round1Ptr(d.Score)
		}
	}
}

func roundTarget(t *TargetResult) {
	if t == nil {
		return
	}
	t.Low = numfmt.Round1Ptr(t.Low)
	t.High = numfmt.Round1Ptr(t.High)
	t.Actual = numfmt.Round1Ptr(t.Actual)
	t.Delta = numfmt.Round1Ptr(t.Delta)
	t.DeviationPct = numfmt.Round1Ptr(t.DeviationPct)
	t.Score = numfmt.Round1Ptr(t.Score)
}
