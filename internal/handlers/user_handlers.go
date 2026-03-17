package handlers

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"remnafamily/internal/config"
	"remnafamily/internal/database"
	"remnafamily/internal/keyboards"
	"remnafamily/internal/messages"
	"remnafamily/internal/services"
	"remnafamily/internal/types"
	"remnafamily/internal/utils"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// UserHandlers handles user-facing commands and messages
type UserHandlers struct {
	deps *types.Dependencies
}

type captchaSessionState struct {
	SessionID    string
	CorrectIndex int
	ExpiresAt    time.Time
	MessageID    int
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

	if !h.ensureUserPassedCaptcha(ctx, b, userID) {
		return
	}

	// Clear any previous state
	h.deps.State.ClearUserState(userID)

	h.showStartFlow(ctx, b, userID)
}

// HandleLanguage handles the /language command
func (h *UserHandlers) HandleLanguage(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	userID := update.Message.From.ID
	if !h.ensureUserPassedCaptcha(ctx, b, userID) {
		return
	}

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

func (h *UserHandlers) showStartFlow(ctx context.Context, b *bot.Bot, userID int64) {
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

func (h *UserHandlers) ensureUserPassedCaptcha(ctx context.Context, b *bot.Bot, userID int64) bool {
	if !h.isCaptchaEnabled() {
		return true
	}

	passed, err := h.deps.DB.HasPassedCaptcha(ctx, userID)
	if err != nil {
		utils.LogError("check user captcha status", err)
	}
	if passed {
		return true
	}

	h.sendCaptchaChallenge(ctx, b, userID, 0)
	return false
}

func (h *UserHandlers) isCaptchaEnabled() bool {
	return h.deps.CaptchaService != nil && config.YAMLCfg != nil && config.YAMLCfg.Captcha.Enabled
}

func (h *UserHandlers) sendCaptchaChallenge(ctx context.Context, b *bot.Bot, userID int64, replaceMessageID int) bool {
	if !h.isCaptchaEnabled() {
		return true
	}

	challenge, err := h.deps.CaptchaService.GenerateChallenge(config.YAMLCfg.Captcha)
	if err != nil {
		utils.LogError("generate captcha challenge", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    userID,
			Text:      messages.GetCaptchaErrorMessage(),
			ParseMode: models.ParseModeHTML,
		})
		return false
	}

	text := messages.GetCaptchaPromptMessage(challenge.Question, config.YAMLCfg.Captcha.TimeoutSeconds)
	keyboard := keyboards.CaptchaKeyboard(challenge.SessionID, challenge.Options)
	messageID := replaceMessageID

	if replaceMessageID > 0 {
		if _, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      userID,
			MessageID:   replaceMessageID,
			Text:        text,
			ParseMode:   models.ParseModeHTML,
			ReplyMarkup: keyboard,
		}); err != nil {
			utils.LogError("edit captcha message", err)
			messageID = 0
		}
	}

	if messageID == 0 {
		h.deleteCaptchaMessage(ctx, b, userID)

		msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      userID,
			Text:        text,
			ParseMode:   models.ParseModeHTML,
			ReplyMarkup: keyboard,
		})
		if err != nil {
			utils.LogError("send captcha challenge", err)
			return false
		}
		messageID = msg.ID
	}

	h.deps.State.ClearUserState(userID)
	h.deps.State.SetUserState(userID, config.StateWaitingCaptcha)
	h.deps.State.SetUserData(userID, "captcha_session", &captchaSessionState{
		SessionID:    challenge.SessionID,
		CorrectIndex: challenge.CorrectIndex,
		ExpiresAt:    challenge.ExpiresAt,
		MessageID:    messageID,
	})

	return true
}

func (h *UserHandlers) getCaptchaSession(userID int64) (*captchaSessionState, bool) {
	value := h.deps.State.GetUserData(userID, "captcha_session")
	session, ok := value.(*captchaSessionState)
	if !ok || session == nil {
		return nil, false
	}
	return session, true
}

