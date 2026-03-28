package utils

import "testing"

func TestSanitizeTelegramHTML_AllowsWhitelistedTags(t *testing.T) {
	input := `<b>Hello</b> <i>world</i> <tg-spoiler>secret</tg-spoiler> <script>alert(1)</script>`
	got := SanitizeTelegramHTML(input)
	want := `<b>Hello</b> <i>world</i> <tg-spoiler>secret</tg-spoiler> &lt;script&gt;alert(1)&lt;/script&gt;`
	if got != want {
		t.Fatalf("unexpected sanitized html:\nwant: %q\n got: %q", want, got)
	}
}

func TestRichTextHTMLForDisplay_PreservesFormattingAndLineBreaks(t *testing.T) {
	input := "<blockquote>Quoted</blockquote>\n<b>Bold</b>"
	got := RichTextHTMLForDisplay(input)
	want := "<blockquote>Quoted</blockquote><br /><b>Bold</b>"
	if got != want {
		t.Fatalf("unexpected display html:\nwant: %q\n got: %q", want, got)
	}
}
