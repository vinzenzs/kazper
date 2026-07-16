package fuelplan

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers wires GET /nutrition/fuel-plan onto a Gin router group.
type Handlers struct {
	svc       *Service
	defaultTZ string
	logger    *slog.Logger
	// now is injectable so the default-window tests don't race the clock.
	now func() time.Time
}

func NewHandlers(svc *Service, defaultTZ string, logger *slog.Logger) *Handlers {
	return &Handlers{svc: svc, defaultTZ: defaultTZ, logger: logger, now: time.Now}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.GET("/nutrition/fuel-plan", h.fuelPlan)
}

// fuelPlan godoc
// @Summary      Periodized carb targets from planned training load
// @Description  Classifies each day in the window by its PLANNED load and suggests a matching carbohydrate target — "fuel for the work required". Tier from total planned TSS: `rest` (no planned session) / `easy` (< 60) / `moderate` (60–150) / `heavy` (> 150, or any single planned session of 150 min or more — a long endurance day is glycogen-expensive at any intensity). Tiers map to a fixed 3 / 5 / 7 / 9 g/kg ladder, multiplied by the latest smoothed body-weight trend (echoed with its date) to give `suggested_carbs_g`, reported beside that date's currently-effective goal carbs (override > phase template > default) and the `delta_g` between them. Days past the last materialized plan day carry `plan_missing: true` — a rest suggestion and a no-plan-data suggestion must not look alike. No weight data degrades to tiers and g/kg without gram targets (`reason: "weight_missing"`). Window defaults to today plus six days, capped at 14. SUGGESTIONS ONLY: compute-on-read, persists nothing, and never writes a goal or override — applying a day's number is the existing per-date goal-override PUT.
// @Tags         nutrition
// @Produce      json
// @Param        from  query  string  false  "Inclusive start date YYYY-MM-DD (defaults to today)"
// @Param        to    query  string  false  "Inclusive end date YYYY-MM-DD (defaults to from + 6 days); max 14-day span"
// @Param        tz    query  string  false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Success      200  {object}  Plan
// @Failure      400  {object}  map[string]interface{}  "range_required | date_invalid | range_invalid | range_too_large | tz_invalid"
// @Security     BearerAuth
// @Router       /nutrition/fuel-plan [get]
func (h *Handlers) fuelPlan(c *gin.Context) {
	tzStr := c.Query("tz")
	if tzStr == "" {
		tzStr = h.defaultTZ
	}
	loc, err := time.LoadLocation(tzStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tz_invalid"})
		return
	}

	fromStr := c.Query("from")
	toStr := c.Query("to")

	var from, to time.Time
	switch {
	case fromStr == "" && toStr == "":
		// Default: today plus six, in the athlete's timezone.
		now := h.now().In(loc)
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		to = from.AddDate(0, 0, DefaultWindowDays-1)
	case fromStr == "" || toStr == "":
		// Half a window can't be inferred: defaulting the missing bound would
		// silently answer a question the caller didn't ask.
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_required"})
		return
	default:
		from, err = time.Parse("2006-01-02", fromStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
			return
		}
		to, err = time.Parse("2006-01-02", toStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
			return
		}
	}

	if from.After(to) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_invalid"})
		return
	}
	if days := int(to.Sub(from).Hours()/24) + 1; days > MaxRangeDays {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_too_large", "max_days": MaxRangeDays})
		return
	}

	out, err := h.svc.PlanFor(c.Request.Context(), Params{From: from, To: to, Loc: loc})
	if err != nil {
		if h.logger != nil {
			h.logger.Error("fuel plan failed", "err", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fuel_plan_failed"})
		return
	}
	c.JSON(http.StatusOK, out)
}
