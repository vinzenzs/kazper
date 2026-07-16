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
	rg.GET("/workouts/:id/intervals", h.intervals)
	rg.GET("/workouts/:id/quadrant", h.quadrant)
	rg.GET("/workouts/:id/stride", h.stride)
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

// intervals godoc
// @Summary      Detect work intervals from a stored power stream
// @Description  Detects sustained work efforts in the workout's stored 1 Hz power stream with a deterministic, parameter-free procedure: a 30-second centered rolling mean, a work/rest threshold derived from the ride's own smoothed power distribution by Otsu's method (reported as `threshold_w`), merging of work spans separated by ≤ 30 s, and discarding of assembled spans shorter than 60 s. Returns each interval's `n`/`start_s`/`end_s`/`duration_s`/`avg_w`/`max_w`/`kj` (avg/max/kj over the raw stream), the rest gaps between them, and a summary. A ride whose smoothed power isn't meaningfully bimodal returns `200` with `threshold_w: null`, `intervals: []`, and `reason: "no_distinct_efforts"`. Cycling power only. Compute-on-read; nothing persisted.
// @Tags         workouts
// @Produce      json
// @Param        id  path  string  true  "Workout UUID"
// @Success      200  {object}  IntervalsResult
// @Failure      404  {object}  map[string]string  "workout_not_found | streams_not_found | power_stream_missing"
// @Security     BearerAuth
// @Router       /workouts/{id}/intervals [get]
func (h *Handlers) intervals(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workout_not_found"})
		return
	}
	res, err := h.svc.DetectIntervals(c.Request.Context(), id)
	if err != nil {
		writeErr(c, err)
		return
	}
	roundIntervals(res)
	c.JSON(http.StatusOK, res)
}

// roundIntervals rounds the response at the boundary: watts to whole numbers,
// kJ to 1dp. Detection runs at full precision.
func roundIntervals(res *IntervalsResult) {
	if res.ThresholdW != nil {
		t := math.Round(*res.ThresholdW)
		res.ThresholdW = &t
	}
	for i := range res.Intervals {
		res.Intervals[i].AvgW = math.Round(res.Intervals[i].AvgW)
		res.Intervals[i].MaxW = math.Round(res.Intervals[i].MaxW)
		res.Intervals[i].KJ = numfmt.Round1(res.Intervals[i].KJ)
	}
	for i := range res.Rests {
		res.Rests[i].AvgW = math.Round(res.Rests[i].AvgW)
	}
	res.Summary.MeanEffortS = numfmt.Round1(res.Summary.MeanEffortS)
	res.Summary.MeanEffortW = math.Round(res.Summary.MeanEffortW)
}

// quadrant godoc
// @Summary      Force/velocity quadrant analysis from power + cadence streams
// @Description  Classifies each pedaling sample of the workout's stored power + cadence streams into a Coggan force/velocity quadrant. Per sample: `CPV = cadence × crank × 2π/60` (m/s) and `AEPF = power / CPV` (N), split at the reference point implied by `cp_watts` at `cadence_rpm` (crank default 172.5 mm). Returns the share of pedaling time in each quadrant (I high-force/high-velocity … IV low-force/high-velocity), pedaling vs excluded (coasting/dropout) seconds, the reference point, and a downsampled scatter (≤1000 points; omitted under `summary_only=true`). `cp_watts` and `cadence_rpm` are explicit params (compose with `cp_model` + the athlete's cadence — no config coupling). Cycling power+cadence only. Compute-on-read; nothing persisted.
// @Tags         workouts
// @Produce      json
// @Param        id           path   string   true   "Workout UUID"
// @Param        cp_watts     query  number   true   "Critical power / threshold in watts (> 0)"
// @Param        cadence_rpm  query  number   true   "Reference (self-selected) cadence in rpm (> 0)"
// @Param        crank_mm     query  number   false  "Crank length in mm (default 172.5; 100–220)"
// @Param        summary_only query  boolean  false  "Omit the scatter, return shares only"
// @Success      200  {object}  QuadrantResult
// @Failure      400  {object}  map[string]string  "cp_invalid | cadence_invalid | crank_invalid"
// @Failure      404  {object}  map[string]string  "workout_not_found | streams_not_found | power_stream_missing | cadence_stream_missing"
// @Security     BearerAuth
// @Router       /workouts/{id}/quadrant [get]
func (h *Handlers) quadrant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workout_not_found"})
		return
	}
	cp, ok := positiveFloatParam(c, "cp_watts", "cp_invalid")
	if !ok {
		return
	}
	cadence, ok := positiveFloatParam(c, "cadence_rpm", "cadence_invalid")
	if !ok {
		return
	}
	crank := defaultCrankMM
	if raw := c.Query("crank_mm"); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < crankMinMM || v > crankMaxMM {
			c.JSON(http.StatusBadRequest, gin.H{"error": "crank_invalid"})
			return
		}
		crank = v
	}
	summaryOnly := c.Query("summary_only") == "true"

	res, err := h.svc.Quadrant(c.Request.Context(), id, cp, cadence, crank, summaryOnly)
	if err != nil {
		writeErr(c, err)
		return
	}
	roundQuadrant(res)
	c.JSON(http.StatusOK, res)
}

