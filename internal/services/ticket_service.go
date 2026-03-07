package services

import (
	"context"
	"fmt"
	"strings"

	"remnafamily/internal/config"
	"remnafamily/internal/database"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// TicketService handles ticket-related business logic
type TicketService struct {
	db *database.Repository
}

// NewTicketService creates a new ticket service
func NewTicketService(db *database.Repository) *TicketService {
	return &TicketService{db: db}
}

// CreateTicket creates a new support ticket with a forum topic
func (s *TicketService) CreateTicket(ctx context.Context, b *bot.Bot, userID int64, userName, userUsername *string, subject string) (*database.Ticket, error) {
	// Check if user is banned
	banned, err := s.db.IsUserBanned(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("check user ban status: %w", err)
	}
	if banned {
		return nil, fmt.Errorf("user is banned from creating tickets")
	}

	// Check open tickets limit
	openCount, err := s.db.GetOpenTicketsCountByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get open tickets count: %w", err)
	}
	if openCount >= config.YAMLCfg.Tickets.MaxOpenPerUser {
		return nil, fmt.Errorf("maximum open tickets limit reached (%d)", config.YAMLCfg.Tickets.MaxOpenPerUser)
	}

	// Generate ticket number
	ticketNumber, err := s.db.GetNextTicketNumber(ctx)
	if err != nil {
		return nil, fmt.Errorf("generate ticket number: %w", err)
	}

	// Create forum topic name
	displayName := "User"
	if userUsername != nil && *userUsername != "" {
		displayName = "@" + *userUsername
	} else if userName != nil && *userName != "" {
		displayName = *userName
	}
	
	topicName := strings.ReplaceAll(config.YAMLCfg.Thread.NameFormat, "{ticket_number}", ticketNumber)
	topicName = strings.ReplaceAll(topicName, "{user_name}", displayName)
	
	// Truncate topic name if too long (Telegram limit is 128 characters)
	if len(topicName) > 128 {
		topicName = topicName[:125] + "..."
	}

	// Create forum topic in support chat
	topic, err := b.CreateForumTopic(ctx, &bot.CreateForumTopicParams{
		ChatID: config.Cfg.SupportChatID,
		Name:   topicName,
		IconColor: config.YAMLCfg.Thread.IconColor,
	})
	if err != nil {
		return nil, fmt.Errorf("create forum topic: %w", err)
	}

	// Create ticket in database
	subjectPtr := &subject
	if subject == "" {
		subjectPtr = nil
	}

	ticket := &database.Ticket{
		TicketNumber:  ticketNumber,
		UserID:        userID,
		UserName:      userName,
		UserUsername:  userUsername,
		SupportChatID: config.Cfg.SupportChatID,
		ThreadID:      int64(topic.MessageThreadID),
		Status:        config.TicketStatusOpen,
		Priority:      config.YAMLCfg.Tickets.DefaultPriority,
		Subject:       subjectPtr,
	}

	if err := s.db.CreateTicket(ctx, ticket); err != nil {
		// Try to close the topic if ticket creation fails
		_, _ = b.CloseForumTopic(ctx, &bot.CloseForumTopicParams{
			ChatID:          config.Cfg.SupportChatID,
			MessageThreadID: topic.MessageThreadID,
		})
		return nil, fmt.Errorf("create ticket in database: %w", err)
	}

	return ticket, nil
}

// CloseTicket closes a ticket and the associated forum topic
func (s *TicketService) CloseTicket(ctx context.Context, b *bot.Bot, ticketID int, closedBy int64, closedByName string) error {
	ticket, err := s.db.GetTicketByID(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("get ticket: %w", err)
	}

	// Close the ticket in database
	if err := s.db.CloseTicket(ctx, ticketID, closedBy, closedByName); err != nil {
		return fmt.Errorf("close ticket in database: %w", err)
	}

	// Close the forum topic (if it still exists)
	_, err = b.CloseForumTopic(ctx, &bot.CloseForumTopicParams{
		ChatID:          ticket.SupportChatID,
		MessageThreadID: int(ticket.ThreadID),
	})
	if err != nil {
		// Check if topic was deleted - this is not an error condition
		errStr := err.Error()
		if strings.Contains(errStr, "TOPIC_ID_INVALID") ||
			strings.Contains(errStr, "TOPIC_DELETED") ||
			strings.Contains(errStr, "TOPIC_CLOSED") ||
			strings.Contains(errStr, "message thread not found") {
			// Topic doesn't exist or is already closed - that's fine
			return nil
		}
		// Log error but don't fail - the ticket is already closed in DB
		return fmt.Errorf("close forum topic: %w", err)
	}

	return nil
}

