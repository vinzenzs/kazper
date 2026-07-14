package effortanalytics

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/kazper/internal/numfmt"
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
	// The POST /workouts/:id/streams ingest route moved to the activity-streams
	// capability (persist-activity-streams), which persists the raw arrays and
	// then delegates the mean-maximal computation back here via ComputeAndReplace.
	// effort-analytics keeps the read-side curve.
	rg.GET("/workouts/power-curve", h.curve)
	rg.GET("/workouts/cp-model", h.cpModel)
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
	from, to, loc, ok := h.parseWindow(c)
	if !ok {
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

// cpModel godoc
// @Summary      Critical-power (CP2) model over a window
// @Description  Fits the 2-parameter critical-power model to the window's best-effort records (bike **power** only): for each ladder duration between 2 and 30 minutes it takes the windowed best (the same per-duration MAX the power curve serves) as one fit point and computes an OLS fit in work–time form, returning `model` with `cp_watts`, `w_prime_kj`, `r_squared`, `rmse_w` plus the `points` used. The model is **advisory** — this endpoint never reads or writes athlete-config; comparing CP against the configured FTP is the caller's job. When the window can't support a fit the response is still `200` with `model: null` and a `reason` (`insufficient_points` / `span_too_narrow`). Compute-on-read; nothing persisted. Range capped at 400 days.
// @Tags         workouts
// @Produce      json
// @Param        from  query  string  true   "Inclusive start date YYYY-MM-DD"
// @Param        to    query  string  true   "Inclusive end date YYYY-MM-DD"
// @Param        tz    query  string  false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Success      200  {object}  CPModelResult
// @Failure      400  {object}  map[string]interface{}  "range_required | date_invalid | range_invalid | range_too_large | tz_invalid"
// @Security     BearerAuth
// @Router       /workouts/cp-model [get]
func (h *Handlers) cpModel(c *gin.Context) {
	from, to, loc, ok := h.parseWindow(c)
	if !ok {
		return
	}
	res, err := h.svc.CPModelFor(c.Request.Context(), CurveParams{From: from, To: to, Loc: loc})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cp_model_failed"})
		return
	}
	res.From = from.Format("2006-01-02")
	res.To = to.Format("2006-01-02")
	res.TZ = loc.String()
	roundCPResult(res)
	c.JSON(http.StatusOK, res)
}

// parseWindow parses the shared from/to/tz window contract, writing the 400 and
// returning ok=false on any violation. Reused by the curve and cp-model reads.
func (h *Handlers) parseWindow(c *gin.Context) (from, to time.Time, loc *time.Location, ok bool) {
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
	to, err = time.Parse("2006-01-02", toStr)
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
	loc, err = h.resolveTZ(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tz_invalid"})
		return
	}
	return from, to, loc, true
}

// roundCPResult applies numfmt rounding at the response boundary: CP/RMSE and W′
// (kJ) to 1dp, R² to 3dp, and each point's watts to 1dp. Storage/fit stay full
// precision.
func roundCPResult(res *CPModelResult) {
	if res.Model != nil {
		res.Model.CPWatts = numfmt.Round1(res.Model.CPWatts)
		res.Model.WPrimeKJ = numfmt.Round1(res.Model.WPrimeKJ)
		res.Model.RSquared = numfmt.Round3(res.Model.RSquared)
		res.Model.RMSEW = numfmt.Round1(res.Model.RMSEW)
	}
	for i := range res.Points {
		res.Points[i].Watts = numfmt.Round1(res.Points[i].Watts)
	}
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
