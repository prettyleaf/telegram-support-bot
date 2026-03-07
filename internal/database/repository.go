package database

import (
	"context"
	"fmt"
	"time"

	"remnafamily/internal/config"
	"remnafamily/internal/utils"
)

// Repository handles all database operations
type Repository struct{}

// NewRepository creates a new database repository
func NewRepository() *Repository {
	return &Repository{}
}

// Ticket Operations

// CreateTicket creates a new support ticket
func (r *Repository) CreateTicket(ctx context.Context, ticket *Ticket) error {
	// Sanitize string fields to ensure valid UTF-8
	ticket.UserName = utils.SanitizeUTF8Ptr(ticket.UserName)
	ticket.UserUsername = utils.SanitizeUTF8Ptr(ticket.UserUsername)
	ticket.Subject = utils.SanitizeUTF8Ptr(ticket.Subject)

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
	query := `SELECT * FROM tickets WHERE user_id = $1 AND status = 'open' ORDER BY created_at DESC LIMIT 1`
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

// GetNextTicketNumber generates the next ticket number
func (r *Repository) GetNextTicketNumber(ctx context.Context) (string, error) {
	var maxID int
	query := `SELECT COALESCE(MAX(id), 0) + 1 FROM tickets`
	err := pool.QueryRow(ctx, query).Scan(&maxID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", maxID), nil
}

// Ticket Message Operations

// CreateTicketMessage creates a new message in a ticket
func (r *Repository) CreateTicketMessage(ctx context.Context, msg *TicketMessage) error {
	// Sanitize string fields to ensure valid UTF-8
	msg.SenderName = utils.SanitizeUTF8Ptr(msg.SenderName)
	msg.SenderUsername = utils.SanitizeUTF8Ptr(msg.SenderUsername)
	msg.MessageText = utils.SanitizeUTF8Ptr(msg.MessageText)

	query := `
		INSERT INTO ticket_messages (ticket_id, sender_id, sender_name, sender_username, is_admin, message_text, message_id, original_message_id, has_media, media_type)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
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
			&msg.HasMedia, &msg.MediaType, &msg.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		messages = append(messages, &msg)
	}
	return messages, nil
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
