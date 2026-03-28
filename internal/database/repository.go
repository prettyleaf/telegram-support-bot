package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"remnafamily/internal/config"
	"remnafamily/internal/utils"

	"github.com/jackc/pgx/v5"
)

// Repository handles all database operations
type Repository struct{}

// NewRepository creates a new database repository
func NewRepository() *Repository {
	return &Repository{}
}

// Ticket Operations

func (r *Repository) acquireAdvisoryLock(ctx context.Context, key int64) (func() error, error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, key); err != nil {
		conn.Release()
		return nil, err
	}

	released := false
	return func() error {
		if released {
			return nil
		}
		released = true

		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, unlockErr := conn.Exec(unlockCtx, `SELECT pg_advisory_unlock($1)`, key)
		conn.Release()
		return unlockErr
	}, nil
}

// CreateTicket creates a new support ticket
func (r *Repository) CreateTicket(ctx context.Context, ticket *Ticket) error {
	// Sanitize string fields to ensure valid UTF-8
	ticket.UserName = utils.SanitizeUTF8Ptr(ticket.UserName)
	ticket.UserUsername = utils.SanitizeUTF8Ptr(ticket.UserUsername)
	ticket.Subject = utils.SanitizeUTF8Ptr(ticket.Subject)

	if ticket.ID > 0 {
		query := `
			INSERT INTO tickets (id, ticket_number, user_id, user_name, user_username, support_chat_id, thread_id, status, priority, subject)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING id, created_at, updated_at
		`
		return pool.QueryRow(ctx, query,
			ticket.ID,
			ticket.TicketNumber,
			ticket.UserID,
			ticket.UserName,
			ticket.UserUsername,
			ticket.SupportChatID,
			ticket.ThreadID,
			ticket.Status,
			ticket.Priority,
			ticket.Subject,
		).Scan(&ticket.ID, &ticket.CreatedAt, &ticket.UpdatedAt)
	}

	query := `
		INSERT INTO tickets (ticket_number, user_id, user_name, user_username, support_chat_id, thread_id, status, priority, subject)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at
	`
	return pool.QueryRow(ctx, query,
		ticket.TicketNumber,
		ticket.UserID,
		ticket.UserName,
		ticket.UserUsername,
		ticket.SupportChatID,
		ticket.ThreadID,
		ticket.Status,
		ticket.Priority,
		ticket.Subject,
	).Scan(&ticket.ID, &ticket.CreatedAt, &ticket.UpdatedAt)
}

// GetTicketByID retrieves a ticket by its database ID
func (r *Repository) GetTicketByID(ctx context.Context, id int) (*Ticket, error) {
	query := `SELECT * FROM tickets WHERE id = $1`
	return r.scanTicket(ctx, query, id)
}

// GetTicketByNumber retrieves a ticket by its ticket number
func (r *Repository) GetTicketByNumber(ctx context.Context, ticketNumber string) (*Ticket, error) {
	query := `SELECT * FROM tickets WHERE ticket_number = $1`
	return r.scanTicket(ctx, query, ticketNumber)
}

// GetTicketByThreadID retrieves a ticket by its thread ID
func (r *Repository) GetTicketByThreadID(ctx context.Context, threadID int64) (*Ticket, error) {
	query := `SELECT * FROM tickets WHERE thread_id = $1`
	return r.scanTicket(ctx, query, threadID)
}

// GetOpenTicketByUserID retrieves the most recent open ticket for a user
func (r *Repository) GetOpenTicketByUserID(ctx context.Context, userID int64) (*Ticket, error) {
	query := `SELECT * FROM tickets WHERE user_id = $1 AND status IN ('open', 'pending') ORDER BY created_at DESC LIMIT 1`
	return r.scanTicket(ctx, query, userID)
}

