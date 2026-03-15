package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// FSM States for ticket creation
const (
	StateNone                 = ""
	StateWaitingCaptcha       = "waiting_captcha"
	StateWaitingTicketSubject = "waiting_ticket_subject"
	StateWaitingTicketMessage = "waiting_ticket_message"
	StateInTicket             = "in_ticket"
)

// Ticket statuses
const (
	TicketStatusOpen     = "open"
	TicketStatusClosed   = "closed"
	TicketStatusPending  = "pending"
	TicketStatusResolved = "resolved"
)

// Ticket priorities
const (
	PriorityLow    = "low"
	PriorityNormal = "normal"
	PriorityHigh   = "high"
	PriorityUrgent = "urgent"
)

type Config struct {
	BotToken         string
	AdminIDs         []int64 // Admin IDs from env (can interact with tickets)
	SupportChatID    int64   // The forum/supergroup chat ID for support threads
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string
	PostgresHost     string
	PostgresPort     string
	RemnawaveURL     string
	RemnawaveToken   string
	RemnawaveMode    string
	RemnawaveHeaders map[string]string
}

// Global configuration variables
var (
	Cfg     *Config     // .env configuration
	YAMLCfg *YAMLConfig // YAML configuration (loaded from config.yaml)
)

func Load() error {
	if err := godotenv.Load(); err != nil {
		fmt.Println("Warning: .env file not found, using environment variables")
	}

	Cfg = &Config{}

	var err error
	if Cfg.BotToken, err = getRequiredEnv("BOT_TOKEN"); err != nil {
		return err
	}
	if Cfg.PostgresUser, err = getRequiredEnv("POSTGRES_USER"); err != nil {
		return err
	}
	if Cfg.PostgresPassword, err = getRequiredEnv("POSTGRES_PASSWORD"); err != nil {
		return err
	}
	if Cfg.PostgresDB, err = getRequiredEnv("POSTGRES_DB"); err != nil {
		return err
	}
	if Cfg.PostgresHost, err = getRequiredEnv("POSTGRES_HOST"); err != nil {
		return err
	}
	if Cfg.PostgresPort, err = getRequiredEnv("POSTGRES_PORT"); err != nil {
		return err
	}

	// Parse SUPPORT_CHAT_ID
	supportChatIDStr := os.Getenv("SUPPORT_CHAT_ID")
	if supportChatIDStr == "" {
		return fmt.Errorf("SUPPORT_CHAT_ID is not set")
	}
	supportChatID, err := strconv.ParseInt(supportChatIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("SUPPORT_CHAT_ID must be a valid number: %w", err)
	}
	Cfg.SupportChatID = supportChatID

	// Parse ADMIN_IDS (comma-separated list of admin user IDs)
	adminIDsStr := os.Getenv("ADMIN_IDS")
	if adminIDsStr == "" {
		return fmt.Errorf("ADMIN_IDS is not set")
	}

	Cfg.AdminIDs = []int64{}
	for _, idStr := range strings.Split(adminIDsStr, ",") {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		adminID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return fmt.Errorf("ADMIN_IDS must contain valid numbers: %w", err)
		}
		Cfg.AdminIDs = append(Cfg.AdminIDs, adminID)
	}

	if len(Cfg.AdminIDs) == 0 {
		return fmt.Errorf("at least one admin ID is required in ADMIN_IDS")
	}

	Cfg.RemnawaveURL = strings.TrimSpace(os.Getenv("REMNAWAVE_URL"))
	Cfg.RemnawaveToken = strings.TrimSpace(os.Getenv("REMNAWAVE_TOKEN"))
	Cfg.RemnawaveMode = strings.ToLower(strings.TrimSpace(os.Getenv("REMNAWAVE_MODE")))
	if Cfg.RemnawaveMode == "" {
		Cfg.RemnawaveMode = "remote"
	}
	Cfg.RemnawaveHeaders = parseKeyValueEnv(os.Getenv("REMNAWAVE_HEADERS"))

	return nil
}

func getRequiredEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("required environment variable %s is not set", key)
	}
	return value, nil
}

// IsAdmin checks if a user ID is in the admin list
func IsAdmin(userID int64) bool {
	for _, id := range Cfg.AdminIDs {
		if id == userID {
			return true
		}
	}
	return false
}

// HasRemnawave reports whether the panel integration is configured.
func (c *Config) HasRemnawave() bool {
	if c == nil {
		return false
	}
	return c.RemnawaveURL != "" && c.RemnawaveToken != ""
}

func parseKeyValueEnv(value string) map[string]string {
	result := make(map[string]string)
	for _, item := range strings.Split(value, ";") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		parts := strings.SplitN(item, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if key == "" || val == "" {
			continue
		}

		result[key] = val
	}
	return result
}
