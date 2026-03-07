package messages

import (
	"fmt"
	"strings"

	"remnafamily/internal/config"
	"remnafamily/internal/database"
)

// Language selection message
func GetLanguageSelectionMessage() string {
	return `🌐 <b>Please select your language / Пожалуйста, выберите язык:</b>`
}

// Welcome message
func GetWelcomeMessage() string {
	return GetWelcomeMessageLang("en")
}

// GetWelcomeMessageLang returns welcome message in specified language
func GetWelcomeMessageLang(lang string) string {
	if lang == database.LangRussian {
		return `👋 <b>Добро пожаловать в Поддержку!</b>

Отправьте мне сообщение, чтобы создать тикет.
Наша команда ответит как можно скорее.

<b>Команды:</b>
/start - Запустить бота
/newticket - Создать новый тикет
/mytickets - Посмотреть ваши тикеты
/cancel - Отменить текущую операцию
/language - Сменить язык`
	}
	
	if config.YAMLCfg != nil && config.YAMLCfg.Messages.Welcome != "" {
		return config.YAMLCfg.Messages.Welcome
	}
	return `👋 <b>Welcome to Support!</b>

Send me a message to create a support ticket.
Our team will respond as soon as possible.

<b>Commands:</b>
/start - Start the bot
/newticket - Create a new ticket
/mytickets - View your tickets
/cancel - Cancel current operation
/language - Change language`
}

// Ticket created message
func GetTicketCreatedMessage(ticketNumber string) string {
	return GetTicketCreatedMessageLang(ticketNumber, "en")
}

// GetTicketCreatedMessageLang returns ticket created message in specified language
func GetTicketCreatedMessageLang(ticketNumber string, lang string) string {
	var msg string
	if lang == database.LangRussian {
		msg = `✅ <b>Тикет #{ticket_number} создан!</b>

Наша команда поддержки уведомлена и скоро ответит.
Все ваши сообщения будут переданы команде поддержки.`
	} else {
		msg = config.YAMLCfg.Messages.TicketCreated
		if msg == "" {
			msg = `✅ <b>Ticket #{ticket_number} created!</b>

Our support team has been notified and will respond shortly.
All your messages will be forwarded to the support team.`
		}
	}
	return strings.ReplaceAll(msg, "{ticket_number}", ticketNumber)
}

// Ticket closed message
func GetTicketClosedMessage(ticketNumber string) string {
	return GetTicketClosedMessageLang(ticketNumber, "en")
}

// GetTicketClosedMessageLang returns ticket closed message in specified language
func GetTicketClosedMessageLang(ticketNumber string, lang string) string {
	var msg string
	if lang == database.LangRussian {
		msg = `🔒 <b>Тикет #{ticket_number} закрыт.</b>

Если вам нужна дополнительная помощь, вы можете создать новый тикет с помощью /newticket`
	} else {
		msg = config.YAMLCfg.Messages.TicketClosed
		if msg == "" {
			msg = `🔒 <b>Ticket #{ticket_number} has been closed.</b>

If you need further assistance, you can create a new ticket with /newticket`
		}
	}
	return strings.ReplaceAll(msg, "{ticket_number}", ticketNumber)
}

// Ticket reopened message
func GetTicketReopenedMessage(ticketNumber string) string {
	msg := config.YAMLCfg.Messages.TicketReopened
	if msg == "" {
		msg = `🔓 <b>Ticket #{ticket_number} has been reopened.</b>

You can continue the conversation.`
	}
	return strings.ReplaceAll(msg, "{ticket_number}", ticketNumber)
}

// User banned message
func GetUserBannedMessage() string {
	return GetUserBannedMessageLang("en")
}

// GetUserBannedMessageLang returns user banned message in specified language
func GetUserBannedMessageLang(lang string) string {
	if lang == database.LangRussian {
		return `⛔ <b>Вам запрещено создавать тикеты поддержки.</b>

Если вы считаете это ошибкой, пожалуйста, свяжитесь с администратором.`
	}
	if config.YAMLCfg != nil && config.YAMLCfg.Messages.UserBanned != "" {
		return config.YAMLCfg.Messages.UserBanned
	}
	return `⛔ <b>You have been banned from creating support tickets.</b>

If you believe this is a mistake, please contact an administrator.`
}