// GetLatestTicketByUserID retrieves the most recent ticket for a user regardless of status.
func (r *Repository) GetLatestTicketByUserID(ctx context.Context, userID int64) (*Ticket, error) {
	query := `SELECT * FROM tickets WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`
	return r.scanTicket(ctx, query, userID)
}

// GetTicketsByUserID retrieves all tickets for a user
func (r *Repository) GetTicketsByUserID(ctx context.Context, userID int64, limit int) ([]*Ticket, error) {
	query := `SELECT * FROM tickets WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`
	return r.scanTickets(ctx, query, userID, limit)
}

// GetOpenTicketsCountByUserID counts open tickets for a user
func (r *Repository) GetOpenTicketsCountByUserID(ctx context.Context, userID int64) (int, error) {
	query := `SELECT COUNT(*) FROM tickets WHERE user_id = $1 AND status IN ('open', 'pending')`
	var count int
	err := pool.QueryRow(ctx, query, userID).Scan(&count)
	return count, err
}

// GetAllOpenTickets retrieves all open tickets
func (r *Repository) GetAllOpenTickets(ctx context.Context) ([]*Ticket, error) {
	query := `SELECT * FROM tickets WHERE status IN ('open', 'pending') ORDER BY created_at ASC`
	return r.scanTickets(ctx, query)
}

// GetInactiveOpenTickets retrieves open tickets whose last activity is older than the cutoff.
func (r *Repository) GetInactiveOpenTickets(ctx context.Context, cutoff time.Time) ([]*Ticket, error) {
	query := `
		SELECT t.*
		FROM tickets t
		LEFT JOIN (
			SELECT ticket_id, MAX(created_at) AS last_message_at
			FROM ticket_messages
			GROUP BY ticket_id
		) tm ON tm.ticket_id = t.id
		WHERE t.status IN ('open', 'pending')
			AND COALESCE(tm.last_message_at, t.created_at) < $1
		ORDER BY COALESCE(tm.last_message_at, t.created_at) ASC
	`
	return r.scanTickets(ctx, query, cutoff)
}

// GetTicketsPage retrieves a paginated ticket list by filter.
func (r *Repository) GetTicketsPage(ctx context.Context, filter string, limit, offset int) ([]*Ticket, error) {
	switch strings.ToLower(filter) {
	case "open":
		query := `SELECT * FROM tickets WHERE status IN ('open', 'pending') ORDER BY created_at ASC LIMIT $1 OFFSET $2`
		return r.scanTickets(ctx, query, limit, offset)
	case "closed":
		query := `SELECT * FROM tickets WHERE status IN ('closed', 'resolved') ORDER BY COALESCE(closed_at, created_at) DESC LIMIT $1 OFFSET $2`
		return r.scanTickets(ctx, query, limit, offset)
	default:
		query := `SELECT * FROM tickets ORDER BY created_at DESC LIMIT $1 OFFSET $2`
		return r.scanTickets(ctx, query, limit, offset)
	}
}

// CountTicketsByFilter counts tickets for the requested filter.
func (r *Repository) CountTicketsByFilter(ctx context.Context, filter string) (int, error) {
	var query string
	switch strings.ToLower(filter) {
	case "open":
		query = `SELECT COUNT(*) FROM tickets WHERE status IN ('open', 'pending')`
	case "closed":
		query = `SELECT COUNT(*) FROM tickets WHERE status IN ('closed', 'resolved')`
	default:
		query = `SELECT COUNT(*) FROM tickets`
	}

	var count int
	err := pool.QueryRow(ctx, query).Scan(&count)
	return count, err
}

// GetClosedTickets retrieves closed tickets with pagination
func (r *Repository) GetClosedTickets(ctx context.Context, limit, offset int) ([]*Ticket, error) {
	query := `SELECT * FROM tickets WHERE status IN ('closed', 'resolved') ORDER BY closed_at DESC LIMIT $1 OFFSET $2`
	return r.scanTickets(ctx, query, limit, offset)
}