func (h *UserHandlers) deleteCaptchaMessage(ctx context.Context, b *bot.Bot, userID int64) {
	session, ok := h.getCaptchaSession(userID)
	if !ok || session.MessageID == 0 {
		return
	}

	_, _ = b.DeleteMessage(ctx, &bot.DeleteMessageParams{
		ChatID:    userID,
		MessageID: session.MessageID,
	})
}

// HandleNewTicket handles the /newticket command
func (h *UserHandlers) HandleNewTicket(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	userID := update.Message.From.ID
	user := update.Message.From

	if !h.ensureUserPassedCaptcha(ctx, b, userID) {
		return
	}

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

	if !h.ensureUserPassedCaptcha(ctx, b, userID) {
		return
	}

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

	if !h.ensureUserPassedCaptcha(ctx, b, userID) {
		return
	}

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

	if !h.ensureUserPassedCaptcha(ctx, b, userID) {
		return
	}

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

	h.maybeOfferHWIDPanel(ctx, b, ticket, message)
}

// HandleUserCallback handles callbacks from users
func (h *UserHandlers) HandleUserCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	callback := update.CallbackQuery
	data := callback.Data
	userID := callback.From.ID

	answered := false
	answerCallback := func(text string, showAlert bool) {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
			Text:            text,
			ShowAlert:       showAlert,
		})
		answered = true
	}

	defer func() {
		if answered {
			return
		}
		answerCallback("", false)
	}()

	parts := strings.Split(data, ":")
	if parts[0] == "captcha" {
		h.handleCaptchaCallback(ctx, b, callback, parts, answerCallback)
		return
	}

	if h.isCaptchaEnabled() {
		passed, err := h.deps.DB.HasPassedCaptcha(ctx, userID)
		if err != nil {
			utils.LogError("check user captcha status before callback", err)
		}
		if !passed {
			h.sendCaptchaChallenge(ctx, b, userID, 0)
			answerCallback(messages.GetCaptchaRequiredAlert(), true)
			return
		}
	}

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

	case "hwid":
		h.handleHWIDCallback(ctx, b, callback, parts)

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

func (h *UserHandlers) handleCaptchaCallback(
	ctx context.Context,
	b *bot.Bot,
	callback *models.CallbackQuery,
	parts []string,
	answerCallback func(text string, showAlert bool),
) {
	if callback == nil || callback.Message.Message == nil || len(parts) < 3 {
		return
	}

	userID := callback.From.ID
	messageID := callback.Message.Message.ID
	optionIndex, err := strconv.Atoi(parts[2])
	if err != nil {
		return
	}

	session, ok := h.getCaptchaSession(userID)
	if !ok || session.SessionID != parts[1] || time.Now().After(session.ExpiresAt) {
		h.sendCaptchaChallenge(ctx, b, userID, messageID)
		answerCallback(messages.GetCaptchaExpiredAlert(), true)
		return
	}

	if optionIndex != session.CorrectIndex {
		h.sendCaptchaChallenge(ctx, b, userID, messageID)
		answerCallback(messages.GetCaptchaWrongAnswerAlert(), true)
		return
	}

	if err := h.deps.DB.MarkCaptchaPassed(ctx, userID); err != nil {
		utils.LogError("mark user captcha passed", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    userID,
			Text:      messages.GetCaptchaErrorMessage(),
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	h.deps.State.ClearUserState(userID)
	_, _ = b.DeleteMessage(ctx, &bot.DeleteMessageParams{
		ChatID:    userID,
		MessageID: messageID,
	})

	answerCallback(messages.GetCaptchaPassedAlert(), false)
	h.showStartFlow(ctx, b, userID)
}

func (h *UserHandlers) maybeOfferHWIDPanel(ctx context.Context, b *bot.Bot, ticket *database.Ticket, message *models.Message) {
	if ticket == nil || message == nil || message.From == nil {
		return
	}
	if h.deps.RemnawaveService == nil || config.YAMLCfg == nil || !config.YAMLCfg.HWID.Enabled {
		return
	}
	if pendingTicketID, ok := h.deps.State.GetUserData(ticket.UserID, "hwid_prompt_ticket_id").(int); ok && pendingTicketID == ticket.ID {
		return
	}

	if !matchesHWIDTrigger(extractMessageSearchText(message)) {
		return
	}

	lang := h.getUserLanguage(ctx, ticket.UserID)
	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      ticket.UserID,
		Text:        messages.GetHWIDPromptMessage(lang),
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboards.HWIDPromptKeyboard(ticket.ID, lang),
	}); err != nil {
		utils.LogError("send hwid prompt", err)
		return
	}

	h.deps.State.SetUserData(ticket.UserID, "hwid_prompt_ticket_id", ticket.ID)
}

