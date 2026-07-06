package effortanalytics

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// maxRangeDays caps the curve window; year-to-date and longer all-time-ish
// windows are the point of a mean-maximal curve, so it clears a full year.
const maxRangeDays = 400

// Handlers wires the effort-analytics endpoints onto a Gin router group.
type Handlers struct {
	svc           *Service
	defaultTZName string
	logger        *slog.Logger
}

func NewHandlers(svc *Service, defaultTZ string, logger *slog.Logger) *Handlers {
	return &Handlers{svc: svc, defaultTZName: defaultTZ, logger: logger}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/workouts/:id/streams", h.ingest)
	rg.GET("/workouts/power-curve", h.curve)
}

// ingest godoc
// @Summary      Ingest a workout's power/speed streams
// @Description  Accepts contiguous 1 Hz power (W) and/or speed (m/s) sample arrays for a completed workout, computes the mean-maximal value at the fixed duration ladder (5s…60m), and stores the compact best-effort records (replacing any prior set for the workout). Raw streams are not persisted. An empty body writes nothing. Distance/power/speed are workout-only and feed no nutrition total.
// @Tags         workouts
// @Accept       json
// @Produce      json
// @Param        id       path  string        true  "Workout id (uuid)"
// @Param        streams  body  StreamPayload  true  "1 Hz power/speed sample arrays"
// @Success      200  {object}  IngestResult
// @Failure      400  {object}  map[string]string  "invalid_body"
// @Failure      404  {object}  map[string]string  "workout_not_found"
// @Security     BearerAuth
// @Router       /workouts/{id}/streams [post]
func (h *Handlers) ingest(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workout_not_found"})
		return
	}
	var p StreamPayload
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	n, err := h.svc.IngestStreams(c.Request.Context(), id, p)
	if err != nil {
		if errors.Is(err, ErrWorkoutNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "workout_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ingest_failed"})
		return
	}
	c.JSON(http.StatusOK, IngestResult{RecordsWritten: n})
}

// curve godoc
// @Summary      Mean-maximal power/pace curve over a window
// @Description  Returns, per ladder duration, the best (max) mean value achieved across completed workouts in `[from, to]`, with the contributing workout id and date. Metric is derived from `sport`: cycling → power (W), run/swim → speed (m/s, rendered as pace by clients). Range capped at 400 days. Empty window returns an empty points list.
// @Tags         workouts
// @Produce      json
// @Param        from   query  string  true   "Inclusive start date YYYY-MM-DD"
// @Param        to     query  string  true   "Inclusive end date YYYY-MM-DD"
// @Param        sport  query  string  false  "bike (→power) | run | swim (→speed); defaults to bike"
// @Param        tz     query  string  false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Success      200  {object}  Curve
// @Failure      400  {object}  map[string]interface{}  "range_required | date_invalid | range_invalid | range_too_large | tz_invalid"
// @Security     BearerAuth
// @Router       /workouts/power-curve [get]
func (h *Handlers) curve(c *gin.Context) {
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

	sport := c.Query("sport")
	metric := metricForSport(sport)
	points, err := h.svc.CurveFor(c.Request.Context(), CurveParams{From: from, To: to, Loc: loc, Metric: metric})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "curve_failed"})
		return
	}
	if points == nil {
		points = []CurvePoint{}
	}
	c.JSON(http.StatusOK, Curve{
		From:   from.Format("2006-01-02"),
		To:     to.Format("2006-01-02"),
		TZ:     loc.String(),
		Sport:  sport,
		Metric: metric,
		Points: points,
	})
}

// metricForSport maps a sport to its curve metric: cycling → power, everything
// else (run/swim/…) → speed. An empty sport defaults to power (cycling).
func metricForSport(sport string) Metric {
	if sport == "" || sport == "bike" {
		return MetricPower
	}
	return MetricSpeed
}

func (h *Handlers) resolveTZ(c *gin.Context) (*time.Location, error) {
	tz := c.Query("tz")
	if tz == "" {
		tz = h.defaultTZName
		h.logger.Warn("power curve used default tz", "route", c.FullPath(), "default_tz", tz)
	}
	return time.LoadLocation(tz)
}
