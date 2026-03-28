package utils

import "strings"

var richTextTagReplacer = strings.NewReplacer(
	"&lt;b&gt;", "<b>",
	"&lt;/b&gt;", "</b>",
	"&lt;strong&gt;", "<strong>",
	"&lt;/strong&gt;", "</strong>",
	"&lt;i&gt;", "<i>",
	"&lt;/i&gt;", "</i>",
	"&lt;em&gt;", "<em>",
	"&lt;/em&gt;", "</em>",
	"&lt;u&gt;", "<u>",
	"&lt;/u&gt;", "</u>",
	"&lt;ins&gt;", "<ins>",
	"&lt;/ins&gt;", "</ins>",
	"&lt;s&gt;", "<s>",
	"&lt;/s&gt;", "</s>",
	"&lt;strike&gt;", "<strike>",
	"&lt;/strike&gt;", "</strike>",
	"&lt;del&gt;", "<del>",
	"&lt;/del&gt;", "</del>",
	"&lt;code&gt;", "<code>",
	"&lt;/code&gt;", "</code>",
	"&lt;pre&gt;", "<pre>",
	"&lt;/pre&gt;", "</pre>",
	"&lt;blockquote&gt;", "<blockquote>",
	"&lt;/blockquote&gt;", "</blockquote>",
	"&lt;tg-spoiler&gt;", "<tg-spoiler>",
	"&lt;/tg-spoiler&gt;", "</tg-spoiler>",
)

// SanitizeTelegramHTML escapes arbitrary HTML and then restores a small
// whitelist of Telegram-compatible formatting tags.
func SanitizeTelegramHTML(s string) string {
	s = strings.TrimSpace(NormalizeUTF8(s))
	if s == "" {
		return ""
	}
	return richTextTagReplacer.Replace(EscapeHTML(s))
}

// RichTextHTMLForDisplay converts rich ticket text into safe display HTML.
func RichTextHTMLForDisplay(s string) string {
	s = strings.ReplaceAll(NormalizeUTF8(s), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = SanitizeTelegramHTML(s)
	if s == "" {
		return ""
	}
	return strings.ReplaceAll(s, "\n", "<br />")
}
