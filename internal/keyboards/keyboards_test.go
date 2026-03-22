package keyboards

import (
	"testing"

	"remnafamily/internal/database"
)

func TestNewTicketKeyboardRuIncludesMainMenuDevicesShortcut(t *testing.T) {
	keyboard := NewTicketKeyboardRu()
	if keyboard == nil || len(keyboard.InlineKeyboard) < 3 || len(keyboard.InlineKeyboard[2]) == 0 {
		t.Fatalf("expected devices shortcut row")
	}

	button := keyboard.InlineKeyboard[2][0]
	if button.Text != "📱 Устройства" {
		t.Fatalf("expected russian devices text, got %q", button.Text)
	}
	if button.CallbackData != "hwid:menu:main" {
		t.Fatalf("expected main menu devices callback, got %q", button.CallbackData)
	}
}

func TestNewTicketKeyboardRuIncludesMyTicketsShortcut(t *testing.T) {
	keyboard := NewTicketKeyboardRu()
	if keyboard == nil || len(keyboard.InlineKeyboard) < 2 || len(keyboard.InlineKeyboard[1]) == 0 {
		t.Fatalf("expected my tickets shortcut row")
	}

	button := keyboard.InlineKeyboard[1][0]
	if button.Text != "🎫 Мои тикеты" {
		t.Fatalf("expected russian my tickets text, got %q", button.Text)
	}
	if button.CallbackData != "menu:mytickets" {
		t.Fatalf("expected my tickets callback, got %q", button.CallbackData)
	}
}

func TestUserTicketListKeyboardIncludesListDevicesShortcut(t *testing.T) {
	keyboard := UserTicketListKeyboard(nil, database.LangRussian)
	if keyboard == nil || len(keyboard.InlineKeyboard) < 2 {
		t.Fatalf("expected keyboard rows")
	}

	actionsRow := keyboard.InlineKeyboard[len(keyboard.InlineKeyboard)-2]
	if len(actionsRow) != 2 {
		t.Fatalf("expected two quick action buttons, got %d", len(actionsRow))
	}
	if actionsRow[1].CallbackData != "hwid:menu:list" {
		t.Fatalf("expected ticket list devices callback, got %q", actionsRow[1].CallbackData)
	}
}

func TestUserTicketListKeyboardIncludesMainMenuBackButton(t *testing.T) {
	keyboard := UserTicketListKeyboard(nil, database.LangRussian)
	if keyboard == nil || len(keyboard.InlineKeyboard) == 0 {
		t.Fatalf("expected keyboard rows")
	}

	lastRow := keyboard.InlineKeyboard[len(keyboard.InlineKeyboard)-1]
	if len(lastRow) != 1 {
		t.Fatalf("expected single back button, got %d buttons", len(lastRow))
	}
	if lastRow[0].Text != "⬅️ Главное меню" {
		t.Fatalf("expected russian back text, got %q", lastRow[0].Text)
	}
	if lastRow[0].CallbackData != "menu:main" {
		t.Fatalf("expected main menu callback, got %q", lastRow[0].CallbackData)
	}
}

func TestUserTicketDetailsKeyboardIncludesCloseButtonForOpenTicket(t *testing.T) {
	keyboard := UserTicketDetailsKeyboard(42, true, database.LangRussian)
	if keyboard == nil || len(keyboard.InlineKeyboard) != 2 {
		t.Fatalf("expected close and navigation rows")
	}

	button := keyboard.InlineKeyboard[0][0]
	if button.Text != "🔒 Закрыть тикет" {
		t.Fatalf("expected close button text, got %q", button.Text)
	}
	if button.CallbackData != "myticket:close:42" {
		t.Fatalf("expected close callback, got %q", button.CallbackData)
	}

	navRow := keyboard.InlineKeyboard[1]
	if len(navRow) != 2 {
		t.Fatalf("expected two navigation buttons, got %d", len(navRow))
	}
	if navRow[0].CallbackData != "menu:mytickets" {
		t.Fatalf("expected back-to-list callback, got %q", navRow[0].CallbackData)
	}
	if navRow[1].CallbackData != "menu:main" {
		t.Fatalf("expected main menu callback, got %q", navRow[1].CallbackData)
	}
}

func TestUserTicketDetailsKeyboardForClosedTicketKeepsNavigationOnly(t *testing.T) {
	keyboard := UserTicketDetailsKeyboard(42, false, database.LangRussian)
	if keyboard == nil || len(keyboard.InlineKeyboard) != 1 {
		t.Fatalf("expected navigation row only")
	}

	row := keyboard.InlineKeyboard[0]
	if len(row) != 2 {
		t.Fatalf("expected two navigation buttons, got %d", len(row))
	}
	if row[0].Text != "⬅️ Назад" {
		t.Fatalf("expected back text, got %q", row[0].Text)
	}
	if row[1].Text != "🏠 Главное меню" {
		t.Fatalf("expected main menu text, got %q", row[1].Text)
	}
}