// ReopenTicket reopens a closed ticket and the associated forum topic
func (s *TicketService) ReopenTicket(ctx context.Context, b *bot.Bot, ticketID int) error {
	ticket, err := s.db.GetTicketByID(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("get ticket: %w", err)
	}

	// Reopen the ticket in database
	if err := s.db.ReopenTicket(ctx, ticketID); err != nil {
		return fmt.Errorf("reopen ticket in database: %w", err)
	}

	// Try to reopen the forum topic
	_, err = b.ReopenForumTopic(ctx, &bot.ReopenForumTopicParams{
		ChatID:          ticket.SupportChatID,
		MessageThreadID: int(ticket.ThreadID),
	})
	if err != nil {
		// Check if topic was deleted - try to recreate it
		errStr := err.Error()
		if strings.Contains(errStr, "TOPIC_ID_INVALID") ||
			strings.Contains(errStr, "TOPIC_DELETED") ||
			strings.Contains(errStr, "message thread not found") {
			// Topic doesn't exist - recreate it
			if recreateErr := s.RecreateTopicForTicket(ctx, b, ticket); recreateErr != nil {
				return fmt.Errorf("recreate topic: %w", recreateErr)
			}
			return nil
		}
		return fmt.Errorf("reopen forum topic: %w", err)
	}

	return nil
}

// GetTicketByThreadID gets a ticket by its thread ID
func (s *TicketService) GetTicketByThreadID(ctx context.Context, threadID int64) (*database.Ticket, error) {
	return s.db.GetTicketByThreadID(ctx, threadID)
}

// GetOpenTicketByUserID gets the most recent open ticket for a user
func (s *TicketService) GetOpenTicketByUserID(ctx context.Context, userID int64) (*database.Ticket, error) {
	return s.db.GetOpenTicketByUserID(ctx, userID)
}

// GetTicketsByUserID gets all tickets for a user
func (s *TicketService) GetTicketsByUserID(ctx context.Context, userID int64, limit int) ([]*database.Ticket, error) {
	return s.db.GetTicketsByUserID(ctx, userID, limit)
}

// GetAllOpenTickets gets all open tickets
func (s *TicketService) GetAllOpenTickets(ctx context.Context) ([]*database.Ticket, error) {
	return s.db.GetAllOpenTickets(ctx)
}

// GetClosedTickets gets closed tickets with pagination
func (s *TicketService) GetClosedTickets(ctx context.Context, limit, offset int) ([]*database.Ticket, error) {
	return s.db.GetClosedTickets(ctx, limit, offset)
}

// GetAllTickets gets all tickets with pagination
func (s *TicketService) GetAllTickets(ctx context.Context, limit, offset int) ([]*database.Ticket, error) {
	return s.db.GetAllTickets(ctx, limit, offset)
}

// SaveMessage saves a message to the ticket history
func (s *TicketService) SaveMessage(ctx context.Context, msg *database.TicketMessage) error {
	return s.db.CreateTicketMessage(ctx, msg)
}

// RecreateTopicForTicket recreates a deleted forum topic for a ticket
func (s *TicketService) RecreateTopicForTicket(ctx context.Context, b *bot.Bot, ticket *database.Ticket) error {
	// Create forum topic name
	displayName := ticket.GetDisplayName()

	topicName := strings.ReplaceAll(config.YAMLCfg.Thread.NameFormat, "{ticket_number}", ticket.TicketNumber)
	topicName = strings.ReplaceAll(topicName, "{user_name}", displayName)

	// Truncate topic name if too long (Telegram limit is 128 characters)
	if len(topicName) > 128 {
		topicName = topicName[:125] + "..."
	}

	// Create forum topic in support chat
	topic, err := b.CreateForumTopic(ctx, &bot.CreateForumTopicParams{
		ChatID:    config.Cfg.SupportChatID,
		Name:      topicName,
		IconColor: config.YAMLCfg.Thread.IconColor,
	})
	if err != nil {
		return fmt.Errorf("create forum topic: %w", err)
	}

	// Update ticket in database with new thread ID
	if err := s.db.UpdateTicketThreadID(ctx, ticket.ID, int64(topic.MessageThreadID)); err != nil {
		return fmt.Errorf("update ticket thread ID: %w", err)
	}

	// Update the ticket object
	ticket.ThreadID = int64(topic.MessageThreadID)

	// Send notification to the new thread
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          ticket.SupportChatID,
		MessageThreadID: int(ticket.ThreadID),
		Text: fmt.Sprintf("🔄 <b>Topic Recreated</b>\n\n"+
			"This topic was recreated because the original was deleted.\n\n"+
			"👤 <b>User:</b> %s\n"+
			"🆔 <b>ID:</b> <code>%d</code>\n"+
			"📅 <b>Created:</b> %s",
			ticket.GetDisplayName(),
			ticket.UserID,
			ticket.CreatedAt.Format("02 Jan 2006 15:04")),
		ParseMode: models.ParseModeHTML,
	})

	return nil
}

