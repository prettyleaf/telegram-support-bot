package config

import (
	"fmt"
	"strings"
	"time"
)

const (
	defaultBusinessHoursTimezone = "UTC"
	defaultBusinessHoursStart    = "09:00"
	defaultBusinessHoursEnd      = "18:00"
)

// BusinessHoursConfig controls when the support team is considered online.
type BusinessHoursConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Timezone string `yaml:"timezone"`
	Start    string `yaml:"start"`
	End      string `yaml:"end"`
}

func normalizeBusinessHoursConfig(cfg BusinessHoursConfig) (BusinessHoursConfig, error) {
	startMinutes, startValue, err := parseBusinessHoursClock(cfg.Start, defaultBusinessHoursStart)
	if err != nil {
		return cfg, fmt.Errorf("business_hours.start: %w", err)
	}

	endMinutes, endValue, err := parseBusinessHoursClock(cfg.End, defaultBusinessHoursEnd)
	if err != nil {
		return cfg, fmt.Errorf("business_hours.end: %w", err)
	}

	cfg.Start = startValue
	cfg.End = endValue
	cfg.Timezone = strings.TrimSpace(cfg.Timezone)
	if cfg.Timezone == "" {
		cfg.Timezone = defaultBusinessHoursTimezone
	}

	if !cfg.Enabled {
		return cfg, nil
	}

	if _, err := time.LoadLocation(cfg.Timezone); err != nil {
		return cfg, fmt.Errorf("business_hours.timezone: %w", err)
	}

	if startMinutes == endMinutes {
		return cfg, fmt.Errorf("business_hours.start and business_hours.end must be different")
	}

	return cfg, nil
}

// ScheduleLabel returns a human-readable description of configured working hours.
func (cfg BusinessHoursConfig) ScheduleLabel() string {
	return fmt.Sprintf("%s-%s %s", cfg.Start, cfg.End, cfg.Timezone)
}

// IsWithin reports whether the provided time is inside the configured business hours.
func (cfg BusinessHoursConfig) IsWithin(now time.Time) (bool, error) {
	if !cfg.Enabled {
		return true, nil
	}

	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return false, fmt.Errorf("load business hours timezone: %w", err)
	}

	startMinutes, _, err := parseBusinessHoursClock(cfg.Start, defaultBusinessHoursStart)
	if err != nil {
		return false, fmt.Errorf("parse business_hours.start: %w", err)
	}

	endMinutes, _, err := parseBusinessHoursClock(cfg.End, defaultBusinessHoursEnd)
	if err != nil {
		return false, fmt.Errorf("parse business_hours.end: %w", err)
	}

	localNow := now.In(location)
	currentMinutes := localNow.Hour()*60 + localNow.Minute()

	if startMinutes < endMinutes {
		return currentMinutes >= startMinutes && currentMinutes < endMinutes, nil
	}

	return currentMinutes >= startMinutes || currentMinutes < endMinutes, nil
}

func parseBusinessHoursClock(value string, fallback string) (int, string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}

	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return 0, "", fmt.Errorf("invalid value %q, expected HH:MM", value)
	}

	return parsed.Hour()*60 + parsed.Minute(), parsed.Format("15:04"), nil
}
