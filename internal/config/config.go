// Package config loads runtime configuration via Viper.
//
// Both the `serve` and `mcp` subcommands consume configuration through the
// same Load() function so env var precedence, defaults, and validation live
// in one place. Cobra flags take precedence over environment variables, which
// take precedence over built-in defaults.
package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// garminEncKeyBytes is the required decoded length of GARMIN_TOKEN_ENC_KEY
// (AES-256 → 32-byte key).
const garminEncKeyBytes = 32

// APIBasePath is the version prefix every domain REST endpoint is served under
// (per add-api-versioning). It is the single source of truth shared by the Gin
// route group, the in-process chat loopback dispatcher, and the NUTRITION_API_URL
// default below — so the server and its in-binary callers can never drift. Infra
// endpoints (/healthz, /readyz, /swagger) are deliberately NOT under this prefix.
// The swag `@BasePath` annotation in cmd/kazper/main.go mirrors this literally
// (a comment can't reference a Go const) — keep them in sync.
const APIBasePath = "/api/v1"

// Config is the resolved runtime configuration. Field tags use the env var
// names that prior versions of the api and mcp binaries already accepted.
type Config struct {
	// HTTP API
	DatabaseURL string `mapstructure:"DATABASE_URL"`
	HTTPAddr    string `mapstructure:"HTTP_ADDR"`
	MobileToken string `mapstructure:"MOBILE_API_TOKEN"`
	AgentToken  string `mapstructure:"AGENT_API_TOKEN"`
	// Garmin integration (opt-in, per add-garmin-auth-token). GarminToken is the
	// dedicated bearer identity (client_id="garmin") the garmin-bridge calls
	// under; when empty the /garmin/token endpoints return 503 garmin_disabled.
	// GarminTokenEncKey is the base64-encoded AES-256 key used to encrypt the
	// stored token blob at rest; required only when GarminToken is set.
	GarminToken       string `mapstructure:"GARMIN_API_TOKEN"`
	GarminTokenEncKey string `mapstructure:"GARMIN_TOKEN_ENC_KEY"`
	// Web dashboard Basic-auth credential (opt-in, per add-coach-dashboard). When
	// BOTH WebUser and WebPassword are set, the browser dashboard authenticates as
	// client_id="web" with full access; either empty leaves the web identity off.
	// Basic auth is base64, not encryption — reach the dashboard over TLS/Tailscale.
	WebUser     string `mapstructure:"WEB_USER"`
	WebPassword string `mapstructure:"WEB_PASSWORD"`
	// Home location for the weather/heat arc (add-location-periods). Optional and
	// gated on BOTH halves — a one-sided pair is an operator mistake, so it fails
	// fast at startup (the WEB_USER/WEB_PASSWORD pattern). Held as strings rather
	// than floats deliberately: 0 is a real coordinate, so a numeric zero value
	// could not be told apart from "unset". Home is quasi-static infrastructure
	// like DEFAULT_USER_TZ — moving house is a config change, not a travel log.
	HomeLat string `mapstructure:"HOME_LAT"`
	HomeLon string `mapstructure:"HOME_LON"`
	// GarminBridgeURL is the in-cluster base URL of the garmin-bridge (per
	// add-garmin-mcp-login). When set, /garmin/login + /garmin/login/mfa proxy
	// to it; when empty those endpoints return 503 garmin_disabled. Optional and
	// independent of GarminToken (the bridge owns its own token identity).
	GarminBridgeURL string `mapstructure:"GARMIN_BRIDGE_URL"`
	// FCM push (opt-in, per add-garmin-relogin-push). Push is enabled only when
	// BOTH FCMProjectID and FCMServiceAccountJSON are set; otherwise the push
	// surface is inert (503 push_disabled, relogin notification is a no-op).
	// FCMServiceAccountJSON is either inline service-account JSON or a path to a
	// JSON file; it is a secret and is redacted in any config dump.
	FCMProjectID          string        `mapstructure:"FCM_PROJECT_ID"`
	FCMServiceAccountJSON string        `mapstructure:"FCM_SERVICE_ACCOUNT_JSON"`
	DefaultUserTZ         string        `mapstructure:"DEFAULT_USER_TZ"`
	OFFTimeout            time.Duration `mapstructure:"-"`
	OFFTimeoutSeconds     int           `mapstructure:"OFF_TIMEOUT_SECONDS"`
	OFFUserAgentContact   string        `mapstructure:"OFF_USER_AGENT_CONTACT"`
	IdempotencyTTL        time.Duration `mapstructure:"-"`
	IdempotencyTTLHours   int           `mapstructure:"IDEMPOTENCY_TTL_HOURS"`
	MigrateOnStart        bool          `mapstructure:"MIGRATE_ON_START"`
	SwaggerEnabled        bool          `mapstructure:"SWAGGER_ENABLED"`

	// Server hardening (harden-server-baseline). HTTPRequestTimeout bounds every
	// /api/v1 request (streaming/long routes exempt); parsed from the duration
	// string HTTP_REQUEST_TIMEOUT (default 30s). MaxRequestBodyBytes caps request
	// bodies on /api/v1 (self-capped routes exempt). MetricsEnabled gates the
	// opt-in /metrics endpoint (default off — root is ingress-exposed).
	HTTPRequestTimeout    time.Duration `mapstructure:"-"`
	HTTPRequestTimeoutStr string        `mapstructure:"HTTP_REQUEST_TIMEOUT"`
	MaxRequestBodyBytes   int64         `mapstructure:"MAX_REQUEST_BODY_BYTES"`
	MetricsEnabled        bool          `mapstructure:"METRICS_ENABLED"`

	// FeedSecret is the shared secret the external Strapi shield presents (header
	// X-Feed-Key) to the public race feed (public-race-feed). When empty, the
	// GET /public/race-feed endpoint is disabled (503 feed_disabled). It is NOT a
	// bearer identity — it gates that one route only. Secret; redacted in dumps.
	FeedSecret string `mapstructure:"FEED_SECRET"`

	// MCP server
	NutritionAPIURL          string        `mapstructure:"NUTRITION_API_URL"`
	MCPRequestTimeout        time.Duration `mapstructure:"-"`
	MCPRequestTimeoutSeconds int           `mapstructure:"MCP_REQUEST_TIMEOUT_SECONDS"`

	// Vision (Claude). When AnthropicAPIKey is unset the meals/from_photo
	// endpoint returns 503; the rest of the API runs unchanged. Per
	// add-meal-from-photo.
	AnthropicAPIKey       string        `mapstructure:"ANTHROPIC_API_KEY"`
	ClaudeVisionModel     string        `mapstructure:"CLAUDE_VISION_MODEL"`
	VisionTimeout         time.Duration `mapstructure:"-"`
	VisionTimeoutSeconds  int           `mapstructure:"VISION_TIMEOUT_SECONDS"`
	MealFromPhotoMaxBytes int64         `mapstructure:"MEAL_FROM_PHOTO_MAX_BYTES"`

	// Cookidoo recipe import (server-side fetch + JSON-LD parse). Always on;
	// only the per-request timeout is configurable.
	CookidooTimeout        time.Duration `mapstructure:"-"`
	CookidooTimeoutSeconds int           `mapstructure:"COOKIDOO_TIMEOUT_SECONDS"`

	// Nutrition chat (POST /chat). Reuses ANTHROPIC_API_KEY; when that is unset
	// the endpoint returns 503 chat_unavailable. The agent loop streams from the
	// Anthropic Messages API and dispatches tools as loopback HTTP calls.
	ChatModel              string        `mapstructure:"CHAT_MODEL"`
	ChatMaxToolRounds      int           `mapstructure:"CHAT_MAX_TOOL_ROUNDS"`
	ChatMaxHistoryMessages int           `mapstructure:"CHAT_MAX_HISTORY_MESSAGES"`
	ChatRequestTimeout     time.Duration `mapstructure:"-"`
	ChatRequestTimeoutSecs int           `mapstructure:"CHAT_REQUEST_TIMEOUT_SECONDS"`
	ChatDietaryPreferences string        `mapstructure:"CHAT_DIETARY_PREFERENCES"`
}

