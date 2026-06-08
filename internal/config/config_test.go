package config

import (
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

func TestLoadDefaults(t *testing.T) {
	v := New()
	v.Set("DATABASE_URL", "postgres://x")
	v.Set("MOBILE_API_TOKEN", "mobile-token-xxxxxxxxxxxxxx")
	v.Set("AGENT_API_TOKEN", "agent-token-xxxxxxxxxxxxxxx")

	c, err := Load(v)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if c.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr default = %q, want :8080", c.HTTPAddr)
	}
	if c.DefaultUserTZ != "UTC" {
		t.Errorf("DefaultUserTZ default = %q, want UTC", c.DefaultUserTZ)
	}
	if c.OFFTimeout.Seconds() != 5 {
		t.Errorf("OFFTimeout default = %s, want 5s", c.OFFTimeout)
	}
	if c.IdempotencyTTL.Hours() != 24 {
		t.Errorf("IdempotencyTTL default = %s, want 24h", c.IdempotencyTTL)
	}
	if !c.MigrateOnStart {
		t.Errorf("MigrateOnStart default = false, want true")
	}
	if c.SwaggerEnabled {
		t.Errorf("SwaggerEnabled default = true, want false")
	}
	if err := c.ValidateForServe(); err != nil {
		t.Errorf("ValidateForServe with defaults: %v", err)
	}
}

func TestValidateForServe_MissingRequired(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Config)
		want string
	}{
		{"missing db", func(c *Config) { c.DatabaseURL = "" }, "DATABASE_URL"},
		{"missing mobile", func(c *Config) { c.MobileToken = "" }, "MOBILE_API_TOKEN"},
		{"missing agent", func(c *Config) { c.AgentToken = "" }, "AGENT_API_TOKEN"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validServeConfig()
			tc.mut(c)
			err := c.ValidateForServe()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want substring %q", err, tc.want)
			}
		})
	}
}

func TestValidateForServe_InvalidTZ(t *testing.T) {
	c := validServeConfig()
	c.DefaultUserTZ = "Not/A/Zone"
	err := c.ValidateForServe()
	if err == nil || !strings.Contains(err.Error(), "Not/A/Zone") {
		t.Errorf("expected error mentioning Not/A/Zone, got %v", err)
	}
}

func TestFlagOverridesEnv(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("MOBILE_API_TOKEN", "m")
	t.Setenv("AGENT_API_TOKEN", "a")

	v := New()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("addr", "", "")
	if err := fs.Parse([]string{"--addr", ":9090"}); err != nil {
		t.Fatalf("flag parse: %v", err)
	}
	if err := BindFlags(v, fs); err != nil {
		t.Fatalf("BindFlags: %v", err)
	}

	c, err := Load(v)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.HTTPAddr != ":9090" {
		t.Errorf("HTTPAddr = %q, want :9090 (flag should win)", c.HTTPAddr)
	}
}

func TestEnvOverridesDefault(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":7777")
	v := New()
	c, err := Load(v)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.HTTPAddr != ":7777" {
		t.Errorf("HTTPAddr = %q, want :7777", c.HTTPAddr)
	}
}

func TestTokenRedaction(t *testing.T) {
	c := validServeConfig()
	c.MobileToken = "super-secret-mobile-12345"
	c.AgentToken = "super-secret-agent-67890"
	c.DatabaseURL = "postgres://user:hunter2@host/db"

	out := c.String()
	if strings.Contains(out, "super-secret-mobile-12345") {
		t.Errorf("String() leaked mobile token: %q", out)
	}
	if strings.Contains(out, "super-secret-agent-67890") {
		t.Errorf("String() leaked agent token: %q", out)
	}
	if strings.Contains(out, "hunter2") {
		t.Errorf("String() leaked DB password: %q", out)
	}

	r := c.Redacted()
	if r.MobileToken == c.MobileToken {
		t.Errorf("Redacted() did not redact MobileToken")
	}
	if r.AgentToken == c.AgentToken {
		t.Errorf("Redacted() did not redact AgentToken")
	}
}

func TestValidateForMCP(t *testing.T) {
	v := New()
	v.Set("AGENT_API_TOKEN", "a")
	c, err := Load(v)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := c.ValidateForMCP(); err != nil {
		t.Errorf("ValidateForMCP with defaults: %v", err)
	}

	c.NutritionAPIURL = "not a url"
	if err := c.ValidateForMCP(); err == nil {
		t.Errorf("expected error for bad URL, got nil")
	}
}

func validServeConfig() *Config {
	v := New()
	v.Set("DATABASE_URL", "postgres://x")
	v.Set("MOBILE_API_TOKEN", "m")
	v.Set("AGENT_API_TOKEN", "a")
	c, _ := Load(v)
	return c
}