// isTopicDeletedError checks if the error indicates the topic was deleted
func isTopicDeletedError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "topic_deleted") ||
		strings.Contains(errStr, "topic deleted") ||
		strings.Contains(errStr, "message_thread_not_found") ||
		strings.Contains(errStr, "thread not found") ||
		strings.Contains(errStr, "topic_closed") ||
		strings.Contains(errStr, "invalid_message_thread_id") ||
		strings.Contains(errStr, "bad request: message thread not found")
}

// checkAndRecreateTopicIfNeeded checks if the topic exists and recreates it if needed
func (s *TicketService) checkAndRecreateTopicIfNeeded(ctx context.Context, b *bot.Bot, ticket *database.Ticket) error {
	// If thread ID is 0 or 1 (general topic), we need to recreate
	if ticket.ThreadID <= 1 {
		return s.RecreateTopicForTicket(ctx, b, ticket)
	}
	return nil
}

// ForwardMessageToThread forwards a user message to the ticket thread
func (s *TicketService) ForwardMessageToThread(ctx context.Context, b *bot.Bot, ticket *database.Ticket, message *models.Message) (*models.Message, error) {
	// First check if topic needs recreation (thread ID is 0 or 1)
	if err := s.checkAndRecreateTopicIfNeeded(ctx, b, ticket); err != nil {
		return nil, fmt.Errorf("failed to ensure topic exists: %w", err)
	}

	// Build message text with user info header
	displayName := ticket.GetDisplayName()
	header := fmt.Sprintf("📩 <b>From:</b> %s (ID: <code>%d</code>)\n\n", displayName, ticket.UserID)

	var sentMsg *models.Message
	var err error

	// Handle different message types
	if message.Text != "" {
		sentMsg, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          ticket.SupportChatID,
			MessageThreadID: int(ticket.ThreadID),
			Text:            header + message.Text,
			ParseMode:       models.ParseModeHTML,
		})
	} else if message.Photo != nil && len(message.Photo) > 0 {
		// Get the largest photo
		photo := message.Photo[len(message.Photo)-1]
		caption := header
		if message.Caption != "" {
			caption += message.Caption
		}
		sentMsg, err = b.SendPhoto(ctx, &bot.SendPhotoParams{
			ChatID:          ticket.SupportChatID,
			MessageThreadID: int(ticket.ThreadID),
			Photo:           &models.InputFileString{Data: photo.FileID},
			Caption:         caption,
			ParseMode:       models.ParseModeHTML,
		})
	} else if message.Document != nil {
		caption := header
		if message.Caption != "" {
			caption += message.Caption
		}
		sentMsg, err = b.SendDocument(ctx, &bot.SendDocumentParams{
			ChatID:          ticket.SupportChatID,
			MessageThreadID: int(ticket.ThreadID),
			Document:        &models.InputFileString{Data: message.Document.FileID},
			Caption:         caption,
			ParseMode:       models.ParseModeHTML,
		})
	} else if message.Video != nil {
		caption := header
		if message.Caption != "" {
			caption += message.Caption
		}
		sentMsg, err = b.SendVideo(ctx, &bot.SendVideoParams{
			ChatID:          ticket.SupportChatID,
			MessageThreadID: int(ticket.ThreadID),
			Video:           &models.InputFileString{Data: message.Video.FileID},
			Caption:         caption,
			ParseMode:       models.ParseModeHTML,
		})
	} else if message.Voice != nil {
		sentMsg, err = b.SendVoice(ctx, &bot.SendVoiceParams{
			ChatID:          ticket.SupportChatID,
			MessageThreadID: int(ticket.ThreadID),
			Voice:           &models.InputFileString{Data: message.Voice.FileID},
			Caption:         header,
			ParseMode:       models.ParseModeHTML,
		})
	} else if message.Audio != nil {
		caption := header
		if message.Caption != "" {
			caption += message.Caption
		}
		sentMsg, err = b.SendAudio(ctx, &bot.SendAudioParams{
			ChatID:          ticket.SupportChatID,
			MessageThreadID: int(ticket.ThreadID),
			Audio:           &models.InputFileString{Data: message.Audio.FileID},
			Caption:         caption,
			ParseMode:       models.ParseModeHTML,
		})
	} else if message.Sticker != nil {
		// Send header first, then sticker
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          ticket.SupportChatID,
			MessageThreadID: int(ticket.ThreadID),
			Text:            header + "📎 Sticker:",
			ParseMode:       models.ParseModeHTML,
		})
		sentMsg, err = b.SendSticker(ctx, &bot.SendStickerParams{
			ChatID:          ticket.SupportChatID,
			MessageThreadID: int(ticket.ThreadID),
			Sticker:         &models.InputFileString{Data: message.Sticker.FileID},
		})
	} else if message.VideoNote != nil {
		sentMsg, err = b.SendVideoNote(ctx, &bot.SendVideoNoteParams{
			ChatID:          ticket.SupportChatID,
			MessageThreadID: int(ticket.ThreadID),
			VideoNote:       &models.InputFileString{Data: message.VideoNote.FileID},
		})
	} else {
		// Unknown message type - just forward text info
		sentMsg, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          ticket.SupportChatID,
			MessageThreadID: int(ticket.ThreadID),
			Text:            header + "<i>📎 Unsupported message type</i>",
			ParseMode:       models.ParseModeHTML,
		})
	}

	// If topic was deleted, recreate it and retry
	if isTopicDeletedError(err) {
		if recreateErr := s.RecreateTopicForTicket(ctx, b, ticket); recreateErr != nil {
			return nil, fmt.Errorf("topic deleted and failed to recreate: %w", recreateErr)
		}
		// Retry sending the message with the new thread ID
		return s.ForwardMessageToThread(ctx, b, ticket, message)
	}

	// Check if the message was sent to the wrong thread (e.g., general topic)
	// This can happen if Telegram silently redirects to general topic
	if err == nil && sentMsg != nil && sentMsg.MessageThreadID != int(ticket.ThreadID) {
		// Message went to wrong thread, recreate topic and retry
		if recreateErr := s.RecreateTopicForTicket(ctx, b, ticket); recreateErr != nil {
			return nil, fmt.Errorf("message sent to wrong thread, failed to recreate topic: %w", recreateErr)
		}
		// Retry sending the message with the new thread ID
		return s.ForwardMessageToThread(ctx, b, ticket, message)
	}

	return sentMsg, err
}