// GetAllTickets retrieves all tickets with pagination
func (r *Repository) GetAllTickets(ctx context.Context, limit, offset int) ([]*Ticket, error) {
	query := `SELECT * FROM tickets ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	return r.scanTickets(ctx, query, limit, offset)
}

// UpdateTicketStatus updates the status of a ticket
func (r *Repository) UpdateTicketStatus(ctx context.Context, ticketID int, status string) error {
	query := `UPDATE tickets SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err := pool.Exec(ctx, query, status, ticketID)
	return err
}

// CloseTicket closes a ticket
func (r *Repository) CloseTicket(ctx context.Context, ticketID int, closedBy int64, closedByName string) error {
	query := `
		UPDATE tickets 
		SET status = $1, closed_at = NOW(), closed_by = $2, closed_by_name = $3, updated_at = NOW()
		WHERE id = $4
	`
	_, err := pool.Exec(ctx, query, config.TicketStatusClosed, closedBy, closedByName, ticketID)
	return err
}

// ReopenTicket reopens a closed ticket
func (r *Repository) ReopenTicket(ctx context.Context, ticketID int) error {
	query := `
		UPDATE tickets 
		SET status = $1, closed_at = NULL, closed_by = NULL, closed_by_name = NULL, updated_at = NOW()
		WHERE id = $2
	`
	_, err := pool.Exec(ctx, query, config.TicketStatusOpen, ticketID)
	return err
}

// UpdateTicketPriority updates the priority of a ticket
func (r *Repository) UpdateTicketPriority(ctx context.Context, ticketID int, priority string) error {
	query := `UPDATE tickets SET priority = $1, updated_at = NOW() WHERE id = $2`
	_, err := pool.Exec(ctx, query, priority, ticketID)
	return err
}

// UpdateTicketThreadID updates the thread ID of a ticket
func (r *Repository) UpdateTicketThreadID(ctx context.Context, ticketID int, threadID int64) error {
	query := `UPDATE tickets SET thread_id = $1, updated_at = NOW() WHERE id = $2`
	_, err := pool.Exec(ctx, query, threadID, ticketID)
	return err
}

// AcquireTicketCreationLock serializes ticket creation for a specific user across workers.
func (r *Repository) AcquireTicketCreationLock(ctx context.Context, userID int64) (func() error, error) {
	return r.acquireAdvisoryLock(ctx, userID)
}

// AcquireTicketThreadLock serializes topic recreation/message forwarding for a specific ticket.
func (r *Repository) AcquireTicketThreadLock(ctx context.Context, ticketID int) (func() error, error) {
	return r.acquireAdvisoryLock(ctx, -int64(ticketID)-1)
}

// ReserveNextTicketIdentity reserves the next ticket ID and returns the formatted ticket number.
func (r *Repository) ReserveNextTicketIdentity(ctx context.Context) (int, string, error) {
	var nextID int64
	query := `SELECT nextval(pg_get_serial_sequence('tickets', 'id'))`
	err := pool.QueryRow(ctx, query).Scan(&nextID)
	if err != nil {
		return 0, "", err
	}

	return int(nextID), formatTicketNumber(nextID), nil
}

func formatTicketNumber(id int64) string {
	number := fmt.Sprintf("%d", id)
	prefix := ""
	if config.YAMLCfg != nil {
		prefix = strings.TrimSpace(config.YAMLCfg.Tickets.NumberPrefix)
	}
	if prefix == "" {
		return number
	}

	return fmt.Sprintf("%s-%s", prefix, number)
}

// Ticket Message Operations

