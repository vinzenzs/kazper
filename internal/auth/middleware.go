package auth

import (
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ClientID is one of the resolved client identities set on the request context.
type ClientID string

const (
	ClientMobile ClientID = "mobile"
	ClientAgent  ClientID = "agent"
	ClientGarmin ClientID = "garmin"
	// ClientWeb is the browser dashboard identity, authenticated via HTTP Basic
	// auth against WEB_USER/WEB_PASSWORD. Optional — recognized only when both
	// are configured. Full access in v1 (per add-coach-dashboard).
	ClientWeb ClientID = "web"
)

const clientContextKey = "auth.client_id"

// Middleware returns a Gin middleware that requires Authorization: Bearer <token>
// matching one of the two configured tokens. On success the resolved client id
// is stored on the request context; on failure the request is aborted with 401.
func Middleware(cfg Config) gin.HandlerFunc {
	mobile := []byte(cfg.MobileToken)
	agent := []byte(cfg.AgentToken)
	// The garmin identity is optional; recognize it only when configured.
	garmin := []byte(cfg.GarminToken)
	garminEnabled := cfg.GarminToken != ""
	// The web (browser dashboard) identity is optional, gated on BOTH WEB_USER
	// and WEB_PASSWORD being set, mirroring how garmin is gated. The expected
	// Basic credential is the base64 of "user:password".
	webUser := []byte(cfg.WebUser)
	webPassword := []byte(cfg.WebPassword)
	webEnabled := cfg.WebUser != "" && cfg.WebPassword != ""

	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			abort(c, http.StatusUnauthorized, "auth_required")
			return
		}

		const bearerPrefix = "Bearer "
		const basicPrefix = "Basic "
		switch {
		case strings.HasPrefix(header, bearerPrefix):
			token := []byte(strings.TrimPrefix(header, bearerPrefix))
			switch {
			case subtle.ConstantTimeCompare(token, mobile) == 1:
				c.Set(clientContextKey, ClientMobile)
			case subtle.ConstantTimeCompare(token, agent) == 1:
				c.Set(clientContextKey, ClientAgent)
			case garminEnabled && subtle.ConstantTimeCompare(token, garmin) == 1:
				c.Set(clientContextKey, ClientGarmin)
			default:
				abort(c, http.StatusUnauthorized, "auth_invalid")
				return
			}
		case strings.HasPrefix(header, basicPrefix):
			// The web identity exists only when configured; absent that, no Basic
			// credential is valid.
			if !webEnabled || !validBasic(strings.TrimPrefix(header, basicPrefix), webUser, webPassword) {
				abort(c, http.StatusUnauthorized, "auth_invalid")
				return
			}
			c.Set(clientContextKey, ClientWeb)
		default:
			abort(c, http.StatusUnauthorized, "auth_required")
			return
		}
		c.Next()
	}
}

// validBasic decodes a base64 Basic-auth payload ("user:password") and
// constant-time compares both halves against the configured web credential.
// The comparisons run unconditionally (even on a malformed payload) so timing
// does not leak which half differs or whether decoding succeeded.
func validBasic(encoded string, wantUser, wantPassword []byte) bool {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return false
	}
	user, password, found := strings.Cut(string(decoded), ":")
	userOK := subtle.ConstantTimeCompare([]byte(user), wantUser) == 1
	passwordOK := subtle.ConstantTimeCompare([]byte(password), wantPassword) == 1
	return found && userOK && passwordOK
}

// BasicRealm is the single Basic-auth realm shared by the dashboard shell and
// the API calls it makes, so the browser prompts once and reuses the cached
// credential for every same-origin request. Sent as the WWW-Authenticate
// challenge value.
const BasicRealm = `Basic realm="Kazper Coach", charset="UTF-8"`

// WebEnabled reports whether the optional browser-dashboard identity is
// configured (both WEB_USER and WEB_PASSWORD set).
func WebEnabled(cfg Config) bool {
	return cfg.WebUser != "" && cfg.WebPassword != ""
}

// ValidWebBasic reports whether the raw Authorization header carries a valid
// `Basic` credential for the configured web identity. Returns false when the
// web identity is disabled, the scheme is not Basic, or the credential does not
// match. Used by the SPA-shell gate (the API path lives in Middleware).
func ValidWebBasic(authorization string, cfg Config) bool {
	if !WebEnabled(cfg) {
		return false
	}
	const basicPrefix = "Basic "
	if !strings.HasPrefix(authorization, basicPrefix) {
		return false
	}
	return validBasic(strings.TrimPrefix(authorization, basicPrefix), []byte(cfg.WebUser), []byte(cfg.WebPassword))
}

// ClientFromContext returns the client id set by the auth middleware, or empty
// string if the request has not been authenticated.
func ClientFromContext(c *gin.Context) ClientID {
	v, ok := c.Get(clientContextKey)
	if !ok {
		return ""
	}
	id, _ := v.(ClientID)
	return id
}

func abort(c *gin.Context, status int, code string) {
	c.AbortWithStatusJSON(status, gin.H{"error": code})
}
