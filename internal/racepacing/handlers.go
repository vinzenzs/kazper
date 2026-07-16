package racepacing

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/numfmt"
	"github.com/vinzenzs/kazper/internal/races"
)

// Handlers wires the pacing-plan endpoints onto a Gin router group.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.GET("/races/:id/pacing-plan", h.plan)
	rg.PUT("/races/:id/pacing-plan/overrides/:ordinal", h.setOverride)
	rg.DELETE("/races/:id/pacing-plan/overrides/:ordinal", h.deleteOverride)
}

// plan godoc
// @Summary      Per-leg race pacing plan
// @Description  Computes, per race leg, a duration-banded intensity target from the athlete-config thresholds: bike legs a power band as a % of `ftp_watts`, run legs a pace band vs `threshold_pace_sec_per_km`, swim legs a pace band per 100 m vs `threshold_swim_pace_sec_per_100m` (CSS). Each leg carries a `source` (computed/override/none), per-leg `intensity_factor` and `estimated_tss`, and a `rationale`; the race carries `estimated_tss_total`, `tss_complete`, and a `missing_thresholds` union. Unset thresholds degrade the affected legs only (still 200). Compute-on-read; nothing computed is persisted. Power lives only in `_w` fields, run pace in `_sec_per_km`, swim pace in `_sec_per_100m` (unit isolation). Optional `weather=true` additionally geocodes the race `location`, fetches the race window's forecast, and annotates each computable leg with a `heat_adjusted` sibling BESIDE its original band (originals never change), plus a race-level `heat` block — so "plan A cool / plan B hot" can be read side by side. Without the flag the response is byte-identical to the base contract. Weather-mode degradations keep the base plan and say why: `heat_reason` of `location_ungeocodable` (the race's location text geocodes nowhere), `forecast_out_of_range` (beyond ~16 days — a race two weeks out has no reliable forecast, which is why the flag is opt-in), or `weather_unavailable`.
// @Tags         races
// @Produce      json
// @Param        id       path   string  true   "Race id (uuid)"
// @Param        weather  query  boolean false  "Annotate heat-adjusted bands from the race-day forecast (opt-in; originals retained)"
// @Success      200  {object}  PacingPlan
// @Failure      404  {object}  map[string]string  "race_not_found"
// @Security     BearerAuth
// @Router       /races/{id}/pacing-plan [get]
func (h *Handlers) plan(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "race_not_found"})
		return
	}
	plan, err := h.svc.PlanWithWeather(c.Request.Context(), id, c.Query("weather") == "true")
	if err != nil {
		writeErr(c, err)
		return
	}
	roundPlan(plan)
	c.JSON(http.StatusOK, plan)
}

// setOverride godoc
// @Summary      Set a race leg's manual pacing override
// @Description  Full-replaces the pacing override for one leg, keyed by the leg's ordinal so it survives the wholesale leg-replacement of `PATCH /races/{id}`. Exactly one unit family (both low and high) must be populated and must match the leg's discipline: `target_power_*_w` for a bike leg, `target_pace_*_sec_per_km` for a run leg, `target_pace_*_sec_per_100m` for a swim leg. An `Idempotency-Key` header is rejected (PUT is full-replace).
// @Tags         races
// @Accept       json
// @Produce      json
// @Param        id       path  string         true  "Race id (uuid)"
// @Param        ordinal  path  int            true  "Leg ordinal"
// @Param        override body  OverrideInput  true  "One unit-family band matching the leg's discipline"
// @Success      204  "No Content"
// @Failure      400  {object}  map[string]string  "override_discipline_mismatch | override_target_required | override_unit_conflict | override_band_invalid | idempotency_unsupported_for_put"
// @Failure      404  {object}  map[string]string  "race_not_found | leg_not_found"
// @Security     BearerAuth
// @Router       /races/{id}/pacing-plan/overrides/{ordinal} [put]
func (h *Handlers) setOverride(c *gin.Context) {
	id, ordinal, ok := parseRaceOrdinal(c)
	if !ok {
		return
	}
	var in OverrideInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "override_target_required"})
		return
	}
	if err := h.svc.SetOverride(c.Request.Context(), id, ordinal, in); err != nil {
		writeErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// deleteOverride godoc
// @Summary      Clear a race leg's manual pacing override
// @Description  Removes the leg's stored override; the leg reverts to the computed band on the next read.
// @Tags         races
// @Produce      json
// @Param        id       path  string  true  "Race id (uuid)"
// @Param        ordinal  path  int     true  "Leg ordinal"
// @Success      204  "No Content"
// @Failure      404  {object}  map[string]string  "race_not_found | override_not_found"
// @Security     BearerAuth
// @Router       /races/{id}/pacing-plan/overrides/{ordinal} [delete]
func (h *Handlers) deleteOverride(c *gin.Context) {
	id, ordinal, ok := parseRaceOrdinal(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteOverride(c.Request.Context(), id, ordinal); err != nil {
		writeErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func parseRaceOrdinal(c *gin.Context) (uuid.UUID, int, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "race_not_found"})
		return uuid.UUID{}, 0, false
	}
	ordinal, err := strconv.Atoi(c.Param("ordinal"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "leg_not_found"})
		return uuid.UUID{}, 0, false
	}
	return id, ordinal, true
}

func writeErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, races.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "race_not_found"})
	case errors.Is(err, ErrLegNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "leg_not_found"})
	case errors.Is(err, ErrOverrideNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "override_not_found"})
	case errors.Is(err, ErrOverrideDisciplineMismatch):
		c.JSON(http.StatusBadRequest, gin.H{"error": "override_discipline_mismatch"})
	case errors.Is(err, ErrOverrideTargetRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": "override_target_required"})
	case errors.Is(err, ErrOverrideUnitConflict):
		c.JSON(http.StatusBadRequest, gin.H{"error": "override_unit_conflict"})
	case errors.Is(err, ErrOverrideBandInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "override_band_invalid"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "pacing_plan_failed"})
	}
}

// roundPlan applies numfmt rounding at the response boundary: pace targets and
// TSS to 1dp, intensity_factor to 2dp; power targets are already integer watts.
func roundPlan(p *PacingPlan) {
	p.EstimatedTSSTotal = numfmt.Round1(p.EstimatedTSSTotal)
	for i := range p.Legs {
		l := &p.Legs[i]
		l.TargetPaceLowSecPerKM = numfmt.Round1Ptr(l.TargetPaceLowSecPerKM)
		l.TargetPaceHighSecPerKM = numfmt.Round1Ptr(l.TargetPaceHighSecPerKM)
		l.TargetPaceLowSecPer100m = numfmt.Round1Ptr(l.TargetPaceLowSecPer100m)
		l.TargetPaceHighSecPer100m = numfmt.Round1Ptr(l.TargetPaceHighSecPer100m)
		l.IntensityFactor = numfmt.Round2Ptr(l.IntensityFactor)
		l.EstimatedTSS = numfmt.Round1Ptr(l.EstimatedTSS)
	}
}
