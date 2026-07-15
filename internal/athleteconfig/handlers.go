package athleteconfig

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/vinzenzs/kazper/internal/auth"
	"github.com/vinzenzs/kazper/internal/numfmt"
)

// Handlers exposes GET/PUT /athlete-config.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.GET("/athlete-config", h.get)
	rg.PUT("/athlete-config", h.put)
	rg.GET("/athlete-config/history", h.history)
	rg.GET("/athlete-config/garmin-detected", h.getDetection)
	rg.PUT("/athlete-config/garmin-detected", h.putDetection)
	rg.GET("/athlete-config/effective", h.getEffective)
	rg.PUT("/athlete-config/sources", h.putSources)
}

// get godoc
// @Summary      Get the athlete physiology configuration (singleton)
// @Description  Returns the single athlete-config row (FTP, thresholds, max HR, lactate-threshold HR, HR-zone and optional power-zone boundaries), or null before any config has been set. Capture-only mirror; these values feed no fueling computation.
// @Tags         athlete-config
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "{\"athlete_config\": AthleteConfig | null, \"sources\": [string]}"
// @Security     BearerAuth
// @Router       /athlete-config [get]
func (h *Handlers) get(c *gin.Context) {
	ctx := c.Request.Context()
	cfg, err := h.svc.Get(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "athlete_config_get_failed"})
		return
	}
	sources, err := h.svc.GetSources(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "athlete_config_get_failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"athlete_config": round(cfg), "sources": sources})
}

// put godoc
// @Summary      Set or replace the athlete physiology configuration (singleton)
// @Description  Full-replace semantics: absent fields are stored as NULL (cleared), matching PUT /goals. Every field is optional and must be > 0 when present. `Idempotency-Key` is NOT accepted on PUT — supplying the header returns `400 idempotency_unsupported_for_put`. Garmin is source-of-truth: the daily sync re-issues this PUT and overwrites manual edits.
// @Tags         athlete-config
// @Accept       json
// @Produce      json
// @Param        body  body  AthleteConfig  true  "Athlete config payload"
// @Success      200  {object}  map[string]interface{}  "{\"athlete_config\": AthleteConfig}"
// @Failure      400  {object}  map[string]string  "athlete_config_value_invalid | invalid_json | idempotency_unsupported_for_put"
// @Failure      403  {object}  map[string]string  "forbidden — the garmin identity cannot write the deliberate config"
// @Security     BearerAuth
// @Router       /athlete-config [put]
func (h *Handlers) put(c *gin.Context) {
	// The configured physiology and its threshold_history are exclusively
	// deliberate human/coach records — the automated garmin writer is refused
	// here and routed to PUT /athlete-config/garmin-detected instead.
	if auth.ClientFromContext(c) == auth.ClientGarmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	var cfg AthleteConfig
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &cfg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
			return
		}
	}
	stored, err := h.svc.Put(c.Request.Context(), &cfg)
	if err != nil {
		var verr *ValidationError
		if errors.As(err, &verr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "athlete_config_value_invalid", "field": verr.Field})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "athlete_config_upsert_failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"athlete_config": round(stored)})
}

// history godoc
// @Summary      Dated history of the athlete physiology configuration
// @Description  Returns the dated snapshots of the athlete-config state, ascending by `effective_from`, so threshold progression (e.g. "FTP 240 → 255 → 270 this season") is answerable from data. A snapshot is recorded only when a PUT changes physiology; the daily Garmin re-PUT of an unchanged config records nothing. The seed baseline uses `effective_from = 1970-01-01` (the oldest known state). Optional inclusive `from`/`to` bounds filter on `effective_from`. Empty history returns `{"history":[]}` (no 404). Pace floats are rounded to 1dp; storage keeps full precision.
// @Tags         athlete-config
// @Produce      json
// @Param        from  query  string  false  "Inclusive lower bound on effective_from (YYYY-MM-DD)"
// @Param        to    query  string  false  "Inclusive upper bound on effective_from (YYYY-MM-DD)"
// @Success      200  {object}  map[string]interface{}  "{\"history\": [ThresholdSnapshot]}"
// @Failure      400  {object}  map[string]string  "date_invalid | range_invalid"
// @Security     BearerAuth
// @Router       /athlete-config/history [get]
func (h *Handlers) history(c *gin.Context) {
	from, ok := parseDateParam(c, "from")
	if !ok {
		return
	}
	to, ok := parseDateParam(c, "to")
	if !ok {
		return
	}
	if from != nil && to != nil && from.After(*to) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_invalid"})
		return
	}
	snaps, err := h.svc.History(c.Request.Context(), from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "athlete_config_history_failed"})
		return
	}
	out := make([]*ThresholdSnapshot, 0, len(snaps))
	for _, s := range snaps {
		out = append(out, roundSnapshot(s))
	}
	c.JSON(http.StatusOK, gin.H{"history": out})
}

