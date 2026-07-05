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