func (h *UserHandlers) handleHWIDCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, parts []string) {
	if callback == nil || callback.Message.Message == nil || len(parts) < 3 {
		return
	}

	userID := callback.From.ID
	lang := h.getUserLanguage(ctx, userID)
	msg := callback.Message.Message

	ticketID, err := strconv.Atoi(parts[2])
	if err != nil {
		return
	}

	switch parts[1] {
	case "y":
		h.clearHWIDPromptState(userID)
		ticket, err := h.getOwnedTicket(ctx, userID, ticketID)
		if err != nil {
			h.renderHWIDText(ctx, b, userID, msg.ID, messages.GetHWIDTicketUnavailableMessage(lang))
			return
		}

		priorityText := h.raiseHWIDTicketPriority(ctx, b, ticket, lang)
		h.openHWIDPanel(ctx, b, ticket, msg.ID, lang, priorityText)

	case "n":
		h.clearHWIDPromptState(userID)
		_, _ = b.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    userID,
			MessageID: msg.ID,
		})

	case "u":
		if len(parts) < 4 {
			return
		}
		ticket, err := h.getOwnedTicket(ctx, userID, ticketID)
		if err != nil {
			h.renderHWIDText(ctx, b, userID, msg.ID, messages.GetHWIDTicketUnavailableMessage(lang))
			return
		}
		h.renderHWIDManagement(ctx, b, ticket, msg.ID, lang, "", parts[3])

	case "ru":
		ticket, err := h.getOwnedTicket(ctx, userID, ticketID)
		if err != nil {
			h.renderHWIDText(ctx, b, userID, msg.ID, messages.GetHWIDTicketUnavailableMessage(lang))
			return
		}
		h.openHWIDPanel(ctx, b, ticket, msg.ID, lang, "")

	case "r":
		if len(parts) < 4 {
			return
		}
		ticket, err := h.getOwnedTicket(ctx, userID, ticketID)
		if err != nil {
			h.renderHWIDText(ctx, b, userID, msg.ID, messages.GetHWIDTicketUnavailableMessage(lang))
			return
		}
		h.renderHWIDManagement(ctx, b, ticket, msg.ID, lang, "", parts[3])

	case "d":
		if len(parts) < 5 {
			return
		}
		ticket, err := h.getOwnedTicket(ctx, userID, ticketID)
		if err != nil {
			h.renderHWIDText(ctx, b, userID, msg.ID, messages.GetHWIDTicketUnavailableMessage(lang))
			return
		}
		h.deleteHWIDDevice(ctx, b, ticket, msg.ID, lang, parts[3], parts[4])
	}
}