// CreateTicketMessage creates a new message in a ticket
func (r *Repository) CreateTicketMessage(ctx context.Context, msg *TicketMessage) error {
	// Sanitize string fields to ensure valid UTF-8
	msg.SenderName = utils.SanitizeUTF8Ptr(msg.SenderName)
	msg.SenderUsername = utils.SanitizeUTF8Ptr(msg.SenderUsername)
	msg.MessageText = utils.SanitizeUTF8Ptr(msg.MessageText)
	msg.MediaFileName = utils.SanitizeUTF8Ptr(msg.MediaFileName)
	msg.MediaMimeType = utils.SanitizeUTF8Ptr(msg.MediaMimeType)

	query := `
		INSERT INTO ticket_messages (
			ticket_id, sender_id, sender_name, sender_username, is_admin, message_text,
			message_id, original_message_id, has_media, media_type, media_file_id,
			media_file_name, media_mime_type
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, created_at
	`
	return pool.QueryRow(ctx, query,
		msg.TicketID,
		msg.SenderID,
		msg.SenderName,
		msg.SenderUsername,
		msg.IsAdmin,
		msg.MessageText,
		msg.MessageID,
		msg.OriginalMessageID,
		msg.HasMedia,
		msg.MediaType,
		msg.MediaFileID,
		msg.MediaFileName,
		msg.MediaMimeType,
	).Scan(&msg.ID, &msg.CreatedAt)
}

