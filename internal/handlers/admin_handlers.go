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
	if !config.IsAdmin(update.Message.From.ID) {
		return
	}

	page, filter := parseTicketListArgs(update.Message.Text)
	if err := h.sendTicketList(ctx, b, update.Message.Chat.ID, update.Message.MessageThreadID, page, filter, 0); err != nil {
		utils.LogError("send ticket list", err)
	}
}

// HandleStats handles the /stats command.
func (h *AdminHandlers) HandleStats(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}
	if !config.IsAdmin(update.Message.From.ID) {
		return
	}

	days := 7
	parts := strings.Fields(update.Message.Text)
	if len(parts) >= 2 {
		parsedDays, err := strconv.Atoi(parts[1])
		if err != nil || parsedDays <= 0 {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				MessageThreadID: update.Message.MessageThreadID,
				Text:            "❌ Usage: /stats [days]",
				ParseMode:       models.ParseModeHTML,
			})
			return
		}
		days = parsedDays
	}

	endDate := time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	startDate := endDate.AddDate(0, 0, -days)

	stats, err := h.deps.TicketService.GetTicketStats(ctx, startDate, endDate)
	if err != nil {
		utils.LogError("get ticket stats", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "❌ Failed to load ticket statistics.",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	openCount, err := h.deps.TicketService.CountTicketsByFilter(ctx, "open")
	if err != nil {
		utils.LogError("count open tickets", err)
		openCount = 0
	}

	closedCount, err := h.deps.TicketService.CountTicketsByFilter(ctx, "closed")
	if err != nil {
		utils.LogError("count closed tickets", err)
		closedCount = 0
	}

	text := fmt.Sprintf(
		"📈 <b>Ticket Statistics</b>\n\n"+
			"🗓️ <b>Period:</b> last %d day(s)\n"+
			"📬 <b>Opened:</b> %d\n"+
			"🔒 <b>Closed:</b> %d\n"+
			"🟢 <b>Currently Open:</b> %d\n"+
			"📦 <b>Total Closed:</b> %d",
		days,
		stats.TicketsOpened,
		stats.TicketsClosed,
		openCount,
		closedCount,
	)

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            text,
		ParseMode:       models.ParseModeHTML,
	})
}

// HandleCanned handles the /canned command.
func (h *AdminHandlers) HandleCanned(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}
	if !config.IsAdmin(update.Message.From.ID) {
		return
	}

	responses, err := h.deps.TicketService.GetAllCannedResponses(ctx)
	if err != nil {
		utils.LogError("get canned responses", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "❌ Failed to load canned responses.",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	if len(responses) == 0 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "📭 No canned responses configured.",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	var sb strings.Builder
	sb.WriteString("🗂️ <b>Canned Responses</b>\n\n")
	for _, response := range responses {
		sb.WriteString(fmt.Sprintf("• <code>/%s</code> - %s\n", utils.EscapeHTML(response.Shortcut), utils.EscapeHTML(response.Title)))
	}
	sb.WriteString("\n<i>Use /shortcut inside a ticket thread to send a canned reply.</i>")

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            sb.String(),
		ParseMode:       models.ParseModeHTML,
		ReplyMarkup:     keyboards.CannedResponsesKeyboard(responses),
	})
}

// HandleAddCanned handles the /addcanned command.
func (h *AdminHandlers) HandleAddCanned(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}
	if !config.IsAdmin(update.Message.From.ID) {
		return
	}

	command := firstField(update.Message.Text)
	payload := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, command))
	parts := strings.SplitN(payload, "|", 2)
	if len(parts) != 2 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "❌ Usage: /addcanned [shortcut] [title] | [content]",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	left := strings.Fields(strings.TrimSpace(parts[0]))
	if len(left) < 2 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "❌ Usage: /addcanned [shortcut] [title] | [content]",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	shortcut := sanitizeShortcut(left[0])
	title := strings.TrimSpace(strings.Join(left[1:], " "))
	content := strings.TrimSpace(parts[1])
	if shortcut == "" || title == "" || content == "" {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "❌ Shortcut, title, and content are all required.",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}
	if !isValidShortcut(shortcut) {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "❌ Shortcut must use only letters, numbers, and underscores.",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	err := h.deps.TicketService.CreateCannedResponse(ctx, shortcut, title, content, update.Message.From.ID)
	if err != nil {
		utils.LogError("create canned response", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "❌ Failed to save canned response. The shortcut may already exist.",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            fmt.Sprintf("✅ Saved canned response <code>/%s</code>.", shortcut),
		ParseMode:       models.ParseModeHTML,
	})
}