// ForwardReplyToUser forwards an admin reply to the user
func (s *TicketService) ForwardReplyToUser(ctx context.Context, b *bot.Bot, ticket *database.Ticket, message *models.Message, adminName string) (*models.Message, error) {
	header := fmt.Sprintf("💬 <b>Support Reply</b> (Ticket #%s)\n\n", ticket.TicketNumber)
	
	var sentMsg *models.Message
	var err error

	// Handle different message types
	if message.Text != "" {
		sentMsg, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    ticket.UserID,
			Text:      header + message.Text,
			ParseMode: models.ParseModeHTML,
		})
	} else if message.Photo != nil && len(message.Photo) > 0 {
		photo := message.Photo[len(message.Photo)-1]
		caption := header
		if message.Caption != "" {
			caption += message.Caption
		}
		sentMsg, err = b.SendPhoto(ctx, &bot.SendPhotoParams{
			ChatID:    ticket.UserID,
			Photo:     &models.InputFileString{Data: photo.FileID},
			Caption:   caption,
			ParseMode: models.ParseModeHTML,
		})
	} else if message.Document != nil {
		caption := header
		if message.Caption != "" {
			caption += message.Caption
		}
		sentMsg, err = b.SendDocument(ctx, &bot.SendDocumentParams{
			ChatID:    ticket.UserID,
			Document:  &models.InputFileString{Data: message.Document.FileID},
			Caption:   caption,
			ParseMode: models.ParseModeHTML,
		})
	} else if message.Video != nil {
		caption := header
		if message.Caption != "" {
			caption += message.Caption
		}
		sentMsg, err = b.SendVideo(ctx, &bot.SendVideoParams{
			ChatID:    ticket.UserID,
			Video:     &models.InputFileString{Data: message.Video.FileID},
			Caption:   caption,
			ParseMode: models.ParseModeHTML,
		})
	} else if message.Voice != nil {
		sentMsg, err = b.SendVoice(ctx, &bot.SendVoiceParams{
			ChatID:    ticket.UserID,
			Voice:     &models.InputFileString{Data: message.Voice.FileID},
			Caption:   header,
			ParseMode: models.ParseModeHTML,
		})
	} else if message.Audio != nil {
		caption := header
		if message.Caption != "" {
			caption += message.Caption
		}
		sentMsg, err = b.SendAudio(ctx, &bot.SendAudioParams{
			ChatID:    ticket.UserID,
			Audio:     &models.InputFileString{Data: message.Audio.FileID},
			Caption:   caption,
			ParseMode: models.ParseModeHTML,
		})
	} else if message.Sticker != nil {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    ticket.UserID,
			Text:      header,
			ParseMode: models.ParseModeHTML,
		})
		sentMsg, err = b.SendSticker(ctx, &bot.SendStickerParams{
			ChatID:  ticket.UserID,
			Sticker: &models.InputFileString{Data: message.Sticker.FileID},
		})
	} else if message.VideoNote != nil {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    ticket.UserID,
			Text:      header,
			ParseMode: models.ParseModeHTML,
		})
		sentMsg, err = b.SendVideoNote(ctx, &bot.SendVideoNoteParams{
			ChatID:    ticket.UserID,
			VideoNote: &models.InputFileString{Data: message.VideoNote.FileID},
		})
	} else {
		sentMsg, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    ticket.UserID,
			Text:      header + "<i>📎 Unsupported message type</i>",
			ParseMode: models.ParseModeHTML,
		})
	}

	return sentMsg, err
}