func (h *UserHandlers) openHWIDPanel(ctx context.Context, b *bot.Bot, ticket *database.Ticket, messageID int, lang string, prefixText string) {
	users, err := h.deps.RemnawaveService.GetUsersByTelegramID(ctx, ticket.UserID)
	if err != nil {
		utils.LogError("get remnawave users by telegram id", err)
		h.renderHWIDText(ctx, b, ticket.UserID, messageID, joinHWIDSections(prefixText, messages.GetHWIDPanelErrorMessage(lang)))
		return
	}
	if len(users) == 0 {
		h.renderHWIDText(ctx, b, ticket.UserID, messageID, joinHWIDSections(prefixText, messages.GetHWIDNoPanelUsersMessage(lang)))
		return
	}

	if len(users) == 1 {
		h.renderHWIDManagement(ctx, b, ticket, messageID, lang, prefixText, shortUUID(users[0].UUID))
		return
	}

	sort.Slice(users, func(i, j int) bool {
		left := users[i].Username + users[i].UUID
		right := users[j].Username + users[j].UUID
		return left < right
	})

	options := make([]keyboards.CallbackOption, 0, len(users))
	for _, user := range users {
		options = append(options, keyboards.CallbackOption{
			Text: fmt.Sprintf("👤 %s", formatPanelUserLabel(user)),
			Data: fmt.Sprintf("hwid:u:%d:%s", ticket.ID, shortUUID(user.UUID)),
		})
	}

	refreshLabel := "🔄 Refresh"
	if lang == database.LangRussian {
		refreshLabel = "🔄 Обновить"
	}

	_, err = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      ticket.UserID,
		MessageID:   messageID,
		Text:        joinHWIDSections(prefixText, messages.GetHWIDUserSelectionMessage(lang)),
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboards.HWIDUserSelectionKeyboard(options, refreshLabel, fmt.Sprintf("hwid:ru:%d", ticket.ID)),
	})
	if err != nil {
		utils.LogError("edit hwid user selection", err)
	}
}

func (h *UserHandlers) renderHWIDManagement(ctx context.Context, b *bot.Bot, ticket *database.Ticket, messageID int, lang string, prefixText string, userPrefix string) {
	panelUsers, err := h.deps.RemnawaveService.GetUsersByTelegramID(ctx, ticket.UserID)
	if err != nil {
		utils.LogError("refresh remnawave users", err)
		h.renderHWIDText(ctx, b, ticket.UserID, messageID, joinHWIDSections(prefixText, messages.GetHWIDPanelErrorMessage(lang)))
		return
	}

	panelUser, ok := findPanelUserByPrefix(panelUsers, userPrefix)
	if !ok {
		h.renderHWIDText(ctx, b, ticket.UserID, messageID, joinHWIDSections(prefixText, messages.GetHWIDNoPanelUsersMessage(lang)))
		return
	}

	devices, err := h.deps.RemnawaveService.GetUserDevices(ctx, panelUser.UUID)
	if err != nil {
		utils.LogError("get remnawave hwid devices", err)
		h.renderHWIDText(ctx, b, ticket.UserID, messageID, joinHWIDSections(prefixText, messages.GetHWIDPanelErrorMessage(lang)))
		return
	}

	lastDeleteAt, err := h.deps.DB.GetUserLastHWIDDeleteAt(ctx, ticket.UserID)
	if err != nil {
		utils.LogError("get user hwid delete cooldown", err)
	}

	cooldownRemaining := h.getHWIDDeleteCooldownRemaining(lastDeleteAt)
	body := buildHWIDManagementText(ticket, panelUser, devices, cooldownRemaining, lang)
	body = joinHWIDSections(prefixText, body)

	deviceButtons := make([]keyboards.CallbackOption, 0, len(devices))
	if cooldownRemaining <= 0 {
		for idx, device := range devices {
			deviceButtons = append(deviceButtons, keyboards.CallbackOption{
				Text: fmt.Sprintf("🗑 %d. %s", idx+1, truncateDeviceButtonLabel(device, lang)),
				Data: fmt.Sprintf("hwid:d:%d:%s:%s", ticket.ID, userPrefix, shortHWIDToken(device.HWID)),
			})
		}
	}

	refreshLabel := "🔄 Refresh"
	if lang == database.LangRussian {
		refreshLabel = "🔄 Обновить"
	}

	_, err = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      ticket.UserID,
		MessageID:   messageID,
		Text:        body,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboards.HWIDManagementKeyboard(deviceButtons, refreshLabel, fmt.Sprintf("hwid:r:%d:%s", ticket.ID, userPrefix)),
	})
	if err != nil {
		utils.LogError("edit hwid management", err)
	}
}