// envKeys lists every environment variable Config recognises. Listed
// explicitly so missing values become validation errors rather than silently
// resolving to zero, and so they show up in `--help`-style introspection.
// ErrHomeLocationIncomplete is returned when exactly one half of the
// HOME_LAT/HOME_LON pair is set — coordinates are meaningless alone.
var ErrHomeLocationIncomplete = errors.New("HOME_LAT and HOME_LON must be set together")

var envKeys = []string{
	"DATABASE_URL",
	"HTTP_ADDR",
	"MOBILE_API_TOKEN",
	"AGENT_API_TOKEN",
	"GARMIN_API_TOKEN",
	"GARMIN_TOKEN_ENC_KEY",
	"GARMIN_BRIDGE_URL",
	"WEB_USER",
	"WEB_PASSWORD",
	"HOME_LAT",
	"HOME_LON",
	"FCM_PROJECT_ID",
	"FCM_SERVICE_ACCOUNT_JSON",
	"DEFAULT_USER_TZ",
	"OFF_TIMEOUT_SECONDS",
	"OFF_USER_AGENT_CONTACT",
	"IDEMPOTENCY_TTL_HOURS",
	"MIGRATE_ON_START",
	"SWAGGER_ENABLED",
	"HTTP_REQUEST_TIMEOUT",
	"MAX_REQUEST_BODY_BYTES",
	"METRICS_ENABLED",
	"FEED_SECRET",
	"NUTRITION_API_URL",
	"MCP_REQUEST_TIMEOUT_SECONDS",
	"ANTHROPIC_API_KEY",
	"CLAUDE_VISION_MODEL",
	"VISION_TIMEOUT_SECONDS",
	"MEAL_FROM_PHOTO_MAX_BYTES",
	"COOKIDOO_TIMEOUT_SECONDS",
	"CHAT_MODEL",
	"CHAT_MAX_TOOL_ROUNDS",
	"CHAT_MAX_HISTORY_MESSAGES",
	"CHAT_REQUEST_TIMEOUT_SECONDS",
	"CHAT_DIETARY_PREFERENCES",
}

