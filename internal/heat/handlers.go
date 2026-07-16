package heat

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/workouts"
)

// Handlers wires GET /workouts/:id/heat.
type Handlers struct {
	svc    *Service
	logger *slog.Logger
}

func NewHandlers(svc *Service, logger *slog.Logger) *Handlers {
	return &Handlers{svc: svc, logger: logger}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.GET("/workouts/:id/heat", h.heat)
}

// heat godoc
// @Summary      Heat load and suggested adjustment for a planned session
// @Description  For a PLANNED workout: resolves the session date's location (the same primitive `/locations/resolve` exposes — the echoed name shows which city the forecast came from), fetches the session window's forecast, and computes a composite `heat_load_c` = heat index (temperature × humidity) + a solar penalty scaled down by cloud cover − bounded wind cooling. Acclimatization is DERIVED, not asked for: `low` (< 2) / `medium` (2–4) / `good` (≥ 5) qualifying sessions — outdoor, completed, ≥ 60 min, session heat index ≥ 25 °C, trailing 14 days — with the qualifying workout ids echoed so the level traces back to real rides. The suggested adjustment is a percentage reduction off the effective baseline (FTP for bike, threshold pace for run) from a fixed heat-load × duration × acclimatization table, plus a fluid note scaled from the measured sweat rate when one exists (a generic 600 ml/hr default is flagged as such). Degradations are `200`s: an `indoor` planned session returns `not_applicable: true` without fetching weather; a null environment computes with `assumed_outdoor: true`; no resolvable location → `reason: "location_unconfigured"`; forecast trouble → `reason: "weather_unavailable"`. A non-planned workout returns 409 — this is a pre-session question. **This is a practice HEURISTIC, not WBGT and not physiology** — there is no solar sensor, cloud cover is the proxy, and every constant is v1 and echoed. Compute-on-read: persists nothing and writes no target anywhere; applying an adjustment means proposing edits to the scheduled workout through the existing confirmed flows.
// @Tags         workouts
// @Produce      json
// @Param        id  path  string  true  "Workout UUID"
// @Success      200  {object}  Report
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

	out, err := h.svc.ReportFor(c.Request.Context(), id)
	if err != nil {
		switch {
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
