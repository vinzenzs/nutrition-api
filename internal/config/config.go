// Package config loads runtime configuration via Viper.
//
// Both the `serve` and `mcp` subcommands consume configuration through the
// same Load() function so env var precedence, defaults, and validation live
// in one place. Cobra flags take precedence over environment variables, which
// take precedence over built-in defaults.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config is the resolved runtime configuration. Field tags use the env var
// names that prior versions of the api and mcp binaries already accepted.
type Config struct {
	// HTTP API
	DatabaseURL         string        `mapstructure:"DATABASE_URL"`
	HTTPAddr            string        `mapstructure:"HTTP_ADDR"`
	MobileToken         string        `mapstructure:"MOBILE_API_TOKEN"`
	AgentToken          string        `mapstructure:"AGENT_API_TOKEN"`
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
}

// envKeys lists every environment variable Config recognises. Listed
// explicitly so missing values become validation errors rather than silently
// resolving to zero, and so they show up in `--help`-style introspection.
var envKeys = []string{
	"DATABASE_URL",
	"HTTP_ADDR",
	"MOBILE_API_TOKEN",
	"AGENT_API_TOKEN",
	"DEFAULT_USER_TZ",
	"OFF_TIMEOUT_SECONDS",
	"OFF_USER_AGENT_CONTACT",
	"IDEMPOTENCY_TTL_HOURS",
	"MIGRATE_ON_START",
	"SWAGGER_ENABLED",
	"NUTRITION_API_URL",
	"MCP_REQUEST_TIMEOUT_SECONDS",
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
	return nil
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