func (h *UserHandlers) deleteHWIDDevice(ctx context.Context, b *bot.Bot, ticket *database.Ticket, messageID int, lang string, userPrefix string, deviceToken string) {
	panelUsers, err := h.deps.RemnawaveService.GetUsersByTelegramID(ctx, ticket.UserID)
	if err != nil {
		utils.LogError("refresh remnawave users before delete", err)
		h.renderHWIDText(ctx, b, ticket.UserID, messageID, messages.GetHWIDPanelErrorMessage(lang))
		return
	}

	panelUser, ok := findPanelUserByPrefix(panelUsers, userPrefix)
	if !ok {
		h.renderHWIDText(ctx, b, ticket.UserID, messageID, messages.GetHWIDNoPanelUsersMessage(lang))
		return
	}

	lastDeleteAt, err := h.deps.DB.GetUserLastHWIDDeleteAt(ctx, ticket.UserID)
	if err != nil {
		utils.LogError("get user hwid delete cooldown", err)
	}

	cooldownRemaining := h.getHWIDDeleteCooldownRemaining(lastDeleteAt)
	if cooldownRemaining > 0 {
		h.renderHWIDManagement(ctx, b, ticket, messageID, lang, messages.GetHWIDDeleteCooldownMessage(cooldownRemaining, lang), userPrefix)
		return
	}

	devices, err := h.deps.RemnawaveService.GetUserDevices(ctx, panelUser.UUID)
	if err != nil {
		utils.LogError("refresh remnawave devices before delete", err)
		h.renderHWIDText(ctx, b, ticket.UserID, messageID, messages.GetHWIDPanelErrorMessage(lang))
		return
	}

	var selectedDevice *services.RemnawaveDevice
	for i := range devices {
		if shortHWIDToken(devices[i].HWID) == deviceToken {
			selectedDevice = &devices[i]
			break
		}
	}
	if selectedDevice == nil {
		h.renderHWIDManagement(ctx, b, ticket, messageID, lang, messages.GetHWIDPanelErrorMessage(lang), userPrefix)
		return
	}

	if err := h.deps.RemnawaveService.DeleteUserDevice(ctx, panelUser.UUID, selectedDevice.HWID); err != nil {
		utils.LogError("delete remnawave hwid device", err)
		h.renderHWIDText(ctx, b, ticket.UserID, messageID, messages.GetHWIDPanelErrorMessage(lang))
		return
	}
	if err := h.deps.DB.SetUserLastHWIDDeleteAt(ctx, ticket.UserID, time.Now().UTC()); err != nil {
		utils.LogError("save user hwid delete timestamp", err)
	}

	h.renderHWIDManagement(ctx, b, ticket, messageID, lang, messages.GetHWIDDeleteSuccessMessage(lang), userPrefix)
}

func (h *UserHandlers) getOwnedTicket(ctx context.Context, userID int64, ticketID int) (*database.Ticket, error) {
	ticket, err := h.deps.TicketService.GetTicketByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket == nil || ticket.UserID != userID || !ticket.IsOpen() {
		return nil, fmt.Errorf("ticket is unavailable")
	}
	return ticket, nil
}

