package athleteconfig

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

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
}

// get godoc
// @Summary      Get the athlete physiology configuration (singleton)
// @Description  Returns the single athlete-config row (FTP, thresholds, max HR, lactate-threshold HR, HR-zone and optional power-zone boundaries), or null before any config has been set. Capture-only mirror; these values feed no fueling computation.
// @Tags         athlete-config
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "{\"athlete_config\": AthleteConfig | null}"
// @Security     BearerAuth
// @Router       /athlete-config [get]
func (h *Handlers) get(c *gin.Context) {
	cfg, err := h.svc.Get(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "athlete_config_get_failed"})
		return
	}
	if cfg == nil {
		c.JSON(http.StatusOK, gin.H{"athlete_config": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"athlete_config": round(cfg)})
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
// @Security     BearerAuth
// @Router       /athlete-config [put]
func (h *Handlers) put(c *gin.Context) {
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
