package messages

import (
	"strings"
	"testing"
	"unicode/utf8"

	"remnafamily/internal/database"
)

func TestFormatUserTicketListLangKeepsValidUTF8WhenSubjectIsTruncated(t *testing.T) {
	subject := strings.Repeat("a", 36) + "Ж" + strings.Repeat("b", 10)
	text := FormatUserTicketListLang([]*database.Ticket{
		{
			ID:           1,
			TicketNumber: "42",
			Status:       "open",
			Subject:      &subject,
		},
	}, database.LangRussian)

	if !utf8.ValidString(text) {
		t.Fatalf("expected valid UTF-8, got %q", text)
	}
}
