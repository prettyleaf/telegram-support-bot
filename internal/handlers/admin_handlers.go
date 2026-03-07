package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"remnafamily/internal/config"
	"remnafamily/internal/database"
	"remnafamily/internal/keyboards"
	"remnafamily/internal/messages"
	"remnafamily/internal/types"
	"remnafamily/internal/utils"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// AdminHandlers handles admin commands and messages
type AdminHandlers struct {
	deps *types.Dependencies
}

// NewAdminHandlers creates a new admin handlers instance
func NewAdminHandlers(deps *types.Dependencies) *AdminHandlers {
	return &AdminHandlers{deps: deps}
}

// HandleAdminHelp handles the /help command for admins
func (h *AdminHandlers) HandleAdminHelp(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	if !config.IsAdmin(update.Message.From.ID) {
		return
	}

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      messages.GetAdminHelpMessage(),
		ParseMode: models.ParseModeHTML,
	})
}

// HandleTickets handles the /tickets command (list all tickets with pagination)
func (h *AdminHandlers) HandleTickets(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	utils.LogDebug("HandleTickets called by user %d", update.Message.From.ID)

	if !config.IsAdmin(update.Message.From.ID) {
		utils.LogDebug("User %d is not admin, ignoring", update.Message.From.ID)
		return
	}

	utils.LogDebug("Processing /tickets command")

	// Parse arguments: /tickets [page] [filter]
	// filter can be: all, open, closed (default: all)
	page := 1
	filter := "all"
	parts := strings.Fields(update.Message.Text)
	
	for i := 1; i < len(parts); i++ {
		if p, err := strconv.Atoi(parts[i]); err == nil && p > 0 {
			page = p
		} else if parts[i] == "open" || parts[i] == "closed" || parts[i] == "all" {
			filter = parts[i]
		}
	}

	const pageSize = 10
	offset := (page - 1) * pageSize

	var tickets []*database.Ticket
	var err error

	switch filter {
	case "open":
		tickets, err = h.deps.TicketService.GetAllOpenTickets(ctx)
	case "closed":
		tickets, err = h.deps.TicketService.GetClosedTickets(ctx, pageSize+1, offset)
	default: // "all"
		tickets, err = h.deps.TicketService.GetAllTickets(ctx, pageSize+1, offset)
	}

	utils.LogDebug("Got %d tickets, err: %v", len(tickets), err)

	if err != nil {
		utils.LogError("get tickets", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      "❌ Failed to load tickets.",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	hasNextPage := len(tickets) > pageSize
	if hasNextPage {
		tickets = tickets[:pageSize]
	}

	totalPages := page
	if hasNextPage {
		totalPages = page + 1
	}

	text := messages.FormatAllTicketList(tickets, page, totalPages)
	
	// Add filter and navigation hints
	text += fmt.Sprintf("\n<i>Filter: %s</i>", filter)
	if hasNextPage || page > 1 {
		text += "\n<i>Use /tickets [page] [open|closed|all]</i>"
	}

	// Sanitize text to ensure valid UTF-8 (in case DB has corrupted data)
	text = utils.SanitizeUTF8(text)

	utils.LogDebug("Sending message, text length: %d, chat: %d", len(text), update.Message.Chat.ID)

	resp, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboards.TicketListKeyboard(tickets, page),
	})
	if err != nil {
		utils.LogError("send tickets message", err)
	} else {
		utils.LogDebug("Message sent successfully, message_id: %d", resp.ID)
	}
}

