package web

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"remnafamily/internal/database"
	"remnafamily/internal/utils"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

type passwordLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type bootstrapResponse struct {
	Authenticated bool                 `json:"authenticated"`
	User          *webAdminUserDTO     `json:"user,omitempty"`
	Methods       authMethodsDTO       `json:"methods"`
	Telegram      *telegramMethodDTO   `json:"telegram,omitempty"`
	Passkeys      []webAdminPasskeyDTO `json:"passkeys,omitempty"`
}

type authMethodsDTO struct {
	Password bool `json:"password"`
	Passkey  bool `json:"passkey"`
	GitHub   bool `json:"github"`
	Telegram bool `json:"telegram"`
}

type telegramMethodDTO struct {
	BotUsername string `json:"botUsername"`
	BotID       int64  `json:"botId"`
}

type webAdminUserDTO struct {
	ID          int     `json:"id"`
	Username    *string `json:"username,omitempty"`
	DisplayName string  `json:"displayName"`
	GitHubLogin *string `json:"githubLogin,omitempty"`
	GitHubEmail *string `json:"githubEmail,omitempty"`
	TelegramID  *int64  `json:"telegramId,omitempty"`
}

type webAdminPasskeyDTO struct {
	ID         int        `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
}

type githubUser struct {
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var (
		userDTO       *webAdminUserDTO
		passkeys      []webAdminPasskeyDTO
		authenticated bool
	)

	if adminUser, err := s.getSessionUser(ctx, r); err == nil && adminUser != nil {
		authenticated = true
		userDTO = toWebAdminUserDTO(adminUser)
		if items, err := s.deps.DB.GetWebAdminPasskeysByUserID(ctx, adminUser.ID); err == nil {
			passkeys = toPasskeyDTOs(items)
		}
	}

	response := bootstrapResponse{
		Authenticated: authenticated,
		User:          userDTO,
		Methods: authMethodsDTO{
			Password: s.cfg.WebAdmin.Auth.PasswordEnabled,
			Passkey:  s.cfg.WebAdmin.Auth.PasskeyEnabled,
			GitHub:   s.cfg.WebAdmin.Auth.GitHubEnabled,
			Telegram: s.cfg.WebAdmin.Auth.TelegramEnabled && s.botUsername != "" && s.botID != 0,
		},
		Passkeys: passkeys,
	}

	if response.Methods.Telegram {
		response.Telegram = &telegramMethodDTO{
			BotUsername: s.botUsername,
			BotID:       s.botID,
		}
	}

	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleCurrentSession(w http.ResponseWriter, r *http.Request) {
	adminUser, err := s.getSessionUser(r.Context(), r)
	if err != nil {
		s.writeJSON(w, http.StatusOK, bootstrapResponse{
			Authenticated: false,
			Methods: authMethodsDTO{
				Password: s.cfg.WebAdmin.Auth.PasswordEnabled,
				Passkey:  s.cfg.WebAdmin.Auth.PasskeyEnabled,
				GitHub:   s.cfg.WebAdmin.Auth.GitHubEnabled,
				Telegram: s.cfg.WebAdmin.Auth.TelegramEnabled && s.botUsername != "" && s.botID != 0,
			},
		})
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"authenticated": true,
		"user":          toWebAdminUserDTO(adminUser),
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.clearSession(r.Context(), w, r)
	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handlePasswordLogin(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.WebAdmin.Auth.PasswordEnabled {
		s.writeError(w, http.StatusNotFound, "password authentication is disabled")
		return
	}

	var req passwordLoginRequest
	if err := s.readJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	hash, ok := s.passwordHash(req.Username)
	if !ok || comparePasswordHash(hash, req.Password) != nil {
		s.writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	adminUser, err := s.ensurePasswordAdminUser(r.Context(), req.Username)
	if err != nil {
		utils.LogError("ensure password admin user", err)
		s.writeError(w, http.StatusInternalServerError, "failed to create admin session")
		return
	}

	if err := s.createSession(r.Context(), w, r, adminUser.ID, "password"); err != nil {
		utils.LogError("create password session", err)
		s.writeError(w, http.StatusInternalServerError, "failed to create admin session")
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"authenticated": true,
		"user":          toWebAdminUserDTO(adminUser),
	})
}

func (s *Server) handleGitHubStart(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.WebAdmin.Auth.GitHubEnabled || s.githubOAuth == nil {
		s.writeError(w, http.StatusNotFound, "github authentication is disabled")
		return
	}

	state, err := randomToken(24)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to initialize github login")
		return
	}

	if err := s.store.SetJSON(r.Context(), s.stateKey("github", state), githubState{State: state}, 10*time.Minute); err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to initialize github login")
		return
	}

	http.Redirect(w, r, s.githubOAuth.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

func (s *Server) handleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.WebAdmin.Auth.GitHubEnabled || s.githubOAuth == nil {
		s.writeError(w, http.StatusNotFound, "github authentication is disabled")
		return
	}

	state := strings.TrimSpace(r.URL.Query().Get("state"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if state == "" || code == "" {
		s.writeError(w, http.StatusBadRequest, "missing github callback parameters")
		return
	}

	var stored githubState
	if err := s.store.GetJSON(r.Context(), s.stateKey("github", state), &stored); err != nil || stored.State != state {
		s.writeError(w, http.StatusUnauthorized, "invalid github oauth state")
		return
	}
	_ = s.store.Delete(r.Context(), s.stateKey("github", state))

	token, err := s.githubOAuth.Exchange(r.Context(), code)
	if err != nil {
		utils.LogError("exchange github code", err)
		s.writeError(w, http.StatusUnauthorized, "github authorization failed")
		return
	}

	client := s.githubOAuth.Client(r.Context(), token)
	profile, emails, err := s.fetchGitHubIdentity(r.Context(), client)
	if err != nil {
		utils.LogError("fetch github identity", err)
		s.writeError(w, http.StatusUnauthorized, "failed to fetch github identity")
		return
	}

	email := pickGitHubAllowedEmail(emails)
	if !s.isAllowedGitHubAccount(profile.Login, email) {
		s.writeError(w, http.StatusForbidden, "github account is not allowed")
		return
	}

	adminUser, err := s.ensureGitHubAdminUser(r.Context(), profile.Login, email, firstNonEmpty(profile.Name, profile.Login))
	if err != nil {
		utils.LogError("ensure github admin user", err)
		s.writeError(w, http.StatusInternalServerError, "failed to create admin session")
		return
	}

	if err := s.createSession(r.Context(), w, r, adminUser.ID, "github"); err != nil {
		utils.LogError("create github session", err)
		s.writeError(w, http.StatusInternalServerError, "failed to create admin session")
		return
	}

	http.Redirect(w, r, adminIndexPath(s.cfg.WebAdmin.BasePath), http.StatusTemporaryRedirect)
}

func (s *Server) handleTelegramVerify(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.WebAdmin.Auth.TelegramEnabled {
		s.writeError(w, http.StatusNotFound, "telegram authentication is disabled")
		return
	}

	var payload telegramLoginPayload
	if err := s.readJSON(r, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid telegram payload")
		return
	}
	if !s.isAllowedTelegramID(payload.ID) {
		s.writeError(w, http.StatusForbidden, "telegram account is not allowed")
		return
	}
	if err := verifyTelegramLoginPayload(payload, s.cfg.BotToken); err != nil {
		s.writeError(w, http.StatusUnauthorized, "telegram authentication failed")
		return
	}

	var username *string
	if payload.Username != "" {
		value := payload.Username
		username = &value
	}
	displayName := strings.TrimSpace(strings.TrimSpace(payload.FirstName + " " + payload.LastName))
	if displayName == "" && payload.Username != "" {
		displayName = payload.Username
	}
	if displayName == "" {
		displayName = fmt.Sprintf("Telegram %d", payload.ID)
	}

	adminUser, err := s.ensureTelegramAdminUser(r.Context(), payload.ID, username, displayName)
	if err != nil {
		utils.LogError("ensure telegram admin user", err)
		s.writeError(w, http.StatusInternalServerError, "failed to create admin session")
		return
	}

	if err := s.createSession(r.Context(), w, r, adminUser.ID, "telegram"); err != nil {
		utils.LogError("create telegram session", err)
		s.writeError(w, http.StatusInternalServerError, "failed to create admin session")
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"authenticated": true,
		"user":          toWebAdminUserDTO(adminUser),
	})
}

func (s *Server) handlePasskeyRegisterOptions(w http.ResponseWriter, r *http.Request, adminUser *database.WebAdminUser) {
	if !s.cfg.WebAdmin.Auth.PasskeyEnabled || s.webauthn == nil {
		s.writeError(w, http.StatusNotFound, "passkey authentication is disabled")
		return
	}

	passkeyUser, _, err := s.loadPasskeyUser(r.Context(), adminUser)
	if err != nil {
		utils.LogError("load passkey user", err)
		s.writeError(w, http.StatusInternalServerError, "failed to initialize passkey registration")
		return
	}

	options, sessionData, err := s.webauthn.BeginRegistration(
		passkeyUser,
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired,
			UserVerification: protocol.VerificationRequired,
		}),
	)
	if err != nil {
		utils.LogError("begin passkey registration", err)
		s.writeError(w, http.StatusInternalServerError, "failed to initialize passkey registration")
		return
	}

	if err := s.store.SetJSON(r.Context(), s.stateKey("passkey-register", strconv.Itoa(adminUser.ID)), sessionData, 10*time.Minute); err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to initialize passkey registration")
		return
	}

	s.writeJSON(w, http.StatusOK, options)
}

func (s *Server) handlePasskeyRegisterVerify(w http.ResponseWriter, r *http.Request, adminUser *database.WebAdminUser) {
	if !s.cfg.WebAdmin.Auth.PasskeyEnabled || s.webauthn == nil {
		s.writeError(w, http.StatusNotFound, "passkey authentication is disabled")
		return
	}

	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		name = "Passkey"
	}

	passkeyUser, _, err := s.loadPasskeyUser(r.Context(), adminUser)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to finalize passkey registration")
		return
	}

	var savedSessionData webauthn.SessionData
	if err := s.store.GetJSON(r.Context(), s.stateKey("passkey-register", strconv.Itoa(adminUser.ID)), &savedSessionData); err != nil {
		s.writeError(w, http.StatusUnauthorized, "passkey registration session expired")
		return
	}

	credential, err := s.webauthn.FinishRegistration(passkeyUser, savedSessionData, r)
	if err != nil {
		utils.LogError("finish passkey registration", err)
		s.writeError(w, http.StatusUnauthorized, "passkey registration failed")
		return
	}
	_ = s.store.Delete(r.Context(), s.stateKey("passkey-register", strconv.Itoa(adminUser.ID)))

	payload, err := json.Marshal(credential)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to persist passkey")
		return
	}

	if _, err := s.deps.DB.CreateWebAdminPasskey(r.Context(), adminUser.ID, name, encodeCredentialID(credential.ID), payload); err != nil {
		utils.LogError("create web admin passkey", err)
		s.writeError(w, http.StatusInternalServerError, "failed to persist passkey")
		return
	}

	items, _ := s.deps.DB.GetWebAdminPasskeysByUserID(r.Context(), adminUser.ID)
	s.writeJSON(w, http.StatusOK, toPasskeyDTOs(items))
}

func (s *Server) handlePasskeyLoginOptions(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.WebAdmin.Auth.PasskeyEnabled || s.webauthn == nil {
		s.writeError(w, http.StatusNotFound, "passkey authentication is disabled")
		return
	}

	options, sessionData, err := s.webauthn.BeginDiscoverableLogin(
		webauthn.WithUserVerification(protocol.VerificationRequired),
	)
	if err != nil {
		utils.LogError("begin passkey login", err)
		s.writeError(w, http.StatusInternalServerError, "failed to initialize passkey login")
		return
	}

	token, err := randomToken(16)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to initialize passkey login")
		return
	}
	if err := s.store.SetJSON(r.Context(), s.stateKey("passkey-login", token), sessionData, 10*time.Minute); err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to initialize passkey login")
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"requestId": token,
		"options":   options,
	})
}

func (s *Server) handlePasskeyLoginVerify(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.WebAdmin.Auth.PasskeyEnabled || s.webauthn == nil {
		s.writeError(w, http.StatusNotFound, "passkey authentication is disabled")
		return
	}

	requestID := strings.TrimSpace(r.URL.Query().Get("requestId"))
	if requestID == "" {
		s.writeError(w, http.StatusBadRequest, "missing passkey request id")
		return
	}

	var savedSessionData webauthn.SessionData
	if err := s.store.GetJSON(r.Context(), s.stateKey("passkey-login", requestID), &savedSessionData); err != nil {
		s.writeError(w, http.StatusUnauthorized, "passkey login session expired")
		return
	}

	user, credential, err := s.webauthn.FinishPasskeyLogin(func(rawID, userHandle []byte) (webauthn.User, error) {
		adminUser, err := s.deps.DB.GetWebAdminUserByWebauthnUserID(r.Context(), userHandle)
		if err != nil || adminUser == nil {
			return nil, err
		}
		passkeyUser, _, err := s.loadPasskeyUser(r.Context(), adminUser)
		return passkeyUser, err
	}, savedSessionData, r)
	if err != nil {
		utils.LogError("finish passkey login", err)
		s.writeError(w, http.StatusUnauthorized, "passkey login failed")
		return
	}
	_ = s.store.Delete(r.Context(), s.stateKey("passkey-login", requestID))

	passkeyUser, ok := user.(*passkeyUser)
	if !ok || passkeyUser.user == nil {
		s.writeError(w, http.StatusUnauthorized, "passkey login failed")
		return
	}
	_ = s.deps.DB.TouchWebAdminPasskey(r.Context(), encodeCredentialID(credential.ID))

	if err := s.createSession(r.Context(), w, r, passkeyUser.user.ID, "passkey"); err != nil {
		utils.LogError("create passkey session", err)
		s.writeError(w, http.StatusInternalServerError, "failed to create admin session")
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"authenticated": true,
		"user":          toWebAdminUserDTO(passkeyUser.user),
	})
}

func (s *Server) handleListPasskeys(w http.ResponseWriter, r *http.Request, adminUser *database.WebAdminUser) {
	items, err := s.deps.DB.GetWebAdminPasskeysByUserID(r.Context(), adminUser.ID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to load passkeys")
		return
	}
	s.writeJSON(w, http.StatusOK, toPasskeyDTOs(items))
}

func (s *Server) handleDeletePasskey(w http.ResponseWriter, r *http.Request, adminUser *database.WebAdminUser) {
	passkeyID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid passkey id")
		return
	}
	if err := s.deps.DB.DeleteWebAdminPasskey(r.Context(), adminUser.ID, passkeyID); err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to delete passkey")
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) ensurePasswordAdminUser(ctx context.Context, username string) (*database.WebAdminUser, error) {
	handle, err := randomWebauthnHandle()
	if err != nil {
		return nil, err
	}
	return s.deps.DB.UpsertWebAdminPasswordUser(ctx, username, username, handle)
}

func (s *Server) ensureGitHubAdminUser(ctx context.Context, login, email, displayName string) (*database.WebAdminUser, error) {
	if email != "" {
		user, err := s.deps.DB.GetWebAdminUserByGitHubEmail(ctx, email)
		if err != nil {
			return nil, err
		}
		if user != nil {
			return s.deps.DB.UpdateWebAdminGitHubIdentity(ctx, user.ID, login, email, displayName)
		}
	}
	if login != "" {
		user, err := s.deps.DB.GetWebAdminUserByGitHubLogin(ctx, login)
		if err != nil {
			return nil, err
		}
		if user != nil {
			return s.deps.DB.UpdateWebAdminGitHubIdentity(ctx, user.ID, login, email, displayName)
		}
	}

	handle, err := randomWebauthnHandle()
	if err != nil {
		return nil, err
	}
	return s.deps.DB.CreateWebAdminGitHubUser(ctx, login, email, displayName, handle)
}

func (s *Server) ensureTelegramAdminUser(ctx context.Context, telegramID int64, username *string, displayName string) (*database.WebAdminUser, error) {
	handle, err := randomWebauthnHandle()
	if err != nil {
		return nil, err
	}
	return s.deps.DB.UpsertWebAdminTelegramUser(ctx, telegramID, username, displayName, handle)
}

func (s *Server) loadPasskeyUser(ctx context.Context, adminUser *database.WebAdminUser) (*passkeyUser, []*database.WebAdminPasskey, error) {
	passkeys, err := s.deps.DB.GetWebAdminPasskeysByUserID(ctx, adminUser.ID)
	if err != nil {
		return nil, nil, err
	}

	credentials := make([]webauthn.Credential, 0, len(passkeys))
	for _, passkey := range passkeys {
		credential, err := decodeStoredCredential(passkey.Credential)
		if err != nil {
			return nil, nil, err
		}
		credentials = append(credentials, credential)
	}

	return &passkeyUser{
		user:        adminUser,
		credentials: credentials,
	}, passkeys, nil
}

func (s *Server) fetchGitHubIdentity(ctx context.Context, client *http.Client) (*githubUser, []githubEmail, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("github user request failed: %s", strings.TrimSpace(string(body)))
	}

	var profile githubUser
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, nil, err
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err = client.Do(req)
	if err != nil {
		return &profile, nil, err
	}
	defer resp.Body.Close()

	var emails []githubEmail
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
			return &profile, nil, err
		}
	}

	return &profile, emails, nil
}

func pickGitHubAllowedEmail(emails []githubEmail) string {
	for _, item := range emails {
		if item.Primary && item.Verified {
			return item.Email
		}
	}
	for _, item := range emails {
		if item.Verified {
			return item.Email
		}
	}
	return ""
}

func (s *Server) isAllowedGitHubAccount(login, email string) bool {
	allowedEmailSet := make(map[string]struct{}, len(s.cfg.WebAdmin.Auth.GitHubAllowedEmails))
	for _, item := range s.cfg.WebAdmin.Auth.GitHubAllowedEmails {
		allowedEmailSet[strings.ToLower(item)] = struct{}{}
	}
	allowedLoginSet := make(map[string]struct{}, len(s.cfg.WebAdmin.Auth.GitHubAllowedLogins))
	for _, item := range s.cfg.WebAdmin.Auth.GitHubAllowedLogins {
		allowedLoginSet[strings.ToLower(item)] = struct{}{}
	}

	if len(allowedEmailSet) == 0 && len(allowedLoginSet) == 0 {
		return false
	}
	if email != "" {
		if _, ok := allowedEmailSet[strings.ToLower(email)]; ok {
			return true
		}
	}
	if login != "" {
		if _, ok := allowedLoginSet[strings.ToLower(login)]; ok {
			return true
		}
	}
	return false
}

func (s *Server) isAllowedTelegramID(userID int64) bool {
	for _, allowedID := range s.cfg.WebAdmin.Auth.TelegramAllowedIDs {
		if allowedID == userID {
			return true
		}
	}
	return false
}

func verifyTelegramLoginPayload(payload telegramLoginPayload, botToken string) error {
	if payload.ID == 0 || payload.AuthDate == 0 || payload.Hash == "" {
		return fmt.Errorf("missing required telegram login fields")
	}
	if time.Since(time.Unix(payload.AuthDate, 0)) > 10*time.Minute {
		return fmt.Errorf("telegram login payload expired")
	}

	values := map[string]string{
		"auth_date":  strconv.FormatInt(payload.AuthDate, 10),
		"first_name": payload.FirstName,
		"id":         strconv.FormatInt(payload.ID, 10),
		"last_name":  payload.LastName,
		"photo_url":  payload.PhotoURL,
		"username":   payload.Username,
	}

	keys := make([]string, 0, len(values))
	for key, value := range values {
		if value == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, key+"="+values[key])
	}
	checkString := strings.Join(lines, "\n")

	secret := sha256.Sum256([]byte(botToken))
	mac := hmac.New(sha256.New, secret[:])
	_, _ = mac.Write([]byte(checkString))
	expectedHash := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expectedHash), []byte(strings.ToLower(payload.Hash))) {
		return fmt.Errorf("invalid telegram payload hash")
	}

	return nil
}

func randomWebauthnHandle() ([]byte, error) {
	token, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	return []byte(token), nil
}

func toWebAdminUserDTO(user *database.WebAdminUser) *webAdminUserDTO {
	if user == nil {
		return nil
	}
	return &webAdminUserDTO{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		GitHubLogin: user.GitHubLogin,
		GitHubEmail: user.GitHubEmail,
		TelegramID:  user.TelegramID,
	}
}

func toPasskeyDTOs(items []*database.WebAdminPasskey) []webAdminPasskeyDTO {
	result := make([]webAdminPasskeyDTO, 0, len(items))
	for _, item := range items {
		result = append(result, webAdminPasskeyDTO{
			ID:         item.ID,
			Name:       item.Name,
			CreatedAt:  item.CreatedAt,
			LastUsedAt: item.LastUsedAt,
		})
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
