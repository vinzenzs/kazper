package workoutstats

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// maxRangeDays caps the window. Unlike nutrition summary's 92, workout stats
// support year-to-date as a first-class range, so the cap comfortably clears a
// full year. Per-day workout aggregation is cheap (far fewer rows than meals).
const maxRangeDays = 400

// Handlers wires GET /workouts/summary onto a Gin router group.
type Handlers struct {
	svc           *Service
	defaultTZName string
	logger        *slog.Logger
}

func NewHandlers(svc *Service, defaultTZ string, logger *slog.Logger) *Handlers {
	return &Handlers{svc: svc, defaultTZName: defaultTZ, logger: logger}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.GET("/workouts/summary", h.summary)
	rg.GET("/workouts/intensity-distribution", h.distribution)
}

// distribution godoc
// @Summary      Time-in-zone intensity distribution across a date range
// @Description  Aggregates completed workouts' stored HR-zone seconds (secs_in_zone_1..5) in `[from, to]` into per-zone shares: a window total with band collapse (low=Z1+Z2, moderate=Z3, high=Z4+Z5) and a classification label (polarized | pyramidal | threshold | mixed | null), a by-sport breakdown, and a Monday-start weekly trend (buckets only for weeks with a completed workout; edge weeks clipped to the window). Also returns a by_training_focus session-count axis + unclassified_focus_count. A completed workout with all-null zones is excluded from sums but counted (missing_zone_data_count, per-week + window). Zone shares are computed over total_zone_secs (zoned time, not elapsed); share_pct is omitted when a group has no zone time. Planned excluded. Range capped at 400 days. Read-only. Distinct from and never merged into the volume summary or any nutrition total.
// @Tags         workouts
// @Produce      json
// @Param        from  query  string  true   "Inclusive start date YYYY-MM-DD"
// @Param        to    query  string  true   "Inclusive end date YYYY-MM-DD"
// @Param        tz    query  string  false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Success      200  {object}  Distribution
// @Failure      400  {object}  map[string]interface{}  "range_required | date_invalid | range_invalid | range_too_large | tz_invalid"
// @Security     BearerAuth
// @Router       /workouts/intensity-distribution [get]
func (h *Handlers) distribution(c *gin.Context) {
	p, ok := h.parseWindow(c)
	if !ok {
		return
	}
	out, err := h.svc.DistributionFor(c.Request.Context(), p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "intensity_distribution_failed"})
		return
	}
	c.JSON(http.StatusOK, out)
}

// parseWindow validates from/to/tz and returns the resolved Params, or writes
// the matching 400 and returns ok=false. Shared by the summary + distribution
// endpoints (identical window vocabulary).
func (h *Handlers) parseWindow(c *gin.Context) (Params, bool) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_required"})
		return Params{}, false
	}
	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
		return Params{}, false
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
		return Params{}, false
	}
	if from.After(to) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_invalid"})
		return Params{}, false
	}
	if days := int(to.Sub(from).Hours()/24) + 1; days > maxRangeDays {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_too_large", "max_days": maxRangeDays})
		return Params{}, false
	}
	loc, err := h.resolveTZ(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tz_invalid"})
		return Params{}, false
	}
	return Params{From: from, To: to, Loc: loc}, true
}

// summary godoc
// @Summary      Workout volume totals across a date range
// @Description  Aggregates completed workouts in `[from, to]` into a per-day series (one bucket per calendar day, zero-filled) plus a window total, each carrying count, total duration (min), total distance (m), total elevation gain (m), total kcal, and a by-sport count breakdown. Distance/elevation/duration are workout-only and are never merged into nutrition totals. Planned workouts are excluded. Nullable metrics are summed present-only. Range is capped at 400 days so year-to-date is supported.
// @Tags         workouts
// @Produce      json
// @Param        from  query  string  true   "Inclusive start date YYYY-MM-DD"
// @Param        to    query  string  true   "Inclusive end date YYYY-MM-DD"
// @Param        tz    query  string  false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Success      200  {object}  Summary
// @Failure      400  {object}  map[string]interface{}  "range_required | date_invalid | range_invalid | range_too_large | tz_invalid"
// @Security     BearerAuth
// @Router       /workouts/summary [get]
func (h *Handlers) summary(c *gin.Context) {
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
	if days := int(to.Sub(from).Hours()/24) + 1; days > maxRangeDays {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_too_large", "max_days": maxRangeDays})
		return
	}
	loc, err := h.resolveTZ(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tz_invalid"})
		return
	}

	out, err := h.svc.SummaryFor(c.Request.Context(), Params{From: from, To: to, Loc: loc})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "workout_stats_failed"})
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handlers) resolveTZ(c *gin.Context) (*time.Location, error) {
	tz := c.Query("tz")
	if tz == "" {
		tz = h.defaultTZName
		h.logger.Warn("workout stats used default tz", "route", c.FullPath(), "default_tz", tz)
	}
	return time.LoadLocation(tz)
}
