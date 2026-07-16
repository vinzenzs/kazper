package workoutfueling

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/workouts"
)

// Handlers wires the /workouts/:id/fueling endpoint onto a Gin router group.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.GET("/workouts/:id/fueling", h.fueling)
	rg.GET("/workouts/:id/sweat-rate", h.sweatRate)
	rg.GET("/workouts/:id/fueling-plan", h.fuelingPlan)
}

// fuelingPlan godoc
// @Summary      Planned workout fueling plan: burn estimate + intake prescription
// @Description  For a PLANNED workout, estimates the session's work and carbohydrate burn and prescribes intake. Work: `kJ = planned_tss / 100 × effective FTP × 3.6` (exact under the TSS definition), with energy per the standard cycling kJ≈kcal convention. Carb burn: kcal × a CHO fraction selected by planned IF (`< 0.60` → 45%, `0.60–0.75` → 55%, `0.75–0.85` → 70%, `> 0.85` → 80%; IF derived as `sqrt(planned_tss/100 ÷ hours)`) ÷ 4 kcal/g. Intake: the duration ladder (`< 60 min` → 0, `60–150 min` → 30–60 g/hr, `> 150 min` → 60–90 g/hr), its ceiling clamped by the optional `carbs_per_hr` capacity (the athlete's tested gut tolerance — discovered in rehearsal, not by this endpoint). `projected_deficit_g` is burn minus the maximum prescribed intake: the number that says whether post-ride carbs need emphasis. Every input behind the numbers is echoed. Degradations state what they lack: no planned TSS and no duration → `plan_data_missing`; duration without TSS, or missing effective FTP → the intake ladder without a burn estimate (`tss_missing` / `ftp_missing`). A non-planned workout returns 409 — the pre-session question is the product. Compute-on-read: persists nothing, feeds no daily nutrition total. These are conventions and a planning anchor, not a metabolic lab.
// @Tags         workouts
// @Produce      json
// @Param        id            path   string  true   "Workout UUID"
// @Param        carbs_per_hr  query  number  false  "Tested gut capacity in g/hr (> 0, <= 130); clamps the prescription's upper bound"
// @Success      200  {object}  FuelingPlan
// @Failure      400  {object}  map[string]string  "carbs_per_hr_invalid"
// @Failure      404  {object}  map[string]string  "not_found"
// @Failure      409  {object}  map[string]string  "workout_not_planned"
// @Security     BearerAuth
// @Router       /workouts/{id}/fueling-plan [get]
func (h *Handlers) fuelingPlan(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}

	var capacity *float64
	if s := c.Query("carbs_per_hr"); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "carbs_per_hr_invalid"})
			return
		}
		capacity = &v
	}

	out, err := h.svc.FuelingPlanFor(c.Request.Context(), id, capacity)
	if err != nil {
		switch {
		case errors.Is(err, workouts.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		case errors.Is(err, ErrWorkoutNotPlanned):
			c.JSON(http.StatusConflict, gin.H{"error": "workout_not_planned"})
		case errors.Is(err, ErrCarbsPerHrInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"error": "carbs_per_hr_invalid"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "fueling_plan_failed"})
		}
		return
	}
	c.JSON(http.StatusOK, out)
}