// HandleDelCanned handles the /delcanned command.
func (h *AdminHandlers) HandleDelCanned(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}
	if !config.IsAdmin(update.Message.From.ID) {
		return
	}

	parts := strings.Fields(update.Message.Text)
	if len(parts) < 2 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "❌ Usage: /delcanned [shortcut]",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	shortcut := sanitizeShortcut(parts[1])
	if shortcut == "" || !isValidShortcut(shortcut) {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "❌ Invalid canned response shortcut.",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	if err := h.deps.TicketService.DeleteCannedResponse(ctx, shortcut); err != nil {
		utils.LogError("delete canned response", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "❌ Failed to delete canned response.",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            fmt.Sprintf("✅ Deleted canned response <code>/%s</code>.", shortcut),
		ParseMode:       models.ParseModeHTML,
	})
}

// HandleAdminDynamicCommand handles unknown slash commands in admin ticket threads.
func (h *AdminHandlers) HandleAdminDynamicCommand(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	message := update.Message
	if !config.IsAdmin(message.From.ID) || message.Chat.ID != config.Cfg.SupportChatID {
		return
	}

	command := extractCommandName(message.Text)
	if command == "" || isBuiltInAdminCommand(command) {
		return
	}

	response, err := h.deps.TicketService.GetCannedResponse(ctx, command)
	if err != nil || response == nil {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            fmt.Sprintf("❌ Unknown admin command or canned response: <code>/%s</code>", utils.EscapeHTML(command)),
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	ticket, err := h.getTicketFromMessage(ctx, message)
	if err != nil || ticket == nil {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            "❌ Canned responses can only be sent from a ticket thread.",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	originalMessageID := int64(message.ID)
	if err := h.sendCannedResponseToTicket(ctx, b, ticket, message.From, response, &originalMessageID); err != nil {
		utils.LogError("send canned response", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            "❌ Failed to send canned response to the user.",
			ParseMode:       models.ParseModeHTML,
		})
		return
	}

	_, _ = b.SetMessageReaction(ctx, &bot.SetMessageReactionParams{
		ChatID:    message.Chat.ID,
		MessageID: message.ID,
		Reaction:  []models.ReactionType{{Type: "emoji", ReactionTypeEmoji: &models.ReactionTypeEmoji{Emoji: "✅"}}},
	})
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
	adminNameHTML := utils.EscapeHTML(adminName)

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
		Text:            fmt.Sprintf("🔒 <b>Ticket #%s closed by %s</b>", ticket.TicketNumber, adminNameHTML),
		ParseMode:       models.ParseModeHTML,
		ReplyMarkup:     keyboards.TicketActionsKeyboard(ticket.ID, false),
	})

	// Also notify in ticket thread if we're not already there
	if update.Message.MessageThreadID != int(ticket.ThreadID) && ticket.ThreadID > 1 {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          ticket.SupportChatID,
			MessageThreadID: int(ticket.ThreadID),
			Text:            fmt.Sprintf("🔒 <b>Ticket closed by %s</b>", adminNameHTML),
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
	adminNameHTML := utils.EscapeHTML(adminName)
	utils.LogTicket("Ticket #%s reopened by %s", ticket.TicketNumber, adminName)

	// Notify in thread
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            fmt.Sprintf("🔓 <b>Ticket reopened by %s</b>", adminNameHTML),
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
	if config.YAMLCfg != nil && !config.YAMLCfg.Tickets.PriorityEnabled {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "⚠️ Ticket priority changes are disabled in configuration.",
			ParseMode:       models.ParseModeHTML,
		})
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
		Text:            messages.FormatTicketDetailsForAdmin(ticket),
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
			displayName = "@" + utils.EscapeHTML(*u.UserUsername)
		} else if u.UserName != nil && *u.UserName != "" {
			displayName = utils.EscapeHTML(*u.UserName)
		}
		sb.WriteString(fmt.Sprintf("• %s", displayName))
		if u.Reason != nil && *u.Reason != "" {
			sb.WriteString(fmt.Sprintf(" - %s", utils.EscapeHTML(*u.Reason)))
		}
		sb.WriteString("\n")
	}

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      sb.String(),
		ParseMode: models.ParseModeHTML,
		ReplyMarkup: keyboards.BannedUsersKeyboard(users),
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
			adminName = utils.EscapeHTML(*n.AdminName)
		}
		sb.WriteString(fmt.Sprintf("• <b>%s</b> (%s):\n%s\n\n",
			adminName,
			n.CreatedAt.Format("02 Jan 15:04"),
			utils.EscapeHTML(n.NoteText)))
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
	if err != nil || ticket == nil {
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
		h.handleTicketCallback(ctx, b, callback, adminID, parts)

	case "priority":
		h.handlePriorityCallback(ctx, b, callback, parts)

	case "tickets":
		h.handleTicketsCallback(ctx, b, callback, parts)
	case "banned":
		h.handleBannedCallback(ctx, b, callback, parts)
	case "canned":
		h.handleCannedCallback(ctx, b, callback, parts)
	}
}

