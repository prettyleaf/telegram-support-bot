package services

import "testing"

func TestEscapedTelegramCaption(t *testing.T) {
	t.Parallel()

	if got := escapedTelegramCaption("Hello <b>world</b>"); got != "Hello &lt;b&gt;world&lt;/b&gt;" {
		t.Fatalf("escapedTelegramCaption() = %q, want %q", got, "Hello &lt;b&gt;world&lt;/b&gt;")
	}
}

func TestEscapedTelegramCaptionBlank(t *testing.T) {
	t.Parallel()

	if got := escapedTelegramCaption("   "); got != "" {
		t.Fatalf("escapedTelegramCaption() = %q, want empty string", got)
	}
}

func TestBuildSupportUserBody(t *testing.T) {
	t.Parallel()

	if got := buildSupportUserBody(nil, "<b>Hello</b>"); got != "<b>Hello</b>" {
		t.Fatalf("buildSupportUserBody() = %q, want %q", got, "<b>Hello</b>")
	}
}

func TestBuildSupportUserBodyEmpty(t *testing.T) {
	t.Parallel()

	if got := buildSupportUserBody(nil, ""); got != "" {
		t.Fatalf("buildSupportUserBody() = %q, want empty string", got)
	}
}
