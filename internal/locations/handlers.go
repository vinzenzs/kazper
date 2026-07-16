package locations

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// maxRangeDays caps the window read at the workout/analytics tier — travel
// history is sparse, so a season-wide read is reasonable.
const maxRangeDays = 400

// Handlers wires the location-period CRUD + resolution endpoints.
type Handlers struct {
	svc       *Service
	defaultTZ string
}

func NewHandlers(svc *Service, defaultTZ string) *Handlers {
	return &Handlers{svc: svc, defaultTZ: defaultTZ}
}

func (h *Handlers) Register(rg *gin.RouterGroup) {
	// /locations/resolve is registered before /locations/:id so the literal
	// segment wins the route match.
	rg.GET("/locations/resolve", h.resolve)
	rg.POST("/locations", h.create)
	rg.GET("/locations", h.list)
	rg.GET("/locations/:id", h.get)
	rg.DELETE("/locations/:id", h.delete)
}

type createBody struct {
	StartDate string   `json:"start_date"`
	EndDate   string   `json:"end_date"`
	Name      string   `json:"name"`
	Lat       *float64 `json:"lat"`
	Lon       *float64 `json:"lon"`
	Note      *string  `json:"note"`
	Place     string   `json:"place"`
}

// create godoc
// @Summary      Log a travel location period
// @Description  Records where the athlete is over an inclusive date range, so weather/heat reads for those dates resolve to that place instead of home. Supply either explicit `lat` (-90..90) + `lon` (-180..180), or a `place` name to geocode server-side (which also fills `name` when omitted); explicit coordinates win when both are given. A `place` matching nothing is `400 place_not_found`; geocoding being down is `503 geocoding_unavailable` — the write is refused rather than stored without real coordinates. City-grade precision is all the forecast needs. Overlapping periods are ACCEPTED: a weekend trip nested inside a training camp resolves to the weekend for its dates (latest `start_date` wins). No PATCH — corrections and trip extensions are delete + re-log. `Idempotency-Key` is supported. Nothing downstream is precomputed, so sessions already scheduled in the range follow the trip automatically.
// @Tags         locations
// @Accept       json
// @Produce      json
// @Param        Idempotency-Key  header  string      false  "Optional client-supplied idempotency key"
// @Param        body  body  createBody  true  "Location period"
// @Success      201   {object}  map[string]interface{}  "{\"location\": Period}"
// @Failure      400   {object}  map[string]string  "name_required | lat_lon_invalid | date_invalid | range_invalid | note_too_long | place_not_found | invalid_json"
// @Failure      503   {object}  map[string]string  "geocoding_unavailable"
// @Security     BearerAuth
// @Router       /locations [post]
func (h *Handlers) create(c *gin.Context) {
	var b createBody
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	p, err := h.svc.Create(c.Request.Context(), CreateInput{
		StartDate: b.StartDate, EndDate: b.EndDate, Name: b.Name,
		Lat: b.Lat, Lon: b.Lon, Note: b.Note, Place: b.Place,
	})
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"location": p})
}

// list godoc
// @Summary      List location periods overlapping a window
// @Description  Returns every period intersecting `[from, to]` (inclusive) ascending by `start_date`. Overlap, not containment: a camp starting before the window and ending inside it is still returned. `200` with an empty array when none. Range capped at 400 days.
// @Tags         locations
// @Produce      json
// @Param        from  query  string  true  "Inclusive start date YYYY-MM-DD"
// @Param        to    query  string  true  "Inclusive end date YYYY-MM-DD; max 400-day span"
// @Success      200   {object}  map[string]interface{}  "{\"locations\": [Period]}"
// @Failure      400   {object}  map[string]interface{}  "range_required | date_invalid | range_invalid | range_too_large"
// @Security     BearerAuth
// @Router       /locations [get]
func (h *Handlers) list(c *gin.Context) {
	from := c.Query("from")
	to := c.Query("to")
	if from == "" || to == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_required"})
		return
	}
	f, err := time.Parse(dateLayout, from)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
		return
	}
	t, err := time.Parse(dateLayout, to)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
		return
	}
	if t.Before(f) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_invalid"})
		return
	}
	if days := int(t.Sub(f).Hours()/24) + 1; days > maxRangeDays {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_too_large", "max_days": maxRangeDays})
		return
	}

	out, err := h.svc.List(c.Request.Context(), from, to)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"locations": out})
}

// get godoc
// @Summary      Get a location period
// @Tags         locations
// @Produce      json
// @Param        id  path  string  true  "Location period UUID"
// @Success      200  {object}  map[string]interface{}  "{\"location\": Period}"
// @Failure      404  {object}  map[string]string  "not_found"
// @Security     BearerAuth
// @Router       /locations/{id} [get]
func (h *Handlers) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	p, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"location": p})
}

// delete godoc
// @Summary      Delete a location period
// @Description  Removes a period. This is also the correction path — there is no PATCH; re-log with the right dates or coordinates.
// @Tags         locations
// @Produce      json
// @Param        id  path  string  true  "Location period UUID"
// @Success      204  "No Content"
// @Failure      404  {object}  map[string]string  "not_found"
// @Security     BearerAuth
// @Router       /locations/{id} [delete]
func (h *Handlers) delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		writeErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// resolve godoc
// @Summary      Resolve the effective location for a date
// @Description  Returns the one location a date resolves to: the covering travel period with the latest `start_date` (`source: "travel"`), else the configured home coordinates (`source: "home"`, `name: "home"`), else `404 location_unconfigured` when neither exists. This is the SAME primitive every weather consumer reads, so when a heat forecast surprises, one call shows which location produced it — the endpoint can never disagree with behavior.
// @Tags         locations
// @Produce      json
// @Param        date  query  string  false  "Date YYYY-MM-DD (defaults to today in the server timezone)"
// @Success      200  {object}  Resolved
// @Failure      400  {object}  map[string]string  "date_invalid"
// @Failure      404  {object}  map[string]string  "location_unconfigured"
// @Security     BearerAuth
// @Router       /locations/resolve [get]
func (h *Handlers) resolve(c *gin.Context) {
	var date time.Time
	if s := c.Query("date"); s != "" {
		d, err := time.Parse(dateLayout, s)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
			return
		}
		date = d
	} else {
		loc, err := time.LoadLocation(h.defaultTZ)
		if err != nil {
			loc = time.UTC
		}
		date = time.Now().In(loc)
	}

	out, err := h.svc.LocationOn(c.Request.Context(), date)
	if err != nil {
		writeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// writeErr maps sentinels 1:1 onto API error codes.
func writeErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
	case errors.Is(err, ErrUnconfigured):
		c.JSON(http.StatusNotFound, gin.H{"error": "location_unconfigured"})
	case errors.Is(err, ErrNameRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": "name_required"})
	case errors.Is(err, ErrLatLonInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "lat_lon_invalid"})
	case errors.Is(err, ErrDateInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_invalid"})
	case errors.Is(err, ErrRangeInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "range_invalid"})
	case errors.Is(err, ErrNoteTooLong):
		c.JSON(http.StatusBadRequest, gin.H{"error": "note_too_long"})
	case errors.Is(err, ErrPlaceNotFound):
		c.JSON(http.StatusBadRequest, gin.H{"error": "place_not_found"})
	case errors.Is(err, ErrGeocodingUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "geocoding_unavailable"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "location_failed"})
	}
}