func (h *AdminHandlers) handleTicketCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, adminID int64, parts []string) {
	if callback.Message == nil || callback.Message.Message == nil || len(parts) < 3 {
		return
	}

	action := parts[1]
	ticketID, err := strconv.Atoi(parts[2])
	if err != nil {
		return
	}

	ticket, err := h.deps.TicketService.GetTicketByID(ctx, ticketID)
	if err != nil || ticket == nil {
		return
	}

	chatID := callback.Message.Message.Chat.ID
	messageID := callback.Message.Message.ID
	threadID := callback.Message.Message.MessageThreadID

	switch action {
	case "view":
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: threadID,
			Text:            messages.FormatTicketDetailsForAdmin(ticket),
			ParseMode:       models.ParseModeHTML,
			ReplyMarkup:     keyboards.TicketViewKeyboard(ticket.ID, ticket.IsOpen(), ticket.ThreadID),
		})

	case "close":
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        fmt.Sprintf("⚠️ <b>Close ticket #%s?</b>", ticket.TicketNumber),
			ParseMode:   models.ParseModeHTML,
			ReplyMarkup: keyboards.ConfirmCloseKeyboard(ticket.ID),
		})

	case "cancelclose":
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        messages.FormatTicketDetailsForAdmin(ticket),
			ParseMode:   models.ParseModeHTML,
			ReplyMarkup: keyboards.TicketViewKeyboard(ticket.ID, ticket.IsOpen(), ticket.ThreadID),
		})

	case "confirmclose":
		adminName := utils.GetUserFullName(&callback.From)
		adminNameHTML := utils.EscapeHTML(adminName)
		if err := h.deps.TicketService.CloseTicket(ctx, b, ticketID, adminID, adminName); err != nil {
			utils.LogError("close ticket via callback", err)
			return
		}

		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        fmt.Sprintf("🔒 <b>Ticket closed by %s</b>", adminNameHTML),
			ParseMode:   models.ParseModeHTML,
			ReplyMarkup: keyboards.TicketActionsKeyboard(ticketID, false),
		})

		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    ticket.UserID,
			Text:      messages.GetTicketClosedMessage(ticket.TicketNumber),
			ParseMode: models.ParseModeHTML,
		})

		h.deps.State.ClearUserState(ticket.UserID)

	case "reopen":
		if err := h.deps.TicketService.ReopenTicket(ctx, b, ticketID); err != nil {
			utils.LogError("reopen ticket via callback", err)
			return
		}

		adminName := utils.GetUserDisplayName(&callback.From)
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        fmt.Sprintf("🔓 <b>Ticket reopened by %s</b>", utils.EscapeHTML(adminName)),
			ParseMode:   models.ParseModeHTML,
			ReplyMarkup: keyboards.TicketActionsKeyboard(ticketID, true),
		})

		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    ticket.UserID,
			Text:      messages.GetTicketReopenedMessage(ticket.TicketNumber),
			ParseMode: models.ParseModeHTML,
		})

	case "info":
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: threadID,
			Text:            messages.FormatTicketDetailsForAdmin(ticket),
			ParseMode:       models.ParseModeHTML,
		})

	case "priority":
		if config.YAMLCfg != nil && !config.YAMLCfg.Tickets.PriorityEnabled {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:          chatID,
				MessageThreadID: threadID,
				Text:            "⚠️ Ticket priority changes are disabled in configuration.",
				ParseMode:       models.ParseModeHTML,
			})
			return
		}

		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: threadID,
			Text:            fmt.Sprintf("🎯 Select priority for ticket #%s:", ticket.TicketNumber),
			ParseMode:       models.ParseModeHTML,
			ReplyMarkup:     keyboards.PriorityKeyboard(ticketID),
		})

	case "notes":
		notes, err := h.deps.TicketService.GetTicketNotes(ctx, ticketID)
		if err != nil {
			utils.LogError("get notes via callback", err)
			return
		}

		if len(notes) == 0 {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:          chatID,
				MessageThreadID: threadID,
				Text:            fmt.Sprintf("📭 No notes for ticket #%s.", ticket.TicketNumber),
				ParseMode:       models.ParseModeHTML,
			})
			return
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("📝 <b>Notes for Ticket #%s:</b>\n\n", ticket.TicketNumber))
		for _, note := range notes {
			adminName := "Admin"
			if note.AdminName != nil {
				adminName = utils.EscapeHTML(*note.AdminName)
			}
			sb.WriteString(fmt.Sprintf("• <b>%s</b> (%s):\n%s\n\n",
				adminName,
				note.CreatedAt.Format("02 Jan 15:04"),
				utils.EscapeHTML(note.NoteText)))
		}

		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: threadID,
			Text:            sb.String(),
			ParseMode:       models.ParseModeHTML,
		})

	case "goto":
		if ticket.ThreadID <= 1 {
			return
		}

		internalChatID := strings.TrimPrefix(strconv.FormatInt(-config.Cfg.SupportChatID, 10), "100")
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: threadID,
			Text:            fmt.Sprintf("💬 <a href=\"https://t.me/c/%s/%d\">Go to ticket #%s topic</a>", internalChatID, ticket.ThreadID, ticket.TicketNumber),
			ParseMode:       models.ParseModeHTML,
		})
	}
}

