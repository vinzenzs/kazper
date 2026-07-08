package garmincontrol

import (
	"bytes"
	"io"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// maxExportBodyBytes is the response cap for the activity-export path ONLY. A
// FIT/GPX blob base64-encoded inside a JSON envelope can be a few hundred KB,
// far above the 16 KB maxBodyBytes used for the small JSON control responses.
const maxExportBodyBytes = 8 * 1024 * 1024

// backfill godoc
// @Summary      Trigger a Garmin history backfill over a date range (async)
// @Description  Forwards `{from, to}` to the bridge's `POST /sync/backfill`, which validates + caps the range, opens a sync run, and runs the paced day-by-day replay in the BACKGROUND — returning `202 {run_id, from, to, days_total}` immediately (verbatim). Poll `GET /garmin/sync-status?run_id=…` for the outcome. Re-runs are safe (date-keyed upserts + external_id dedup).
// @Tags         garmin
// @Accept       json
// @Produce      json
// @Param        body  body  map[string]interface{}  true  "{ from, to } (YYYY-MM-DD, inclusive)"
// @Success      202  {object}  map[string]interface{}  "{ run_id, from, to, days_total } — poll sync-status for the outcome"
// @Failure      400  {object}  map[string]interface{}  "range_too_large | date_invalid"
// @Failure      502  {object}  map[string]string  "garmin_bridge_unreachable"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/backfill [post]
func (h *Handlers) backfill(c *gin.Context) {
	if !h.enabled() {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "garmin_disabled"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBodyBytes))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request_body_unreadable"})
		return
	}
	// The bridge now returns 202 immediately (it runs the replay in the
	// background), so this is a short verbatim forward like the other proxies —
	// no bespoke long-timeout client.
	h.proxy(c, http.MethodPost, "/sync/backfill", body, maxBodyBytes)
}

// deleteWorkoutObject godoc
// @Summary      Delete a workout's structured Garmin object (reconciliation reap)
// @Description  Looks up the workout, deletes its stored Garmin workout OBJECT from the library via the bridge, and clears `garmin_workout_id` (leaving `garmin_schedule_id` — deleting the object does not remove a still-present calendar entry; use unschedule for the full teardown). Idempotent: no-op success when no object id is stored or the object is already gone.
// @Tags         garmin
// @Produce      json
// @Param        workout_id  path  string  true  "Workout UUID"
// @Success      200  {object}  map[string]interface{}  "{ deleted, workout }"
// @Failure      404  {object}  map[string]string  "workout_not_found"
// @Failure      502  {object}  map[string]string  "garmin_bridge_unreachable | garmin_error"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/workout/{workout_id} [delete]
func (h *Handlers) deleteWorkoutObject(c *gin.Context) {
	if !h.enabled() {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "garmin_disabled"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workout_not_found"})
		return
	}
	ctx, cancel := h.bridgeCtx(c, scheduleTimeout)
	defer cancel()
	w, err := h.workoutsRepo.GetByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workout_not_found"})
		return
	}
	if w.GarminWorkoutID == nil {
		c.JSON(http.StatusOK, gin.H{"deleted": false, "workout": w})
		return
	}
	if err := h.bridgeDeleteWorkout(ctx, *w.GarminWorkoutID); err != nil {
		h.respondScheduleErr(c, err)
		return
	}
	// Clear only the workout id; the schedule id (if any) stays — the calendar
	// entry is removed by unschedule, not by deleting the object.
	if err := h.workoutsRepo.SetGarminIDs(ctx, id, nil, w.GarminScheduleID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update_failed"})
		return
	}
	updated, _ := h.workoutsRepo.GetByID(ctx, id)
	c.JSON(http.StatusOK, gin.H{"deleted": true, "workout": updated})
}