// New returns a Viper instance pre-bound to all known environment variables
// and built-in defaults. Use this when wiring Cobra flags via BindFlags.
func New() *viper.Viper {
	v := viper.New()
	v.SetDefault("HTTP_ADDR", ":8080")
	v.SetDefault("DEFAULT_USER_TZ", "UTC")
	v.SetDefault("OFF_TIMEOUT_SECONDS", 5)
	v.SetDefault("IDEMPOTENCY_TTL_HOURS", 24)
	v.SetDefault("MIGRATE_ON_START", true)
	v.SetDefault("SWAGGER_ENABLED", false)
	v.SetDefault("HTTP_REQUEST_TIMEOUT", "30s")
	v.SetDefault("MAX_REQUEST_BODY_BYTES", 1<<20) // 1 MiB
	v.SetDefault("METRICS_ENABLED", false)
	v.SetDefault("NUTRITION_API_URL", "http://localhost:8080"+APIBasePath)
	v.SetDefault("MCP_REQUEST_TIMEOUT_SECONDS", 10)
	v.SetDefault("CLAUDE_VISION_MODEL", "claude-sonnet-4-6")
	v.SetDefault("VISION_TIMEOUT_SECONDS", 15)
	v.SetDefault("MEAL_FROM_PHOTO_MAX_BYTES", 10*1024*1024) // 10MB
	v.SetDefault("COOKIDOO_TIMEOUT_SECONDS", 15)
	v.SetDefault("CHAT_MODEL", "claude-sonnet-4-6")
	v.SetDefault("CHAT_MAX_TOOL_ROUNDS", 8)
	v.SetDefault("CHAT_MAX_HISTORY_MESSAGES", 40)
	v.SetDefault("CHAT_REQUEST_TIMEOUT_SECONDS", 120)
	v.SetDefault("CHAT_DIETARY_PREFERENCES", "vegetarian")
	v.AutomaticEnv()
	for _, k := range envKeys {
		_ = v.BindEnv(k)
	}
	return v
}

// Load resolves the configuration from the supplied Viper (or a fresh one if
// nil), applies defaults, and returns a populated Config. Validation is left
// to the caller via ValidateForServe / ValidateForMigrate / ValidateForMCP.
func Load(v *viper.Viper) (*Config, error) {
	if v == nil {
		v = New()
	}
	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, fmt.Errorf("config unmarshal: %w", err)
	}
	c.OFFTimeout = time.Duration(c.OFFTimeoutSeconds) * time.Second
	c.IdempotencyTTL = time.Duration(c.IdempotencyTTLHours) * time.Hour
	c.MCPRequestTimeout = time.Duration(c.MCPRequestTimeoutSeconds) * time.Second
	c.VisionTimeout = time.Duration(c.VisionTimeoutSeconds) * time.Second
	c.CookidooTimeout = time.Duration(c.CookidooTimeoutSeconds) * time.Second
	c.ChatRequestTimeout = time.Duration(c.ChatRequestTimeoutSecs) * time.Second
	if c.HTTPRequestTimeoutStr != "" {
		d, err := time.ParseDuration(c.HTTPRequestTimeoutStr)
		if err != nil {
			return nil, fmt.Errorf("invalid HTTP_REQUEST_TIMEOUT %q: %w", c.HTTPRequestTimeoutStr, err)
		}
		c.HTTPRequestTimeout = d
	}
	return &c, nil
}

