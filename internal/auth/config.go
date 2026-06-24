package auth

import (
	"errors"
	"fmt"
)

// Config holds the static bearer tokens the API accepts. MobileToken and
// AgentToken are required; GarminToken is the OPTIONAL third identity for the
// garmin-bridge (per add-garmin-auth-token) — when empty the garmin client is
// simply not recognized.
type Config struct {
	MobileToken string
	AgentToken  string
	GarminToken string
	// WebUser / WebPassword are the OPTIONAL HTTP Basic credential for the
	// browser dashboard (client_id="web", per add-coach-dashboard). Recognized
	// only when BOTH are set; either empty leaves the web identity disabled.
	WebUser     string
	WebPassword string
}

// minTokenBytes is the minimum acceptable length for a token in bytes.
const minTokenBytes = 16

var (
	ErrTokenMissing  = errors.New("auth token unset")
	ErrTokenTooShort = errors.New("auth token shorter than 16 bytes")
	ErrTokensEqual   = errors.New("MOBILE_API_TOKEN and AGENT_API_TOKEN must differ")
	ErrGarminEqual   = errors.New("GARMIN_API_TOKEN must differ from MOBILE_API_TOKEN and AGENT_API_TOKEN")
	ErrWebIncomplete = errors.New("WEB_USER and WEB_PASSWORD must be set together")
)

// Validate enforces non-empty, ≥16-byte, and distinct token invariants. The
// optional GarminToken, when set, must also be ≥16 bytes and differ from the
// other two; when unset it imposes no constraint.
func (c Config) Validate() error {
	if c.MobileToken == "" {
		return fmt.Errorf("MOBILE_API_TOKEN: %w", ErrTokenMissing)
	}
	if c.AgentToken == "" {
		return fmt.Errorf("AGENT_API_TOKEN: %w", ErrTokenMissing)
	}
	if len(c.MobileToken) < minTokenBytes {
		return fmt.Errorf("MOBILE_API_TOKEN: %w", ErrTokenTooShort)
	}
	if len(c.AgentToken) < minTokenBytes {
		return fmt.Errorf("AGENT_API_TOKEN: %w", ErrTokenTooShort)
	}
	if c.MobileToken == c.AgentToken {
		return ErrTokensEqual
	}
	if c.GarminToken != "" {
		if len(c.GarminToken) < minTokenBytes {
			return fmt.Errorf("GARMIN_API_TOKEN: %w", ErrTokenTooShort)
		}
		if c.GarminToken == c.MobileToken || c.GarminToken == c.AgentToken {
			return ErrGarminEqual
		}
	}
	// The web identity is optional and gated on both halves; partial config is a
	// likely operator mistake, so fail fast rather than silently disable it.
	if (c.WebUser == "") != (c.WebPassword == "") {
		return ErrWebIncomplete
	}
	return nil
}