// listGarminWorkouts godoc
// @Summary      List the Garmin workout library through the bridge
// @Tags         garmin
// @Produce      json
// @Param        start  query  int  false  "Pagination start offset (passthrough)"
// @Param        limit  query  int  false  "Pagination limit (passthrough)"
// @Success      200  {object}  map[string]interface{}  "the bridge workout-library response verbatim"
// @Failure      502  {object}  map[string]string  "garmin_bridge_unreachable"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/workouts [get]
func (h *Handlers) listGarminWorkouts(c *gin.Context) {
	if !h.enabled() {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "garmin_disabled"})
		return
	}
	q := url.Values{}
	if v := c.Query("start"); v != "" {
		q.Set("start", v)
	}
	if v := c.Query("limit"); v != "" {
		q.Set("limit", v)
	}
	path := "/workouts"
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	h.proxy(c, http.MethodGet, path, nil, maxBodyBytes)
}

// getGarminWorkout godoc
// @Summary      Read one Garmin library workout through the bridge
// @Tags         garmin
// @Produce      json
// @Param        garmin_workout_id  path  string  true  "Garmin workout object id"
// @Success      200  {object}  map[string]interface{}  "the bridge workout response verbatim"
// @Failure      502  {object}  map[string]string  "garmin_bridge_unreachable"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/workout/{garmin_workout_id} [get]
func (h *Handlers) getGarminWorkout(c *gin.Context) {
	if !h.enabled() {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "garmin_disabled"})
		return
	}
	h.proxy(c, http.MethodGet, "/workouts/"+url.PathEscape(c.Param("id")), nil, maxBodyBytes)
}

// pushHydration godoc
// @Summary      Push logged hydration back to Garmin (opt-in write)
// @Description  Forwards `{value_ml, date}` to the bridge, which sets/replaces that day's hydration on Garmin. The only write FROM us TO Garmin — invoked deliberately, nothing pushes automatically.
// @Tags         garmin
// @Accept       json
// @Produce      json
// @Param        body  body  map[string]interface{}  true  "{ value_ml, date }"
// @Success      200  {object}  map[string]interface{}  "the bridge response verbatim"
// @Failure      502  {object}  map[string]string  "garmin_bridge_unreachable | garmin_error"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/hydration [post]
func (h *Handlers) pushHydration(c *gin.Context) {
	if !h.enabled() {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "garmin_disabled"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBodyBytes))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request_body_unreadable"})
		return
	}
	h.proxy(c, http.MethodPost, "/hydration", body, maxBodyBytes)
}

// exportActivity godoc
// @Summary      Export an activity's FIT/GPX/TCX file (base64 envelope)
// @Description  Forwards to the bridge, returning `{activity_id, format, filename, content_base64}` verbatim — the file bytes base64-encoded in JSON. `format` defaults to `fit`. Upload is out of scope.
// @Tags         garmin
// @Produce      json
// @Param        activity_id  path   string  true   "Garmin activity id"
// @Param        format       query  string  false  "fit (default) | gpx | tcx | kml | csv"
// @Success      200  {object}  map[string]interface{}  "{ activity_id, format, filename, content_base64 }"
// @Failure      502  {object}  map[string]string  "garmin_bridge_unreachable | garmin_error"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/activity/{activity_id}/export [get]
func (h *Handlers) exportActivity(c *gin.Context) {
	if !h.enabled() {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "garmin_disabled"})
		return
	}
	path := "/activity/" + url.PathEscape(c.Param("activity_id")) + "/export"
	if v := c.Query("format"); v != "" {
		path += "?format=" + url.QueryEscape(v)
	}
	// Blob-sized response → the raised export cap, not the 16 KB control cap.
	h.proxy(c, http.MethodGet, path, nil, maxExportBodyBytes)
}

// proxy forwards a request to the bridge and copies its status + body back
// verbatim, reading at most `limit` bytes. Transport failures surface as
// 502 garmin_bridge_unreachable. Used by the verbatim-passthrough endpoints.
func (h *Handlers) proxy(c *gin.Context, method, bridgePath string, body []byte, limit int64) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	ctx, cancel := h.bridgeCtx(c, interactiveTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, h.bridgeURL+bridgePath, reader)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "garmin_bridge_unreachable"})
		return
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := h.client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "garmin_bridge_unreachable"})
		return
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "garmin_bridge_unreachable"})
		return
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/json"
	}
	c.Data(resp.StatusCode, ct, out)
}
