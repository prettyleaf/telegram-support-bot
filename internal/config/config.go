package config

import (
	"fmt"
	"net/url"
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
	BotToken              string
	AdminIDs              []int64
	SupportChatID         int64
	HWIDDeleteLogThreadID int
	PostgresUser          string
	PostgresPassword      string
	PostgresDB            string
	PostgresHost          string
	PostgresPort          string
	RemnawaveURL          string
	RemnawaveToken        string
	RemnawaveMode         string
	RemnawaveHeaders      map[string]string
	WebAdmin              WebAdminConfig
	Valkey                ValkeyConfig
}

type WebAdminConfig struct {
	Enabled           bool
	ListenAddr        string
	PublicURL         string
	PublicOrigin      string
	PublicHost        string
	BasePath          string
	SessionCookieName string
	SessionTTLHours   int
	Auth              WebAdminAuthConfig
}

type WebAdminAuthConfig struct {
	PasswordEnabled     bool
	PasswordUsers       map[string]string
	PasskeyEnabled      bool
	PasskeyDisplayName  string
	PasskeyRPID         string
	PasskeyOrigins      []string
	GitHubEnabled       bool
	GitHubClientID      string
	GitHubClientSecret  string
	GitHubAllowedEmails []string
	GitHubAllowedLogins []string
	TelegramEnabled     bool
	TelegramAllowedIDs  []int64
}

type ValkeyConfig struct {
	Addr     string
	Password string
	DB       int
}

// Global configuration variables
var (
	Cfg     *Config
	YAMLCfg *YAMLConfig
)

func normalizeWebAdminBasePath(rawBasePath, publicURL string) string {
	basePath := strings.TrimSpace(rawBasePath)
	if basePath == "" {
		basePath = inferWebAdminBasePath(publicURL)
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	basePath = strings.TrimRight(basePath, "/")
	if basePath == "" {
		return "/"
	}
	return basePath
}

func inferWebAdminBasePath(publicURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(publicURL))
	if err != nil || parsed.Path == "" {
		return "/"
	}
	return parsed.Path
}

