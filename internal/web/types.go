package web

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"remnafamily/internal/database"

	"github.com/go-webauthn/webauthn/webauthn"
)

type adminSession struct {
	ID          string    `json:"id"`
	AdminUserID int       `json:"adminUserId"`
	AuthMethod  string    `json:"authMethod"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

type githubState struct {
	State string `json:"state"`
}

type telegramLoginPayload struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	PhotoURL  string `json:"photo_url"`
	AuthDate  int64  `json:"auth_date"`
	Hash      string `json:"hash"`
}

type apiResponse struct {
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

type passkeyUser struct {
	user        *database.WebAdminUser
	credentials []webauthn.Credential
}

func (u *passkeyUser) WebAuthnID() []byte {
	return u.user.WebauthnUserID
}

func (u *passkeyUser) WebAuthnName() string {
	if u.user.Username != nil && *u.user.Username != "" {
		return *u.user.Username
	}
	if u.user.GitHubEmail != nil && *u.user.GitHubEmail != "" {
		return *u.user.GitHubEmail
	}
	if u.user.GitHubLogin != nil && *u.user.GitHubLogin != "" {
		return *u.user.GitHubLogin
	}
	if u.user.TelegramID != nil {
		return fmt.Sprintf("telegram-%d", *u.user.TelegramID)
	}
	return fmt.Sprintf("admin-%d", u.user.ID)
}

func (u *passkeyUser) WebAuthnDisplayName() string {
	return u.user.DisplayName
}

func (u *passkeyUser) WebAuthnCredentials() []webauthn.Credential {
	return u.credentials
}

func decodeStoredCredential(payload []byte) (webauthn.Credential, error) {
	var credential webauthn.Credential
	err := json.Unmarshal(payload, &credential)
	return credential, err
}

func encodeCredentialID(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}
