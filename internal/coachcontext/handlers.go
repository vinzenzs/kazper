package coachcontext

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const dateFormat = "2006-01-02"

// Handlers wires GET /context/training and GET /context/recovery.
type Handlers struct {
	svc           *Service
	defaultTZName string
	logger        *slog.Logger
}

func NewHandlers(svc *Service, defaultTZ string, logger *slog.Logger) *Handlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handlers{svc: svc, defaultTZName: defaultTZ, logger: logger}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.GET("/context/training", h.training)
	rg.GET("/context/recovery", h.recovery)
}

// resolveDate parses the date query param (defaulting to today in loc) and the
// loc from the tz query param (defaulting to the server zone). Returns an error
// code string ("" on success).
func (h *Handlers) resolveDate(c *gin.Context) (time.Time, *time.Location, string) {
	tzName := c.Query("tz")
	if tzName == "" {
		tzName = h.defaultTZName
	}
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return time.Time{}, nil, "tz_invalid"
	}
	dateStr := c.Query("date")
	if dateStr == "" {
		now := time.Now().In(loc)
		return now, loc, ""
	}
	date, err := time.ParseInLocation(dateFormat, dateStr, loc)
	if err != nil {
		return time.Time{}, nil, "date_invalid"
	}
	return date, loc, ""
}

func intQuery(c *gin.Context, key string) int {
	if v := c.Query(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0 // 0 → service substitutes the default
}

// training godoc
// @Summary      Training context bundle
// @Description  One read for grounding training advice: the covering training phase, the latest fitness snapshot (VO2max, acute/chronic load, training status, race predictors) with derived ACWR, a recent-load summary plus recent completed workouts (lookback_days, default 14), and upcoming planned workouts (lookahead_days, default 7). Composition-only over existing repos; for per-entry detail use the dedicated tools (list_workouts, list_fitness_metrics, …).
// @Tags         coach-context
// @Produce      json
// @Param        date            query  string  false  "Calendar date YYYY-MM-DD (defaults to today)"
// @Param        tz              query  string  false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Param        lookback_days   query  int     false  "Completed-workout/fitness lookback window (default 14, max 90)"
// @Param        lookahead_days  query  int     false  "Planned-workout lookahead window (default 7, max 60)"
// @Success      200  {object}  TrainingContext
// @Failure      400  {object}  map[string]string  "date_invalid | tz_invalid"
// @Failure      500  {object}  map[string]string  "context_failed"
// @Security     BearerAuth
// @Router       /context/training [get]
func (h *Handlers) training(c *gin.Context) {
	date, loc, code := h.resolveDate(c)
	if code != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": code})
		return
	}
	out, err := h.svc.BuildTraining(c.Request.Context(), date, loc,
		intQuery(c, "lookback_days"), intQuery(c, "lookahead_days"))
	if err != nil {
		h.logger.Warn("training context build failed", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "context_failed"})
		return
	}
	c.JSON(http.StatusOK, out)
}

// recovery godoc
// @Summary      Recovery context bundle
// @Description  One read for grounding recovery/readiness advice: the latest recovery snapshot on/before the date (sleep, HRV, resting HR, body battery, training readiness, …) plus the recent trend over `days` (default 7). Composition-only; for per-day detail use list_recovery_metrics.
// @Tags         coach-context
// @Produce      json
// @Param        date  query  string  false  "Calendar date YYYY-MM-DD (defaults to today)"
// @Param        tz    query  string  false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Param        days  query  int     false  "Trend window in days (default 7, max 90)"
// @Success      200  {object}  RecoveryContext
// @Failure      400  {object}  map[string]string  "date_invalid | tz_invalid"
// @Failure      500  {object}  map[string]string  "context_failed"
// @Security     BearerAuth
// @Router       /context/recovery [get]
func (h *Handlers) recovery(c *gin.Context) {
	date, _, code := h.resolveDate(c)
	if code != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": code})
		return
	}
	out, err := h.svc.BuildRecovery(c.Request.Context(), date, intQuery(c, "days"))
	if err != nil {
		h.logger.Warn("recovery context build failed", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "context_failed"})
		return
	}
	c.JSON(http.StatusOK, out)
}
