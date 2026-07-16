package heat

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/workouts"
)

// Handlers wires GET /workouts/:id/heat.
type Handlers struct {
	svc       *Service
	defaultTZ string
	logger    *slog.Logger
}

func NewHandlers(svc *Service, defaultTZ string, logger *slog.Logger) *Handlers {
	if defaultTZ == "" {
		defaultTZ = "UTC"
	}
	return &Handlers{svc: svc, defaultTZ: defaultTZ, logger: logger}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.GET("/workouts/:id/heat", h.heat)
	rg.GET("/workouts/heat-analytics", h.analytics)
}

// maxRangeDays caps the analytics window at the workout/analytics tier.
const maxRangeDays = 400

// analytics godoc
// @Summary      Heat-vs-performance evidence over history
// @Description  Buckets the window's OUTDOOR completed workouts by session heat index (`<20` / `20-25` / `25-30` / `>30` °C) and reports, per bucket, the session count and the mean duration, EF, decoupling, and power relative to the window's own baseline (100 = this athlete's mean over the window). Adds Spearman correlations of EF and decoupling against heat index, gated at 10 pairs (`insufficient_pairs` below it — sparse data can't produce a confident number). Indoor sessions are excluded (a trainer's temperature says nothing about racing in the heat); sessions with a null environment are included and counted in `assumed_outdoor` so the caveat is visible; sessions with no stored temperature are skipped. **Read the duration confound**: hot sessions skew long, and duration is reported per bucket precisely so a "heat" effect that is really a distance effect is visible. This is DESCRIPTIVE, not a model fit — it exists as the evidence stream for a human refining the heat-adjustment constants, and nothing here refits anything. Compute-on-read; persists nothing. Range capped at 400 days.
// @Tags         workouts
// @Produce      json
// @Param        from  query  string  true   "Inclusive start date YYYY-MM-DD"
// @Param        to    query  string  true   "Inclusive end date YYYY-MM-DD; max 400-day span"
// @Param        tz    query  string  false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Success      200  {object}  Analytics
// @Failure      400  {object}  map[string]interface{}  "range_required | date_invalid | range_invalid | range_too_large | tz_invalid"
// @Security     BearerAuth
// @Router       /workouts/heat-analytics [get]
func (h *Handlers) analytics(c *gin.Context) {
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
	tz := c.Query("tz")
	if tz == "" {
		tz = h.defaultTZ
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tz_invalid"})
		return
	}

	out, err := h.svc.AnalyticsFor(c.Request.Context(), from, to, loc)
	if err != nil {
		if h.logger != nil {
			h.logger.Error("heat analytics failed", "err", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "heat_analytics_failed"})
		return
	}
	c.JSON(http.StatusOK, out)
}

// heat godoc
// @Summary      Heat load and suggested adjustment for a planned session
// @Description  For a PLANNED workout: resolves the session date's location (the same primitive `/locations/resolve` exposes — the echoed name shows which city the forecast came from), fetches the session window's forecast, and computes a composite `heat_load_c` = heat index (temperature × humidity) + a solar penalty scaled down by cloud cover − bounded wind cooling. Acclimatization is DERIVED, not asked for: `low` (< 2) / `medium` (2–4) / `good` (≥ 5) qualifying sessions — outdoor, completed, ≥ 60 min, session heat index ≥ 25 °C, trailing 14 days — with the qualifying workout ids echoed so the level traces back to real rides. The suggested adjustment is a percentage reduction off the effective baseline (FTP for bike, threshold pace for run) from a fixed heat-load × duration × acclimatization table, plus a fluid note scaled from the measured sweat rate when one exists (a generic 600 ml/hr default is flagged as such). **Window anchoring**: a session scheduled by date alone carries no start time (it lands at midnight), so the forecast would otherwise score pre-dawn hours — such a session is re-anchored at the configured `DEFAULT_TRAINING_START` and the response says `assumed_start: true` with `start_source: "assumed"`. A session with a real start time anchors there (`start_source: "workout"`). The optional `start=HH:MM` overrides both (`start_source: "param"`) — two calls answer "before 07:00 vs past 09:00". The scored `window` is always echoed. Degradations are `200`s: an `indoor` planned session returns `not_applicable: true` without fetching weather; a null environment computes with `assumed_outdoor: true`; no resolvable location → `reason: "location_unconfigured"`; forecast trouble → `reason: "weather_unavailable"`. A non-planned workout returns 409 — this is a pre-session question. **This is a practice HEURISTIC, not WBGT and not physiology** — there is no solar sensor, cloud cover is the proxy, and every constant is v1 and echoed. Compute-on-read: persists nothing and writes no target anywhere; applying an adjustment means proposing edits to the scheduled workout through the existing confirmed flows.
// @Tags         workouts
// @Produce      json
// @Param        id     path   string  true   "Workout UUID"
// @Param        start  query  string  false  "Score the session as if it started at this local HH:MM (24-hour) — the 'what if I go out later' question"
// @Success      200  {object}  Report
// @Failure      400  {object}  map[string]string  "start_invalid"
// @Failure      404  {object}  map[string]string  "not_found"
// @Failure      409  {object}  map[string]string  "workout_not_planned"
// @Security     BearerAuth
// @Router       /workouts/{id}/heat [get]
func (h *Handlers) heat(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}

	out, err := h.svc.ReportForWithStart(c.Request.Context(), id, c.Query("start"))
	if err != nil {
		switch {
		case errors.Is(err, ErrStartInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"error": "start_invalid"})
		case errors.Is(err, workouts.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		case errors.Is(err, ErrWorkoutNotPlanned):
			c.JSON(http.StatusConflict, gin.H{"error": "workout_not_planned"})
		default:
			if h.logger != nil {
				h.logger.Error("heat report failed", "err", err)
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "heat_failed"})
		}
		return
	}
	c.JSON(http.StatusOK, out)
}
