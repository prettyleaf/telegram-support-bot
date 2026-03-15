package messages

import (
	"fmt"
	"strings"
	"time"

	"remnafamily/internal/config"
	"remnafamily/internal/database"
	"remnafamily/internal/utils"
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
Все следующие сообщения в этом чате будут автоматически добавляться в этот тикет.`
	} else {
		if config.YAMLCfg != nil {
			msg = config.YAMLCfg.Messages.TicketCreated
		}
		if msg == "" {
			msg = `✅ <b>Ticket #{ticket_number} created!</b>

Our support team has been notified and will respond shortly.
All next messages in this chat will be added to this ticket automatically.`
		}
	}
	return strings.ReplaceAll(msg, "{ticket_number}", ticketNumber)
}

// GetTicketWelcomeMessageLang returns the automated support welcome message in specified language.
func GetTicketWelcomeMessageLang(ticketNumber string, lang string) string {
	var msg string
	if lang == database.LangRussian {
		if config.YAMLCfg != nil {
			msg = config.YAMLCfg.Messages.TicketWelcomeRu
		}
		if msg == "" {
			msg = `👋 <b>Здравствуйте!</b>

Ваш тикет #{ticket_number} открыт. Наша команда поддержки скоро его посмотрит.

Если хотите, можете отправить дополнительные детали в этом чате.`
		}
	} else {
		if config.YAMLCfg != nil {
			msg = config.YAMLCfg.Messages.TicketWelcome
		}
		if msg == "" {
			msg = `👋 <b>Hello!</b>

Your ticket #{ticket_number} is now open. Our support team will review it shortly.

You can send any extra details in this chat at any time.`
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
		if config.YAMLCfg != nil {
			msg = config.YAMLCfg.Messages.TicketClosed
		}
		if msg == "" {
			msg = `🔒 <b>Ticket #{ticket_number} has been closed.</b>

If you need further assistance, you can create a new ticket with /newticket`
		}
	}
	return strings.ReplaceAll(msg, "{ticket_number}", ticketNumber)
}

// Ticket reopened message
func GetTicketReopenedMessage(ticketNumber string) string {
	msg := ""
	if config.YAMLCfg != nil {
		msg = config.YAMLCfg.Messages.TicketReopened
	}
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
			sb.WriteString(fmt.Sprintf("   📝 %s\n", truncate(utils.EscapeHTML(*t.Subject), 40)))
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
		sb.WriteString(fmt.Sprintf("%s <b>#%s</b> - %s\n", priorityEmoji, t.TicketNumber, utils.EscapeHTML(t.GetDisplayName())))
		if t.Subject != nil && *t.Subject != "" {
			sb.WriteString(fmt.Sprintf("   📝 %s\n", truncate(utils.EscapeHTML(*t.Subject), 40)))
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
		sb.WriteString(fmt.Sprintf("%s <b>#%s</b> - %s\n", statusEmoji, t.TicketNumber, utils.EscapeHTML(t.GetDisplayName())))
		sb.WriteString(fmt.Sprintf("   📊 %s", t.Status))
		if t.ClosedAt != nil && t.ClosedByName != nil {
			sb.WriteString(fmt.Sprintf(" (closed by %s)", utils.EscapeHTML(*t.ClosedByName)))
		}
		sb.WriteString("\n")
		if t.Subject != nil && *t.Subject != "" {
			sb.WriteString(fmt.Sprintf("   📝 %s\n", truncate(utils.EscapeHTML(*t.Subject), 40)))
		}
		sb.WriteString(fmt.Sprintf("   📅 %s\n\n", t.CreatedAt.Format("02 Jan 2006 15:04")))
	}

	return sb.String()
}

// FormatTicketDetailsForAdmin formats ticket details for admins.
func FormatTicketDetailsForAdmin(t *database.Ticket) string {
	return formatTicketDetails(t, true)
}

// FormatTicketDetailsForUser formats ticket details for users.
func FormatTicketDetailsForUser(t *database.Ticket) string {
	return formatTicketDetails(t, false)
}

func formatTicketDetails(t *database.Ticket, includeCloser bool) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("🎫 <b>Ticket #%s</b>\n\n", t.TicketNumber))
	sb.WriteString(fmt.Sprintf("%s <b>Status:</b> %s\n", getStatusEmoji(t.Status), strings.Title(t.Status)))
	sb.WriteString(fmt.Sprintf("%s <b>Priority:</b> %s\n", getPriorityEmoji(t.Priority), strings.Title(t.Priority)))
	sb.WriteString(fmt.Sprintf("👤 <b>User:</b> %s (ID: <code>%d</code>)\n", utils.EscapeHTML(t.GetDisplayName()), t.UserID))

	if t.Subject != nil && *t.Subject != "" {
		sb.WriteString(fmt.Sprintf("📝 <b>Subject:</b> %s\n", utils.EscapeHTML(*t.Subject)))
	}

	sb.WriteString(fmt.Sprintf("📅 <b>Created:</b> %s\n", t.CreatedAt.Format("02 Jan 2006 15:04")))

	if t.ClosedAt != nil {
		sb.WriteString(fmt.Sprintf("🔒 <b>Closed:</b> %s", t.ClosedAt.Format("02 Jan 2006 15:04")))
		if includeCloser && t.ClosedByName != nil {
			sb.WriteString(fmt.Sprintf(" by %s", utils.EscapeHTML(*t.ClosedByName)))
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
/stats [days] - View ticket statistics

<b>Canned Responses:</b>
/canned - List canned responses
/addcanned [shortcut] [title] | [content] - Add canned response
/delcanned [shortcut] - Delete canned response
/[shortcut] - Use canned response

Reply to messages in ticket threads to respond to users.`
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

// GetTicketCooldownError returns ticket creation cooldown message in specified language.
func GetTicketCooldownError(seconds int, lang string) string {
	if lang == database.LangRussian {
		return fmt.Sprintf("⏳ Пожалуйста, подождите ещё %d сек. перед созданием нового тикета.", seconds)
	}
	return fmt.Sprintf("⏳ Please wait %d more second(s) before creating a new ticket.", seconds)
}

// GetRateLimitError returns message rate limit error in specified language.
func GetRateLimitError(lang string) string {
	if lang == database.LangRussian {
		return "⏳ Слишком много сообщений. Пожалуйста, подождите немного и попробуйте снова."
	}
	return "⏳ Too many messages too quickly. Please wait a moment and try again."
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

// GetHWIDPromptMessage returns the prompt shown after an HWID-related trigger.
func GetHWIDPromptMessage(lang string) string {
	if lang == database.LangRussian {
		return "🧩 <b>Похоже, вопрос связан с HWID или подключёнными устройствами.</b>\n\nХотите изменить свои подключённые устройства прямо в панели?"
	}
	return "🧩 <b>Your message looks related to HWID or connected devices.</b>\n\nDo you want to manage your connected devices directly in the panel?"
}

// GetHWIDUserSelectionMessage asks the user to choose a panel account.
func GetHWIDUserSelectionMessage(lang string) string {
	if lang == database.LangRussian {
		return "👤 <b>Найдено несколько профилей панели.</b>\n\nВыберите, для какого профиля показать управление подключёнными устройствами."
	}
	return "👤 <b>Multiple panel users were found.</b>\n\nChoose which panel profile should be used for device management."
}

// GetHWIDPanelErrorMessage returns a generic panel integration error.
func GetHWIDPanelErrorMessage(lang string) string {
	if lang == database.LangRussian {
		return "⚠️ Не удалось открыть управление устройствами в панели. Администраторы уже увидят повышенный приоритет тикета."
	}
	return "⚠️ Failed to open panel device management. The ticket priority has still been escalated for admins."
}

// GetHWIDNoPanelUsersMessage returns the response when no panel profile is found for a Telegram user.
func GetHWIDNoPanelUsersMessage(lang string) string {
	if lang == database.LangRussian {
		return "⚠️ Я не нашёл профиль панели, связанный с вашим Telegram ID. Тикет уже поднят по приоритету, администраторы это увидят."
	}
	return "⚠️ I couldn't find a panel profile linked to your Telegram ID. The ticket has already been escalated for admins."
}

// GetHWIDTicketUnavailableMessage returns the response when the related ticket is no longer open.
func GetHWIDTicketUnavailableMessage(lang string) string {
	if lang == database.LangRussian {
		return "⚠️ Этот тикет уже недоступен для управления устройствами."
	}
	return "⚠️ This ticket is no longer available for device management."
}

// GetHWIDDeleteSuccessMessage confirms that a device has been removed.
func GetHWIDDeleteSuccessMessage(lang string) string {
	if lang == database.LangRussian {
		return "✅ Устройство удалено из панели."
	}
	return "✅ The device was removed from the panel."
}

// GetHWIDDeleteCooldownMessage returns the cooldown message for HWID deletion.
func GetHWIDDeleteCooldownMessage(remaining time.Duration, lang string) string {
	if remaining < time.Minute {
		remaining = time.Minute
	}
	hours := int(remaining.Hours())
	minutes := int(remaining.Minutes()) % 60

	if lang == database.LangRussian {
		if hours > 0 {
			return fmt.Sprintf("⏳ Следующее удаление будет доступно через %d ч. %d мин.", hours, minutes)
		}
		return fmt.Sprintf("⏳ Следующее удаление будет доступно через %d мин.", minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("⏳ The next device removal will be available in %d hour(s) %d minute(s).", hours, minutes)
	}
	return fmt.Sprintf("⏳ The next device removal will be available in %d minute(s).", minutes)
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
