package config

import "testing"

func TestGetOptionalIntEnvReturnsZeroWhenMissing(t *testing.T) {
	t.Setenv("HWID_DELETE_LOG_THREAD_ID", "")

	value, err := getOptionalIntEnv("HWID_DELETE_LOG_THREAD_ID")
	if err != nil {
		t.Fatalf("expected no error for empty value, got %v", err)
	}
	if value != 0 {
		t.Fatalf("expected zero, got %d", value)
	}
}

func TestGetOptionalIntEnvParsesValue(t *testing.T) {
	t.Setenv("HWID_DELETE_LOG_THREAD_ID", "42")

	value, err := getOptionalIntEnv("HWID_DELETE_LOG_THREAD_ID")
	if err != nil {
		t.Fatalf("expected parsed int, got error %v", err)
	}
	if value != 42 {
		t.Fatalf("expected 42, got %d", value)
	}
}
