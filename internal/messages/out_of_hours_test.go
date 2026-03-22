package messages

import (
	"strings"
	"testing"

	"remnafamily/internal/config"
	"remnafamily/internal/database"
)

func TestGetTicketOutOfHoursMessageLangIncludesWorkingHours(t *testing.T) {
	previousCfg := config.YAMLCfg
	config.YAMLCfg = nil
	t.Cleanup(func() {
		config.YAMLCfg = previousCfg
	})

	got := GetTicketOutOfHoursMessageLang("42", "10:00-19:00 Europe/Moscow", database.LangRussian)

	if !strings.Contains(got, "42") {
		t.Fatalf("expected ticket number in message, got %q", got)
	}
	if !strings.Contains(got, "10:00-19:00 Europe/Moscow") {
		t.Fatalf("expected working hours in message, got %q", got)
	}
}