// GetTicketMessages retrieves all messages for a ticket
func (r *Repository) GetTicketMessages(ctx context.Context, ticketID int) ([]*TicketMessage, error) {
	query := `SELECT * FROM ticket_messages WHERE ticket_id = $1 ORDER BY created_at ASC`
	rows, err := pool.Query(ctx, query, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*TicketMessage
	for rows.Next() {
		var msg TicketMessage
		err := rows.Scan(
			&msg.ID, &msg.TicketID, &msg.SenderID, &msg.SenderName, &msg.SenderUsername,
			&msg.IsAdmin, &msg.MessageText, &msg.MessageID, &msg.OriginalMessageID,
			&msg.HasMedia, &msg.MediaType, &msg.MediaFileID, &msg.MediaFileName,
			&msg.MediaMimeType, &msg.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		messages = append(messages, &msg)
	}
	return messages, nil
}

// GetTicketMessage retrieves a single message by ticket and message ID.
func (r *Repository) GetTicketMessage(ctx context.Context, ticketID int, messageID int) (*TicketMessage, error) {
	query := `SELECT * FROM ticket_messages WHERE ticket_id = $1 AND id = $2 LIMIT 1`

	var msg TicketMessage
	err := pool.QueryRow(ctx, query, ticketID, messageID).Scan(
		&msg.ID, &msg.TicketID, &msg.SenderID, &msg.SenderName, &msg.SenderUsername,
		&msg.IsAdmin, &msg.MessageText, &msg.MessageID, &msg.OriginalMessageID,
		&msg.HasMedia, &msg.MediaType, &msg.MediaFileID, &msg.MediaFileName,
		&msg.MediaMimeType, &msg.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &msg, nil
}

// Banned Users Operations

// BanUser bans a user from creating tickets
func (r *Repository) BanUser(ctx context.Context, ban *BannedUser) error {
	query := `
		INSERT INTO banned_users (user_id, user_name, user_username, banned_by, banned_by_name, reason)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id) DO UPDATE SET
			banned_by = EXCLUDED.banned_by,
			banned_by_name = EXCLUDED.banned_by_name,
			reason = EXCLUDED.reason,
			banned_at = NOW()
		RETURNING id, banned_at
	`
	return pool.QueryRow(ctx, query,
		ban.UserID,
		ban.UserName,
		ban.UserUsername,
		ban.BannedBy,
		ban.BannedByName,
		ban.Reason,
	).Scan(&ban.ID, &ban.BannedAt)
}

// UnbanUser removes a user from the ban list
func (r *Repository) UnbanUser(ctx context.Context, userID int64) error {
	query := `DELETE FROM banned_users WHERE user_id = $1`
	_, err := pool.Exec(ctx, query, userID)
	return err
}

// IsUserBanned checks if a user is banned
func (r *Repository) IsUserBanned(ctx context.Context, userID int64) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM banned_users WHERE user_id = $1)`
	var exists bool
	err := pool.QueryRow(ctx, query, userID).Scan(&exists)
	return exists, err
}

// GetBannedUser retrieves ban info for a user
func (r *Repository) GetBannedUser(ctx context.Context, userID int64) (*BannedUser, error) {
	query := `SELECT * FROM banned_users WHERE user_id = $1`
	row := pool.QueryRow(ctx, query, userID)

	var ban BannedUser
	err := row.Scan(
		&ban.ID, &ban.UserID, &ban.UserName, &ban.UserUsername,
		&ban.BannedBy, &ban.BannedByName, &ban.Reason, &ban.BannedAt,
	)
	if err != nil {
		return nil, err
	}
	return &ban, nil
}

// GetAllBannedUsers retrieves all banned users
func (r *Repository) GetAllBannedUsers(ctx context.Context) ([]*BannedUser, error) {
	query := `SELECT * FROM banned_users ORDER BY banned_at DESC`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*BannedUser
	for rows.Next() {
		var ban BannedUser
		err := rows.Scan(
			&ban.ID, &ban.UserID, &ban.UserName, &ban.UserUsername,
			&ban.BannedBy, &ban.BannedByName, &ban.Reason, &ban.BannedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, &ban)
	}
	return users, nil
}

// Canned Responses Operations

// CreateCannedResponse creates a new canned response
func (r *Repository) CreateCannedResponse(ctx context.Context, resp *CannedResponse) error {
	query := `
		INSERT INTO canned_responses (shortcut, title, content, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at
	`
	return pool.QueryRow(ctx, query,
		resp.Shortcut,
		resp.Title,
		resp.Content,
		resp.CreatedBy,
	).Scan(&resp.ID, &resp.CreatedAt, &resp.UpdatedAt)
}

// GetCannedResponse retrieves a canned response by shortcut
func (r *Repository) GetCannedResponse(ctx context.Context, shortcut string) (*CannedResponse, error) {
	query := `SELECT * FROM canned_responses WHERE shortcut = $1`
	row := pool.QueryRow(ctx, query, shortcut)

	var resp CannedResponse
	err := row.Scan(
		&resp.ID, &resp.Shortcut, &resp.Title, &resp.Content,
		&resp.CreatedBy, &resp.CreatedAt, &resp.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetAllCannedResponses retrieves all canned responses
func (r *Repository) GetAllCannedResponses(ctx context.Context) ([]*CannedResponse, error) {
	query := `SELECT * FROM canned_responses ORDER BY shortcut ASC`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var responses []*CannedResponse
	for rows.Next() {
		var resp CannedResponse
		err := rows.Scan(
			&resp.ID, &resp.Shortcut, &resp.Title, &resp.Content,
			&resp.CreatedBy, &resp.CreatedAt, &resp.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		responses = append(responses, &resp)
	}
	return responses, nil
}

// DeleteCannedResponse deletes a canned response
func (r *Repository) DeleteCannedResponse(ctx context.Context, shortcut string) error {
	query := `DELETE FROM canned_responses WHERE shortcut = $1`
	_, err := pool.Exec(ctx, query, shortcut)
	return err
}

// Ticket Notes Operations

// CreateTicketNote creates a new internal note on a ticket
func (r *Repository) CreateTicketNote(ctx context.Context, note *TicketNote) error {
	query := `
		INSERT INTO ticket_notes (ticket_id, admin_id, admin_name, note_text)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`
	return pool.QueryRow(ctx, query,
		note.TicketID,
		note.AdminID,
		note.AdminName,
		note.NoteText,
	).Scan(&note.ID, &note.CreatedAt)
}

// GetTicketNotes retrieves all notes for a ticket
func (r *Repository) GetTicketNotes(ctx context.Context, ticketID int) ([]*TicketNote, error) {
	query := `SELECT * FROM ticket_notes WHERE ticket_id = $1 ORDER BY created_at ASC`
	rows, err := pool.Query(ctx, query, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []*TicketNote
	for rows.Next() {
		var note TicketNote
		err := rows.Scan(&note.ID, &note.TicketID, &note.AdminID, &note.AdminName, &note.NoteText, &note.CreatedAt)
		if err != nil {
			return nil, err
		}
		notes = append(notes, &note)
	}
	return notes, nil
}

// Statistics Operations

// GetTicketStats retrieves ticket statistics for a date range
func (r *Repository) GetTicketStats(ctx context.Context, startDate, endDate time.Time) (*TicketStats, error) {
	query := `
		SELECT 
			COUNT(*) FILTER (WHERE created_at >= $1 AND created_at < $2) as tickets_opened,
			COUNT(*) FILTER (WHERE closed_at >= $1 AND closed_at < $2) as tickets_closed
		FROM tickets
	`
	var stats TicketStats
	err := pool.QueryRow(ctx, query, startDate, endDate).Scan(&stats.TicketsOpened, &stats.TicketsClosed)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// User Settings Operations

// GetUserLanguage gets the language setting for a user
func (r *Repository) GetUserLanguage(ctx context.Context, userID int64) (string, error) {
	query := `SELECT language FROM user_settings WHERE user_id = $1`
	var lang string
	err := pool.QueryRow(ctx, query, userID).Scan(&lang)
	if err != nil {
		return LangEnglish, err // Default to English
	}
	return lang, nil
}

// SetUserLanguage sets the language setting for a user
func (r *Repository) SetUserLanguage(ctx context.Context, userID int64, language string) error {
	query := `
		INSERT INTO user_settings (user_id, language)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET language = $2, updated_at = NOW()
	`
	_, err := pool.Exec(ctx, query, userID, language)
	return err
}

// HasUserSettings checks if user has settings configured
func (r *Repository) HasUserSettings(ctx context.Context, userID int64) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM user_settings WHERE user_id = $1)`
	var exists bool
	err := pool.QueryRow(ctx, query, userID).Scan(&exists)
	return exists, err
}

// HasPassedCaptcha checks whether a user has completed the onboarding captcha.
func (r *Repository) HasPassedCaptcha(ctx context.Context, userID int64) (bool, error) {
	query := `SELECT captcha_passed FROM user_onboarding WHERE user_id = $1`
	var passed bool
	err := pool.QueryRow(ctx, query, userID).Scan(&passed)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return passed, nil
}

// MarkCaptchaPassed persists a successful onboarding captcha check.
func (r *Repository) MarkCaptchaPassed(ctx context.Context, userID int64) error {
	query := `
		INSERT INTO user_onboarding (user_id, captcha_passed, captcha_passed_at)
		VALUES ($1, TRUE, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			captcha_passed = TRUE,
			captcha_passed_at = NOW(),
			updated_at = NOW()
	`
	_, err := pool.Exec(ctx, query, userID)
	return err
}

// GetUserLastHWIDDeleteAt returns the timestamp of the last HWID removal action for a user.
func (r *Repository) GetUserLastHWIDDeleteAt(ctx context.Context, userID int64) (*time.Time, error) {
	query := `SELECT last_hwid_delete_at FROM user_settings WHERE user_id = $1`
	var value sql.NullTime
	err := pool.QueryRow(ctx, query, userID).Scan(&value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if !value.Valid {
		return nil, nil
	}

	ts := value.Time
	return &ts, nil
}

// SetUserLastHWIDDeleteAt persists the timestamp of the latest HWID removal action.
func (r *Repository) SetUserLastHWIDDeleteAt(ctx context.Context, userID int64, deletedAt time.Time) error {
	query := `
		INSERT INTO user_settings (user_id, language, last_hwid_delete_at)
		VALUES ($1, 'en', $2)
		ON CONFLICT (user_id) DO UPDATE SET
			last_hwid_delete_at = EXCLUDED.last_hwid_delete_at,
			updated_at = NOW()
	`
	_, err := pool.Exec(ctx, query, userID, deletedAt)
	return err
}

// Web Admin Operations

func (r *Repository) GetWebAdminUserByID(ctx context.Context, id int) (*WebAdminUser, error) {
	return r.scanWebAdminUser(ctx, `SELECT * FROM web_admin_users WHERE id = $1`, id)
}

func (r *Repository) GetWebAdminUserByUsername(ctx context.Context, username string) (*WebAdminUser, error) {
	return r.scanWebAdminUser(ctx, `SELECT * FROM web_admin_users WHERE username = $1`, username)
}

func (r *Repository) GetWebAdminUserByGitHubEmail(ctx context.Context, email string) (*WebAdminUser, error) {
	return r.scanWebAdminUser(ctx, `SELECT * FROM web_admin_users WHERE github_email = $1`, email)
}

func (r *Repository) GetWebAdminUserByGitHubLogin(ctx context.Context, login string) (*WebAdminUser, error) {
	return r.scanWebAdminUser(ctx, `SELECT * FROM web_admin_users WHERE github_login = $1`, login)
}

func (r *Repository) GetWebAdminUserByTelegramID(ctx context.Context, telegramID int64) (*WebAdminUser, error) {
	return r.scanWebAdminUser(ctx, `SELECT * FROM web_admin_users WHERE telegram_id = $1`, telegramID)
}

func (r *Repository) GetWebAdminUserByWebauthnUserID(ctx context.Context, webauthnUserID []byte) (*WebAdminUser, error) {
	return r.scanWebAdminUser(ctx, `SELECT * FROM web_admin_users WHERE webauthn_user_id = $1`, webauthnUserID)
}

func (r *Repository) UpsertWebAdminPasswordUser(ctx context.Context, username, displayName string, webauthnUserID []byte) (*WebAdminUser, error) {
	query := `
		INSERT INTO web_admin_users (username, display_name, webauthn_user_id, last_login_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (username) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			last_login_at = NOW(),
			updated_at = NOW()
		RETURNING *
	`
	return r.scanWebAdminUser(ctx, query, username, displayName, webauthnUserID)
}

func (r *Repository) CreateWebAdminGitHubUser(ctx context.Context, login, email, displayName string, webauthnUserID []byte) (*WebAdminUser, error) {
	query := `
		INSERT INTO web_admin_users (display_name, github_login, github_email, webauthn_user_id, last_login_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING *
	`
	return r.scanWebAdminUser(ctx, query, displayName, login, email, webauthnUserID)
}

func (r *Repository) UpdateWebAdminGitHubIdentity(ctx context.Context, id int, login, email, displayName string) (*WebAdminUser, error) {
	query := `
		UPDATE web_admin_users
		SET display_name = $2,
			github_login = $3,
			github_email = $4,
			last_login_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
		RETURNING *
	`
	return r.scanWebAdminUser(ctx, query, id, displayName, login, email)
}

func (r *Repository) UpsertWebAdminTelegramUser(ctx context.Context, telegramID int64, username *string, displayName string, webauthnUserID []byte) (*WebAdminUser, error) {
	query := `
		INSERT INTO web_admin_users (username, display_name, telegram_id, webauthn_user_id, last_login_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (telegram_id) DO UPDATE SET
			username = EXCLUDED.username,
			display_name = EXCLUDED.display_name,
			last_login_at = NOW(),
			updated_at = NOW()
		RETURNING *
	`
	return r.scanWebAdminUser(ctx, query, username, displayName, telegramID, webauthnUserID)
}

func (r *Repository) UpdateWebAdminLastLogin(ctx context.Context, id int) error {
	_, err := pool.Exec(ctx, `UPDATE web_admin_users SET last_login_at = NOW(), updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (r *Repository) CreateWebAdminPasskey(ctx context.Context, adminUserID int, name, credentialID string, credential []byte) (*WebAdminPasskey, error) {
	query := `
		INSERT INTO web_admin_passkeys (admin_user_id, name, credential_id, credential)
		VALUES ($1, $2, $3, $4)
		RETURNING id, admin_user_id, name, credential_id, credential, created_at, updated_at, last_used_at
	`
	return r.scanWebAdminPasskey(ctx, query, adminUserID, name, credentialID, credential)
}

func (r *Repository) DeleteWebAdminPasskey(ctx context.Context, adminUserID int, passkeyID int) error {
	_, err := pool.Exec(ctx, `DELETE FROM web_admin_passkeys WHERE id = $1 AND admin_user_id = $2`, passkeyID, adminUserID)
	return err
}

func (r *Repository) GetWebAdminPasskeysByUserID(ctx context.Context, adminUserID int) ([]*WebAdminPasskey, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, admin_user_id, name, credential_id, credential, created_at, updated_at, last_used_at
		FROM web_admin_passkeys
		WHERE admin_user_id = $1
		ORDER BY created_at ASC
	`, adminUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*WebAdminPasskey
	for rows.Next() {
		var passkey WebAdminPasskey
		if err := rows.Scan(
			&passkey.ID,
			&passkey.AdminUserID,
			&passkey.Name,
			&passkey.CredentialID,
			&passkey.Credential,
			&passkey.CreatedAt,
			&passkey.UpdatedAt,
			&passkey.LastUsedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, &passkey)
	}
	return result, nil
}

func (r *Repository) TouchWebAdminPasskey(ctx context.Context, credentialID string) error {
	_, err := pool.Exec(ctx, `
		UPDATE web_admin_passkeys
		SET last_used_at = NOW(), updated_at = NOW()
		WHERE credential_id = $1
	`, credentialID)
	return err
}

// Helper functions

func (r *Repository) scanTicket(ctx context.Context, query string, args ...interface{}) (*Ticket, error) {
	row := pool.QueryRow(ctx, query, args...)

	var t Ticket
	err := row.Scan(
		&t.ID, &t.TicketNumber, &t.UserID, &t.UserName, &t.UserUsername,
		&t.SupportChatID, &t.ThreadID, &t.Status, &t.Priority, &t.Subject,
		&t.CreatedAt, &t.UpdatedAt, &t.ClosedAt, &t.ClosedBy, &t.ClosedByName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (r *Repository) scanTickets(ctx context.Context, query string, args ...interface{}) ([]*Ticket, error) {
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []*Ticket
	for rows.Next() {
		var t Ticket
		err := rows.Scan(
			&t.ID, &t.TicketNumber, &t.UserID, &t.UserName, &t.UserUsername,
			&t.SupportChatID, &t.ThreadID, &t.Status, &t.Priority, &t.Subject,
			&t.CreatedAt, &t.UpdatedAt, &t.ClosedAt, &t.ClosedBy, &t.ClosedByName,
		)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, &t)
	}
	return tickets, nil
}

func (r *Repository) scanWebAdminUser(ctx context.Context, query string, args ...interface{}) (*WebAdminUser, error) {
	row := pool.QueryRow(ctx, query, args...)

	var user WebAdminUser
	err := row.Scan(
		&user.ID,
		&user.Username,
		&user.DisplayName,
		&user.GitHubLogin,
		&user.GitHubEmail,
		&user.TelegramID,
		&user.WebauthnUserID,
		&user.LastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *Repository) scanWebAdminPasskey(ctx context.Context, query string, args ...interface{}) (*WebAdminPasskey, error) {
	row := pool.QueryRow(ctx, query, args...)

	var passkey WebAdminPasskey
	err := row.Scan(
		&passkey.ID,
		&passkey.AdminUserID,
		&passkey.Name,
		&passkey.CredentialID,
		&passkey.Credential,
		&passkey.CreatedAt,
		&passkey.UpdatedAt,
		&passkey.LastUsedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &passkey, nil
}