// HandleClose handles the /close command
func (h *AdminHandlers) HandleClose(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	admin := update.Message.From
	if !config.IsAdmin(admin.ID) {
		return
	}

	// Try to get ticket from argument or thread
	ticket, err := h.getTicketFromCommandOrThread(ctx, update.Message)
	if err != nil {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "❌ Usage: /close [ticket_number] or use in a ticket thread.",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	if ticket.IsClosed() {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            fmt.Sprintf("⚠️ Ticket #%s is already closed.", ticket.TicketNumber),
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	adminName := utils.GetUserFullName(admin)

	err = h.deps.TicketService.CloseTicket(ctx, b, ticket.ID, admin.ID, adminName)
	if err != nil {
		utils.LogError("close ticket", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "❌ Failed to close ticket.",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	utils.LogTicket("Ticket #%s closed by %s", ticket.TicketNumber, adminName)

	// Notify in current chat
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            fmt.Sprintf("🔒 <b>Ticket #%s closed by %s</b>", ticket.TicketNumber, adminName),
		ParseMode:       models.ParseModeHTML,
		ReplyMarkup:     keyboards.TicketActionsKeyboard(ticket.ID, false),
	})

	// Also notify in ticket thread if we're not already there
	if update.Message.MessageThreadID != int(ticket.ThreadID) && ticket.ThreadID > 1 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          ticket.SupportChatID,
			MessageThreadID: int(ticket.ThreadID),
			Text:            fmt.Sprintf("🔒 <b>Ticket closed by %s</b>", adminName),
			ParseMode:       models.ParseModeHTML,
		})
	}

	// Notify user
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    ticket.UserID,
		Text:      messages.GetTicketClosedMessage(ticket.TicketNumber),
		ParseMode: models.ParseModeHTML,
	})

	// Clear user state
	h.deps.State.ClearUserState(ticket.UserID)
}

// HandleReopen handles the /reopen command
func (h *AdminHandlers) HandleReopen(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	admin := update.Message.From
	if !config.IsAdmin(admin.ID) {
		return
	}

	// Try to get ticket from argument or thread
	ticket, err := h.getTicketFromCommandOrThread(ctx, update.Message)
	if err != nil {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "❌ Usage: /reopen [ticket_number] or use in a ticket thread.",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	if ticket.IsOpen() {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            fmt.Sprintf("⚠️ Ticket #%s is already open.", ticket.TicketNumber),
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	err = h.deps.TicketService.ReopenTicket(ctx, b, ticket.ID)
	if err != nil {
		utils.LogError("reopen ticket", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "❌ Failed to reopen ticket.",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	adminName := utils.GetUserDisplayName(admin)
	utils.LogTicket("Ticket #%s reopened by %s", ticket.TicketNumber, adminName)

	// Notify in thread
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            fmt.Sprintf("🔓 <b>Ticket reopened by %s</b>", adminName),
		ParseMode:       models.ParseModeHTML,
		ReplyMarkup:     keyboards.TicketActionsKeyboard(ticket.ID, true),
	})

	// Notify user
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    ticket.UserID,
		Text:      messages.GetTicketReopenedMessage(ticket.TicketNumber),
		ParseMode: models.ParseModeHTML,
	})
}

// HandlePriority handles the /priority command
func (h *AdminHandlers) HandlePriority(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	if !config.IsAdmin(update.Message.From.ID) {
		return
	}

	parts := strings.Fields(update.Message.Text)
	
	// Try to get ticket from argument or thread
	var ticket *database.Ticket
	var err error
	var priorityArg string
	
	// Check if first arg is a ticket number
	if len(parts) >= 2 {
		ticket, err = h.deps.TicketService.GetTicketByNumber(ctx, strings.TrimPrefix(parts[1], "#"))
		if err == nil && ticket != nil {
			// First arg was ticket number, priority is second arg
			if len(parts) >= 3 {
				priorityArg = parts[2]
			}
		} else {
			// First arg might be priority, try to get ticket from thread
			ticket, err = h.getTicketFromMessage(ctx, update.Message)
			if err == nil {
				priorityArg = parts[1]
			}
		}
	} else {
		ticket, err = h.getTicketFromMessage(ctx, update.Message)
	}
	
	if ticket == nil || err != nil {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "❌ Usage: /priority [ticket_number] [low|normal|high|urgent] or use in a ticket thread.",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	if priorityArg == "" {
		// Show priority keyboard
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            fmt.Sprintf("Select priority for ticket #%s:", ticket.TicketNumber),
			ParseMode:       models.ParseModeHTML,
			ReplyMarkup:     keyboards.PriorityKeyboard(ticket.ID),
		})
		return
	}

	priority := strings.ToLower(priorityArg)
	validPriorities := map[string]bool{
		"low": true, "normal": true, "high": true, "urgent": true,
	}

	if !validPriorities[priority] {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "❌ Invalid priority. Use: low, normal, high, or urgent",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	err = h.deps.TicketService.UpdateTicketPriority(ctx, ticket.ID, priority)
	if err != nil {
		utils.LogError("update priority", err)
		return
	}

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            fmt.Sprintf("✅ Ticket #%s priority set to <b>%s</b>", ticket.TicketNumber, priority),
		ParseMode:       models.ParseModeHTML,
	})
}

