package database

import (
	"time"
)

// Ticket represents a support ticket
type Ticket struct {
	ID            int        `db:"id"`
	TicketNumber  string     `db:"ticket_number"`
	UserID        int64      `db:"user_id"`
	UserName      *string    `db:"user_name"`
	UserUsername  *string    `db:"user_username"`
	SupportChatID int64      `db:"support_chat_id"`
	ThreadID      int64      `db:"thread_id"`
	Status        string     `db:"status"`
	Priority      string     `db:"priority"`
	Subject       *string    `db:"subject"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
	ClosedAt      *time.Time `db:"closed_at"`
	ClosedBy      *int64     `db:"closed_by"`
	ClosedByName  *string    `db:"closed_by_name"`
}

// TicketMessage represents a message in a ticket thread
type TicketMessage struct {
	ID                int       `db:"id"`
	TicketID          int       `db:"ticket_id"`
	SenderID          int64     `db:"sender_id"`
	SenderName        *string   `db:"sender_name"`
	SenderUsername    *string   `db:"sender_username"`
	IsAdmin           bool      `db:"is_admin"`
	MessageText       *string   `db:"message_text"`
	MessageID         *int64    `db:"message_id"`
	OriginalMessageID *int64    `db:"original_message_id"`
	HasMedia          bool      `db:"has_media"`
	MediaType         *string   `db:"media_type"`
	MediaFileID       *string   `db:"media_file_id"`
	MediaFileName     *string   `db:"media_file_name"`
	MediaMimeType     *string   `db:"media_mime_type"`
	CreatedAt         time.Time `db:"created_at"`
}

// TicketNote represents an internal admin note on a ticket
type TicketNote struct {
	ID        int       `db:"id"`
	TicketID  int       `db:"ticket_id"`
	AdminID   int64     `db:"admin_id"`
	AdminName *string   `db:"admin_name"`
	NoteText  string    `db:"note_text"`
	CreatedAt time.Time `db:"created_at"`
}

// BannedUser represents a user banned from creating tickets
type BannedUser struct {
	ID           int       `db:"id"`
	UserID       int64     `db:"user_id"`
	UserName     *string   `db:"user_name"`
	UserUsername *string   `db:"user_username"`
	BannedBy     int64     `db:"banned_by"`
	BannedByName *string   `db:"banned_by_name"`
	Reason       *string   `db:"reason"`
	BannedAt     time.Time `db:"banned_at"`
}

// CannedResponse represents a predefined response
type CannedResponse struct {
	ID        int       `db:"id"`
	Shortcut  string    `db:"shortcut"`
	Title     string    `db:"title"`
	Content   string    `db:"content"`
	CreatedBy int64     `db:"created_by"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// TicketStats represents daily ticket statistics
type TicketStats struct {
	ID                       int       `db:"id"`
	Date                     time.Time `db:"date"`
	TicketsOpened            int       `db:"tickets_opened"`
	TicketsClosed            int       `db:"tickets_closed"`
	AvgResponseTimeMinutes   *int      `db:"avg_response_time_minutes"`
	AvgResolutionTimeMinutes *int      `db:"avg_resolution_time_minutes"`
}

// UserSettings represents user preferences
type UserSettings struct {
	ID               int        `db:"id"`
	UserID           int64      `db:"user_id"`
	Language         string     `db:"language"`
	LastHWIDDeleteAt *time.Time `db:"last_hwid_delete_at"`
	CreatedAt        time.Time  `db:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
}

// UserOnboarding represents whether a user passed the onboarding captcha.
type UserOnboarding struct {
	UserID          int64      `db:"user_id"`
	CaptchaPassed   bool       `db:"captcha_passed"`
	CaptchaPassedAt *time.Time `db:"captcha_passed_at"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
}

// WebAdminUser represents an admin account for the web panel.
type WebAdminUser struct {
	ID             int        `db:"id"`
	Username       *string    `db:"username"`
	DisplayName    string     `db:"display_name"`
	GitHubLogin    *string    `db:"github_login"`
	GitHubEmail    *string    `db:"github_email"`
	TelegramID     *int64     `db:"telegram_id"`
	WebauthnUserID []byte     `db:"webauthn_user_id"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
	LastLoginAt    *time.Time `db:"last_login_at"`
}

// WebAdminPasskey stores a registered passkey for a web admin.
type WebAdminPasskey struct {
	ID           int        `db:"id"`
	AdminUserID  int        `db:"admin_user_id"`
	Name         string     `db:"name"`
	CredentialID string     `db:"credential_id"`
	Credential   []byte     `db:"credential"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
	LastUsedAt   *time.Time `db:"last_used_at"`
}

// Language constants
const (
	LangEnglish = "en"
	LangRussian = "ru"
)

// Helper methods

// GetDisplayName returns a display name for the ticket user
func (t *Ticket) GetDisplayName() string {
	if t.UserUsername != nil && *t.UserUsername != "" {
		return "@" + *t.UserUsername
	}
	if t.UserName != nil && *t.UserName != "" {
		return *t.UserName
	}
	return "User"
}

// IsOpen returns true if the ticket is open
func (t *Ticket) IsOpen() bool {
	return t.Status == "open" || t.Status == "pending"
}

// IsClosed returns true if the ticket is closed
func (t *Ticket) IsClosed() bool {
	return t.Status == "closed" || t.Status == "resolved"
}