// BanUser bans a user from creating tickets
func (s *TicketService) BanUser(ctx context.Context, userID int64, userName, userUsername *string, bannedBy int64, bannedByName, reason string) error {
	ban := &database.BannedUser{
		UserID:       userID,
		UserName:     userName,
		UserUsername: userUsername,
		BannedBy:     bannedBy,
		BannedByName: &bannedByName,
		Reason:       &reason,
	}
	return s.db.BanUser(ctx, ban)
}

// UnbanUser unbans a user
func (s *TicketService) UnbanUser(ctx context.Context, userID int64) error {
	return s.db.UnbanUser(ctx, userID)
}

// IsUserBanned checks if a user is banned
func (s *TicketService) IsUserBanned(ctx context.Context, userID int64) (bool, error) {
	return s.db.IsUserBanned(ctx, userID)
}

// GetCannedResponse gets a canned response by shortcut
func (s *TicketService) GetCannedResponse(ctx context.Context, shortcut string) (*database.CannedResponse, error) {
	return s.db.GetCannedResponse(ctx, shortcut)
}

// GetAllCannedResponses gets all canned responses
func (s *TicketService) GetAllCannedResponses(ctx context.Context) ([]*database.CannedResponse, error) {
	return s.db.GetAllCannedResponses(ctx)
}

// AddNote adds an internal note to a ticket
func (s *TicketService) AddNote(ctx context.Context, ticketID int, adminID int64, adminName, noteText string) error {
	note := &database.TicketNote{
		TicketID:  ticketID,
		AdminID:   adminID,
		AdminName: &adminName,
		NoteText:  noteText,
	}
	return s.db.CreateTicketNote(ctx, note)
}

// GetTicketNotes gets all notes for a ticket
func (s *TicketService) GetTicketNotes(ctx context.Context, ticketID int) ([]*database.TicketNote, error) {
	return s.db.GetTicketNotes(ctx, ticketID)
}

// UpdateTicketPriority updates the priority of a ticket
func (s *TicketService) UpdateTicketPriority(ctx context.Context, ticketID int, priority string) error {
	return s.db.UpdateTicketPriority(ctx, ticketID, priority)
}

// GetTicketByID gets a ticket by ID
func (s *TicketService) GetTicketByID(ctx context.Context, ticketID int) (*database.Ticket, error) {
	return s.db.GetTicketByID(ctx, ticketID)
}

// GetTicketByNumber gets a ticket by number
func (s *TicketService) GetTicketByNumber(ctx context.Context, ticketNumber string) (*database.Ticket, error) {
	return s.db.GetTicketByNumber(ctx, ticketNumber)
}

// GetAllBannedUsers gets all banned users
func (s *TicketService) GetAllBannedUsers(ctx context.Context) ([]*database.BannedUser, error) {
	return s.db.GetAllBannedUsers(ctx)
}