// HandleInfo handles the /info command
func (h *AdminHandlers) HandleInfo(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	if !config.IsAdmin(update.Message.From.ID) {
		return
	}

	// Try to get ticket from argument or thread
	ticket, err := h.getTicketFromCommandOrThread(ctx, update.Message)
	if err != nil {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "❌ Usage: /info [ticket_number] or use in a ticket thread.",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            messages.FormatTicketDetails(ticket),
		ParseMode:       models.ParseModeHTML,
		ReplyMarkup:     keyboards.TicketActionsKeyboard(ticket.ID, ticket.IsOpen()),
	})
}

// HandleBan handles the /ban command
func (h *AdminHandlers) HandleBan(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	admin := update.Message.From
	if !config.IsAdmin(admin.ID) {
		return
	}

	parts := strings.Fields(update.Message.Text)
	if len(parts) < 2 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      "Usage: /ban [user_id] [reason]",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	userID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      "❌ Invalid user ID.",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	reason := ""
	if len(parts) > 2 {
		reason = strings.Join(parts[2:], " ")
	}

	adminName := utils.GetUserDisplayName(admin)
	err = h.deps.TicketService.BanUser(ctx, userID, nil, nil, admin.ID, adminName, reason)
	if err != nil {
		utils.LogError("ban user", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      "❌ Failed to ban user.",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	utils.LogAdmin("User %d banned by %s", userID, adminName)

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      fmt.Sprintf("✅ User <code>%d</code> has been banned.", userID),
		ParseMode: models.ParseModeHTML,
	})

	// Notify the user
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    userID,
		Text:      messages.GetUserBannedMessage(),
		ParseMode: models.ParseModeHTML,
	})
}

// HandleUnban handles the /unban command
func (h *AdminHandlers) HandleUnban(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	if !config.IsAdmin(update.Message.From.ID) {
		return
	}

	parts := strings.Fields(update.Message.Text)
	if len(parts) < 2 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      "Usage: /unban [user_id]",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	userID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      "❌ Invalid user ID.",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	err = h.deps.TicketService.UnbanUser(ctx, userID)
	if err != nil {
		utils.LogError("unban user", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      "❌ Failed to unban user.",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      fmt.Sprintf("✅ User <code>%d</code> has been unbanned.", userID),
		ParseMode: models.ParseModeHTML,
	})
}

// HandleBanned handles the /banned command (list banned users)
func (h *AdminHandlers) HandleBanned(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	if !config.IsAdmin(update.Message.From.ID) {
		return
	}

	users, err := h.deps.TicketService.GetAllBannedUsers(ctx)
	if err != nil {
		utils.LogError("get banned users", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      "❌ Failed to load banned users.",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	if len(users) == 0 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      "📭 No banned users.",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	var sb strings.Builder
	sb.WriteString("🚫 <b>Banned Users:</b>\n\n")
	for _, u := range users {
		displayName := fmt.Sprintf("<code>%d</code>", u.UserID)
		if u.UserUsername != nil && *u.UserUsername != "" {
			displayName = "@" + *u.UserUsername
		} else if u.UserName != nil && *u.UserName != "" {
			displayName = *u.UserName
		}
		sb.WriteString(fmt.Sprintf("• %s", displayName))
		if u.Reason != nil && *u.Reason != "" {
			sb.WriteString(fmt.Sprintf(" - %s", *u.Reason))
		}
		sb.WriteString("\n")
	}

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      sb.String(),
		ParseMode: models.ParseModeHTML,
	})
}

