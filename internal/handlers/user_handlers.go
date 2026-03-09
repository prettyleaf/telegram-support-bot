package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"remnafamily/internal/config"
	"remnafamily/internal/database"
	"remnafamily/internal/keyboards"
	"remnafamily/internal/messages"
	"remnafamily/internal/types"
	"remnafamily/internal/utils"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// UserHandlers handles user-facing commands and messages
type UserHandlers struct {
	deps *types.Dependencies
}

// NewUserHandlers creates a new user handlers instance
func NewUserHandlers(deps *types.Dependencies) *UserHandlers {
	return &UserHandlers{deps: deps}
}

// HandleStart handles the /start command
func (h *UserHandlers) HandleStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	userID := update.Message.From.ID

	// Check if user is banned
	banned, err := h.deps.TicketService.IsUserBanned(ctx, userID)
	if err != nil {
		utils.LogError("check user ban status", err)
	}
	if banned {
		lang := h.getUserLanguage(ctx, userID)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    userID,
			Text:      messages.GetUserBannedMessageLang(lang),
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Clear any previous state
	h.deps.State.ClearUserState(userID)

	// Check if user has language preference set
	hasSettings, err := h.deps.DB.HasUserSettings(ctx, userID)
	if err != nil {
		utils.LogError("check user settings", err)
	}

	// If no language set, show language selection
	if !hasSettings {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      userID,
			Text:        messages.GetLanguageSelectionMessage(),
			ParseMode:   models.ParseModeHTML,
			ReplyMarkup: keyboards.LanguageSelectionKeyboard(),
		})
		return
	}

	lang := h.getUserLanguage(ctx, userID)

	// Check if user has an open ticket
	ticket, err := h.deps.TicketService.GetOpenTicketByUserID(ctx, userID)
	if err == nil && ticket != nil {
		// User has an open ticket - let them know
		var text string
		if lang == "ru" {
			text = fmt.Sprintf(`%s

📬 <b>У вас есть открытый тикет:</b> #%s

Просто отправьте сообщение, и наша команда поддержки его получит.`,
				messages.GetWelcomeMessageLang(lang), ticket.TicketNumber)
		} else {
			text = fmt.Sprintf(`%s

📬 <b>You have an open ticket:</b> #%s

Just send your message and our support team will receive it.`,
				messages.GetWelcomeMessageLang(lang), ticket.TicketNumber)
		}
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    userID,
			Text:      text,
			ParseMode: models.ParseModeHTML,
		})
		h.deps.State.SetUserState(userID, config.StateInTicket)
		h.deps.State.SetUserData(userID, "ticket_id", ticket.ID)
		return
	}

	// Send welcome message
	var keyboard *models.InlineKeyboardMarkup
	if lang == "ru" {
		keyboard = keyboards.NewTicketKeyboardRu()
	} else {
		keyboard = keyboards.NewTicketKeyboard()
	}
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      userID,
		Text:        messages.GetWelcomeMessageLang(lang),
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	})
}

// HandleLanguage handles the /language command
func (h *UserHandlers) HandleLanguage(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	userID := update.Message.From.ID

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      userID,
		Text:        messages.GetLanguageSelectionMessage(),
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboards.LanguageSelectionKeyboard(),
	})
}

// getUserLanguage gets user's language preference
func (h *UserHandlers) getUserLanguage(ctx context.Context, userID int64) string {
	lang, err := h.deps.DB.GetUserLanguage(ctx, userID)
	if err != nil {
		return "en"
	}
	return lang
}