// Format ticket list for user
func FormatUserTicketList(tickets []*database.Ticket) string {
	return FormatUserTicketListLang(tickets, "en")
}

// FormatUserTicketListLang formats ticket list for user in specified language
func FormatUserTicketListLang(tickets []*database.Ticket, lang string) string {
	if len(tickets) == 0 {
		if lang == database.LangRussian {
			return "📭 У вас пока нет тикетов.\n\nИспользуйте /newticket для создания тикета."
		}
		return "📭 You have no tickets yet.\n\nUse /newticket to create a support ticket."
	}

	var sb strings.Builder
	if lang == database.LangRussian {
		sb.WriteString("📋 <b>Ваши тикеты:</b>\n\n")
	} else {
		sb.WriteString("📋 <b>Your Tickets:</b>\n\n")
	}

	for _, t := range tickets {
		statusEmoji := getStatusEmoji(t.Status)
		sb.WriteString(fmt.Sprintf("%s <b>#%s</b> - %s\n", statusEmoji, t.TicketNumber, t.Status))
		if t.Subject != nil && *t.Subject != "" {
			sb.WriteString(fmt.Sprintf("   📝 %s\n", truncate(*t.Subject, 40)))
		}
		sb.WriteString(fmt.Sprintf("   📅 %s\n\n", t.CreatedAt.Format("02 Jan 2006 15:04")))
	}

	return sb.String()
}

// Format ticket list for admins
func FormatAdminTicketList(tickets []*database.Ticket) string {
	if len(tickets) == 0 {
		return "📭 No open tickets."
	}

	var sb strings.Builder
	sb.WriteString("📋 <b>Open Tickets:</b>\n\n")

	for _, t := range tickets {
		priorityEmoji := getPriorityEmoji(t.Priority)
		sb.WriteString(fmt.Sprintf("%s <b>#%s</b> - %s\n", priorityEmoji, t.TicketNumber, t.GetDisplayName()))
		if t.Subject != nil && *t.Subject != "" {
			sb.WriteString(fmt.Sprintf("   📝 %s\n", truncate(*t.Subject, 40)))
		}
		sb.WriteString(fmt.Sprintf("   📅 %s\n\n", t.CreatedAt.Format("02 Jan 2006 15:04")))
	}

	return sb.String()
}

