package config

import "testing"

func TestGetOptionalIntEnvReturnsZeroWhenMissing(t *testing.T) {
	t.Setenv("HWID_DELETE_LOG_THREAD_ID", "")

	value, err := getOptionalIntEnv("HWID_DELETE_LOG_THREAD_ID")
	if err != nil {
		t.Fatalf("expected no error for empty value, got %v", err)
	}
	if value != 0 {
		t.Fatalf("expected zero, got %d", value)
	}
}

func TestGetOptionalIntEnvParsesValue(t *testing.T) {
	t.Setenv("HWID_DELETE_LOG_THREAD_ID", "42")

	value, err := getOptionalIntEnv("HWID_DELETE_LOG_THREAD_ID")
	if err != nil {
		t.Fatalf("expected parsed int, got error %v", err)
	}
	if value != 42 {
		t.Fatalf("expected 42, got %d", value)
	}
}

func TestNormalizeWebAdminBasePath(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		publicURL string
		want      string
	}{
		{
			name:      "explicit admin path",
			raw:       "/admin",
			publicURL: "https://support.example.com",
			want:      "/admin",
		},
		{
			name:      "explicit root path",
			raw:       "/",
			publicURL: "https://support.example.com/admin",
			want:      "/",
		},
		{
			name:      "infer admin path from public url",
			raw:       "",
			publicURL: "https://support.example.com/admin",
			want:      "/admin",
		},
		{
			name:      "infer root path from public url",
			raw:       "",
			publicURL: "https://support.example.com",
			want:      "/",
		},
		{
			name:      "add leading slash and trim trailing slash",
			raw:       "admin/",
			publicURL: "https://support.example.com",
			want:      "/admin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeWebAdminBasePath(tt.raw, tt.publicURL)
			if got != tt.want {
				t.Fatalf("normalizeWebAdminBasePath(%q, %q) = %q, want %q", tt.raw, tt.publicURL, got, tt.want)
			}
		})
	}
}
