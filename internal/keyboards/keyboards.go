package keyboards

import (
	"fmt"

	"remnafamily/internal/database"
	"remnafamily/internal/utils"

	"github.com/go-telegram/bot/models"
)

// CallbackOption is a small helper for inline keyboards with dynamic callbacks.
type CallbackOption struct {
	Text string
	Data string
}

// NewTicketKeyboard creates the keyboard for starting a new ticket
func NewTicketKeyboard() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "📝 Create New Ticket", CallbackData: "ticket:create"},
			},
			{
				{Text: "🎫 My Tickets", CallbackData: "menu:mytickets"},
			},
			{
				{Text: "📱 Devices", CallbackData: "hwid:menu:main"},
			},
		},
	}
}

// NewTicketKeyboardRu creates the keyboard for starting a new ticket (Russian)
func NewTicketKeyboardRu() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "📝 Создать тикет", CallbackData: "ticket:create"},
			},
			{
				{Text: "🎫 Мои тикеты", CallbackData: "menu:mytickets"},
			},
			{
				{Text: "📱 Устройства", CallbackData: "hwid:menu:main"},
			},
		},
	}
}

// LanguageSelectionKeyboard creates the keyboard for language selection
func LanguageSelectionKeyboard() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🇬🇧 English", CallbackData: "lang:en"},
				{Text: "🇷🇺 Русский", CallbackData: "lang:ru"},
			},
		},
	}
}

// CaptchaKeyboard creates an inline keyboard for onboarding captcha answers.
func CaptchaKeyboard(sessionID string, options []string) *models.InlineKeyboardMarkup {
	buttons := make([][]models.InlineKeyboardButton, 0, (len(options)+1)/2)
	row := make([]models.InlineKeyboardButton, 0, 2)

	for idx, option := range options {
		row = append(row, models.InlineKeyboardButton{
			Text:         option,
			CallbackData: fmt.Sprintf("captcha:%s:%d", sessionID, idx),
		})
		if len(row) == 2 || idx == len(options)-1 {
			buttons = append(buttons, row)
			row = make([]models.InlineKeyboardButton, 0, 2)
		}
	}

	return &models.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

// TicketActionsKeyboard creates the keyboard for ticket actions (admin)
func TicketActionsKeyboard(ticketID int, isOpen bool) *models.InlineKeyboardMarkup {
	var buttons [][]models.InlineKeyboardButton

	if isOpen {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{Text: "🔒 Close Ticket", CallbackData: fmt.Sprintf("ticket:close:%d", ticketID)},
		})
	} else {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{Text: "🔓 Reopen Ticket", CallbackData: fmt.Sprintf("ticket:reopen:%d", ticketID)},
		})
	}

	buttons = append(buttons, []models.InlineKeyboardButton{
		{Text: "📊 View Details", CallbackData: fmt.Sprintf("ticket:info:%d", ticketID)},
		{Text: "📤 Export", CallbackData: fmt.Sprintf("ticket:export:%d", ticketID)},
	})

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

// TicketReopenKeyboard creates a compact keyboard with a reopen action.
func TicketReopenKeyboard(ticketID int) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🔓 Reopen Ticket", CallbackData: fmt.Sprintf("ticket:reopen:%d", ticketID)},
			},
		},
	}
}

// TicketViewKeyboard creates the keyboard for viewing a ticket from ticket list
func TicketViewKeyboard(ticketID int, isOpen bool, threadID int64) *models.InlineKeyboardMarkup {
	var buttons [][]models.InlineKeyboardButton

	// First row: Open/Close action
	if isOpen {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{Text: "🔒 Close Ticket", CallbackData: fmt.Sprintf("ticket:close:%d", ticketID)},
		})
	} else {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{Text: "🔓 Reopen Ticket", CallbackData: fmt.Sprintf("ticket:reopen:%d", ticketID)},
		})
	}

	// Second row: Priority and Notes
	buttons = append(buttons, []models.InlineKeyboardButton{
		{Text: "🎯 Set Priority", CallbackData: fmt.Sprintf("ticket:priority:%d", ticketID)},
		{Text: "📝 Notes", CallbackData: fmt.Sprintf("ticket:notes:%d", ticketID)},
	})

	buttons = append(buttons, []models.InlineKeyboardButton{
		{Text: "📤 Export", CallbackData: fmt.Sprintf("ticket:export:%d", ticketID)},
	})

	// Third row: Link to topic if thread exists
	if threadID > 1 {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{Text: "💬 Go to Topic", CallbackData: fmt.Sprintf("ticket:goto:%d", ticketID)},
		})
	}

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