func (h *AdminHandlers) handlePriorityCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, parts []string) {
	if callback.Message == nil || callback.Message.Message == nil || len(parts) < 3 {
		return
	}
	if config.YAMLCfg != nil && !config.YAMLCfg.Tickets.PriorityEnabled {
		return
	}

	ticketID, err := strconv.Atoi(parts[1])
	if err != nil {
		return
	}
	priority := parts[2]

	if err := h.deps.TicketService.UpdateTicketPriority(ctx, ticketID, priority); err != nil {
		utils.LogError("update priority via callback", err)
		return
	}

	_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    callback.Message.Message.Chat.ID,
		MessageID: callback.Message.Message.ID,
		Text:      fmt.Sprintf("✅ Priority set to <b>%s</b>", priority),
		ParseMode: models.ParseModeHTML,
	})
}

func (h *AdminHandlers) handleTicketsCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, parts []string) {
	if callback.Message == nil || callback.Message.Message == nil || len(parts) < 2 {
		return
	}

	filter := "all"
	page := 1

	switch parts[1] {
	case "page":
		if len(parts) >= 3 {
			if parsedPage, err := strconv.Atoi(parts[2]); err == nil && parsedPage > 0 {
				page = parsedPage
			}
		}
		if len(parts) >= 4 {
			filter = normalizeTicketFilter(parts[3])
		}
	case "refresh":
		if len(parts) >= 3 {
			filter = normalizeTicketFilter(parts[2])
		}
		if len(parts) >= 4 {
			if parsedPage, err := strconv.Atoi(parts[3]); err == nil && parsedPage > 0 {
				page = parsedPage
			}
		}
	default:
		return
	}

	if err := h.sendTicketList(ctx, b, callback.Message.Message.Chat.ID, callback.Message.Message.MessageThreadID, page, filter, callback.Message.Message.ID); err != nil {
		utils.LogError("refresh ticket list", err)
	}
}