func (h *UserHandlers) raiseHWIDTicketPriority(ctx context.Context, b *bot.Bot, ticket *database.Ticket, lang string) string {
	oldPriority := ticket.Priority
	adminText := fmt.Sprintf(
		"⚠️ <b>HWID panel access requested</b>\n\nTicket #%s was confirmed by the user for device management.",
		utils.EscapeHTML(ticket.TicketNumber),
	)

	switch {
	case oldPriority == config.PriorityUrgent:
		adminText += "\nCurrent priority: <b>urgent</b>."
	case func() bool {
		err := h.deps.TicketService.UpdateTicketPriority(ctx, ticket.ID, config.PriorityUrgent)
		if err != nil {
			utils.LogError("auto escalate hwid ticket priority", err)
		}
		return err != nil
	}():
		adminText += "\nAutomatic priority escalation failed."
	default:
		adminText += fmt.Sprintf("\nPriority changed from <b>%s</b> to <b>%s</b>.", utils.EscapeHTML(oldPriority), config.PriorityUrgent)
		ticket.Priority = config.PriorityUrgent
	}

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          ticket.SupportChatID,
		MessageThreadID: int(ticket.ThreadID),
		Text:            adminText,
		ParseMode:       models.ParseModeHTML,
	})

	if lang == database.LangRussian {
		if oldPriority == config.PriorityUrgent {
			return fmt.Sprintf("🔴 <b>Тикет #%s уже имеет максимальный приоритет.</b>", utils.EscapeHTML(ticket.TicketNumber))
		}
		if ticket.Priority == config.PriorityUrgent {
			return fmt.Sprintf("🔴 <b>Приоритет тикета #%s повышен до urgent.</b>", utils.EscapeHTML(ticket.TicketNumber))
		}
		return fmt.Sprintf("⚠️ <b>Не удалось автоматически повысить приоритет тикета #%s.</b>", utils.EscapeHTML(ticket.TicketNumber))
	}

	if oldPriority == config.PriorityUrgent {
		return fmt.Sprintf("🔴 <b>Ticket #%s is already at urgent priority.</b>", utils.EscapeHTML(ticket.TicketNumber))
	}
	if ticket.Priority == config.PriorityUrgent {
		return fmt.Sprintf("🔴 <b>Ticket #%s priority was raised to urgent.</b>", utils.EscapeHTML(ticket.TicketNumber))
	}
	return fmt.Sprintf("⚠️ <b>Failed to automatically raise ticket #%s priority.</b>", utils.EscapeHTML(ticket.TicketNumber))
}

func (h *UserHandlers) getHWIDDeleteCooldownRemaining(lastDeleteAt *time.Time) time.Duration {
	if config.YAMLCfg == nil || config.YAMLCfg.HWID.DeleteCooldownHours <= 0 || lastDeleteAt == nil {
		return 0
	}

	nextAllowedAt := lastDeleteAt.Add(time.Duration(config.YAMLCfg.HWID.DeleteCooldownHours) * time.Hour)
	if time.Now().After(nextAllowedAt) {
		return 0
	}
	return time.Until(nextAllowedAt)
}

func (h *UserHandlers) clearHWIDPromptState(userID int64) {
	h.deps.State.SetUserData(userID, "hwid_prompt_ticket_id", 0)
}

func (h *UserHandlers) renderHWIDText(ctx context.Context, b *bot.Bot, chatID int64, messageID int, text string) {
	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{},
	})
	if err != nil {
		utils.LogError("edit hwid message", err)
	}
}

func extractMessageSearchText(message *models.Message) string {
	if message == nil {
		return ""
	}
	text := strings.TrimSpace(message.Text)
	if text == "" {
		text = strings.TrimSpace(message.Caption)
	}
	return strings.ToLower(text)
}

