package effortanalytics

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/kazper/internal/numfmt"
)

// maxRangeDays caps the curve window; year-to-date and longer all-time-ish
// windows are the point of a mean-maximal curve, so it clears a full year.
const maxRangeDays = 400

// WeightProvider resolves the most-recent stored body weight for the W/kg
// denominator when the request omits an explicit weight_kg. A narrow interface
// (the workoutcompliance injection pattern) so effort-analytics stays decoupled
// from the bodyweight package; `found=false` means no entry has ever been logged.
type WeightProvider interface {
	LatestWeightKg(ctx context.Context) (kg float64, found bool, err error)
}

// Handlers wires the effort-analytics endpoints onto a Gin router group.
type Handlers struct {
	svc           *Service
	weight        WeightProvider
	defaultTZName string
	logger        *slog.Logger
}

func NewHandlers(svc *Service, weight WeightProvider, defaultTZ string, logger *slog.Logger) *Handlers {
	return &Handlers{svc: svc, weight: weight, defaultTZName: defaultTZ, logger: logger}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	// The POST /workouts/:id/streams ingest route moved to the activity-streams
	// capability (persist-activity-streams), which persists the raw arrays and
	// then delegates the mean-maximal computation back here via ComputeAndReplace.
	// effort-analytics keeps the read-side curve.
	rg.GET("/workouts/power-curve", h.curve)
	rg.GET("/workouts/cp-model", h.cpModel)
	rg.GET("/workouts/cp-model/history", h.cpModelHistory)
	rg.GET("/workouts/power-profile", h.powerProfile)
	rg.GET("/workouts/durability", h.durability)
}

// curve godoc
// @Summary      Mean-maximal power/pace curve over a window
// @Description  Returns, per ladder duration, the best (max) mean value achieved across completed workouts in `[from, to]`, with the contributing workout id and date. Metric is derived from `sport`: cycling → power (W), run/swim → speed (m/s, rendered as pace by clients). Only workouts of the requested sport contribute — a run's running-power rows never enter the bike curve, and a bike's speed rows never enter run/swim pace curves; multisport workouts match no sport-scoped window. Range capped at 400 days. Empty window returns an empty points list.
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
	if sport == "" {
		sport = "bike"
	}
	metric := metricForSport(sport)
	points, err := h.svc.CurveFor(c.Request.Context(), CurveParams{From: from, To: to, Loc: loc, Metric: metric, Sport: sport})
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
// @Description  Fits the 2-parameter critical-power model to the window's best-effort records (bike **power** only): for each ladder duration between 2 and 30 minutes it takes the windowed best (the same per-duration MAX the power curve serves) as one fit point and computes an OLS fit in work–time form, returning `model` with `cp_watts`, `w_prime_kj`, `r_squared`, `rmse_w` plus the `points` used. The model is **advisory** — this endpoint never reads or writes athlete-config; comparing CP against the configured FTP is the caller's job. When the window can't support a fit the response is still `200` with `model: null` and a `reason` (`insufficient_points` / `span_too_narrow`). A fit whose `r_squared` is below 0.5 is returned but flagged `warning: "poor_fit"` — treat its CP/W′ as unreliable. Only bike workouts' efforts enter the fit (running power is excluded). Compute-on-read; nothing persisted. Range capped at 400 days.
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

// powerProfile godoc
// @Summary      Power-profile ranking against the Coggan tables
// @Description  Ranks the athlete's windowed best power efforts at four benchmark durations — 5 s (neuromuscular), 1 min (anaerobic), 5 min (VO₂max), and the 20-min best as a functional-threshold proxy (no 0.95 haircut) — against the embedded Coggan power-profile tables. Each anchor carries `watts`, `w_per_kg`, the Coggan `category` band, an interpolated `percentile` (0–100, an **estimate** — the category is authoritative), and the contributing workout. `phenotype` (sprinter / time_trialist / climber / all_rounder) is null unless all four anchors are present. W/kg denominator: the `weight_kg` query param (> 0), else the most-recent stored body weight, else `400 weight_data_missing`; the response echoes `weight_source`. `sex` selects the table (male default). **Advisory** — never reads or writes athlete-config. Power only. Compute-on-read; nothing persisted. Range capped at 400 days.
// @Tags         workouts
// @Produce      json
// @Param        from       query  string  true   "Inclusive start date YYYY-MM-DD"
// @Param        to         query  string  true   "Inclusive end date YYYY-MM-DD"
// @Param        tz         query  string  false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Param        weight_kg  query  number  false  "Body-weight override in kg (> 0); falls back to the latest stored weight"
// @Param        sex        query  string  false  "male (default) | female — selects the Coggan table"
// @Success      200  {object}  PowerProfileResult
// @Failure      400  {object}  map[string]interface{}  "range_required | date_invalid | range_invalid | range_too_large | tz_invalid | weight_kg_invalid | sex_invalid | weight_data_missing"
// @Security     BearerAuth
// @Router       /workouts/power-profile [get]
func (h *Handlers) powerProfile(c *gin.Context) {
	from, to, loc, ok := h.parseWindow(c)
	if !ok {
		return
	}
	sex := c.Query("sex")
	switch sex {
	case "":
		sex = sexMale
	case sexMale, sexFemale:
		// ok
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "sex_invalid"})
		return
	}

	weightKg, source, ok := h.resolveWeight(c)
	if !ok {
		return
	}

	res, err := h.svc.PowerProfileFor(c.Request.Context(), CurveParams{From: from, To: to, Loc: loc}, weightKg, sex)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "power_profile_failed"})
		return
	}
	res.From = from.Format("2006-01-02")
	res.To = to.Format("2006-01-02")
	res.TZ = loc.String()
	res.WeightSource = source
	roundPowerProfile(res)
	c.JSON(http.StatusOK, res)
}