// HandleNewTicket handles the /newticket command
func (h *UserHandlers) HandleNewTicket(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	userID := update.Message.From.ID
	user := update.Message.From
	lang := h.getUserLanguage(ctx, userID)

	// Check if user is banned
	banned, err := h.deps.TicketService.IsUserBanned(ctx, userID)
	if err != nil {
		utils.LogError("check user ban status", err)
	}
	if banned {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    userID,
			Text:      messages.GetUserBannedMessageLang(lang),
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Check open tickets limit
	openCount, _ := h.deps.DB.GetOpenTicketsCountByUserID(ctx, userID)
	if openCount >= config.YAMLCfg.Tickets.MaxOpenPerUser {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    userID,
			Text:      messages.GetMaxTicketsError(openCount, lang),
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Create the ticket
	userName := utils.GetUserNamePtr(user)
	userUsername := utils.GetUsernamePtr(user)

	ticket, err := h.deps.TicketService.CreateTicket(ctx, b, userID, userName, userUsername, "")
	if err != nil {
		utils.LogError("create ticket", err)
		errorText := messages.GetTicketCreateError(lang)
		if cooldownSeconds, ok := parseCooldownSeconds(err); ok {
			errorText = messages.GetTicketCooldownError(cooldownSeconds, lang)
		}
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    userID,
			Text:      errorText,
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	utils.LogTicket("New ticket #%s created by user %d", ticket.TicketNumber, userID)

	// Set state to in-ticket
	h.deps.State.SetUserState(userID, config.StateInTicket)
	h.deps.State.SetUserData(userID, "ticket_id", ticket.ID)

	// Notify user
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    userID,
		Text:      messages.GetTicketCreatedMessageLang(ticket.TicketNumber, lang),
		ParseMode: models.ParseModeHTML,
	})

	h.sendAutoTicketWelcome(ctx, b, ticket, lang)

	// Send notification to the thread
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          ticket.SupportChatID,
		MessageThreadID: int(ticket.ThreadID),
		Text: fmt.Sprintf("🆕 <b>New Support Ticket</b>\n\n"+
			"👤 <b>User:</b> %s\n"+
			"🆔 <b>ID:</b> <code>%d</code>\n"+
			"📅 <b>Created:</b> %s\n\n"+
			"<i>Reply to this thread to respond to the user.</i>",
			utils.EscapeHTML(ticket.GetDisplayName()),
			ticket.UserID,
			ticket.CreatedAt.Format("02 Jan 2006 15:04")),
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboards.TicketActionsKeyboard(ticket.ID, true),
	})
}

// HandleMyTickets handles the /mytickets command
func (h *UserHandlers) HandleMyTickets(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	userID := update.Message.From.ID
	lang := h.getUserLanguage(ctx, userID)

	tickets, err := h.deps.TicketService.GetTicketsByUserID(ctx, userID, 10)
	if err != nil {
		utils.LogError("get user tickets", err)
		var errorMsg string
		if lang == "ru" {
			errorMsg = "❌ Не удалось загрузить ваши тикеты. Пожалуйста, попробуйте позже."
		} else {
			errorMsg = "❌ Failed to load your tickets. Please try again later."
		}
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    userID,
			Text:      errorMsg,
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      userID,
		Text:        messages.FormatUserTicketListLang(tickets, lang),
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboards.UserTicketListKeyboard(tickets),
	})
}

// HandleCancel handles the /cancel command
func (h *UserHandlers) HandleCancel(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	userID := update.Message.From.ID
	lang := h.getUserLanguage(ctx, userID)
	h.deps.State.ClearUserState(userID)

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    userID,
		Text:      messages.GetOperationCancelled(lang),
		ParseMode: models.ParseModeHTML,
	})
}

// HandleUserMessage handles regular messages from users
func (h *UserHandlers) HandleUserMessage(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	message := update.Message
	userID := message.From.ID
	user := message.From
	lang := h.getUserLanguage(ctx, userID)

	// Check if user is banned
	banned, err := h.deps.TicketService.IsUserBanned(ctx, userID)
	if err != nil {
		utils.LogError("check user ban status", err)
	}
	if banned {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    userID,
			Text:      messages.GetUserBannedMessageLang(lang),
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	if config.YAMLCfg != nil && !h.deps.State.AllowUserMessage(userID, config.YAMLCfg.RateLimit.MaxMessagesPerMinute, time.Minute) {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    userID,
			Text:      messages.GetRateLimitError(lang),
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Get user state
	state := h.deps.State.GetUserState(userID)

	// If user is in an active ticket, forward the message
	if state == config.StateInTicket {
		ticketID := h.deps.State.GetUserData(userID, "ticket_id")
		if ticketID != nil {
			ticket, err := h.deps.TicketService.GetTicketByID(ctx, ticketID.(int))
			if err == nil && ticket != nil && ticket.IsOpen() {
				h.forwardUserMessage(ctx, b, ticket, message)
				return
			}
		}
	}

	// Check if user has an open ticket
	ticket, err := h.deps.TicketService.GetOpenTicketByUserID(ctx, userID)
	if err == nil && ticket != nil {
		// Forward to existing ticket
		h.deps.State.SetUserState(userID, config.StateInTicket)
		h.deps.State.SetUserData(userID, "ticket_id", ticket.ID)
		h.forwardUserMessage(ctx, b, ticket, message)
		return
	}

	// No open ticket - create one automatically
	userName := utils.GetUserNamePtr(user)
	userUsername := utils.GetUsernamePtr(user)

	// Use the first message as subject (truncated)
	subject := ""
	if message.Text != "" {
		subject = message.Text
		if len(subject) > 100 {
			subject = subject[:97] + "..."
		}
	}

	ticket, err = h.deps.TicketService.CreateTicket(ctx, b, userID, userName, userUsername, subject)
	if err != nil {
		utils.LogError("auto-create ticket", err)
		errorText := messages.GetTicketCreateError(lang)
		if cooldownSeconds, ok := parseCooldownSeconds(err); ok {
			errorText = messages.GetTicketCooldownError(cooldownSeconds, lang)
		}
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    userID,
			Text:      errorText,
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	utils.LogTicket("New ticket #%s auto-created for user %d", ticket.TicketNumber, userID)

	// Set state
	h.deps.State.SetUserState(userID, config.StateInTicket)
	h.deps.State.SetUserData(userID, "ticket_id", ticket.ID)

	// Notify user
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    userID,
		Text:      messages.GetTicketCreatedMessageLang(ticket.TicketNumber, lang),
		ParseMode: models.ParseModeHTML,
	})

	h.sendAutoTicketWelcome(ctx, b, ticket, lang)

	// Send notification to the thread
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          ticket.SupportChatID,
		MessageThreadID: int(ticket.ThreadID),
		Text: fmt.Sprintf("🆕 <b>New Support Ticket</b>\n\n"+
			"👤 <b>User:</b> %s\n"+
			"🆔 <b>ID:</b> <code>%d</code>\n"+
			"📅 <b>Created:</b> %s\n\n"+
			"<i>Reply to this thread to respond to the user.</i>",
			utils.EscapeHTML(ticket.GetDisplayName()),
			ticket.UserID,
			ticket.CreatedAt.Format("02 Jan 2006 15:04")),
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboards.TicketActionsKeyboard(ticket.ID, true),
	})

	// Forward the original message
	h.forwardUserMessage(ctx, b, ticket, message)
}