func matchesHWIDTrigger(text string) bool {
	if text == "" || config.YAMLCfg == nil || len(config.YAMLCfg.HWID.TriggerKeywords) == 0 {
		return false
	}

	for _, keyword := range config.YAMLCfg.HWID.TriggerKeywords {
		if keyword != "" && strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func joinHWIDSections(parts ...string) string {
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		clean = append(clean, part)
	}
	return strings.Join(clean, "\n\n")
}

func findPanelUserByPrefix(users []services.RemnawaveUser, prefix string) (services.RemnawaveUser, bool) {
	for _, user := range users {
		if shortUUID(user.UUID) == prefix {
			return user, true
		}
	}
	return services.RemnawaveUser{}, false
}

func buildHWIDManagementText(ticket *database.Ticket, panelUser services.RemnawaveUser, devices []services.RemnawaveDevice, cooldownRemaining time.Duration, lang string) string {
	var sb strings.Builder
	title := "🧩 <b>HWID Device Management</b>"
	ticketLabel := "Ticket"
	accountLabel := "Account"
	limitLabel := "Device limit"
	connectedLabel := "Connected devices"
	cooldownLabel := "Delete cooldown"
	noDevicesText := "No connected devices were found."

	if lang == database.LangRussian {
		title = "🧩 <b>Управление подключёнными устройствами</b>"
		ticketLabel = "Тикет"
		accountLabel = "Профиль панели"
		limitLabel = "Лимит устройств"
		connectedLabel = "Подключённые устройства"
		cooldownLabel = "Кулдаун удаления"
		noDevicesText = "Подключённые устройства не найдены."
	}

	sb.WriteString(title)
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("🎫 <b>%s:</b> #%s\n", ticketLabel, utils.EscapeHTML(ticket.TicketNumber)))
	sb.WriteString(fmt.Sprintf("👤 <b>%s:</b> %s\n", accountLabel, utils.EscapeHTML(formatPanelUserLabel(panelUser))))
	sb.WriteString(fmt.Sprintf("📏 <b>%s:</b> %s\n", limitLabel, utils.EscapeHTML(formatPanelDeviceLimit(panelUser, lang))))
	sb.WriteString(fmt.Sprintf("📱 <b>%s:</b> %d\n", connectedLabel, len(devices)))

	if cooldownRemaining > 0 {
		sb.WriteString(fmt.Sprintf("⏳ <b>%s:</b> %s\n", cooldownLabel, utils.EscapeHTML(formatDuration(cooldownRemaining, lang))))
	}

	sb.WriteString("\n")
	if len(devices) == 0 {
		sb.WriteString(noDevicesText)
		return sb.String()
	}

	for idx, device := range devices {
		sb.WriteString(fmt.Sprintf("%d. %s\n", idx+1, utils.EscapeHTML(formatDeviceLine(device, lang))))
	}

	return strings.TrimSpace(sb.String())
}

func formatPanelUserLabel(user services.RemnawaveUser) string {
	if strings.TrimSpace(user.Username) == "" {
		return user.UUID
	}
	return fmt.Sprintf("%s (%s)", user.Username, shortUUID(user.UUID))
}

func formatPanelDeviceLimit(user services.RemnawaveUser, lang string) string {
	if user.HWIDDeviceLimit == nil {
		if lang == database.LangRussian {
			return "по умолчанию"
		}
		return "default"
	}
	return strconv.Itoa(*user.HWIDDeviceLimit)
}

func formatDeviceLine(device services.RemnawaveDevice, lang string) string {
	parts := make([]string, 0, 4)
	if value := firstNonEmpty(device.Platform, device.DeviceModel); value != "" {
		parts = append(parts, value)
	}
	if value := valueOrEmpty(device.OSVersion); value != "" {
		parts = append(parts, value)
	}
	if value := valueOrEmpty(device.UserAgent); value != "" {
		parts = append(parts, value)
	}
	parts = append(parts, shortDeviceHWID(device.HWID))

	label := strings.Join(parts, " | ")
	if label != "" {
		return label
	}
	if lang == database.LangRussian {
		return "Неизвестное устройство"
	}
	return "Unknown device"
}

func truncateDeviceButtonLabel(device services.RemnawaveDevice, lang string) string {
	label := formatDeviceLine(device, lang)
	if len(label) <= 28 {
		return label
	}
	return label[:25] + "..."
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func firstNonEmpty(values ...*string) string {
	for _, value := range values {
		if candidate := valueOrEmpty(value); candidate != "" {
			return candidate
		}
	}
	return ""
}

func formatDuration(duration time.Duration, lang string) string {
	if duration < time.Minute {
		duration = time.Minute
	}

	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	if lang == database.LangRussian {
		if hours > 0 {
			return fmt.Sprintf("%d ч. %d мин.", hours, minutes)
		}
		return fmt.Sprintf("%d мин.", minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func shortUUID(value string) string {
	if len(value) <= 8 {
		return value
	}
	return value[:8]
}

func shortDeviceHWID(hwid string) string {
	if len(hwid) <= 12 {
		return hwid
	}
	return hwid[:6] + "..." + hwid[len(hwid)-4:]
}

func shortHWIDToken(hwid string) string {
	sum := sha1.Sum([]byte(hwid))
	return hex.EncodeToString(sum[:])[:12]
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
