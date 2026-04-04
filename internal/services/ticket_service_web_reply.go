package services

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"remnafamily/internal/database"
	"remnafamily/internal/utils"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type SupportUpload struct {
	Filename    string
	ContentType string
	Data        []byte
}

// SendSupportHTMLReply sends a formatted web-panel reply to the user and mirrors
// it into the admin ticket thread.
func (s *TicketService) SendSupportHTMLReply(ctx context.Context, b *bot.Bot, ticket *database.Ticket, htmlBody, adminName string) (*models.Message, *models.Message, error) {
	userMsg, err := s.SendSupportTextToUser(ctx, b, ticket, htmlBody)
	if err != nil {
		return nil, nil, err
	}

	threadMsg, err := s.SendSupportTextToThread(ctx, b, ticket, htmlBody, adminName)
	if err != nil {
		return userMsg, nil, err
	}

	return userMsg, threadMsg, nil
}

// SendSupportTextToThread mirrors a formatted web-panel reply into the admin ticket thread.
func (s *TicketService) SendSupportTextToThread(ctx context.Context, b *bot.Bot, ticket *database.Ticket, htmlBody, adminName string) (*models.Message, error) {
	releaseLock, err := s.db.AcquireTicketThreadLock(ctx, ticket.ID)
	if err != nil {
		return nil, fmt.Errorf("acquire ticket thread lock: %w", err)
	}
	defer func() {
		if unlockErr := releaseLock(); unlockErr != nil {
			utils.LogError("release ticket thread lock", unlockErr)
		}
	}()

	if err := s.ensureTopicReady(ctx, b, ticket); err != nil {
		return nil, fmt.Errorf("failed to ensure topic exists: %w", err)
	}

	return b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          ticket.SupportChatID,
		MessageThreadID: int(ticket.ThreadID),
		Text:            buildSupportThreadBody(adminName, htmlBody),
		ParseMode:       models.ParseModeHTML,
	})
}

// SendSupportUploadReply sends an uploaded file from the web panel to the user
// and mirrors it into the admin ticket thread.
func (s *TicketService) SendSupportUploadReply(
	ctx context.Context,
	b *bot.Bot,
	ticket *database.Ticket,
	upload SupportUpload,
	htmlCaption string,
	adminName string,
) (*models.Message, *models.Message, string, error) {
	mediaType, sendKind := detectSupportUploadKind(upload)

	userMsg, err := sendSupportUploadToUser(ctx, b, ticket, upload, htmlCaption, mediaType, sendKind)
	if err != nil {
		return nil, nil, mediaType, err
	}

	threadMsg, err := s.sendSupportUploadToThread(ctx, b, ticket, upload, htmlCaption, adminName, mediaType, sendKind)
	if err != nil {
		return userMsg, nil, mediaType, err
	}

	return userMsg, threadMsg, mediaType, nil
}

type supportUploadSendKind string

const (
	supportUploadPhoto    supportUploadSendKind = "photo"
	supportUploadVideo    supportUploadSendKind = "video"
	supportUploadDocument supportUploadSendKind = "document"
)

func detectSupportUploadKind(upload SupportUpload) (string, supportUploadSendKind) {
	contentType := strings.ToLower(strings.TrimSpace(upload.ContentType))
	ext := strings.ToLower(filepath.Ext(upload.Filename))

	switch {
	case strings.HasPrefix(contentType, "image/") && ext != ".gif" && ext != ".svg":
		return "photo", supportUploadPhoto
	case strings.HasPrefix(contentType, "video/"):
		return "video", supportUploadVideo
	default:
		return "document", supportUploadDocument
	}
}

func sendSupportUploadToUser(
	ctx context.Context,
	b *bot.Bot,
	ticket *database.Ticket,
	upload SupportUpload,
	htmlCaption string,
	mediaType string,
	sendKind supportUploadSendKind,
) (*models.Message, error) {
	caption := buildSupportUserBody(ticket, htmlCaption)
	input := &models.InputFileUpload{
		Filename: safeUploadFilename(upload.Filename, mediaType),
		Data:     bytes.NewReader(upload.Data),
	}

	switch sendKind {
	case supportUploadPhoto:
		return b.SendPhoto(ctx, &bot.SendPhotoParams{
			ChatID:    ticket.UserID,
			Photo:     input,
			Caption:   caption,
			ParseMode: models.ParseModeHTML,
		})
	case supportUploadVideo:
		return b.SendVideo(ctx, &bot.SendVideoParams{
			ChatID:    ticket.UserID,
			Video:     input,
			Caption:   caption,
			ParseMode: models.ParseModeHTML,
		})
	default:
		return b.SendDocument(ctx, &bot.SendDocumentParams{
			ChatID:    ticket.UserID,
			Document:  input,
			Caption:   caption,
			ParseMode: models.ParseModeHTML,
		})
	}
}

func (s *TicketService) sendSupportUploadToThread(
	ctx context.Context,
	b *bot.Bot,
	ticket *database.Ticket,
	upload SupportUpload,
	htmlCaption string,
	adminName string,
	mediaType string,
	sendKind supportUploadSendKind,
) (*models.Message, error) {
	releaseLock, err := s.db.AcquireTicketThreadLock(ctx, ticket.ID)
	if err != nil {
		return nil, fmt.Errorf("acquire ticket thread lock: %w", err)
	}
	defer func() {
		if unlockErr := releaseLock(); unlockErr != nil {
			utils.LogError("release ticket thread lock", unlockErr)
		}
	}()

	if err := s.ensureTopicReady(ctx, b, ticket); err != nil {
		return nil, fmt.Errorf("failed to ensure topic exists: %w", err)
	}

	caption := buildSupportThreadBody(adminName, htmlCaption)
	input := &models.InputFileUpload{
		Filename: safeUploadFilename(upload.Filename, mediaType),
		Data:     bytes.NewReader(upload.Data),
	}

	switch sendKind {
	case supportUploadPhoto:
		return b.SendPhoto(ctx, &bot.SendPhotoParams{
			ChatID:          ticket.SupportChatID,
			MessageThreadID: int(ticket.ThreadID),
			Photo:           input,
			Caption:         caption,
			ParseMode:       models.ParseModeHTML,
		})
	case supportUploadVideo:
		return b.SendVideo(ctx, &bot.SendVideoParams{
			ChatID:          ticket.SupportChatID,
			MessageThreadID: int(ticket.ThreadID),
			Video:           input,
			Caption:         caption,
			ParseMode:       models.ParseModeHTML,
		})
	default:
		return b.SendDocument(ctx, &bot.SendDocumentParams{
			ChatID:          ticket.SupportChatID,
			MessageThreadID: int(ticket.ThreadID),
			Document:        input,
			Caption:         caption,
			ParseMode:       models.ParseModeHTML,
		})
	}
}

func buildSupportUserBody(_ *database.Ticket, htmlBody string) string {
	return htmlBody
}

func buildSupportThreadBody(adminName string, htmlBody string) string {
	adminName = strings.TrimSpace(adminName)
	if adminName == "" {
		adminName = "Web Admin"
	}

	prefix := fmt.Sprintf("🖥 <b>Reply from %s</b>", utils.EscapeHTML(adminName))
	if strings.TrimSpace(htmlBody) == "" {
		return prefix
	}
	return prefix + "\n\n" + htmlBody
}

func safeUploadFilename(filename, mediaType string) string {
	filename = strings.TrimSpace(filename)
	if filename != "" {
		return filename
	}

	switch mediaType {
	case "photo":
		return "attachment.png"
	case "video":
		return "attachment.mp4"
	default:
		return "attachment.bin"
	}
}
