package garminsyncstatus

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/auth"
)

// Handlers wires the sync-run record endpoints (garmin identity only) and the
// sync-status read (any authenticated identity). `enabled` reflects whether the
// Garmin integration is configured; when false every route returns 503.
type Handlers struct {
	svc     *Service
	enabled bool
}

func NewHandlers(svc *Service, enabled bool) *Handlers {
	return &Handlers{svc: svc, enabled: enabled}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	rg.POST("/garmin/sync-runs", h.open)
	rg.PATCH("/garmin/sync-runs/:id", h.close)
	rg.GET("/garmin/sync-status", h.status)
}

// requireEnabled returns false (after aborting 503) when the integration is off.
func (h *Handlers) requireEnabled(c *gin.Context) bool {
	if !h.enabled {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "garmin_disabled"})
		return false
	}
	return true
}

// requireGarmin enforces the garmin-only write guard (same rule as garminauth).
func (h *Handlers) requireGarmin(c *gin.Context) bool {
	if auth.ClientFromContext(c) != auth.ClientGarmin {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return false
	}
	return true
}

type openRequest struct {
	WindowFrom *string `json:"window_from,omitempty"`
	WindowTo   *string `json:"window_to,omitempty"`
}

// open godoc
// @Summary      Open a Garmin sync run (garmin identity only)
// @Description  Records a new sync run with status `running` and the rolling window the bridge is about to sync, returning the row with its generated id. Restricted to the `garmin` identity; 503 when the Garmin integration is unconfigured.
// @Tags         garmin
// @Accept       json
// @Produce      json
// @Param        body  body  openRequest  false  "Rolling window"
// @Success      201  {object}  SyncRun
// @Failure      403  {object}  map[string]string  "forbidden"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/sync-runs [post]
func (h *Handlers) open(c *gin.Context) {
	if !h.requireEnabled(c) || !h.requireGarmin(c) {
		return
	}
	var req openRequest
	// Body is optional; a malformed body is the only hard error.
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
			return
		}
	}
	run, err := h.svc.Open(c.Request.Context(), req.WindowFrom, req.WindowTo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "open_failed"})
		return
	}
	c.JSON(http.StatusCreated, run)
}

type closeRequest struct {
	Status  string          `json:"status"`
	Error   *string         `json:"error,omitempty"`
	Summary json.RawMessage `json:"summary,omitempty" swaggertype:"object"`
}

// close godoc
// @Summary      Close a Garmin sync run (garmin identity only)
// @Description  Terminates a run by setting `status` to `success` or `error`, stamping `finished_at`, and recording an optional `error` message. Restricted to the `garmin` identity; 404 for an unknown id; 400 when status is not success|error; 503 when unconfigured.
// @Tags         garmin
// @Accept       json
// @Produce      json
// @Param        id    path  string        true  "Sync run UUID"
// @Param        body  body  closeRequest  true  "Terminal status"
// @Success      200  {object}  SyncRun
// @Failure      400  {object}  map[string]string  "invalid_json | status_invalid"
// @Failure      403  {object}  map[string]string  "forbidden"
// @Failure      404  {object}  map[string]string  "sync_run_not_found"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/sync-runs/{id} [patch]
func (h *Handlers) close(c *gin.Context) {
	if !h.requireEnabled(c) || !h.requireGarmin(c) {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "sync_run_not_found"})
		return
	}
	var req closeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	run, err := h.svc.Close(c.Request.Context(), id, req.Status, req.Error, req.Summary)
	if err != nil {
		switch {
		case errors.Is(err, ErrStatusInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"error": "status_invalid"})
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "sync_run_not_found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "close_failed"})
		}
		return
	}
	c.JSON(http.StatusOK, run)
}

// status godoc
// @Summary      Read Garmin sync status
// @Description  Returns a sync run as `latest` (the most recent by default, or the run named by the optional `run_id` query — used to poll a specific async backfill), the timestamp of the newest successful run (independent of `latest`, so a failed latest still shows when data was last good), and a derived `is_stale` flag. Only `success` runs count toward freshness (a `partial`/`error` run does not). Available to any authenticated identity. 404 for an unknown `run_id`; 503 when the Garmin integration is unconfigured.
// @Tags         garmin
// @Produce      json
// @Param        run_id  query  string  false  "Return this specific run as `latest` (e.g. a 202 backfill's run_id)"
// @Success      200  {object}  SyncStatus
// @Failure      404  {object}  map[string]string  "sync_run_not_found"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/sync-status [get]
func (h *Handlers) status(c *gin.Context) {
	if !h.requireEnabled(c) {
		return
	}
	var runID *uuid.UUID
	if v := c.Query("run_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "sync_run_not_found"})
			return
		}
		runID = &id
	}
	out, err := h.svc.Status(c.Request.Context(), runID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "sync_run_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "status_failed"})
		return
	}
	c.JSON(http.StatusOK, out)
}