func (h *AdminHandlers) handleBannedCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, parts []string) {
	if callback.Message == nil || callback.Message.Message == nil || len(parts) < 3 {
		return
	}

	userID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return
	}

	switch parts[1] {
	case "view":
		ban, err := h.deps.TicketService.GetBannedUser(ctx, userID)
		if err != nil || ban == nil {
			return
		}

		displayName := fmt.Sprintf("<code>%d</code>", ban.UserID)
		if ban.UserUsername != nil && *ban.UserUsername != "" {
			displayName = "@" + utils.EscapeHTML(*ban.UserUsername)
		} else if ban.UserName != nil && *ban.UserName != "" {
			displayName = utils.EscapeHTML(*ban.UserName)
		}

		reason := "No reason provided."
		if ban.Reason != nil && *ban.Reason != "" {
			reason = utils.EscapeHTML(*ban.Reason)
		}

		text := fmt.Sprintf(
			"🚫 <b>Banned User</b>\n\n"+
				"👤 <b>User:</b> %s\n"+
				"🆔 <b>ID:</b> <code>%d</code>\n"+
				"📝 <b>Reason:</b> %s\n"+
				"📅 <b>Banned At:</b> %s",
			displayName,
			ban.UserID,
			reason,
			ban.BannedAt.Format("02 Jan 2006 15:04"),
		)

		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      callback.Message.Message.Chat.ID,
			Text:        text,
			ParseMode:   models.ParseModeHTML,
			ReplyMarkup: keyboards.BannedUserActionsKeyboard(userID),
		})

	case "unban":
		if err := h.deps.TicketService.UnbanUser(ctx, userID); err != nil {
			utils.LogError("unban user via callback", err)
			return
		}

		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    callback.Message.Message.Chat.ID,
			Text:      fmt.Sprintf("✅ User <code>%d</code> has been unbanned.", userID),
			ParseMode: models.ParseModeHTML,
		})
	}
}

func (h *AdminHandlers) handleCannedCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, parts []string) {
	if callback.Message == nil || callback.Message.Message == nil || len(parts) < 3 || parts[1] != "use" {
		return
	}

	shortcut := sanitizeShortcut(parts[2])
	if shortcut == "" {
		return
	}

	response, err := h.deps.TicketService.GetCannedResponse(ctx, shortcut)
	if err != nil || response == nil {
		return
	}

	ticket, err := h.getTicketFromMessage(ctx, callback.Message.Message)
	if err != nil || ticket == nil {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    callback.Message.Message.Chat.ID,
			Text:      fmt.Sprintf("📝 <b>%s</b>\n\n%s", utils.EscapeHTML(response.Title), utils.EscapeHTML(response.Content)),
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	if err := h.sendCannedResponseToTicket(ctx, b, ticket, &callback.From, response, nil); err != nil {
		utils.LogError("send canned response via callback", err)
		return
	}

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          callback.Message.Message.Chat.ID,
		MessageThreadID: callback.Message.Message.MessageThreadID,
		Text:            fmt.Sprintf("✅ Sent canned response <code>/%s</code> to the user.", utils.EscapeHTML(shortcut)),
		ParseMode:       models.ParseModeHTML,
	})
}

