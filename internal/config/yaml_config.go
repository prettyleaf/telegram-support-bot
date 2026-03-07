package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// YAMLConfig holds all configuration from config.yaml
type YAMLConfig struct {
	Bot       BotConfig       `yaml:"bot"`
	Tickets   TicketsConfig   `yaml:"tickets"`
	Messages  MessagesConfig  `yaml:"messages"`
	Database  DatabaseConfig  `yaml:"database"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Thread    ThreadConfig    `yaml:"thread"`
}

type BotConfig struct {
	LogLevel string `yaml:"log_level"`
}

type TicketsConfig struct {
	AutoCloseHours   int    `yaml:"auto_close_hours"`
	MaxOpenPerUser   int    `yaml:"max_open_per_user"`
	NumberPrefix     string `yaml:"number_prefix"`
	PriorityEnabled  bool   `yaml:"priority_enabled"`
	DefaultPriority  string `yaml:"default_priority"`
}

type MessagesConfig struct {
	Welcome        string `yaml:"welcome"`
	TicketCreated  string `yaml:"ticket_created"`
	TicketWelcome  string `yaml:"ticket_welcome"`
	TicketWelcomeRu string `yaml:"ticket_welcome_ru"`
	TicketClosed   string `yaml:"ticket_closed"`
	TicketReopened string `yaml:"ticket_reopened"`
	UserBanned     string `yaml:"user_banned"`
}

type DatabaseConfig struct {
	MaxConnections          int `yaml:"max_connections"`
	MinConnections          int `yaml:"min_connections"`
	MaxConnLifetimeMinutes  int `yaml:"max_conn_lifetime_minutes"`
	MaxConnIdleMinutes      int `yaml:"max_conn_idle_minutes"`
	HealthCheckPeriodSeconds int `yaml:"health_check_period_seconds"`
}

type RateLimitConfig struct {
	MaxMessagesPerMinute     int `yaml:"max_messages_per_minute"`
	TicketCreationCooldown   int `yaml:"ticket_creation_cooldown"`
}

type ThreadConfig struct {
	NameFormat string `yaml:"name_format"`
	IconColor  int    `yaml:"icon_color"`
}

var forumTopicIconColors = []int{
	7322096,
	16766590,
	13338331,
	9367192,
	16749490,
	16478047,
}

// LoadYAMLConfig loads configuration from a YAML file
func LoadYAMLConfig(path string) (*YAMLConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg YAMLConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	// Set defaults
	if cfg.Tickets.MaxOpenPerUser == 0 {
		cfg.Tickets.MaxOpenPerUser = 3
	}
	if cfg.Tickets.NumberPrefix == "" {
		cfg.Tickets.NumberPrefix = "TKT"
	}
	if cfg.Tickets.DefaultPriority == "" {
		cfg.Tickets.DefaultPriority = PriorityNormal
	}
	if cfg.Database.MaxConnections == 0 {
		cfg.Database.MaxConnections = 10
	}
	if cfg.Database.MinConnections == 0 {
		cfg.Database.MinConnections = 2
	}
	if cfg.RateLimit.MaxMessagesPerMinute == 0 {
		cfg.RateLimit.MaxMessagesPerMinute = 10
	}
	if cfg.Thread.NameFormat == "" {
		cfg.Thread.NameFormat = "#{ticket_number} - {user_name}"
	}
	cfg.Thread.IconColor = normalizeThreadIconColor(cfg.Thread.IconColor)

	return &cfg, nil
}

func normalizeThreadIconColor(value int) int {
	if value == 0 {
		return forumTopicIconColors[0]
	}

	if value >= 1 && value <= len(forumTopicIconColors) {
		return forumTopicIconColors[value-1]
	}

	for _, allowed := range forumTopicIconColors {
		if value == allowed {
			return value
		}
	}

	return forumTopicIconColors[0]
}
