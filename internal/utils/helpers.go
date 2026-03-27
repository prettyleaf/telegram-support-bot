package utils

import (
	"fmt"
	"strings"

	"github.com/go-telegram/bot/models"
)

// GetUserDisplayName returns a display name for a user
func GetUserDisplayName(user *models.User) string {
	if user == nil {
		return "Unknown"
	}
	if user.Username != "" {
		return "@" + user.Username
	}
	name := user.FirstName
	if user.LastName != "" {
		name += " " + user.LastName
	}
	return name
}

// GetUserFullName returns the full name of a user
func GetUserFullName(user *models.User) string {
	if user == nil {
		return "Unknown"
	}
	name := user.FirstName
	if user.LastName != "" {
		name += " " + user.LastName
	}
	return name
}

// GetUserNamePtr returns a pointer to the user's name
func GetUserNamePtr(user *models.User) *string {
	if user == nil {
		return nil
	}
	name := GetUserFullName(user)
	return &name
}

// GetUsernamePtr returns a pointer to the username
func GetUsernamePtr(user *models.User) *string {
	if user == nil || user.Username == "" {
		return nil
	}
	return &user.Username
}

// FormatTicketInfo formats ticket information for display
func FormatTicketInfo(ticketNumber, status, priority string, userDisplay string) string {
	statusEmoji := "🟢"
	switch status {
	case "closed":
		statusEmoji = "🔴"
	case "pending":
		statusEmoji = "🟡"
	case "resolved":
		statusEmoji = "✅"
	}

	priorityEmoji := "⚪"
	switch priority {
	case "low":
		priorityEmoji = "🔵"
	case "normal":
		priorityEmoji = "🟢"
	case "high":
		priorityEmoji = "🟠"
	case "urgent":
		priorityEmoji = "🔴"
	}

	return fmt.Sprintf(
		"🎫 <b>Ticket:</b> #%s\n"+
			"%s <b>Status:</b> %s\n"+
			"%s <b>Priority:</b> %s\n"+
			"👤 <b>User:</b> %s",
		ticketNumber,
		statusEmoji, strings.Title(status),
		priorityEmoji, strings.Title(priority),
		userDisplay,
	)
}

// EscapeHTML escapes HTML special characters
func EscapeHTML(s string) string {
	s = NormalizeUTF8(s)
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// NormalizeUTF8 replaces invalid UTF-8 byte sequences with the Unicode replacement rune.
func NormalizeUTF8(s string) string {
	return strings.ToValidUTF8(s, "\uFFFD")
}

// TruncateString truncates a string to a maximum length
func TruncateString(s string, maxLen int) string {
	return TruncateStringWithSuffix(s, maxLen, "...")
}

// TruncateStringWithSuffix truncates a string by rune count and appends a suffix when needed.
func TruncateStringWithSuffix(s string, maxLen int, suffix string) string {
	if maxLen <= 0 {
		return ""
	}

	s = NormalizeUTF8(s)
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}

	suffix = NormalizeUTF8(suffix)
	suffixRunes := []rune(suffix)
	if len(suffixRunes) >= maxLen {
		return string(suffixRunes[:maxLen])
	}

	return string(runes[:maxLen-len(suffixRunes)]) + suffix
}
