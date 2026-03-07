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
	StateNone                = ""
	StateWaitingTicketSubject = "waiting_ticket_subject"
	StateWaitingTicketMessage = "waiting_ticket_message"
	StateInTicket            = "in_ticket"
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
	BotToken       string
	AdminIDs       []int64 // Admin IDs from env (can interact with tickets)
	SupportChatID  int64   // The forum/supergroup chat ID for support threads
	PostgresUser   string
	PostgresPassword string
	PostgresDB     string
	PostgresHost   string
	PostgresPort   string
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

	Cfg = &Config{
		BotToken:         getRequiredEnv("BOT_TOKEN"),
		PostgresUser:     getRequiredEnv("POSTGRES_USER"),
		PostgresPassword: getRequiredEnv("POSTGRES_PASSWORD"),
		PostgresDB:       getRequiredEnv("POSTGRES_DB"),
		PostgresHost:     getRequiredEnv("POSTGRES_HOST"),
		PostgresPort:     getRequiredEnv("POSTGRES_PORT"),
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

	return nil
}

func getRequiredEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic(fmt.Sprintf("Required environment variable %s is not set", key))
	}
	return value
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
