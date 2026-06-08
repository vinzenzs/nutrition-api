package auth

import (
	"errors"
	"fmt"
)

// Config holds the two static bearer tokens the API accepts.
type Config struct {
	MobileToken string
	AgentToken  string
}

// minTokenBytes is the minimum acceptable length for a token in bytes.
const minTokenBytes = 16

var (
	ErrTokenMissing  = errors.New("auth token unset")
	ErrTokenTooShort = errors.New("auth token shorter than 16 bytes")
	ErrTokensEqual   = errors.New("MOBILE_API_TOKEN and AGENT_API_TOKEN must differ")
)

// Validate enforces non-empty, ≥16-byte, and distinct token invariants.
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
	return nil
}
