package healthvitals

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const maxWindowDays = 92

// Handlers wires the health-vitals endpoints.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/health-vitals", h.upsert)
	rg.GET("/health-vitals", h.list)
	rg.GET("/health-vitals/:date", h.get)
}

// upsert godoc
// @Summary      Upsert a daily health-vitals snapshot (by date)
// @Description  Creates or full-replaces the health-vitals snapshot (blood pressure + all-day HR/stress) for a calendar date. "POST every day you see" — re-pushing the same date updates in place, omitted fields reset to NULL. Distinct from recovery-metrics; reference context only, never feeds nutrition computation.
// @Tags         health-vitals
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string    false  "Optional client-supplied idempotency key"
// @Param        body             body    Snapshot  true   "Health vitals (date required; metrics optional)"
// @Success      201  {object}  Snapshot  "INSERT"
// @Success      200  {object}  Snapshot  "UPDATE (date already present)"
// @Failure      400  {object}  map[string]string  "date_invalid | bp_systolic_invalid | bp_diastolic_invalid | bp_pulse_invalid | resting_hr_invalid | min_hr_invalid | max_hr_invalid | stress_avg_invalid | stress_max_invalid"
// @Security     BearerAuth
// @Router       /health-vitals [post]
func (h *Handlers) upsert(c *gin.Context) {
	var in Snapshot
	if err := c.ShouldBindJSON(&in); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_json")
		return
	}
	out, created, err := h.svc.Upsert(c.Request.Context(), &in)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, out)
}

// list godoc
// @Summary      List health-vitals snapshots in a date window
// @Tags         health-vitals
// @Produce      json
// @Param        from  query  string  true  "Inclusive lower bound YYYY-MM-DD"
// @Param        to    query  string  true  "Inclusive upper bound YYYY-MM-DD; max 92-day span"
// @Success      200  {object}  map[string]interface{}  "{ health_vitals: [...] }"
// @Failure      400  {object}  map[string]interface{}  "window_required | window_invalid | range_too_large"
// @Security     BearerAuth
// @Router       /health-vitals [get]
func (h *Handlers) list(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		respondError(c, http.StatusBadRequest, "window_required")
		return
	}
	from, err := time.Parse(dateLayout, fromStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "window_invalid")
		return
	}
	to, err := time.Parse(dateLayout, toStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "window_invalid")
		return
	}
	if to.Before(from) {
		respondError(c, http.StatusBadRequest, "window_invalid")
		return
	}
	if to.Sub(from) > time.Duration(maxWindowDays)*24*time.Hour {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_too_large", "max_days": maxWindowDays})
		return
	}
	rows, err := h.svc.ListWindow(c.Request.Context(), fromStr, toStr)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_failed")
		return
	}
	out := make([]*Snapshot, 0, len(rows))
	out = append(out, rows...)
	c.JSON(http.StatusOK, gin.H{"health_vitals": out})
}

// get godoc
// @Summary      Get the health-vitals snapshot for a date
// @Tags         health-vitals
// @Produce      json
// @Param        date  path  string  true  "Date YYYY-MM-DD"
// @Success      200  {object}  Snapshot
// @Failure      404  {object}  map[string]string  "health_vitals_not_found"
// @Security     BearerAuth
// @Router       /health-vitals/{date} [get]
func (h *Handlers) get(c *gin.Context) {
	out, err := h.svc.Get(c.Request.Context(), c.Param("date"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondError(c, http.StatusNotFound, "health_vitals_not_found")
			return
		}
		if errors.Is(err, ErrDateInvalid) {
			respondError(c, http.StatusBadRequest, "date_invalid")
			return
		}
		respondError(c, http.StatusInternalServerError, "get_failed")
		return
	}
	c.JSON(http.StatusOK, out)
}

// ----- helpers -----

func respondError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

func respondServiceError(c *gin.Context, err error) {
	for _, e := range []error{
		ErrDateInvalid, ErrBPSystolicInvalid, ErrBPDiastolicInvalid, ErrBPPulseInvalid,
		ErrRestingHRInvalid, ErrMinHRInvalid, ErrMaxHRInvalid, ErrStressAvgInvalid, ErrStressMaxInvalid,
	} {
		if errors.Is(err, e) {
			respondError(c, http.StatusBadRequest, e.Error())
			return
		}
	}
	respondError(c, http.StatusInternalServerError, "write_failed")
}