// HandleNote handles the /note command
func (h *AdminHandlers) HandleNote(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	admin := update.Message.From
	if !config.IsAdmin(admin.ID) {
		return
	}

	parts := strings.Fields(update.Message.Text)
	
	var ticket *database.Ticket
	var err error
	var noteText string
	
	// Check if first arg is a ticket number
	if len(parts) >= 2 {
		ticket, err = h.deps.TicketService.GetTicketByNumber(ctx, strings.TrimPrefix(parts[1], "#"))
		if err == nil && ticket != nil {
			// First arg was ticket number, note text starts from third word
			if len(parts) >= 3 {
				noteText = strings.Join(parts[2:], " ")
			}
		} else {
			// First arg is part of note, try to get ticket from thread
			ticket, err = h.getTicketFromMessage(ctx, update.Message)
			if err == nil && len(parts) >= 2 {
				noteText = strings.Join(parts[1:], " ")
			}
		}
	} else {
		ticket, err = h.getTicketFromMessage(ctx, update.Message)
	}
	
	if ticket == nil || err != nil {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "❌ Usage: /note [ticket_number] [text] or use in a ticket thread.",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	if noteText == "" {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            fmt.Sprintf("Usage: /note %s [your note text]", ticket.TicketNumber),
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	adminName := utils.GetUserDisplayName(admin)

	err = h.deps.TicketService.AddNote(ctx, ticket.ID, admin.ID, adminName, noteText)
	if err != nil {
		utils.LogError("add note", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "❌ Failed to add note.",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            fmt.Sprintf("📝 Note added to ticket #%s (visible only to admins).", ticket.TicketNumber),
		ParseMode:       models.ParseModeHTML,
	})
}

// HandleNotes handles the /notes command
func (h *AdminHandlers) HandleNotes(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	if !config.IsAdmin(update.Message.From.ID) {
		return
	}

	// Try to get ticket from argument or thread
	ticket, err := h.getTicketFromCommandOrThread(ctx, update.Message)
	if err != nil {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "❌ Usage: /notes [ticket_number] or use in a ticket thread.",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	notes, err := h.deps.TicketService.GetTicketNotes(ctx, ticket.ID)
	if err != nil {
		utils.LogError("get notes", err)
		return
	}

	if len(notes) == 0 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            fmt.Sprintf("📭 No notes for ticket #%s.", ticket.TicketNumber),
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📝 <b>Notes for Ticket #%s:</b>\n\n", ticket.TicketNumber))
	for _, n := range notes {
		adminName := "Admin"
		if n.AdminName != nil {
			adminName = *n.AdminName
		}
		sb.WriteString(fmt.Sprintf("• <b>%s</b> (%s):\n%s\n\n",
			adminName,
			n.CreatedAt.Format("02 Jan 15:04"),
			n.NoteText))
	}

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            sb.String(),
		ParseMode:       models.ParseModeHTML,
	})
}

// HandleAdminMessage handles messages from admins in ticket threads
func (h *AdminHandlers) HandleAdminMessage(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	message := update.Message
	admin := message.From

	if !config.IsAdmin(admin.ID) {
		return
	}

	// Check if this is in the support chat
	if message.Chat.ID != config.Cfg.SupportChatID {
		return
	}

	// Check if this is in a thread
	if message.MessageThreadID == 0 {
		return
	}

	// Skip commands
	if message.Text != "" && strings.HasPrefix(message.Text, "/") {
		return
	}

	// Get ticket from thread
	ticket, err := h.deps.TicketService.GetTicketByThreadID(ctx, int64(message.MessageThreadID))
	if err != nil {
		return // Not a ticket thread
	}

	if ticket.IsClosed() {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            "⚠️ This ticket is closed. Use /reopen to reopen it.",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	// Forward message to user
	adminName := utils.GetUserDisplayName(admin)
	sentMsg, err := h.deps.TicketService.ForwardReplyToUser(ctx, b, ticket, message, adminName)
	if err != nil {
		utils.LogError("forward reply to user", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            "⚠️ Failed to send message to user. They may have blocked the bot.",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	// Save message to database
	msgText := message.Text
	if msgText == "" && message.Caption != "" {
		msgText = message.Caption
	}

	hasMedia := message.Photo != nil || message.Document != nil || message.Video != nil ||
		message.Voice != nil || message.Audio != nil || message.Sticker != nil

	ticketMsg := &database.TicketMessage{
		TicketID:          ticket.ID,
		SenderID:          admin.ID,
		SenderName:        utils.GetUserNamePtr(admin),
		SenderUsername:    utils.GetUsernamePtr(admin),
		IsAdmin:           true,
		MessageText:       &msgText,
		MessageID:         func() *int64 { id := int64(sentMsg.ID); return &id }(),
		OriginalMessageID: func() *int64 { id := int64(message.ID); return &id }(),
		HasMedia:          hasMedia,
	}

	if err := h.deps.TicketService.SaveMessage(ctx, ticketMsg); err != nil {
		utils.LogError("save admin message", err)
	}

	// React to confirm delivery
	_, _ = b.SetMessageReaction(ctx, &bot.SetMessageReactionParams{
		ChatID:    message.Chat.ID,
		MessageID: message.ID,
		Reaction:  []models.ReactionType{{Type: "emoji", ReactionTypeEmoji: &models.ReactionTypeEmoji{Emoji: "✅"}}},
	})
}

// HandleAdminCallback handles callbacks from admins
func (h *AdminHandlers) HandleAdminCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	callback := update.CallbackQuery
	adminID := callback.From.ID

	if !config.IsAdmin(adminID) {
		return
	}

	defer func() {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: callback.ID,
		})
	}()

	data := callback.Data
	parts := strings.Split(data, ":")

	switch parts[0] {
	case "ticket":
		if len(parts) < 3 {
			return
		}
		
		action := parts[1]
		ticketID, err := strconv.Atoi(parts[2])
		if err != nil {
			return
		}

		ticket, err := h.deps.TicketService.GetTicketByID(ctx, ticketID)
		if err != nil {
			return
		}

		switch action {
		case "view":
			// Show ticket details with action buttons
			isOpen := ticket.Status == "open"
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:      callback.Message.Message.Chat.ID,
				Text:        messages.FormatTicketDetails(ticket),
				ParseMode:   models.ParseModeHTML,
				ReplyMarkup: keyboards.TicketViewKeyboard(ticket.ID, isOpen, ticket.ThreadID),
			})
			
		case "close", "confirmclose":
			adminName := utils.GetUserFullName(&callback.From)
			err = h.deps.TicketService.CloseTicket(ctx, b, ticketID, adminID, adminName)
			if err != nil {
				utils.LogError("close ticket via callback", err)
				return
			}

			_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
				ChatID:      callback.Message.Message.Chat.ID,
				MessageID:   callback.Message.Message.ID,
				Text:        fmt.Sprintf("🔒 <b>Ticket closed by %s</b>", adminName),
				ParseMode:   models.ParseModeHTML,
				ReplyMarkup: keyboards.TicketActionsKeyboard(ticketID, false),
			})

			// Notify user
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:    ticket.UserID,
				Text:      messages.GetTicketClosedMessage(ticket.TicketNumber),
				ParseMode: models.ParseModeHTML,
			})

			h.deps.State.ClearUserState(ticket.UserID)

		case "reopen":
			err = h.deps.TicketService.ReopenTicket(ctx, b, ticketID)
			if err != nil {
				utils.LogError("reopen ticket via callback", err)
				return
			}

			adminName := utils.GetUserDisplayName(&callback.From)
			_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
				ChatID:      callback.Message.Message.Chat.ID,
				MessageID:   callback.Message.Message.ID,
				Text:        fmt.Sprintf("🔓 <b>Ticket reopened by %s</b>", adminName),
				ParseMode:   models.ParseModeHTML,
				ReplyMarkup: keyboards.TicketActionsKeyboard(ticketID, true),
			})

			// Notify user
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:    ticket.UserID,
				Text:      messages.GetTicketReopenedMessage(ticket.TicketNumber),
				ParseMode: models.ParseModeHTML,
			})

		case "info":
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:          callback.Message.Message.Chat.ID,
				MessageThreadID: callback.Message.Message.MessageThreadID,
				Text:            messages.FormatTicketDetails(ticket),
				ParseMode:       models.ParseModeHTML,
			})

		case "priority":
			// Show priority selection keyboard
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:      callback.Message.Message.Chat.ID,
				Text:        fmt.Sprintf("🎯 Select priority for ticket #%s:", ticket.TicketNumber),
				ParseMode:   models.ParseModeHTML,
				ReplyMarkup: keyboards.PriorityKeyboard(ticketID),
			})

		case "notes":
			// Show ticket notes
			notes, err := h.deps.TicketService.GetTicketNotes(ctx, ticketID)
			if err != nil {
				utils.LogError("get notes via callback", err)
				return
			}

			if len(notes) == 0 {
				_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID:    callback.Message.Message.Chat.ID,
					Text:      fmt.Sprintf("📭 No notes for ticket #%s.", ticket.TicketNumber),
					ParseMode: models.ParseModeHTML,
				})
				return
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("📝 <b>Notes for Ticket #%s:</b>\n\n", ticket.TicketNumber))
			for _, n := range notes {
				adminName := "Admin"
				if n.AdminName != nil {
					adminName = *n.AdminName
				}
				sb.WriteString(fmt.Sprintf("• <b>%s</b> (%s):\n%s\n\n",
					adminName,
					n.CreatedAt.Format("02 Jan 15:04"),
					n.NoteText))
			}

			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:    callback.Message.Message.Chat.ID,
				Text:      sb.String(),
				ParseMode: models.ParseModeHTML,
			})

		case "goto":
			// Send a link to the topic
			if ticket.ThreadID > 1 {
				_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID:    callback.Message.Message.Chat.ID,
					Text:      fmt.Sprintf("💬 <a href=\"https://t.me/c/%d/%d\">Go to ticket #%s topic</a>", -config.Cfg.SupportChatID - 1000000000000, ticket.ThreadID, ticket.TicketNumber),
					ParseMode: models.ParseModeHTML,
				})
			}
		}

	case "priority":
		if len(parts) < 3 {
			return
		}
		ticketID, err := strconv.Atoi(parts[1])
		if err != nil {
			return
		}
		priority := parts[2]

		err = h.deps.TicketService.UpdateTicketPriority(ctx, ticketID, priority)
		if err != nil {
			utils.LogError("update priority via callback", err)
			return
		}

		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    callback.Message.Message.Chat.ID,
			MessageID: callback.Message.Message.ID,
			Text:      fmt.Sprintf("✅ Priority set to <b>%s</b>", priority),
			ParseMode: models.ParseModeHTML,
		})

	case "tickets":
		if len(parts) >= 2 && parts[1] == "refresh" {
			tickets, err := h.deps.TicketService.GetAllOpenTickets(ctx)
			if err != nil {
				return
			}

			_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
				ChatID:      callback.Message.Message.Chat.ID,
				MessageID:   callback.Message.Message.ID,
				Text:        messages.FormatAdminTicketList(tickets),
				ParseMode:   models.ParseModeHTML,
				ReplyMarkup: keyboards.TicketListKeyboard(tickets, 0),
			})
		}
	}
}

// getTicketFromMessage gets the ticket associated with a message thread
func (h *AdminHandlers) getTicketFromMessage(ctx context.Context, message *models.Message) (*database.Ticket, error) {
	if message.Chat.ID != config.Cfg.SupportChatID {
		return nil, fmt.Errorf("not in support chat")
	}

	if message.MessageThreadID == 0 {
		return nil, fmt.Errorf("not in a thread")
	}

	return h.deps.TicketService.GetTicketByThreadID(ctx, int64(message.MessageThreadID))
}

// getTicketFromCommandOrThread tries to get ticket from command argument first, then falls back to thread
func (h *AdminHandlers) getTicketFromCommandOrThread(ctx context.Context, message *models.Message) (*database.Ticket, error) {
	parts := strings.Fields(message.Text)
	
	// Try to parse ticket number from first argument
	if len(parts) >= 2 {
		ticketNum := strings.TrimPrefix(parts[1], "#")
		ticket, err := h.deps.TicketService.GetTicketByNumber(ctx, ticketNum)
		if err == nil && ticket != nil {
			return ticket, nil
		}
	}
	
	// Fallback to getting ticket from thread
	return h.getTicketFromMessage(ctx, message)
}
