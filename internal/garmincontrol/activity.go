package garmincontrol

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)

// activityGear godoc
// @Summary      Read the gear linked to a Garmin activity
// @Tags         garmin
// @Produce      json
// @Param        activity_id  path  string  true  "Garmin activity id"
// @Success      200  {object}  map[string]interface{}  "the bridge activity-gear response verbatim"
// @Failure      502  {object}  map[string]string  "garmin_bridge_unreachable"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/activity/{activity_id}/gear [get]
func (h *Handlers) activityGear(c *gin.Context) {
	if !h.enabled() {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "garmin_disabled"})
		return
	}
	h.proxy(c, http.MethodGet, "/activity/"+url.PathEscape(c.Param("activity_id"))+"/gear", nil, maxBodyBytes)
}

// downloadWorkout godoc
// @Summary      Download a structured workout's FIT blob (base64 envelope)
// @Description  The structured-workout analogue of the activity export: forwards to the bridge and returns `{garmin_workout_id, format, filename, content_base64}` verbatim. `format` defaults to `fit`.
// @Tags         garmin
// @Produce      json
// @Param        garmin_workout_id  path   string  true   "Garmin workout object id"
// @Param        format             query  string  false  "fit (default) | …"
// @Success      200  {object}  map[string]interface{}  "{ garmin_workout_id, format, filename, content_base64 }"
// @Failure      502  {object}  map[string]string  "garmin_bridge_unreachable | garmin_error"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/workout/{garmin_workout_id}/download [get]
func (h *Handlers) downloadWorkout(c *gin.Context) {
	if !h.enabled() {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "garmin_disabled"})
		return
	}
	path := "/workout/" + url.PathEscape(c.Param("id")) + "/download"
	if v := c.Query("format"); v != "" {
		path += "?format=" + url.QueryEscape(v)
	}
	h.proxy(c, http.MethodGet, path, nil, maxExportBodyBytes)
}

// uploadActivity godoc
// @Summary      Upload a FIT activity to Garmin (opt-in write)
// @Description  Forwards a base64-wrapped FIT payload `{filename, content_base64}` to the bridge, which uploads it via Garmin. Writes FROM this API TO Garmin — explicit-request only.
// @Tags         garmin
// @Accept       json
// @Produce      json
// @Param        body  body  map[string]interface{}  true  "{ filename, content_base64 }"
// @Success      200  {object}  map[string]interface{}  "the bridge upload response verbatim"
// @Failure      502  {object}  map[string]string  "garmin_bridge_unreachable | garmin_error"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/activity/upload [post]
func (h *Handlers) uploadActivity(c *gin.Context) {
	if !h.enabled() {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "garmin_disabled"})
		return
	}
	// FIT payload base64 can exceed the 16 KB control cap — read at the blob cap.
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxExportBodyBytes))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request_body_unreadable"})
		return
	}
	h.proxy(c, http.MethodPost, "/activity/upload", body, maxExportBodyBytes)
}

// renameActivity godoc
// @Summary      Rename a Garmin activity
// @Tags         garmin
// @Accept       json
// @Produce      json
// @Param        activity_id  path  string                  true  "Garmin activity id"
// @Param        body         body  map[string]interface{}  true  "{ name }"
// @Success      200  {object}  map[string]interface{}  "the bridge response verbatim"
// @Failure      400  {object}  map[string]string  "name_required"
// @Failure      502  {object}  map[string]string  "garmin_bridge_unreachable | garmin_error"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/activity/{activity_id} [patch]
func (h *Handlers) renameActivity(c *gin.Context) {
	if !h.enabled() {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "garmin_disabled"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBodyBytes))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request_body_unreadable"})
		return
	}
	var probe struct {
		Name *string `json:"name"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	if probe.Name == nil || *probe.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name_required"})
		return
	}
	h.proxy(c, http.MethodPatch, "/activity/"+url.PathEscape(c.Param("activity_id")), body, maxBodyBytes)
}

// deleteActivity godoc
// @Summary      Delete a Garmin activity (idempotent)
// @Description  Forwards to the bridge, which deletes the Garmin activity. An already-absent activity is a no-op success.
// @Tags         garmin
// @Produce      json
// @Param        activity_id  path  string  true  "Garmin activity id"
// @Success      200  {object}  map[string]interface{}  "the bridge response verbatim"
// @Failure      502  {object}  map[string]string  "garmin_bridge_unreachable | garmin_error"
// @Failure      503  {object}  map[string]string  "garmin_disabled"
// @Security     BearerAuth
// @Router       /garmin/activity/{activity_id} [delete]
func (h *Handlers) deleteActivity(c *gin.Context) {
	if !h.enabled() {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "garmin_disabled"})
		return
	}
	h.proxy(c, http.MethodDelete, "/activity/"+url.PathEscape(c.Param("activity_id")), nil, maxBodyBytes)
}
