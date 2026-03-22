package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadYAMLConfigDefaultsTicketPrefixWhenMissing(t *testing.T) {
	cfg := loadTestYAMLConfig(t, `
tickets:
  max_open_per_user: 3
`)

	if cfg.Tickets.NumberPrefix != "TKT" {
		t.Fatalf("expected default ticket prefix TKT, got %q", cfg.Tickets.NumberPrefix)
	}
}

func TestLoadYAMLConfigAllowsExplicitEmptyTicketPrefix(t *testing.T) {
	cfg := loadTestYAMLConfig(t, `
tickets:
  number_prefix: ""
`)

	if cfg.Tickets.NumberPrefix != "" {
		t.Fatalf("expected empty ticket prefix, got %q", cfg.Tickets.NumberPrefix)
	}
}

func TestLoadYAMLConfigBusinessHoursDefaults(t *testing.T) {
	cfg := loadTestYAMLConfig(t, `
business_hours:
  enabled: true
`)

	if cfg.BusinessHours.Timezone != "UTC" {
		t.Fatalf("expected default timezone UTC, got %q", cfg.BusinessHours.Timezone)
	}
	if cfg.BusinessHours.Start != "09:00" {
		t.Fatalf("expected default start 09:00, got %q", cfg.BusinessHours.Start)
	}
	if cfg.BusinessHours.End != "18:00" {
		t.Fatalf("expected default end 18:00, got %q", cfg.BusinessHours.End)
	}
}

func TestLoadYAMLConfigRejectsInvalidBusinessHoursTimezone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `
business_hours:
  enabled: true
  timezone: "Mars/Olympus"
  start: "09:00"
  end: "18:00"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write test config: %v", err)
	}

	if _, err := LoadYAMLConfig(path); err == nil {
		t.Fatalf("expected invalid business hours timezone to return an error")
	}
}

func loadTestYAMLConfig(t *testing.T, content string) *YAMLConfig {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write test config: %v", err)
	}

	cfg, err := LoadYAMLConfig(path)
	if err != nil {
		t.Fatalf("load yaml config: %v", err)
	}

	return cfg
}