// forwardUserMessage forwards a user message to the ticket thread
func (h *UserHandlers) forwardUserMessage(ctx context.Context, b *bot.Bot, ticket *database.Ticket, message *models.Message) {
	lang := h.getUserLanguage(ctx, ticket.UserID)
	sentMsg, err := h.deps.TicketService.ForwardMessageToThread(ctx, b, ticket, message)
	if err != nil {
		utils.LogError("forward message to thread", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    ticket.UserID,
			Text:      messages.GetMessageSendError(lang),
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Save message to database
	msgText := message.Text
	if msgText == "" && message.Caption != "" {
		msgText = message.Caption
	}

	hasMedia := message.Photo != nil || message.Document != nil || message.Video != nil ||
		message.Voice != nil || message.Audio != nil || message.Sticker != nil || message.VideoNote != nil

	mediaType := ""
	if message.Photo != nil {
		mediaType = "photo"
	} else if message.Document != nil {
		mediaType = "document"
	} else if message.Video != nil {
		mediaType = "video"
	} else if message.Voice != nil {
		mediaType = "voice"
	} else if message.Audio != nil {
		mediaType = "audio"
	} else if message.Sticker != nil {
		mediaType = "sticker"
	} else if message.VideoNote != nil {
		mediaType = "video_note"
	}

	ticketMsg := &database.TicketMessage{
		TicketID:          ticket.ID,
		SenderID:          message.From.ID,
		SenderName:        utils.GetUserNamePtr(message.From),
		SenderUsername:    utils.GetUsernamePtr(message.From),
		IsAdmin:           false,
		MessageText:       &msgText,
		MessageID:         func() *int64 { id := int64(sentMsg.ID); return &id }(),
		OriginalMessageID: func() *int64 { id := int64(message.ID); return &id }(),
		HasMedia:          hasMedia,
		MediaType: func() *string {
			if mediaType == "" {
				return nil
			}
			return &mediaType
		}(),
	}

	if err := h.deps.TicketService.SaveMessage(ctx, ticketMsg); err != nil {
		utils.LogError("save ticket message", err)
	}
}

// HandleUserCallback handles callbacks from users
func (h *UserHandlers) HandleUserCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	callback := update.CallbackQuery
	data := callback.Data
	userID := callback.From.ID

	// Answer callback to remove loading state
	defer func() {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
		})
	}()

	parts := strings.Split(data, ":")

	switch parts[0] {
	case "lang":
		if len(parts) < 2 {
			return
		}
		lang := parts[1]
		if lang != "en" && lang != "ru" {
			return
		}

		// Save language preference
		if err := h.deps.DB.SetUserLanguage(ctx, userID, lang); err != nil {
			utils.LogError("set user language", err)
			return
		}

		// Delete the language selection message
		if callback.Message.Message != nil {
			_, _ = b.DeleteMessage(ctx, &bot.DeleteMessageParams{
				ChatID:    userID,
				MessageID: callback.Message.Message.ID,
			})
		}

		// Send confirmation and welcome message
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    userID,
			Text:      messages.GetLanguageChanged(lang),
			ParseMode: models.ParseModeHTML,
		})

		// Send welcome message with appropriate keyboard
		var keyboard *models.InlineKeyboardMarkup
		if lang == "ru" {
			keyboard = keyboards.NewTicketKeyboardRu()
		} else {
			keyboard = keyboards.NewTicketKeyboard()
		}
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      userID,
			Text:        messages.GetWelcomeMessageLang(lang),
			ParseMode:   models.ParseModeHTML,
			ReplyMarkup: keyboard,
		})

	case "ticket":
		if len(parts) < 2 {
			return
		}
		lang := h.getUserLanguage(ctx, userID)
		switch parts[1] {
		case "create":
			// Create new ticket
			userName := utils.GetUserNamePtr(&callback.From)
			userUsername := utils.GetUsernamePtr(&callback.From)

			ticket, err := h.deps.TicketService.CreateTicket(ctx, b, userID, userName, userUsername, "")
			if err != nil {
				if strings.Contains(err.Error(), "banned") {
					_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
						ChatID:    userID,
						Text:      messages.GetUserBannedMessageLang(lang),
						ParseMode: models.ParseModeHTML,
					})
					return
				}
				if strings.Contains(err.Error(), "limit") {
					_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
						ChatID:    userID,
						Text:      messages.GetMaxTicketsError(config.YAMLCfg.Tickets.MaxOpenPerUser, lang),
						ParseMode: models.ParseModeHTML,
					})
					return
				}
				if cooldownSeconds, ok := parseCooldownSeconds(err); ok {
					_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
						ChatID:    userID,
						Text:      messages.GetTicketCooldownError(cooldownSeconds, lang),
						ParseMode: models.ParseModeHTML,
					})
					return
				}
				utils.LogError("create ticket via callback", err)
				return
			}

			h.deps.State.SetUserState(userID, config.StateInTicket)
			h.deps.State.SetUserData(userID, "ticket_id", ticket.ID)

			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:    userID,
				Text:      messages.GetTicketCreatedMessageLang(ticket.TicketNumber, lang),
				ParseMode: models.ParseModeHTML,
			})

			h.sendAutoTicketWelcome(ctx, b, ticket, lang)

			// Notify in thread
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:          ticket.SupportChatID,
				MessageThreadID: int(ticket.ThreadID),
				Text: fmt.Sprintf("🆕 <b>New Support Ticket</b>\n\n"+
					"👤 <b>User:</b> %s\n"+
					"🆔 <b>ID:</b> <code>%d</code>",
					utils.EscapeHTML(ticket.GetDisplayName()), ticket.UserID),
				ParseMode:   models.ParseModeHTML,
				ReplyMarkup: keyboards.TicketActionsKeyboard(ticket.ID, true),
			})
		}

	case "myticket":
		if len(parts) < 3 {
			return
		}
		if parts[1] == "view" {
			ticketID, err := strconv.Atoi(parts[2])
			if err != nil {
				return
			}

			ticket, err := h.deps.TicketService.GetTicketByID(ctx, ticketID)
			if err != nil || ticket == nil {
				return
			}

			// Verify ownership
			if ticket.UserID != userID {
				return
			}

			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:    userID,
				Text:      messages.FormatTicketDetailsForUser(ticket),
				ParseMode: models.ParseModeHTML,
			})
		}

	case "cancel":
		lang := h.getUserLanguage(ctx, userID)
		h.deps.State.ClearUserState(userID)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    userID,
			Text:      messages.GetOperationCancelled(lang),
			ParseMode: models.ParseModeHTML,
		})
	}
}

func (h *UserHandlers) sendAutoTicketWelcome(ctx context.Context, b *bot.Bot, ticket *database.Ticket, lang string) {
	if ticket == nil {
		return
	}

	body := messages.GetTicketWelcomeMessageLang(ticket.TicketNumber, lang)
	sentMsg, err := h.deps.TicketService.SendSupportTextToUser(ctx, b, ticket, body)
	if err != nil {
		utils.LogError("send automated ticket welcome", err)
		return
	}

	senderName := "Support Bot"
	messageID := int64(sentMsg.ID)
	ticketMsg := &database.TicketMessage{
		TicketID:    ticket.ID,
		SenderID:    0,
		SenderName:  &senderName,
		IsAdmin:     true,
		MessageText: &body,
		MessageID:   &messageID,
	}

	if err := h.deps.TicketService.SaveMessage(ctx, ticketMsg); err != nil {
		utils.LogError("save automated ticket welcome", err)
	}
}

func parseCooldownSeconds(err error) (int, bool) {
	if err == nil {
		return 0, false
	}

	var seconds int
	if _, scanErr := fmt.Sscanf(err.Error(), "ticket creation cooldown active for %d seconds", &seconds); scanErr != nil {
		return 0, false
	}
	return seconds, true
}