// PriorityKeyboard creates the keyboard for setting ticket priority
func PriorityKeyboard(ticketID int) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🔵 Low", CallbackData: fmt.Sprintf("priority:%d:low", ticketID)},
				{Text: "🟢 Normal", CallbackData: fmt.Sprintf("priority:%d:normal", ticketID)},
			},
			{
				{Text: "🟠 High", CallbackData: fmt.Sprintf("priority:%d:high", ticketID)},
				{Text: "🔴 Urgent", CallbackData: fmt.Sprintf("priority:%d:urgent", ticketID)},
			},
		},
	}
}

// TicketListKeyboard creates the keyboard for a paginated ticket list.
func TicketListKeyboard(tickets []*database.Ticket, page int, filter string, hasNextPage bool) *models.InlineKeyboardMarkup {
	var buttons [][]models.InlineKeyboardButton

	for _, t := range tickets {
		statusEmoji := "🟢"
		if t.Status == "closed" {
			statusEmoji = "🔴"
		}

		text := fmt.Sprintf("%s #%s - %s", statusEmoji, t.TicketNumber, t.GetDisplayName())
		text = utils.TruncateString(text, 40)

		buttons = append(buttons, []models.InlineKeyboardButton{
			{Text: text, CallbackData: fmt.Sprintf("ticket:view:%d", t.ID)},
		})
	}

	// Pagination
	var navButtons []models.InlineKeyboardButton
	if page > 1 {
		navButtons = append(navButtons, models.InlineKeyboardButton{
			Text: "⬅️ Previous", CallbackData: fmt.Sprintf("tickets:page:%d:%s", page-1, filter),
		})
	}
	navButtons = append(navButtons, models.InlineKeyboardButton{
		Text: "🔄 Refresh", CallbackData: fmt.Sprintf("tickets:refresh:%s:%d", filter, page),
	})
	if hasNextPage {
		navButtons = append(navButtons, models.InlineKeyboardButton{
			Text: "➡️ Next", CallbackData: fmt.Sprintf("tickets:page:%d:%s", page+1, filter),
		})
	}

	if len(navButtons) > 0 {
		buttons = append(buttons, navButtons)
	}

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

// UserTicketListKeyboard creates the keyboard for user's ticket list
func UserTicketListKeyboard(tickets []*database.Ticket, lang string) *models.InlineKeyboardMarkup {
	var buttons [][]models.InlineKeyboardButton
	createTicketText := "📝 Create New Ticket"
	devicesText := "📱 Devices"
	backText := "⬅️ Main menu"
	if lang == database.LangRussian {
		createTicketText = "📝 Создать тикет"
		devicesText = "📱 Устройства"
		backText = "⬅️ Главное меню"
	}

	for _, t := range tickets {
		statusEmoji := "🟢"
		if t.Status == "closed" {
			statusEmoji = "🔴"
		}

		text := fmt.Sprintf("%s #%s", statusEmoji, t.TicketNumber)
		if t.Subject != nil && *t.Subject != "" {
			subject := utils.TruncateString(*t.Subject, 20)
			text += " - " + subject
		}

		buttons = append(buttons, []models.InlineKeyboardButton{
			{Text: text, CallbackData: fmt.Sprintf("myticket:view:%d", t.ID)},
		})
	}

	buttons = append(buttons, []models.InlineKeyboardButton{
		{Text: createTicketText, CallbackData: "ticket:create"},
		{Text: devicesText, CallbackData: "hwid:menu:list"},
	})
	buttons = append(buttons, []models.InlineKeyboardButton{
		{Text: backText, CallbackData: "menu:main"},
	})

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

// UserTicketDetailsKeyboard creates actions for a user's ticket details view.
func UserTicketDetailsKeyboard(ticketID int, isOpen bool, lang string) *models.InlineKeyboardMarkup {
	closeText := "🔒 Close Ticket"
	backText := "⬅️ Back"
	mainMenuText := "🏠 Main menu"
	if lang == database.LangRussian {
		closeText = "🔒 Закрыть тикет"
		backText = "⬅️ Назад"
		mainMenuText = "🏠 Главное меню"
	}

	buttons := make([][]models.InlineKeyboardButton, 0, 2)
	if isOpen {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{Text: closeText, CallbackData: fmt.Sprintf("myticket:close:%d", ticketID)},
		})
	}
	buttons = append(buttons, []models.InlineKeyboardButton{
		{Text: backText, CallbackData: "menu:mytickets"},
		{Text: mainMenuText, CallbackData: "menu:main"},
	})

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

// UserCloseTicketConfirmKeyboard creates confirmation actions for a user closing a ticket.
func UserCloseTicketConfirmKeyboard(ticketID int, lang string) *models.InlineKeyboardMarkup {
	confirmText := "✅ Yes, Close"
	cancelText := "❌ Cancel"
	if lang == database.LangRussian {
		confirmText = "✅ Да, закрыть"
		cancelText = "❌ Отмена"
	}

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: confirmText, CallbackData: fmt.Sprintf("myticket:confirmclose:%d", ticketID)},
				{Text: cancelText, CallbackData: fmt.Sprintf("myticket:cancelclose:%d", ticketID)},
			},
		},
	}
}

