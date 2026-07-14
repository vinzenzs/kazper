package publicfeed

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers serves the public race feed. It holds the shared secret directly and
// self-gates: an empty secret disables the endpoint.
type Handlers struct {
	svc    *Service
	secret string
}

func NewHandlers(svc *Service, secret string) *Handlers {
	return &Handlers{svc: svc, secret: secret}
}

// Register mounts the feed on the ROOT engine (a sibling of /healthz), OUTSIDE
// auth.Middleware — this is the one unauthenticated data route.
func (h *Handlers) Register(r *gin.Engine) {
	r.GET("/public/race-feed", h.get)
}

// get godoc
// @Summary      Public race feed (secret-gated, non-PII)
// @Description  The only unauthenticated data route: a curated projection of the active macrocycle's A-race (name, date, and a computed countdown) for an external Strapi shield to cache and re-serve publicly. Requires header `X-Feed-Key` equal to `FEED_SECRET` (compared in constant time) — this is NOT a bearer identity and grants access to nothing else. Returns `503 feed_disabled` when `FEED_SECRET` is unset, `401 feed_unauthorized` on a missing/wrong key. When there is no active anchored race the body degrades to `{"race": null, "days_remaining": null}`. No PII is ever returned.
// @Tags         public-feed
// @Produce      json
// @Param        X-Feed-Key  header  string  true  "Shared feed secret (equals FEED_SECRET)"
// @Success      200  {object}  map[string]interface{}  "{\"race\": {\"name\": \"...\", \"race_date\": \"YYYY-MM-DD\"} | null, \"days_remaining\": <int> | null}"
// @Failure      401  {object}  map[string]string  "feed_unauthorized"
// @Failure      503  {object}  map[string]string  "feed_disabled"
// @Router       /public/race-feed [get]
func (h *Handlers) get(c *gin.Context) {
	// Disabled-when-unset is checked first, so an unconfigured feed is 503 with or
	// without a key (an empty secret must never mean "no check").
	if h.secret == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "feed_disabled"})
		return
	}
	key := c.GetHeader("X-Feed-Key")
	if subtle.ConstantTimeCompare([]byte(key), []byte(h.secret)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "feed_unauthorized"})
		return
	}
	feed, err := h.svc.Feed(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "feed_unavailable"})
		return
	}
	c.JSON(http.StatusOK, feed)
}
