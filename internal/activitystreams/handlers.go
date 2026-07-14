package activitystreams

import (
	"errors"
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/numfmt"
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
	rg.GET("/workouts/:id/w-prime-balance", h.wPrimeBalance)
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

// wPrimeBalance godoc
// @Summary      Per-workout W′ balance over the stored power stream
// @Description  Computes the differential (Froncioni–Clarke–Skiba) W′-balance time series over the workout's stored 1 Hz power stream, given caller-supplied critical-power model parameters `cp_watts` and `w_prime_kj` (typically from GET /workouts/cp-model). Returns `params` (echoed), `duration_s`, a `summary` (minimum balance + when, end balance, max depletion %, seconds below 25% W′), and the `series` (W′bal in kJ per point). `summary_only=true` omits the series; `downsample=N` (10..5000) bucket-means the series for charting (the exact minimum always lives in the summary). The balance is NOT clamped at zero — a negative floor / >100% depletion means the supplied W′ is too low (re-fit via cp-model). Advisory: reads no athlete-config. Compute-on-read; nothing persisted.
// @Tags         workouts
// @Produce      json
// @Param        id            path   string   true   "Workout id (uuid)"
// @Param        cp_watts      query   number   true   "Critical power (W), > 0"
// @Param        w_prime_kj    query   number   true   "Anaerobic work capacity W′ (kJ), > 0"
// @Param        summary_only  query   boolean  false  "Omit the series, return params + summary only"
// @Param        downsample    query   int      false  "Bucket-mean the series down to N points (10..5000)"
// @Success      200  {object}  WPrimeBalanceResult
// @Failure      400  {object}  map[string]string  "cp_invalid | w_prime_invalid | downsample_invalid"
// @Failure      404  {object}  map[string]string  "workout_not_found | streams_not_found | power_stream_missing"
// @Security     BearerAuth
// @Router       /workouts/{id}/w-prime-balance [get]
func (h *Handlers) wPrimeBalance(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workout_not_found"})
		return
	}
	cp, ok := positiveFloatParam(c, "cp_watts", "cp_invalid")
	if !ok {
		return
	}
	wp, ok := positiveFloatParam(c, "w_prime_kj", "w_prime_invalid")
	if !ok {
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
	summaryOnly := c.Query("summary_only") == "true"

	res, err := h.svc.WPrimeBalance(c.Request.Context(), id, cp, wp, downsample, summaryOnly)
	if err != nil {
		writeErr(c, err)
		return
	}
	roundWPrime(res)
	c.JSON(http.StatusOK, res)
}

// positiveFloatParam parses a required, strictly-positive, finite float query
// param, writing the 400 (with `code`) and returning ok=false on any failure.
func positiveFloatParam(c *gin.Context, name, code string) (float64, bool) {
	v, err := strconv.ParseFloat(c.Query(name), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": code})
		return 0, false
	}
	return v, true
}

// roundWPrime rounds the response's kJ values + depletion% to 1dp at the
// boundary (params echoed as given; full precision through the recursion).
func roundWPrime(res *WPrimeBalanceResult) {
	res.Summary.MinWPrimeKJ = numfmt.Round1(res.Summary.MinWPrimeKJ)
	res.Summary.EndWPrimeKJ = numfmt.Round1(res.Summary.EndWPrimeKJ)
	res.Summary.MaxDepletionPct = numfmt.Round1(res.Summary.MaxDepletionPct)
	for i := range res.Series {
		res.Series[i] = numfmt.Round1(res.Series[i])
	}
}

func writeErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrWorkoutNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "workout_not_found"})
	case errors.Is(err, ErrStreamsNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "streams_not_found"})
	case errors.Is(err, ErrPowerStreamMissing):
		c.JSON(http.StatusNotFound, gin.H{"error": "power_stream_missing"})
	case errors.Is(err, ErrDownsampleInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "downsample_invalid"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streams_failed"})
	}
}
