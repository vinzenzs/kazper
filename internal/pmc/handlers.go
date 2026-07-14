package pmc

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers wires the PMC read endpoint onto a Gin router group.
type Handlers struct {
	svc           *Service
	defaultTZName string
	logger        *slog.Logger
}

func NewHandlers(svc *Service, defaultTZ string, logger *slog.Logger) *Handlers {
	return &Handlers{svc: svc, defaultTZName: defaultTZ, logger: logger}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.GET("/performance/pmc", h.pmc)
}

// pmc godoc
// @Summary      Performance Management Chart (CTL/ATL/TSB) daily series
// @Description  Compute-on-read Coggan PMC over stored completed-workout TSS: one entry per calendar day in [from, to] with `tss_total`, `ctl` (42-day EWMA fitness), `atl` (7-day EWMA fatigue), `tsb` (yesterday's ctl−atl form), and `ramp_rate` (ctl change over 7 days). The EWMA warms up from the earliest completed workout so values are window-independent (`seed_date` reports the warm-up start). `ramp_alerts` flags Monday-start weeks whose CTL rose more than 8/week. Only completed workouts contribute; a completed workout with NULL tss counts 0 but is surfaced via per-day `missing_tss_count` + window `missing_tss_workouts`. Distinct from Garmin's stored acute/chronic load. Range capped at 400 days; read-only.
// @Tags         performance
// @Produce      json
// @Param        from  query  string  true   "Inclusive start date YYYY-MM-DD"
// @Param        to    query  string  true   "Inclusive end date YYYY-MM-DD"
// @Param        tz    query  string  false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Success      200  {object}  Series
// @Failure      400  {object}  map[string]interface{}  "range_required | date_invalid | range_invalid | range_too_large | tz_invalid"
// @Security     BearerAuth
// @Router       /performance/pmc [get]
func (h *Handlers) pmc(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_required"})
		return
	}
	from, err := time.Parse(isoDate, fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
		return
	}
	to, err := time.Parse(isoDate, toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
		return
	}
	if from.After(to) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_invalid"})
		return
	}
	if days := int(to.Sub(from).Hours()/24) + 1; days > maxWindowDays {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_too_large", "max_days": maxWindowDays})
		return
	}
	loc, err := h.resolveTZ(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tz_invalid"})
		return
	}

	series, err := h.svc.SeriesFor(c.Request.Context(), Params{From: from, To: to, Loc: loc})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "pmc_failed"})
		return
	}
	c.JSON(http.StatusOK, series)
}

func (h *Handlers) resolveTZ(c *gin.Context) (*time.Location, error) {
	tz := c.Query("tz")
	if tz == "" {
		tz = h.defaultTZName
	}
	return time.LoadLocation(tz)
}