// HWIDPromptKeyboard asks the user whether they want to manage panel devices.
func HWIDPromptKeyboard(ticketID int, lang string) *models.InlineKeyboardMarkup {
	yesText := "✅ Yes"
	noText := "❌ No"
	if lang == database.LangRussian {
		yesText = "✅ Да"
		noText = "❌ Нет"
	}

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: yesText, CallbackData: fmt.Sprintf("hwid:y:%d", ticketID)},
				{Text: noText, CallbackData: fmt.Sprintf("hwid:n:%d", ticketID)},
			},
		},
	}
}

// HWIDUserSelectionKeyboard builds account selection buttons for panel users.
func HWIDUserSelectionKeyboard(options []CallbackOption, refreshLabel, refreshCallback, backLabel, backCallback string) *models.InlineKeyboardMarkup {
	buttons := make([][]models.InlineKeyboardButton, 0, len(options)+2)
	for _, option := range options {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{Text: option.Text, CallbackData: option.Data},
		})
	}
	buttons = append(buttons, []models.InlineKeyboardButton{
		{Text: refreshLabel, CallbackData: refreshCallback},
	})
	if backLabel != "" && backCallback != "" {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{Text: backLabel, CallbackData: backCallback},
		})
	}

	return &models.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

// HWIDManagementKeyboard builds the device management actions.
func HWIDManagementKeyboard(deviceButtons []CallbackOption, refreshLabel, refreshCallback, backLabel, backCallback string) *models.InlineKeyboardMarkup {
	buttons := make([][]models.InlineKeyboardButton, 0, len(deviceButtons)+2)
	for _, option := range deviceButtons {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{Text: option.Text, CallbackData: option.Data},
		})
	}
	buttons = append(buttons, []models.InlineKeyboardButton{
		{Text: refreshLabel, CallbackData: refreshCallback},
	})
	if backLabel != "" && backCallback != "" {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{Text: backLabel, CallbackData: backCallback},
		})
	}

	return &models.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

// ConfirmCloseKeyboard creates confirmation keyboard for closing ticket
func ConfirmCloseKeyboard(ticketID int) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ Yes, Close", CallbackData: fmt.Sprintf("ticket:confirmclose:%d", ticketID)},
				{Text: "❌ Cancel", CallbackData: fmt.Sprintf("ticket:cancelclose:%d", ticketID)},
			},
		},
	}
}

// BannedUsersKeyboard creates keyboard for banned users list
func BannedUsersKeyboard(users []*database.BannedUser) *models.InlineKeyboardMarkup {
	var buttons [][]models.InlineKeyboardButton

	for _, u := range users {
		displayName := fmt.Sprintf("%d", u.UserID)
		if u.UserUsername != nil && *u.UserUsername != "" {
			displayName = "@" + *u.UserUsername
		} else if u.UserName != nil && *u.UserName != "" {
			displayName = *u.UserName
		}

		buttons = append(buttons, []models.InlineKeyboardButton{
			{Text: fmt.Sprintf("🚫 %s", displayName), CallbackData: fmt.Sprintf("banned:view:%d", u.UserID)},
		})
	}

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

// BannedUserActionsKeyboard creates actions for a specific banned user.
func BannedUserActionsKeyboard(userID int64) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ Unban User", CallbackData: fmt.Sprintf("banned:unban:%d", userID)},
			},
		},
	}
}

// CannedResponsesKeyboard creates keyboard for canned responses
func CannedResponsesKeyboard(responses []*database.CannedResponse) *models.InlineKeyboardMarkup {
	var buttons [][]models.InlineKeyboardButton

	for _, r := range responses {
		text := fmt.Sprintf("/%s - %s", r.Shortcut, r.Title)
		text = utils.TruncateString(text, 40)

		buttons = append(buttons, []models.InlineKeyboardButton{
			{Text: text, CallbackData: fmt.Sprintf("canned:use:%s", r.Shortcut)},
		})
	}

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

// CancelKeyboard creates a simple cancel button
func CancelKeyboard() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "❌ Cancel", CallbackData: "cancel"},
			},
		},
	}
}