// BindFlags wires Cobra/pflag flags into the supplied Viper so flag values
// take precedence over env vars and defaults.
func BindFlags(v *viper.Viper, fs *pflag.FlagSet) error {
	if f := fs.Lookup("addr"); f != nil {
		if err := v.BindPFlag("HTTP_ADDR", f); err != nil {
			return err
		}
	}
	return nil
}

// ValidateForServe enforces the requirements of the `serve` subcommand:
// a database URL, both bearer tokens, and a usable IANA timezone.
func (c *Config) ValidateForServe() error {
	if c.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	if c.MobileToken == "" {
		return errors.New("MOBILE_API_TOKEN is required")
	}
	if c.AgentToken == "" {
		return errors.New("AGENT_API_TOKEN is required")
	}
	if _, err := time.LoadLocation(c.DefaultUserTZ); err != nil {
		return fmt.Errorf("DEFAULT_USER_TZ %q invalid: %w", c.DefaultUserTZ, err)
	}
	if c.OFFTimeoutSeconds <= 0 {
		return errors.New("OFF_TIMEOUT_SECONDS must be a positive integer")
	}
	// Home location is opt-in and gated on both halves; a partial or malformed
	// pair is a likely operator mistake, so fail fast rather than silently
	// resolving every weather read to "unconfigured".
	if _, _, _, err := c.HomeLocation(); err != nil {
		return err
	}
	if c.IdempotencyTTLHours <= 0 {
		return errors.New("IDEMPOTENCY_TTL_HOURS must be a positive integer")
	}
	// Garmin integration is opt-in: only validate the enc key when the dedicated
	// token is set. Both halves are required together.
	if c.GarminToken != "" {
		if _, err := c.GarminEncKey(); err != nil {
			return err
		}
	}
	// FCM push is opt-in: validate the service-account credential whenever it is
	// supplied, so a malformed credential fails fast at startup rather than at
	// first send. PushEnabled() additionally requires FCM_PROJECT_ID.
	if c.FCMServiceAccountJSON != "" {
		if _, err := c.FCMServiceAccount(); err != nil {
			return err
		}
	}
	return nil
}

// PushEnabled reports whether Android push is configured. Push requires BOTH the
// FCM project id and a service-account credential; with either unset the push
// surface is inert (503 push_disabled, relogin notification is a no-op).
func (c *Config) PushEnabled() bool {
	return c.FCMProjectID != "" && c.FCMServiceAccountJSON != ""
}

// FCMServiceAccount resolves FCM_SERVICE_ACCOUNT_JSON to raw service-account
// JSON — treating a value beginning with '{' as inline JSON and anything else as
// a path to a JSON file — and verifies it parses as a Google service account
// (type "service_account" with a client_email and private_key). Returns an error
// naming the variable when unset or malformed.
func (c *Config) FCMServiceAccount() ([]byte, error) {
	raw := strings.TrimSpace(c.FCMServiceAccountJSON)
	if raw == "" {
		return nil, errors.New("FCM_SERVICE_ACCOUNT_JSON is required when FCM_PROJECT_ID is set")
	}
	data := []byte(raw)
	if !strings.HasPrefix(raw, "{") {
		b, err := os.ReadFile(raw)
		if err != nil {
			return nil, fmt.Errorf("FCM_SERVICE_ACCOUNT_JSON path is unreadable: %w", err)
		}
		data = b
	}
	var sa struct {
		Type        string `json:"type"`
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
	}
	if err := json.Unmarshal(data, &sa); err != nil {
		return nil, fmt.Errorf("FCM_SERVICE_ACCOUNT_JSON is not valid JSON: %w", err)
	}
	if sa.Type != "service_account" || sa.ClientEmail == "" || sa.PrivateKey == "" {
		return nil, errors.New("FCM_SERVICE_ACCOUNT_JSON is not a Google service-account credential")
	}
	return data, nil
}

