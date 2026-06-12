// Package config loads runtime configuration via Viper.
//
// Both the `serve` and `mcp` subcommands consume configuration through the
// same Load() function so env var precedence, defaults, and validation live
// in one place. Cobra flags take precedence over environment variables, which
// take precedence over built-in defaults.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// garminEncKeyBytes is the required decoded length of GARMIN_TOKEN_ENC_KEY
// (AES-256 → 32-byte key).
const garminEncKeyBytes = 32

// Config is the resolved runtime configuration. Field tags use the env var
// names that prior versions of the api and mcp binaries already accepted.
type Config struct {
	// HTTP API
	DatabaseURL         string        `mapstructure:"DATABASE_URL"`
	HTTPAddr            string        `mapstructure:"HTTP_ADDR"`
	MobileToken         string        `mapstructure:"MOBILE_API_TOKEN"`
	AgentToken          string        `mapstructure:"AGENT_API_TOKEN"`
	// Garmin integration (opt-in, per add-garmin-auth-token). GarminToken is the
	// dedicated bearer identity (client_id="garmin") the garmin-bridge calls
	// under; when empty the /garmin/token endpoints return 503 garmin_disabled.
	// GarminTokenEncKey is the base64-encoded AES-256 key used to encrypt the
	// stored token blob at rest; required only when GarminToken is set.
	GarminToken         string        `mapstructure:"GARMIN_API_TOKEN"`
	GarminTokenEncKey   string        `mapstructure:"GARMIN_TOKEN_ENC_KEY"`
	DefaultUserTZ       string        `mapstructure:"DEFAULT_USER_TZ"`
	OFFTimeout          time.Duration `mapstructure:"-"`
	OFFTimeoutSeconds   int           `mapstructure:"OFF_TIMEOUT_SECONDS"`
	OFFUserAgentContact string        `mapstructure:"OFF_USER_AGENT_CONTACT"`
	IdempotencyTTL      time.Duration `mapstructure:"-"`
	IdempotencyTTLHours int           `mapstructure:"IDEMPOTENCY_TTL_HOURS"`
	MigrateOnStart      bool          `mapstructure:"MIGRATE_ON_START"`
	SwaggerEnabled      bool          `mapstructure:"SWAGGER_ENABLED"`

	// MCP server
	NutritionAPIURL          string        `mapstructure:"NUTRITION_API_URL"`
	MCPRequestTimeout        time.Duration `mapstructure:"-"`
	MCPRequestTimeoutSeconds int           `mapstructure:"MCP_REQUEST_TIMEOUT_SECONDS"`

	// Vision (Claude). When AnthropicAPIKey is unset the meals/from_photo
	// endpoint returns 503; the rest of the API runs unchanged. Per
	// add-meal-from-photo.
	AnthropicAPIKey         string        `mapstructure:"ANTHROPIC_API_KEY"`
	ClaudeVisionModel       string        `mapstructure:"CLAUDE_VISION_MODEL"`
	VisionTimeout           time.Duration `mapstructure:"-"`
	VisionTimeoutSeconds    int           `mapstructure:"VISION_TIMEOUT_SECONDS"`
	MealFromPhotoMaxBytes   int64         `mapstructure:"MEAL_FROM_PHOTO_MAX_BYTES"`

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
var envKeys = []string{
	"DATABASE_URL",
	"HTTP_ADDR",
	"MOBILE_API_TOKEN",
	"AGENT_API_TOKEN",
	"GARMIN_API_TOKEN",
	"GARMIN_TOKEN_ENC_KEY",
	"DEFAULT_USER_TZ",
	"OFF_TIMEOUT_SECONDS",
	"OFF_USER_AGENT_CONTACT",
	"IDEMPOTENCY_TTL_HOURS",
	"MIGRATE_ON_START",
	"SWAGGER_ENABLED",
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
	v.SetDefault("NUTRITION_API_URL", "http://localhost:8080")
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
	return nil
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
