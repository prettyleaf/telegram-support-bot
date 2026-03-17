package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// YAMLConfig holds all configuration from config.yaml
type YAMLConfig struct {
	Bot       BotConfig       `yaml:"bot"`
	Tickets   TicketsConfig   `yaml:"tickets"`
	Captcha   CaptchaConfig   `yaml:"captcha"`
	HWID      HWIDConfig      `yaml:"hwid"`
	Messages  MessagesConfig  `yaml:"messages"`
	Database  DatabaseConfig  `yaml:"database"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Thread    ThreadConfig    `yaml:"thread"`
}

type BotConfig struct {
	LogLevel string `yaml:"log_level"`
}

type TicketsConfig struct {
	AutoCloseHours  int    `yaml:"auto_close_hours"`
	MaxOpenPerUser  int    `yaml:"max_open_per_user"`
	NumberPrefix    string `yaml:"number_prefix"`
	PriorityEnabled bool   `yaml:"priority_enabled"`
	DefaultPriority string `yaml:"default_priority"`
}

type CaptchaConfig struct {
	Enabled        bool                    `yaml:"enabled"`
	Type           string                  `yaml:"type"`
	TimeoutSeconds int                     `yaml:"timeout_seconds"`
	Questions      []CaptchaQuestionConfig `yaml:"questions"`
}

type CaptchaQuestionConfig struct {
	Question      string   `yaml:"question"`
	CorrectAnswer string   `yaml:"correct_answer"`
	Options       []string `yaml:"options"`
}

type HWIDConfig struct {
	Enabled             bool     `yaml:"enabled"`
	TriggerKeywords     []string `yaml:"trigger_keywords"`
	DeleteCooldownHours int      `yaml:"delete_cooldown_hours"`
}

type MessagesConfig struct {
	Welcome         string `yaml:"welcome"`
	TicketCreated   string `yaml:"ticket_created"`
	TicketWelcome   string `yaml:"ticket_welcome"`
	TicketWelcomeRu string `yaml:"ticket_welcome_ru"`
	TicketClosed    string `yaml:"ticket_closed"`
	TicketReopened  string `yaml:"ticket_reopened"`
	UserBanned      string `yaml:"user_banned"`
}

type DatabaseConfig struct {
	MaxConnections           int `yaml:"max_connections"`
	MinConnections           int `yaml:"min_connections"`
	MaxConnLifetimeMinutes   int `yaml:"max_conn_lifetime_minutes"`
	MaxConnIdleMinutes       int `yaml:"max_conn_idle_minutes"`
	HealthCheckPeriodSeconds int `yaml:"health_check_period_seconds"`
}

type RateLimitConfig struct {
	MaxMessagesPerMinute   int `yaml:"max_messages_per_minute"`
	TicketCreationCooldown int `yaml:"ticket_creation_cooldown"`
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
	if cfg.Captcha.TimeoutSeconds == 0 {
		cfg.Captcha.TimeoutSeconds = 120
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
	cfg.Captcha = normalizeCaptchaConfig(cfg.Captcha)
	cfg.HWID.TriggerKeywords = normalizeHWIDKeywords(cfg.HWID.TriggerKeywords)
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

func normalizeHWIDKeywords(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))

	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}

	return normalized
}

func normalizeCaptchaConfig(cfg CaptchaConfig) CaptchaConfig {
	cfg.Type = strings.ToLower(strings.TrimSpace(cfg.Type))
	cfg.Questions = normalizeCaptchaQuestions(cfg.Questions)

	if cfg.Type == "" {
		if len(cfg.Questions) > 0 {
			cfg.Type = "custom"
		} else {
			cfg.Type = "math"
		}
	}

	switch cfg.Type {
	case "math", "button", "text", "custom":
	case "question", "questions", "quiz":
		cfg.Type = "custom"
	default:
		cfg.Type = "math"
	}

	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = 120
	}

	if cfg.Type == "custom" && len(cfg.Questions) == 0 {
		cfg.Type = "math"
	}

	return cfg
}

func normalizeCaptchaQuestions(values []CaptchaQuestionConfig) []CaptchaQuestionConfig {
	normalized := make([]CaptchaQuestionConfig, 0, len(values))

	for _, value := range values {
		question := strings.TrimSpace(value.Question)
		correct := strings.TrimSpace(value.CorrectAnswer)
		if question == "" || correct == "" {
			continue
		}

		options := make([]string, 0, len(value.Options)+1)
		seen := make(map[string]struct{}, len(value.Options)+1)
		appendOption := func(option string) {
			option = strings.TrimSpace(option)
			if option == "" {
				return
			}
			key := strings.ToLower(option)
			if _, exists := seen[key]; exists {
				return
			}
			seen[key] = struct{}{}
			options = append(options, option)
		}

		for _, option := range value.Options {
			appendOption(option)
		}
		appendOption(correct)

		if len(options) < 2 {
			continue
		}

		normalized = append(normalized, CaptchaQuestionConfig{
			Question:      question,
			CorrectAnswer: correct,
			Options:       options,
		})
	}

	return normalized
}