// fueling godoc
// @Summary      Workout fueling: pre / intra / post windows
// @Description  Returns three time-anchored buckets (pre / intra / post), each carrying three separate sub-objects of intake around a workout: `nutrition` (from meals), `hydration` (from hydration entries), and `workout_fuel` (from workout-fuel entries — gels / electrolyte drinks / caffeine). Aggregation is by `logged_at` time-window matching, NOT by the `workout_id` tag on intake rows — an untagged meal in the pre-window still contributes. Sub-objects are unit-isolated: no ml inside `nutrition.totals`, no kcal inside `hydration` or `workout_fuel`.
// @Tags         workouts
// @Produce      json
// @Param        id                path   string   true   "Workout UUID"
// @Param        pre_window_min    query  integer  false  "Pre-window length in minutes (default 240, range 0..720)"
// @Param        post_window_min   query  integer  false  "Post-window length in minutes (default 60, range 0..720)"
// @Success      200  {object}  WorkoutFueling
// @Failure      400  {object}  map[string]interface{}  "window_invalid"
// @Failure      404  {object}  map[string]string  "workout_not_found"
// @Security     BearerAuth
// @Router       /workouts/{id}/fueling [get]
func (h *Handlers) fueling(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workout_not_found"})
		return
	}

	preMin := DefaultPreWindowMin
	if s := c.Query("pre_window_min"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < MinWindowMin || v > MaxWindowMin {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "window_invalid",
				"range": gin.H{"min": MinWindowMin, "max": MaxWindowMin},
			})
			return
		}
		preMin = v
	}
	postMin := DefaultPostWindowMin
	if s := c.Query("post_window_min"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < MinWindowMin || v > MaxWindowMin {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "window_invalid",
				"range": gin.H{"min": MinWindowMin, "max": MaxWindowMin},
			})
			return
		}
		postMin = v
	}

	out, err := h.svc.FueledFor(c.Request.Context(), id, preMin, postMin)
	if err != nil {
		if errors.Is(err, workouts.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "workout_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fueling_failed"})
		return
	}
	c.JSON(http.StatusOK, out)
}

// sweatRate godoc
// @Summary      Workout sweat rate: the standard field test
// @Description  Computes sweat rate (ml/hr) over a completed workout: `sweat_loss_ml = (pre_weight_kg − post_weight_kg) × 1000 + fluid_ml`, divided by the workout's elapsed hours. Pre/post weights are REQUIRED positive params — the bodyweight log is daily-grained, not pre/post-session. Fluid is summed from the workout's linked hydration and workout-fuel `quantity_ml` entries and itemized in the response; an optional `fluid_ml_override` (≥ 0) replaces the derived sum for the unlogged-bottle case. A planned workout returns 409; a negative loss or a rate above 5000 ml/hr still returns the numbers with `warning: "implausible_result"`. Compute-on-read: persists nothing, feeds no daily hydration or nutrition total.
// @Tags         workouts
// @Produce      json
// @Param        id                path   string   true   "Workout UUID"
// @Param        pre_weight_kg     query  number   true   "Pre-session body weight (kg, positive)"
// @Param        post_weight_kg    query  number   true   "Post-session body weight (kg, positive)"
// @Param        fluid_ml_override query  number   false  "Override for in-session fluid (ml, ≥ 0); replaces the derived hydration + workout-fuel sum"
// @Success      200  {object}  SweatRate
// @Failure      400  {object}  map[string]string  "pre_weight_invalid / post_weight_invalid / fluid_override_invalid"
// @Failure      404  {object}  map[string]string  "not_found"
// @Failure      409  {object}  map[string]string  "workout_not_completed"
// @Security     BearerAuth
// @Router       /workouts/{id}/sweat-rate [get]
func (h *Handlers) sweatRate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}

	pre, err := parsePositiveFloat(c.Query("pre_weight_kg"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pre_weight_invalid"})
		return
	}
	post, err := parsePositiveFloat(c.Query("post_weight_kg"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "post_weight_invalid"})
		return
	}

	in := SweatRateInput{PreWeightKg: pre, PostWeightKg: post}
	if s := c.Query("fluid_ml_override"); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil || v < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "fluid_override_invalid"})
			return
		}
		in.FluidMlOverride = &v
	}

	out, err := h.svc.SweatRateFor(c.Request.Context(), id, in)
	if err != nil {
		switch {
		case errors.Is(err, workouts.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		case errors.Is(err, ErrWorkoutNotCompleted):
			c.JSON(http.StatusConflict, gin.H{"error": "workout_not_completed"})
		case errors.Is(err, ErrPreWeightInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"error": "pre_weight_invalid"})
		case errors.Is(err, ErrPostWeightInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"error": "post_weight_invalid"})
		case errors.Is(err, ErrFluidOverrideInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"error": "fluid_override_invalid"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "sweat_rate_failed"})
		}
		return
	}
	c.JSON(http.StatusOK, out)
}

// parsePositiveFloat parses a required positive float query param. An empty,
// unparseable, or non-positive value is an error (mapped to the param's 400).
func parsePositiveFloat(s string) (float64, error) {
	if s == "" {
		return 0, errors.New("required")
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 {
		return 0, errors.New("invalid")
	}
	return v, nil
}
