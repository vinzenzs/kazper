package wellness

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers wires the wellness CRUD endpoints.
type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.PUT("/wellness/:date", h.put)
	rg.GET("/wellness/:date", h.get)
	rg.DELETE("/wellness/:date", h.delete)
	rg.GET("/wellness", h.list)
}

// putBody is the decoded PUT payload. Pointers distinguish absent from present;
// a non-integer score fails decode and is reported as wellness_score_invalid.
type putBody struct {
	Fatigue    *int    `json:"fatigue"`
	Soreness   *int    `json:"soreness"`
	Stress     *int    `json:"stress"`
	Mood       *int    `json:"mood"`
	Motivation *int    `json:"motivation"`
	Note       *string `json:"note"`
}

// put godoc
// @Summary      Upsert the wellness entry for a date
// @Description  Full-replace semantics: fields absent from the body are cleared (stored NULL). Five optional 1–5 scores (fatigue/soreness/stress: 1=none→5=severe; mood/motivation: 1=low→5=high) plus an optional note (≤2000 chars); at least one field must be present. `Idempotency-Key` is NOT accepted on PUT — supplying it returns `400 idempotency_unsupported_for_put`.
// @Tags         wellness
// @Accept       json
// @Produce      json
// @Param        date  path  string   true  "Entry date in YYYY-MM-DD"
// @Param        body  body  putBody  true  "Wellness scores + note"
// @Success      200   {object}  map[string]interface{}  "{\"wellness\": Entry}"
// @Failure      400   {object}  map[string]string  "date_invalid | wellness_empty | wellness_score_invalid | note_too_long | invalid_json | idempotency_unsupported_for_put"
// @Security     BearerAuth
// @Router       /wellness/{date} [put]
func (h *Handlers) put(c *gin.Context) {
	date, err := time.Parse(dateFormat, c.Param("date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
		return
	}
	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	var b putBody
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		if field, ok := unknownField(err); ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "wellness_score_invalid", "field": field})
			return
		}
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) && typeErr.Field != "" {
			// A non-integer score (e.g. 7.5 or "x") — name the offending field.
			c.JSON(http.StatusBadRequest, gin.H{"error": "wellness_score_invalid", "field": typeErr.Field})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}

	stored, err := h.svc.Put(c.Request.Context(), date, PutInput{
		Fatigue:    b.Fatigue,
		Soreness:   b.Soreness,
		Stress:     b.Stress,
		Mood:       b.Mood,
		Motivation: b.Motivation,
		Note:       b.Note,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"wellness": stored})
}

// get godoc
// @Summary      Get the wellness entry for a date
// @Tags         wellness
// @Produce      json
// @Param        date  path  string  true  "Entry date in YYYY-MM-DD"
// @Success      200  {object}  map[string]interface{}  "{\"wellness\": Entry}"
// @Failure      400  {object}  map[string]string  "date_invalid"
// @Failure      404  {object}  map[string]string  "not_found"
// @Security     BearerAuth
// @Router       /wellness/{date} [get]
func (h *Handlers) get(c *gin.Context) {
	date, err := time.Parse(dateFormat, c.Param("date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
		return
	}
	e, err := h.svc.Get(c.Request.Context(), date)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"wellness": e})
}

// delete godoc
// @Summary      Delete the wellness entry for a date
// @Tags         wellness
// @Param        date  path  string  true  "Entry date in YYYY-MM-DD"
// @Success      204  "no content"
// @Failure      400  {object}  map[string]string  "date_invalid"
// @Failure      404  {object}  map[string]string  "not_found"
// @Security     BearerAuth
// @Router       /wellness/{date} [delete]
func (h *Handlers) delete(c *gin.Context) {
	date, err := time.Parse(dateFormat, c.Param("date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), date); err != nil {
		writeErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// list godoc
// @Summary      List wellness entries in a date range
// @Description  Returns entries whose `date` falls within `[from, to]` (inclusive) in ascending date order. `200` with an empty array when none exist. The range is capped at 92 days.
// @Tags         wellness
// @Produce      json
// @Param        from  query  string  true   "Inclusive start date YYYY-MM-DD"
// @Param        to    query  string  true   "Inclusive end date YYYY-MM-DD; max 92 days from `from`"
// @Success      200  {object}  map[string]interface{}  "{\"entries\": [Entry]}"
// @Failure      400  {object}  map[string]interface{}  "range_required | date_invalid | range_invalid | range_too_large"
// @Security     BearerAuth
// @Router       /wellness [get]
func (h *Handlers) list(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_required"})
		return
	}
	from, err := time.Parse(dateFormat, fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid", "field": "from"})
		return
	}
	to, err := time.Parse(dateFormat, toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid", "field": "to"})
		return
	}
	entries, err := h.svc.List(c.Request.Context(), from, to)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"entries": entries})
}

// writeErr maps a service/repo sentinel to its API status + code.
func writeErr(c *gin.Context, err error) {
	var scoreErr *ScoreError
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
	case errors.As(err, &scoreErr):
		c.JSON(http.StatusBadRequest, gin.H{"error": "wellness_score_invalid", "field": scoreErr.Field})
	case errors.Is(err, ErrEmpty):
		c.JSON(http.StatusBadRequest, gin.H{"error": "wellness_empty"})
	case errors.Is(err, ErrNoteTooLong):
		c.JSON(http.StatusBadRequest, gin.H{"error": "note_too_long"})
	case errors.Is(err, ErrRangeInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_invalid"})
	case errors.Is(err, ErrRangeTooLarge):
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_too_large", "max_days": MaxRangeDays})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
	}
}

// unknownField parses encoding/json's "unknown field" error message.
func unknownField(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	msg := err.Error()
	const prefix = `json: unknown field "`
	if i := strings.Index(msg, prefix); i >= 0 {
		rest := msg[i+len(prefix):]
		if j := strings.Index(rest, `"`); j >= 0 {
			return rest[:j], true
		}
	}
	return "", false
}
