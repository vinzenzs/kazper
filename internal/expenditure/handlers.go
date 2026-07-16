package expenditure

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers wires GET /nutrition/expenditure onto a Gin router group.
type Handlers struct {
	svc       *Service
	defaultTZ string
	logger    *slog.Logger
}

func NewHandlers(svc *Service, defaultTZ string, logger *slog.Logger) *Handlers {
	return &Handlers{svc: svc, defaultTZ: defaultTZ, logger: logger}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.GET("/nutrition/expenditure", h.expenditure)
}

// expenditure godoc
// @Summary      Adaptive energy expenditure (TDEE) from energy balance
// @Description  Estimates average daily expenditure over the window from the two series already logged: `mean(intake over logged days) − Δ trend-weight × 7700 kcal/kg ÷ window_days`. The mass signal is the body-weight capability's own smoothed trend (7-day trailing) evaluated at the window ends; the values and the dates they were taken at are echoed. A day counts as logged only when it holds at least one meal — an unlogged day is excluded from the mean and counted in `days_unlogged`, never read as zero intake. Honesty gates degrade to `200` with a null estimate: fewer than 14 logged days → `insufficient_logged_days`; fewer than 5 weigh-ins in the window → `insufficient_weigh_ins`. 21–28 day windows read most honestly; a window spanning deliberate glycogen manipulation (carb-load, race taper) moves water mass, not tissue, and distorts the estimate. Under-logged snacks bias the number down. Compute-on-read: persists nothing, reads no goals — comparing against a target and applying a change stay with the caller and the goals endpoints.
// @Tags         nutrition
// @Produce      json
// @Param        from  query  string  true   "Inclusive start date YYYY-MM-DD"
// @Param        to    query  string  true   "Inclusive end date YYYY-MM-DD; max 92-day span"
// @Param        tz    query  string  false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Success      200  {object}  Expenditure
// @Failure      400  {object}  map[string]interface{}  "range_required | date_invalid | range_invalid | range_too_large | tz_invalid"
// @Security     BearerAuth
// @Router       /nutrition/expenditure [get]
func (h *Handlers) expenditure(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_required"})
		return
	}
	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
		return
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
		return
	}
	if from.After(to) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_invalid"})
		return
	}
	if days := int(to.Sub(from).Hours()/24) + 1; days > MaxRangeDays {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_too_large", "max_days": MaxRangeDays})
		return
	}

	tzStr := c.Query("tz")
	if tzStr == "" {
		tzStr = h.defaultTZ
	}
	loc, err := time.LoadLocation(tzStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tz_invalid"})
		return
	}

	out, err := h.svc.EstimateFor(c.Request.Context(), Params{From: from, To: to, Loc: loc})
	if err != nil {
		if h.logger != nil {
			h.logger.Error("expenditure estimate failed", "err", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "expenditure_failed"})
		return
	}
	c.JSON(http.StatusOK, out)
}
