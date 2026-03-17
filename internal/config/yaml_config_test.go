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