// parseDateParam reads an optional YYYY-MM-DD query param. Returns (nil, true)
// when absent, (parsed, true) when valid, and writes a 400 + (nil, false) when
// malformed.
func parseDateParam(c *gin.Context, field string) (*time.Time, bool) {
	raw, present := c.GetQuery(field)
	if !present || raw == "" {
		return nil, true
	}
	t, err := time.ParseInLocation("2006-01-02", raw, time.UTC)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid", "field": field})
		return nil, false
	}
	return &t, true
}

// roundSnapshot rounds the two pace floats at the response boundary (mirrors
// round for the singleton), preserving stored precision.
func roundSnapshot(s *ThresholdSnapshot) *ThresholdSnapshot {
	if s == nil {
		return nil
	}
	out := *s
	out.ThresholdPaceSecPerKm = numfmt.Round1Ptr(s.ThresholdPaceSecPerKm)
	out.ThresholdSwimPaceSecPer100m = numfmt.Round1Ptr(s.ThresholdSwimPaceSecPer100m)
	return &out
}

// round returns a copy with the two float fields rounded to 1dp for the
// response; integer fields and storage stay untouched.
func round(cfg *AthleteConfig) *AthleteConfig {
	if cfg == nil {
		return nil
	}
	out := *cfg
	out.ThresholdPaceSecPerKm = numfmt.Round1Ptr(cfg.ThresholdPaceSecPerKm)
	out.ThresholdSwimPaceSecPer100m = numfmt.Round1Ptr(cfg.ThresholdSwimPaceSecPer100m)
	return &out
}

// roundDetection rounds the detection's pace float at the response boundary.
func roundDetection(d *GarminDetectedThresholds) *GarminDetectedThresholds {
	if d == nil {
		return nil
	}
	out := *d
	out.ThresholdPaceSecPerKm = numfmt.Round1Ptr(d.ThresholdPaceSecPerKm)
	return &out
}

// getDetection godoc
// @Summary      Get the latest Garmin-detected thresholds (advisory singleton)
// @Description  Returns the latest physiology Garmin detected (FTP, lactate-threshold HR, max HR, run threshold pace, HR/power zone maxima) with `detected_at`, or null before any sync has written one. Advisory evidence — NOT the configured values; see GET /athlete-config for the deliberate record and GET /athlete-config/effective for what computations use.
// @Tags         athlete-config
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "{\"garmin_detected\": GarminDetectedThresholds | null}"
// @Security     BearerAuth
// @Router       /athlete-config/garmin-detected [get]
func (h *Handlers) getDetection(c *gin.Context) {
	d, err := h.svc.GetDetection(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "garmin_detected_get_failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"garmin_detected": roundDetection(d)})
}

// putDetection godoc
// @Summary      Record the latest Garmin-detected thresholds (garmin identity only)
// @Description  Full-replaces the advisory detection singleton with the values the daily sync mapped from Garmin. Accepted ONLY from the garmin identity (others → 403); writing a detection never reads or mutates athlete_config or threshold_history. `Idempotency-Key` is NOT accepted on PUT.
// @Tags         athlete-config
// @Accept       json
// @Produce      json
// @Param        body  body  GarminDetectedThresholds  true  "Detected thresholds payload"
// @Success      200  {object}  map[string]interface{}  "{\"garmin_detected\": GarminDetectedThresholds}"
// @Failure      400  {object}  map[string]string  "athlete_config_value_invalid | invalid_json | idempotency_unsupported_for_put"
// @Failure      403  {object}  map[string]string  "forbidden — only the garmin identity may write detections"
// @Security     BearerAuth
// @Router       /athlete-config/garmin-detected [put]
func (h *Handlers) putDetection(c *gin.Context) {
	if auth.ClientFromContext(c) != auth.ClientGarmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	var d GarminDetectedThresholds
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &d); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
			return
		}
	}
	stored, err := h.svc.PutDetection(c.Request.Context(), &d)
	if err != nil {
		var verr *ValidationError
		if errors.As(err, &verr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "athlete_config_value_invalid", "field": verr.Field})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "garmin_detected_upsert_failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"garmin_detected": roundDetection(stored)})
}