func (h *AdminHandlers) sendTicketList(ctx context.Context, b *bot.Bot, chatID int64, threadID int, page int, filter string, editMessageID int) error {
	filter = normalizeTicketFilter(filter)
	if page < 1 {
		page = 1
	}

	totalCount, err := h.deps.TicketService.CountTicketsByFilter(ctx, filter)
	if err != nil {
		return err
	}

	const pageSize = 10
	totalPages := 1
	if totalCount > 0 {
		totalPages = (totalCount + pageSize - 1) / pageSize
	}
	if page > totalPages {
		page = totalPages
	}

	offset := (page - 1) * pageSize
	tickets, err := h.deps.TicketService.GetTicketsPage(ctx, filter, pageSize, offset)
	if err != nil {
		return err
	}

	text := messages.FormatAllTicketList(tickets, page, totalPages)
	text += fmt.Sprintf("\n<i>Filter: %s</i>", filter)
	if totalPages > 1 {
		text += "\n<i>Use /tickets [page] [open|closed|all]</i>"
	}
	text = utils.SanitizeUTF8(text)

	if editMessageID > 0 {
		_, err = b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   editMessageID,
			Text:        text,
			ParseMode:   models.ParseModeHTML,
			ReplyMarkup: keyboards.TicketListKeyboard(tickets, page, filter, page < totalPages),
		})
		return err
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          chatID,
		MessageThreadID: threadID,
		Text:            text,
		ParseMode:       models.ParseModeHTML,
		ReplyMarkup:     keyboards.TicketListKeyboard(tickets, page, filter, page < totalPages),
	})
	return err
}

func (h *AdminHandlers) sendCannedResponseToTicket(ctx context.Context, b *bot.Bot, ticket *database.Ticket, admin *models.User, response *database.CannedResponse, originalMessageID *int64) error {
	if ticket == nil {
		return fmt.Errorf("ticket is required")
	}
	if ticket.IsClosed() {
		return fmt.Errorf("ticket is closed")
	}

	sentMessage, err := h.deps.TicketService.SendSupportTextToUser(ctx, b, ticket, utils.EscapeHTML(response.Content))
	if err != nil {
		return err
	}

	ticketMessage := &database.TicketMessage{
		TicketID:       ticket.ID,
		SenderID:       admin.ID,
		SenderName:     utils.GetUserNamePtr(admin),
		SenderUsername: utils.GetUsernamePtr(admin),
		IsAdmin:        true,
		MessageText:    &response.Content,
		MessageID:      func() *int64 { id := int64(sentMessage.ID); return &id }(),
		OriginalMessageID: originalMessageID,
	}

	return h.deps.TicketService.SaveMessage(ctx, ticketMessage)
}

func parseTicketListArgs(text string) (int, string) {
	page := 1
	filter := "all"

	fields := strings.Fields(text)
	if len(fields) <= 1 {
		return page, filter
	}

	for _, part := range fields[1:] {
		if parsedPage, err := strconv.Atoi(part); err == nil && parsedPage > 0 {
			page = parsedPage
			continue
		}
		filter = normalizeTicketFilter(part)
	}

	return page, filter
}

func normalizeTicketFilter(filter string) string {
	switch strings.ToLower(filter) {
	case "open", "closed", "all":
		return strings.ToLower(filter)
	default:
		return "all"
	}
}

func firstField(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func sanitizeShortcut(shortcut string) string {
	shortcut = strings.TrimSpace(strings.TrimPrefix(shortcut, "/"))
	shortcut = strings.SplitN(shortcut, "@", 2)[0]
	return strings.ToLower(shortcut)
}

func isValidShortcut(shortcut string) bool {
	if shortcut == "" {
		return false
	}
	for _, r := range shortcut {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

func extractCommandName(text string) string {
	if !strings.HasPrefix(text, "/") {
		return ""
	}
	return sanitizeShortcut(firstField(text))
}

func isBuiltInAdminCommand(command string) bool {
	switch command {
	case "help", "tickets", "close", "reopen", "priority", "info", "ban", "unban", "banned", "note", "notes", "stats", "canned", "addcanned", "delcanned":
		return true
	default:
		return false
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

	ticket, err := h.deps.TicketService.GetTicketByThreadID(ctx, int64(message.MessageThreadID))
	if err != nil {
		return nil, err
	}
	if ticket == nil {
		return nil, fmt.Errorf("ticket not found for thread")
	}
	return ticket, nil
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
