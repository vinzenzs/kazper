package activitystreams

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handlers wires the activity-streams endpoints onto a Gin router group.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/workouts/:id/streams", h.ingest)
	rg.GET("/workouts/:id/streams", h.retrieve)
	rg.POST("/workouts/:id/streams/recompute", h.recompute)
}

// ingest godoc
// @Summary      Ingest a workout's raw sample streams
// @Description  Persists contiguous 1 Hz power (W), speed (m/s) and/or heart_rate (bpm) sample arrays for a completed workout (replacing any prior streams), refreshes the mean-maximal best-effort ladder, and derives + stores the execution metrics (variability_index, efficiency_factor, decoupling_pct) onto the workout. An empty body writes nothing. Streams are workout-only and feed no nutrition total.
// @Tags         workouts
// @Accept       json
// @Produce      json
// @Param        id       path  string        true  "Workout id (uuid)"
// @Param        streams  body  StreamPayload  true  "1 Hz power/speed/heart_rate sample arrays"
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
	res, err := h.svc.Ingest(c.Request.Context(), id, p)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// retrieve godoc
// @Summary      Retrieve a workout's stored streams
// @Description  Returns the workout's stored 1 Hz streams keyed by type (absent types omitted). Pass `downsample=N` (10..5000) to bucket-mean each series down to N points for charting; `duration_s` always reflects the full-resolution activity length.
// @Tags         workouts
// @Produce      json
// @Param        id          path   string  true   "Workout id (uuid)"
// @Param        downsample  query  int     false  "Bucket-mean each series down to N points (10..5000)"
// @Success      200  {object}  StreamsResponse
// @Failure      400  {object}  map[string]string  "downsample_invalid"
// @Failure      404  {object}  map[string]string  "workout_not_found | streams_not_found"
// @Security     BearerAuth
// @Router       /workouts/{id}/streams [get]
func (h *Handlers) retrieve(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workout_not_found"})
		return
	}
	var downsample *int
	if raw := c.Query("downsample"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "downsample_invalid"})
			return
		}
		downsample = &n
	}
	res, err := h.svc.Retrieve(c.Request.Context(), id, downsample)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// recompute godoc
// @Summary      Recompute stream-derived metrics from stored streams
// @Description  Reloads the workout's stored streams and rewrites the best-effort ladder and execution metrics (variability_index, efficiency_factor, decoupling_pct) from them, without a re-post. Used after a metric-formula change or a partial earlier ingest.
// @Tags         workouts
// @Produce      json
// @Param        id  path  string  true  "Workout id (uuid)"
// @Success      200  {object}  RecomputeResult
// @Failure      404  {object}  map[string]string  "workout_not_found | streams_not_found"
// @Security     BearerAuth
// @Router       /workouts/{id}/streams/recompute [post]
func (h *Handlers) recompute(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workout_not_found"})
		return
	}
	res, err := h.svc.Recompute(c.Request.Context(), id)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func writeErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrWorkoutNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "workout_not_found"})
	case errors.Is(err, ErrStreamsNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "streams_not_found"})
	case errors.Is(err, ErrDownsampleInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "downsample_invalid"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streams_failed"})
	}
}