// FormatAllTicketList formats all tickets (open and closed) for admin view
func FormatAllTicketList(tickets []*database.Ticket, page, totalPages int) string {
	if len(tickets) == 0 {
		return "📭 No tickets found."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 <b>All Tickets</b> (Page %d/%d)\n\n", page, totalPages))

	for _, t := range tickets {
		statusEmoji := getStatusEmoji(t.Status)
		sb.WriteString(fmt.Sprintf("%s <b>#%s</b> - %s\n", statusEmoji, t.TicketNumber, t.GetDisplayName()))
		sb.WriteString(fmt.Sprintf("   📊 %s", t.Status))
		if t.ClosedAt != nil && t.ClosedByName != nil {
			sb.WriteString(fmt.Sprintf(" (closed by %s)", *t.ClosedByName))
		}
		sb.WriteString("\n")
		if t.Subject != nil && *t.Subject != "" {
			sb.WriteString(fmt.Sprintf("   📝 %s\n", truncate(*t.Subject, 40)))
		}
		sb.WriteString(fmt.Sprintf("   📅 %s\n\n", t.CreatedAt.Format("02 Jan 2006 15:04")))
	}

	return sb.String()
}

// Format ticket details
func FormatTicketDetails(t *database.Ticket) string {
	var sb strings.Builder
	
	sb.WriteString(fmt.Sprintf("🎫 <b>Ticket #%s</b>\n\n", t.TicketNumber))
	sb.WriteString(fmt.Sprintf("%s <b>Status:</b> %s\n", getStatusEmoji(t.Status), strings.Title(t.Status)))
	sb.WriteString(fmt.Sprintf("%s <b>Priority:</b> %s\n", getPriorityEmoji(t.Priority), strings.Title(t.Priority)))
	sb.WriteString(fmt.Sprintf("👤 <b>User:</b> %s (ID: <code>%d</code>)\n", t.GetDisplayName(), t.UserID))
	
	if t.Subject != nil && *t.Subject != "" {
		sb.WriteString(fmt.Sprintf("📝 <b>Subject:</b> %s\n", *t.Subject))
	}
	
	sb.WriteString(fmt.Sprintf("📅 <b>Created:</b> %s\n", t.CreatedAt.Format("02 Jan 2006 15:04")))
	
	if t.ClosedAt != nil {
		sb.WriteString(fmt.Sprintf("🔒 <b>Closed:</b> %s", t.ClosedAt.Format("02 Jan 2006 15:04")))
		if t.ClosedByName != nil {
			sb.WriteString(fmt.Sprintf(" by %s", *t.ClosedByName))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// Admin help message
func GetAdminHelpMessage() string {
	return `👨‍💼 <b>Admin Commands:</b>

<b>Ticket Management:</b>
/tickets [page] [open|closed|all] - List tickets
/close [ticket_number] - Close ticket
/reopen [ticket_number] - Reopen ticket
/priority [ticket_number] [low|normal|high|urgent] - Set priority
/info [ticket_number] - View ticket details
/note [ticket_number] [text] - Add internal note
/notes [ticket_number] - View ticket notes

<b>User Management:</b>
/ban [user_id] [reason] - Ban user from tickets
/unban [user_id] - Unban user
/banned - List banned users

<b>Utilities:</b>
/note [text] - Add internal note to ticket
/notes - View ticket notes
/info - View ticket details
/stats - View ticket statistics

<b>Canned Responses:</b>
/canned - List canned responses
/addcanned [shortcut] [title] | [content] - Add canned response
/delcanned [shortcut] - Delete canned response
/[shortcut] - Use canned response

Reply to messages in ticket threads to respond to users.`
}

// GetMessageSentConfirmation returns message sent confirmation in specified language
func GetMessageSentConfirmation(lang string) string {
	if lang == database.LangRussian {
		return "✅ Сообщение отправлено команде поддержки."
	}
	return "✅ Message sent to support team."
}

// GetMessageSendError returns message send error in specified language
func GetMessageSendError(lang string) string {
	if lang == database.LangRussian {
		return "⚠️ Не удалось отправить сообщение. Пожалуйста, попробуйте снова."
	}
	return "⚠️ Failed to send your message. Please try again."
}

// GetTicketCreateError returns ticket creation error in specified language
func GetTicketCreateError(lang string) string {
	if lang == database.LangRussian {
		return "❌ Не удалось создать тикет. Пожалуйста, попробуйте позже."
	}
	return "❌ Failed to create ticket. Please try again later."
}

// GetMaxTicketsError returns max tickets error in specified language
func GetMaxTicketsError(count int, lang string) string {
	if lang == database.LangRussian {
		return fmt.Sprintf("⚠️ У вас уже есть %d открытых тикетов. Пожалуйста, дождитесь их решения перед созданием нового.", count)
	}
	return fmt.Sprintf("⚠️ You already have %d open ticket(s). Please wait for them to be resolved before creating a new one.", count)
}

// GetOperationCancelled returns operation cancelled message in specified language
func GetOperationCancelled(lang string) string {
	if lang == database.LangRussian {
		return "✅ Операция отменена."
	}
	return "✅ Operation cancelled."
}

// GetLanguageChanged returns language changed message
func GetLanguageChanged(lang string) string {
	if lang == database.LangRussian {
		return "✅ Язык изменён на Русский."
	}
	return "✅ Language changed to English."
}

// Helper functions
func getStatusEmoji(status string) string {
	switch status {
	case "open":
		return "🟢"
	case "closed":
		return "🔴"
	case "pending":
		return "🟡"
	case "resolved":
		return "✅"
	default:
		return "⚪"
	}
}

func getPriorityEmoji(priority string) string {
	switch priority {
	case "low":
		return "🔵"
	case "normal":
		return "🟢"
	case "high":
		return "🟠"
	case "urgent":
		return "🔴"
	default:
		return "⚪"
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