func Load() error {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Warning: failed to load .env: %v\n", err)
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

	supportChatIDStr := os.Getenv("SUPPORT_CHAT_ID")
	if supportChatIDStr == "" {
		return fmt.Errorf("SUPPORT_CHAT_ID is not set")
	}
	supportChatID, err := strconv.ParseInt(supportChatIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("SUPPORT_CHAT_ID must be a valid number: %w", err)
	}
	Cfg.SupportChatID = supportChatID

	hwidDeleteLogThreadID, err := getOptionalIntEnv("HWID_DELETE_LOG_THREAD_ID")
	if err != nil {
		return err
	}
	Cfg.HWIDDeleteLogThreadID = hwidDeleteLogThreadID

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

	Cfg.WebAdmin.Enabled = getOptionalBoolEnv("WEB_ADMIN_ENABLED", false)
	Cfg.WebAdmin.ListenAddr = strings.TrimSpace(os.Getenv("WEB_ADMIN_LISTEN_ADDR"))
	if Cfg.WebAdmin.ListenAddr == "" {
		Cfg.WebAdmin.ListenAddr = ":8080"
	}
	Cfg.WebAdmin.PublicURL = strings.TrimRight(strings.TrimSpace(os.Getenv("WEB_ADMIN_PUBLIC_URL")), "/")
	Cfg.WebAdmin.BasePath = normalizeWebAdminBasePath(os.Getenv("WEB_ADMIN_BASE_PATH"), Cfg.WebAdmin.PublicURL)
	Cfg.WebAdmin.SessionCookieName = strings.TrimSpace(os.Getenv("WEB_ADMIN_SESSION_COOKIE_NAME"))
	if Cfg.WebAdmin.SessionCookieName == "" {
		Cfg.WebAdmin.SessionCookieName = "support_admin_session"
	}
	Cfg.WebAdmin.SessionTTLHours = getOptionalIntEnvWithDefault("WEB_ADMIN_SESSION_TTL_HOURS", 168)

	Cfg.WebAdmin.Auth.PasswordEnabled = getOptionalBoolEnv("WEB_AUTH_PASSWORD_ENABLED", false)
	Cfg.WebAdmin.Auth.PasswordUsers = parseKeyValueEnv(os.Getenv("WEB_AUTH_PASSWORD_USERS"))
	if len(Cfg.WebAdmin.Auth.PasswordUsers) > 0 {
		Cfg.WebAdmin.Auth.PasswordEnabled = true
	}

	Cfg.WebAdmin.Auth.PasskeyEnabled = getOptionalBoolEnv("WEB_AUTH_PASSKEY_ENABLED", false)
	Cfg.WebAdmin.Auth.PasskeyDisplayName = strings.TrimSpace(os.Getenv("WEB_AUTH_PASSKEY_DISPLAY_NAME"))
	if Cfg.WebAdmin.Auth.PasskeyDisplayName == "" {
		Cfg.WebAdmin.Auth.PasskeyDisplayName = "Support Ticket Bot Admin"
	}

	Cfg.WebAdmin.Auth.GitHubEnabled = getOptionalBoolEnv("WEB_AUTH_GITHUB_ENABLED", false)
	Cfg.WebAdmin.Auth.GitHubClientID = strings.TrimSpace(os.Getenv("WEB_AUTH_GITHUB_CLIENT_ID"))
	Cfg.WebAdmin.Auth.GitHubClientSecret = strings.TrimSpace(os.Getenv("WEB_AUTH_GITHUB_CLIENT_SECRET"))
	Cfg.WebAdmin.Auth.GitHubAllowedEmails = parseListEnv("WEB_AUTH_GITHUB_ALLOWED_EMAILS")
	Cfg.WebAdmin.Auth.GitHubAllowedLogins = parseListEnv("WEB_AUTH_GITHUB_ALLOWED_LOGINS")
	if Cfg.WebAdmin.Auth.GitHubClientID != "" && Cfg.WebAdmin.Auth.GitHubClientSecret != "" {
		Cfg.WebAdmin.Auth.GitHubEnabled = true
	}

	Cfg.WebAdmin.Auth.TelegramEnabled = getOptionalBoolEnv("WEB_AUTH_TELEGRAM_ENABLED", false)
	telegramAllowedIDs, err := parseOptionalInt64ListEnv("WEB_AUTH_TELEGRAM_ALLOWED_IDS")
	if err != nil {
		return err
	}
	if len(telegramAllowedIDs) == 0 {
		telegramAllowedIDs = append([]int64(nil), Cfg.AdminIDs...)
	}
	Cfg.WebAdmin.Auth.TelegramAllowedIDs = telegramAllowedIDs

	Cfg.Valkey.Addr = strings.TrimSpace(os.Getenv("VALKEY_ADDR"))
	Cfg.Valkey.Password = os.Getenv("VALKEY_PASSWORD")
	Cfg.Valkey.DB = getOptionalIntEnvWithDefault("VALKEY_DB", 0)

	if Cfg.WebAdmin.Enabled {
		if Cfg.WebAdmin.PublicURL == "" {
			return fmt.Errorf("WEB_ADMIN_PUBLIC_URL is required when WEB_ADMIN_ENABLED=true")
		}

		publicURL, err := url.Parse(Cfg.WebAdmin.PublicURL)
		if err != nil {
			return fmt.Errorf("WEB_ADMIN_PUBLIC_URL must be a valid URL: %w", err)
		}
		if publicURL.Scheme != "http" && publicURL.Scheme != "https" {
			return fmt.Errorf("WEB_ADMIN_PUBLIC_URL must use http or https")
		}
		if publicURL.Host == "" {
			return fmt.Errorf("WEB_ADMIN_PUBLIC_URL must include a host")
		}
		Cfg.WebAdmin.PublicOrigin = publicURL.Scheme + "://" + publicURL.Host
		Cfg.WebAdmin.PublicHost = publicURL.Hostname()
		Cfg.WebAdmin.Auth.PasskeyRPID = Cfg.WebAdmin.PublicHost
		Cfg.WebAdmin.Auth.PasskeyOrigins = []string{Cfg.WebAdmin.PublicOrigin}

		if !Cfg.WebAdmin.Auth.PasswordEnabled &&
			!Cfg.WebAdmin.Auth.PasskeyEnabled &&
			!Cfg.WebAdmin.Auth.GitHubEnabled &&
			!Cfg.WebAdmin.Auth.TelegramEnabled {
			return fmt.Errorf("at least one WEB_AUTH_* method must be enabled when WEB_ADMIN_ENABLED=true")
		}
		if Cfg.WebAdmin.Auth.GitHubEnabled && (Cfg.WebAdmin.Auth.GitHubClientID == "" || Cfg.WebAdmin.Auth.GitHubClientSecret == "") {
			return fmt.Errorf("WEB_AUTH_GITHUB_CLIENT_ID and WEB_AUTH_GITHUB_CLIENT_SECRET are required when GitHub auth is enabled")
		}
	}

	return nil
}

func getRequiredEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("required environment variable %s is not set", key)
	}
	return value, nil
}

func getOptionalIntEnv(key string) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return 0, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid number: %w", key, err)
	}

	return parsed, nil
}

func getOptionalIntEnvWithDefault(key string, fallback int) int {
	value, err := getOptionalIntEnv(key)
	if err != nil {
		return fallback
	}
	if strings.TrimSpace(os.Getenv(key)) == "" {
		return fallback
	}
	return value
}

func getOptionalBoolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}

	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
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

func parseListEnv(key string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}

	items := strings.Split(value, ",")
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		result = append(result, item)
	}
	return result
}

func parseOptionalInt64ListEnv(key string) ([]int64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil, nil
	}

	items := strings.Split(value, ",")
	result := make([]int64, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		parsed, err := strconv.ParseInt(item, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s must contain valid int64 values: %w", key, err)
		}
		result = append(result, parsed)
	}
	return result, nil
}