// resolveWeight returns the W/kg denominator and its provenance: an explicit
// weight_kg param (> 0) wins; otherwise the latest stored body weight; otherwise
// 400 weight_data_missing. Writes the 400 and returns ok=false on any failure.
func (h *Handlers) resolveWeight(c *gin.Context) (kg float64, source string, ok bool) {
	if s := c.Query("weight_kg"); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil || v <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "weight_kg_invalid"})
			return 0, "", false
		}
		return v, WeightSourceParam, true
	}
	stored, found, err := h.weight.LatestWeightKg(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "power_profile_failed"})
		return 0, "", false
	}
	if !found || stored <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "weight_data_missing"})
		return 0, "", false
	}
	return stored, WeightSourceStored, true
}

// roundPowerProfile applies numfmt rounding at the response boundary: W/kg and
// percentile to 1dp, watts to 1dp (already stored rounded). The weight echo is
// rounded to 1dp so a stored 72.53 doesn't leak full precision.
func roundPowerProfile(res *PowerProfileResult) {
	res.WeightKg = numfmt.Round1(res.WeightKg)
	for i := range res.Anchors {
		res.Anchors[i].Watts = numfmt.Round1(res.Anchors[i].Watts)
		res.Anchors[i].WPerKg = numfmt.Round1(res.Anchors[i].WPerKg)
		res.Anchors[i].Percentile = numfmt.Round1(res.Anchors[i].Percentile)
	}
}

// cpModelHistory godoc
// @Summary      Critical-power (CP2) model fitted at weekly anchors over a range
// @Description  Fits the 2-parameter critical-power model at each Monday anchor in `[from, to]`, each over its trailing `window_days` (default 90, bounds [30, 365]) — the CP-over-time trend, the data-derived counterpart to the configured-FTP history. Per anchor: the fitted `model` (or `null` with the gate `reason` when the trailing window can't support a fit — the trend gaps, it doesn't zero). Advisory — this endpoint never reads or writes athlete-config. Compute-on-read; nothing persisted. Range capped at 400 days.
// @Tags         workouts
// @Produce      json
// @Param        from         query  string  true   "Inclusive start date YYYY-MM-DD"
// @Param        to           query  string  true   "Inclusive end date YYYY-MM-DD"
// @Param        tz           query  string  false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Param        window_days  query  int     false  "Trailing fit window per anchor in days (default 90; 30–365)"
// @Success      200  {object}  CPModelHistoryResult
// @Failure      400  {object}  map[string]interface{}  "range_required | date_invalid | range_invalid | range_too_large | tz_invalid | window_days_invalid"
// @Security     BearerAuth
// @Router       /workouts/cp-model/history [get]
func (h *Handlers) cpModelHistory(c *gin.Context) {
	from, to, loc, ok := h.parseWindow(c)
	if !ok {
		return
	}
	windowDays := cpHistoryWindowDefault
	if raw := c.Query("window_days"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < cpHistoryWindowMin || v > cpHistoryWindowMax {
			c.JSON(http.StatusBadRequest, gin.H{"error": "window_days_invalid"})
			return
		}
		windowDays = v
	}
	res, err := h.svc.CPModelHistoryFor(c.Request.Context(), CurveParams{From: from, To: to, Loc: loc}, windowDays)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cp_model_history_failed"})
		return
	}
	res.From = from.Format("2006-01-02")
	res.To = to.Format("2006-01-02")
	res.TZ = loc.String()
	for i := range res.Anchors {
		if res.Anchors[i].Model != nil {
			m := res.Anchors[i].Model
			m.CPWatts = numfmt.Round1(m.CPWatts)
			m.WPrimeKJ = numfmt.Round1(m.WPrimeKJ)
			m.RSquared = numfmt.Round3(m.RSquared)
			m.RMSEW = numfmt.Round1(m.RMSEW)
		}
	}
	c.JSON(http.StatusOK, res)
}

// durability godoc
// @Summary      Fatigue-resistance (durability) fade table over a window
// @Description  For each durability duration (1m/5m/20m), the fresh (tier-0) windowed best power vs the best power whose window starts after 500/1000/1500/2000 kJ of accumulated work, with `fade_pct = (fresh − tier)/fresh × 100` and each entry's contributing workout/date. Tiers with no data in the window are omitted; a window holding only fresh rows returns `reason: "no_tiered_data"` (historical rides gain tiers only after their streams are re-run through the recompute path). Cycling power only. Compute-on-read over stored best-effort rows (no stream scans); nothing persisted. Range capped at 400 days.
// @Tags         workouts
// @Produce      json
// @Param        from  query  string  true   "Inclusive start date YYYY-MM-DD"
// @Param        to    query  string  true   "Inclusive end date YYYY-MM-DD"
// @Param        tz    query  string  false  "IANA timezone (defaults to DEFAULT_USER_TZ)"
// @Success      200  {object}  DurabilityResult
// @Failure      400  {object}  map[string]interface{}  "range_required | date_invalid | range_invalid | range_too_large | tz_invalid"
// @Security     BearerAuth
// @Router       /workouts/durability [get]
func (h *Handlers) durability(c *gin.Context) {
	from, to, loc, ok := h.parseWindow(c)
	if !ok {
		return
	}
	res, err := h.svc.DurabilityFor(c.Request.Context(), CurveParams{From: from, To: to, Loc: loc})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "durability_failed"})
		return
	}
	res.From = from.Format("2006-01-02")
	res.To = to.Format("2006-01-02")
	res.TZ = loc.String()
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
