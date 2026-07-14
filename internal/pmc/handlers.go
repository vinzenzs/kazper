package pmc

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers wires the PMC read endpoints onto a Gin router group.
type Handlers struct {
	svc           *Service
	resolver      MacroResolver
	defaultTZName string
	logger        *slog.Logger
}

func NewHandlers(svc *Service, resolver MacroResolver, defaultTZ string, logger *slog.Logger) *Handlers {
	return &Handlers{svc: svc, resolver: resolver, defaultTZName: defaultTZ, logger: logger}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.GET("/performance/pmc", h.pmc)
	rg.GET("/performance/pmc/target-trajectory", h.targetTrajectory)
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

// targetTrajectory godoc
// @Summary      Planned-vs-actual CTL trajectory for a macrocycle
// @Description  Simulates the target CTL curve implied by the macrocycle's declared phase `target_weekly_tss` (daily = weekly/7 through the same 42-day EWMA as the measured PMC, seeded from the actual CTL on the macrocycle start date) beside the measured CTL to date. Each day carries `target_ctl` + `target_declared` (false for gaps / null-target phases, which decay), plus `actual_ctl`/`delta` up to today. The summary reports `current_delta`, `delta_trend_14d`, and the projected CTL at macrocycle end on plan (`_planned`) vs from the current trajectory (`_current`). `macrocycle_id` is optional — omitted resolves the active macrocycle (containing today, latest start_date). No active/unknown macrocycle → `404 macrocycle_not_found`; a macrocycle whose phases declare no target → `200` with `trajectory: null` and `reason: "targets_missing"`. Compute-on-read; nothing persisted.
// @Tags         performance
// @Produce      json
// @Param        macrocycle_id  query  string  false  "Macrocycle UUID; omitted resolves the active macrocycle"
// @Param        tz             query  string  false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Success      200  {object}  TargetTrajectory
// @Failure      400  {object}  map[string]interface{}  "tz_invalid"
// @Failure      404  {object}  map[string]string  "macrocycle_not_found"
// @Security     BearerAuth
// @Router       /performance/pmc/target-trajectory [get]
func (h *Handlers) targetTrajectory(c *gin.Context) {
	loc, err := h.resolveTZ(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tz_invalid"})
		return
	}
	var idPtr *string
	if id := c.Query("macrocycle_id"); id != "" {
		idPtr = &id
	}
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	macro, err := h.resolver.Resolve(c.Request.Context(), idPtr, today)
	if err != nil {
		if errors.Is(err, ErrMacrocycleNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "macrocycle_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "pmc_failed"})
		return
	}
	traj, err := h.svc.TargetTrajectoryFor(c.Request.Context(), macro, loc, today)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "pmc_failed"})
		return
	}
	c.JSON(http.StatusOK, traj)
}

func (h *Handlers) resolveTZ(c *gin.Context) (*time.Location, error) {
	tz := c.Query("tz")
	if tz == "" {
		tz = h.defaultTZName
	}
	return time.LoadLocation(tz)
}