// HomeLocation parses the configured home coordinates. Returns ok=false when
// the pair is unset (a legitimate state — the athlete simply hasn't configured
// home, and weather consumers degrade to `location_unconfigured` rather than
// guessing a city). An error means the pair is set but malformed.
//
// ValidateForServe rejects a one-sided or malformed pair at startup, so a
// booted server only ever sees "unset" or "valid here".
func (c *Config) HomeLocation() (lat, lon float64, ok bool, err error) {
	if c.HomeLat == "" && c.HomeLon == "" {
		return 0, 0, false, nil
	}
	if (c.HomeLat == "") != (c.HomeLon == "") {
		return 0, 0, false, ErrHomeLocationIncomplete
	}
	lat, err = strconv.ParseFloat(strings.TrimSpace(c.HomeLat), 64)
	if err != nil || lat < -90 || lat > 90 {
		return 0, 0, false, fmt.Errorf("HOME_LAT %q invalid: must be a number in [-90, 90]", c.HomeLat)
	}
	lon, err = strconv.ParseFloat(strings.TrimSpace(c.HomeLon), 64)
	if err != nil || lon < -180 || lon > 180 {
		return 0, 0, false, fmt.Errorf("HOME_LON %q invalid: must be a number in [-180, 180]", c.HomeLon)
	}
	return lat, lon, true, nil
}

// GarminEncKey decodes GARMIN_TOKEN_ENC_KEY from base64 and verifies it is a
// 32-byte AES-256 key. Returns an error when the key is unset or malformed.
// Callers must gate on GarminToken being set before relying on this.
func (c *Config) GarminEncKey() ([]byte, error) {
	if c.GarminTokenEncKey == "" {
		return nil, errors.New("GARMIN_TOKEN_ENC_KEY is required when GARMIN_API_TOKEN is set")
	}
	key, err := base64.StdEncoding.DecodeString(c.GarminTokenEncKey)
	if err != nil {
		return nil, fmt.Errorf("GARMIN_TOKEN_ENC_KEY is not valid base64: %w", err)
	}
	if len(key) != garminEncKeyBytes {
		return nil, fmt.Errorf("GARMIN_TOKEN_ENC_KEY must decode to %d bytes, got %d", garminEncKeyBytes, len(key))
	}
	return key, nil
}

// ValidateForMigrate enforces the requirements of the `migrate` subcommand.
func (c *Config) ValidateForMigrate() error {
	if c.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	return nil
}

// ValidateForMCP enforces the requirements of the `mcp` subcommand.
func (c *Config) ValidateForMCP() error {
	if c.AgentToken == "" {
		return errors.New("AGENT_API_TOKEN is required")
	}
	u, err := url.Parse(c.NutritionAPIURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("NUTRITION_API_URL is not a valid URL: %q", c.NutritionAPIURL)
	}
	if c.MCPRequestTimeoutSeconds <= 0 {
		return errors.New("MCP_REQUEST_TIMEOUT_SECONDS must be a positive integer")
	}
	return nil
}

// NutritionAPIBaseURL returns the parsed API base URL for the MCP client.
// Callers should call ValidateForMCP first.
func (c *Config) NutritionAPIBaseURL() (*url.URL, error) {
	return url.Parse(c.NutritionAPIURL)
}

// Redacted returns a copy of c with secret fields zeroed, safe for logging.
func (c *Config) Redacted() Config {
	cp := *c
	cp.MobileToken = redact(cp.MobileToken)
	cp.AgentToken = redact(cp.AgentToken)
	cp.GarminToken = redact(cp.GarminToken)
	cp.GarminTokenEncKey = redact(cp.GarminTokenEncKey)
	cp.FCMServiceAccountJSON = redact(cp.FCMServiceAccountJSON)
	cp.WebPassword = redact(cp.WebPassword)
	cp.FeedSecret = redact(cp.FeedSecret)
	return cp
}

func redact(s string) string {
	if s == "" {
		return ""
	}
	return "[redacted]"
}

// String renders the config with secrets redacted. Always use this for log
// output, never `%+v` on the bare struct.
func (c *Config) String() string {
	r := c.Redacted()
	var b strings.Builder
	fmt.Fprintf(&b, "Config{HTTPAddr=%s, DefaultUserTZ=%s, MigrateOnStart=%t, SwaggerEnabled=%t, ",
		r.HTTPAddr, r.DefaultUserTZ, r.MigrateOnStart, r.SwaggerEnabled)
	fmt.Fprintf(&b, "OFFTimeout=%s, IdempotencyTTL=%s, NutritionAPIURL=%s, MCPRequestTimeout=%s, ",
		r.OFFTimeout, r.IdempotencyTTL, r.NutritionAPIURL, r.MCPRequestTimeout)
	fmt.Fprintf(&b, "MobileToken=%s, AgentToken=%s, DatabaseURL=%s}",
		r.MobileToken, r.AgentToken, redactURL(r.DatabaseURL))
	return b.String()
}

// redactURL keeps the scheme/host visible but strips userinfo (passwords).
func redactURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "[unparseable]"
	}
	if u.User != nil {
		u.User = url.UserPassword(u.User.Username(), "redacted")
	}
	return u.String()
}