// getEffective godoc
// @Summary      Get the effective physiology config (resolved manual + detected)
// @Description  Returns the resolved view computations consume: per field, the Garmin-detected value where its source is `garmin` and a detection exists, the confirmed value otherwise. `field_sources` annotates each field with `manual` or `garmin`. With an empty source policy this equals GET /athlete-config field-for-field. Null before any config or applied detection exists.
// @Tags         athlete-config
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "{\"effective\": EffectiveConfig | null}"
// @Security     BearerAuth
// @Router       /athlete-config/effective [get]
func (h *Handlers) getEffective(c *gin.Context) {
	eff, err := h.svc.EffectiveConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "athlete_config_effective_failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"effective": roundEffective(eff)})
}

// roundEffective rounds the pace floats of the resolved config at the boundary.
func roundEffective(eff *EffectiveConfig) *EffectiveConfig {
	if eff == nil {
		return nil
	}
	out := *eff
	out.ThresholdPaceSecPerKm = numfmt.Round1Ptr(eff.ThresholdPaceSecPerKm)
	out.ThresholdSwimPaceSecPer100m = numfmt.Round1Ptr(eff.ThresholdSwimPaceSecPer100m)
	return &out
}

// sourcesRequest is the PUT /athlete-config/sources body: the full replacement
// list of garmin-sourced field tokens.
type sourcesRequest struct {
	Sources []string `json:"sources"`
}

// putSources godoc
// @Summary      Set the per-field source policy (which fields read from Garmin)
// @Description  Full-replaces `garmin_sourced_fields` (empty = all manual). Whitelisted tokens: `ftp_watts`, `lactate_threshold_hr`, `max_hr`, `threshold_pace_sec_per_km`, `hr_zones`, `power_zones` (zones flip as whole sets). Mutates ONLY the policy — never the physiology values, never threshold history. Rejected from the garmin identity (403). Flipping a source changes the thresholds computations use; run POST /workouts/recompute-tss when derived TSS should follow. `Idempotency-Key` is NOT accepted on PUT.
// @Tags         athlete-config
// @Accept       json
// @Produce      json
// @Param        body  body  sourcesRequest  true  "Source policy payload"
// @Success      200  {object}  map[string]interface{}  "{\"sources\": [string]}"
// @Failure      400  {object}  map[string]string  "source_field_invalid | invalid_json | idempotency_unsupported_for_put"
// @Failure      403  {object}  map[string]string  "forbidden — the garmin identity cannot set the policy"
// @Security     BearerAuth
// @Router       /athlete-config/sources [put]
func (h *Handlers) putSources(c *gin.Context) {
	if auth.ClientFromContext(c) == auth.ClientGarmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	var req sourcesRequest
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
			return
		}
	}
	if req.Sources == nil {
		req.Sources = []string{}
	}
	stored, err := h.svc.PutSources(c.Request.Context(), req.Sources)
	if err != nil {
		var serr *SourceFieldError
		if errors.As(err, &serr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "source_field_invalid", "field": serr.Field})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "athlete_config_sources_failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sources": stored})
}
