package utils

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateStringPreservesUTF8(t *testing.T) {
	input := strings.Repeat("a", 36) + "Ж" + strings.Repeat("b", 10)

	got := TruncateString(input, 40)

	if !utf8.ValidString(got) {
		t.Fatalf("expected valid UTF-8, got %q", got)
	}
	if got != strings.Repeat("a", 36)+"Ж..." {
		t.Fatalf("unexpected truncation result: %q", got)
	}
}

func TestEscapeHTMLNormalizesInvalidUTF8(t *testing.T) {
	input := string([]byte{'<', 0xff, '>'})

	got := EscapeHTML(input)

	if !utf8.ValidString(got) {
		t.Fatalf("expected valid UTF-8, got %q", got)
	}
	if got != "&lt;�&gt;" {
		t.Fatalf("unexpected escaped value: %q", got)
	}
}
