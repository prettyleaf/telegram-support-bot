package config

import (
	"testing"
	"time"
)

func TestBusinessHoursIsWithinSameDayWindow(t *testing.T) {
	cfg := BusinessHoursConfig{
		Enabled:  true,
		Timezone: "UTC",
		Start:    "09:00",
		End:      "18:00",
	}

	inside, err := cfg.IsWithin(time.Date(2026, time.March, 23, 10, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !inside {
		t.Fatalf("expected 10:30 UTC to be within business hours")
	}

	outside, err := cfg.IsWithin(time.Date(2026, time.March, 23, 20, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if outside {
		t.Fatalf("expected 20:00 UTC to be outside business hours")
	}
}

func TestBusinessHoursIsWithinOvernightWindow(t *testing.T) {
	cfg := BusinessHoursConfig{
		Enabled:  true,
		Timezone: "UTC",
		Start:    "22:00",
		End:      "06:00",
	}

	insideLate, err := cfg.IsWithin(time.Date(2026, time.March, 23, 23, 15, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !insideLate {
		t.Fatalf("expected 23:15 UTC to be within overnight business hours")
	}

	insideEarly, err := cfg.IsWithin(time.Date(2026, time.March, 24, 5, 45, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !insideEarly {
		t.Fatalf("expected 05:45 UTC to be within overnight business hours")
	}

	outside, err := cfg.IsWithin(time.Date(2026, time.March, 24, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if outside {
		t.Fatalf("expected 12:00 UTC to be outside overnight business hours")
	}
}