// roundQuadrant rounds at the boundary: shares 1dp, refs + scatter 2dp. Params
// echoed as given.
func roundQuadrant(res *QuadrantResult) {
	res.Summary.Q1Pct = numfmt.Round1(res.Summary.Q1Pct)
	res.Summary.Q2Pct = numfmt.Round1(res.Summary.Q2Pct)
	res.Summary.Q3Pct = numfmt.Round1(res.Summary.Q3Pct)
	res.Summary.Q4Pct = numfmt.Round1(res.Summary.Q4Pct)
	res.Summary.AEPFRefN = numfmt.Round2(res.Summary.AEPFRefN)
	res.Summary.CPVRefMps = numfmt.Round2(res.Summary.CPVRefMps)
	for i := range res.Scatter {
		res.Scatter[i].AEPFN = numfmt.Round2(res.Scatter[i].AEPFN)
		res.Scatter[i].CPVMps = numfmt.Round2(res.Scatter[i].CPVMps)
	}
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
	case errors.Is(err, ErrCadenceStreamMissing):
		c.JSON(http.StatusNotFound, gin.H{"error": "cadence_stream_missing"})
	case errors.Is(err, ErrSpeedStreamMissing):
		c.JSON(http.StatusNotFound, gin.H{"error": "speed_stream_missing"})
	case errors.Is(err, ErrSportUnsupported):
		c.JSON(http.StatusConflict, gin.H{"error": "sport_unsupported"})
	case errors.Is(err, ErrDownsampleInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "downsample_invalid"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streams_failed"})
	}
}

// stride godoc
// @Summary      Run cadence-vs-step-length decomposition
// @Description  For a RUN, decomposes where speed comes from: turnover or step length, and which one plateaus. Speed = cadence × step length, so per-sample `step_length_m = speed / (cadence/60)` — metres per single ground contact (Garmin's "step length", ~1.0–1.3 m; "stride" conventionally means two steps, hence the naming). Qualifying samples (speed AND cadence both positive) are bucketed into 0.25 m/s speed bins over the observed range; each bin reports its seconds, mean cadence and mean step length, so a plateau is VISIBLE rather than asserted. Because `ln(speed) = ln(cadence) + ln(step)`, the time-weighted log-log slopes over the bin means sum to 1 and split the speed gain into `cadence_contribution_pct` / `step_contribution_pct` — the verdict is always decomposed, never a bare "you are stride-limited" label. Samples with non-positive speed or cadence (standing, dropouts) are excluded and counted in `excluded_s`. Optional `min_speed_mps` ([0.5, 5.0]) trims walk breaks and is echoed when applied; default off, since a fixed cutoff would misread slow trail runs. A run whose bins span < 0.5 m/s returns the bins with `contribution: null` and `reason: "insufficient_speed_range"` — a steady-state run genuinely holds no answer. `summary_only=true` omits the ≤1000-point scatter. Reads best on runs with pace variety (intervals, fartlek, progressions). Compute-on-read: nothing persisted; step length and cadence feed no nutrition/hydration/energy total.
// @Tags         workouts
// @Produce      json
// @Param        id             path   string   true   "Workout UUID"
// @Param        min_speed_mps  query  number   false  "Exclude samples slower than this (m/s, 0.5..5.0) — e.g. walk breaks"
// @Param        summary_only   query  boolean  false  "Omit the scatter (bins + split only)"
// @Success      200  {object}  StrideResult
// @Failure      400  {object}  map[string]string  "min_speed_invalid"
// @Failure      404  {object}  map[string]string  "workout_not_found | streams_not_found | speed_stream_missing | cadence_stream_missing"
// @Failure      409  {object}  map[string]string  "sport_unsupported"
// @Security     BearerAuth
// @Router       /workouts/{id}/stride [get]
func (h *Handlers) stride(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workout_not_found"})
		return
	}

	minSpeed := 0.0 // 0 = no cutoff; the param is opt-in
	if raw := c.Query("min_speed_mps"); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < minSpeedParamLow || v > minSpeedParamHigh {
			c.JSON(http.StatusBadRequest, gin.H{"error": "min_speed_invalid"})
			return
		}
		minSpeed = v
	}
	summaryOnly := c.Query("summary_only") == "true"

	res, err := h.svc.Stride(c.Request.Context(), id, minSpeed, summaryOnly)
	if err != nil {
		writeErr(c, err)
		return
	}
	roundStride(res)
	c.JSON(http.StatusOK, res)
}

// roundStride rounds at the boundary: step length 2dp (a centimetre is the
// meaningful grain), cadence and percentages 1dp. Full precision runs through
// the fit — the split is computed before any rounding.
func roundStride(res *StrideResult) {
	for i := range res.Bins {
		res.Bins[i].CadenceSPM = numfmt.Round1(res.Bins[i].CadenceSPM)
		res.Bins[i].StepLengthM = numfmt.Round2(res.Bins[i].StepLengthM)
		res.Bins[i].SpeedLowMps = numfmt.Round2(res.Bins[i].SpeedLowMps)
		res.Bins[i].SpeedHighMps = numfmt.Round2(res.Bins[i].SpeedHighMps)
	}
	if res.Contribution != nil {
		res.Contribution.CadencePct = numfmt.Round1(res.Contribution.CadencePct)
		res.Contribution.StepPct = numfmt.Round1(res.Contribution.StepPct)
	}
	for i := range res.Scatter {
		res.Scatter[i].SpeedMps = numfmt.Round2(res.Scatter[i].SpeedMps)
		res.Scatter[i].CadenceSPM = numfmt.Round1(res.Scatter[i].CadenceSPM)
		res.Scatter[i].StepLengthM = numfmt.Round2(res.Scatter[i].StepLengthM)
	}
}
