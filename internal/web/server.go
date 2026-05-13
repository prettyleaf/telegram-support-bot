package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"remnafamily/internal/config"
	"remnafamily/internal/database"
	"remnafamily/internal/types"
	"remnafamily/internal/utils"

	"github.com/go-webauthn/webauthn/webauthn"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	ghoauth "golang.org/x/oauth2/github"
)

type contextKey string

const adminUserContextKey contextKey = "web_admin_user"

func adminIndexPath(basePath string) string {
	if strings.TrimSpace(basePath) == "" || basePath == "/" {
		return "/"
	}
	return strings.TrimRight(basePath, "/") + "/"
}

type Server struct {
	cfg         *config.Config
	deps        *types.Dependencies
	httpServer  *http.Server
	store       stateStore
	webauthn    *webauthn.WebAuthn
	githubOAuth *oauth2.Config
	botUsername string
	botID       int64
}

func NewServer(cfg *config.Config, deps *types.Dependencies, botUsername string, botID int64) (*Server, error) {
	if cfg == nil || deps == nil || !cfg.WebAdmin.Enabled {
		return nil, nil
	}

	server := &Server{
		cfg:         cfg,
		deps:        deps,
		store:       newStateStore(cfg.Valkey),
		botUsername: botUsername,
		botID:       botID,
	}

	if cfg.WebAdmin.Auth.PasskeyEnabled {
		instance, err := webauthn.New(&webauthn.Config{
			RPDisplayName: cfg.WebAdmin.Auth.PasskeyDisplayName,
			RPID:          cfg.WebAdmin.Auth.PasskeyRPID,
			RPOrigins:     cfg.WebAdmin.Auth.PasskeyOrigins,
		})
		if err != nil {
			return nil, fmt.Errorf("create webauthn instance: %w", err)
		}
		server.webauthn = instance
	}

	if cfg.WebAdmin.Auth.GitHubEnabled {
		server.githubOAuth = &oauth2.Config{
			ClientID:     cfg.WebAdmin.Auth.GitHubClientID,
			ClientSecret: cfg.WebAdmin.Auth.GitHubClientSecret,
			Scopes:       []string{"read:user", "user:email"},
			Endpoint:     ghoauth.Endpoint,
			RedirectURL:  strings.TrimRight(cfg.WebAdmin.PublicOrigin, "/") + "/api/admin/auth/github/callback",
		}
	}

	mux := http.NewServeMux()
	server.registerRoutes(mux)
	server.httpServer = &http.Server{
		Addr:              cfg.WebAdmin.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return server, nil
}

func (s *Server) Start() error {
	if s == nil || s.httpServer == nil {
		return nil
	}

	go func() {
		utils.LogSystem("Web admin listening on %s", s.cfg.WebAdmin.ListenAddr)
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			utils.LogError("web admin listen", err)
		}
	}()

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	if s == nil || s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/bootstrap", s.handleBootstrap)
	mux.HandleFunc("GET /api/admin/auth/session", s.handleCurrentSession)
	mux.HandleFunc("POST /api/admin/auth/logout", s.handleLogout)
	mux.HandleFunc("POST /api/admin/auth/password/login", s.handlePasswordLogin)
	mux.HandleFunc("GET /api/admin/auth/github/start", s.handleGitHubStart)
	mux.HandleFunc("GET /api/admin/auth/github/callback", s.handleGitHubCallback)
	mux.HandleFunc("POST /api/admin/auth/telegram/verify", s.handleTelegramVerify)
	mux.HandleFunc("POST /api/admin/auth/passkey/register/options", s.withAdmin(s.handlePasskeyRegisterOptions))
	mux.HandleFunc("POST /api/admin/auth/passkey/register/verify", s.withAdmin(s.handlePasskeyRegisterVerify))
	mux.HandleFunc("POST /api/admin/auth/passkey/login/options", s.handlePasskeyLoginOptions)
	mux.HandleFunc("POST /api/admin/auth/passkey/login/verify", s.handlePasskeyLoginVerify)
	mux.HandleFunc("GET /api/admin/auth/passkeys", s.withAdmin(s.handleListPasskeys))
	mux.HandleFunc("DELETE /api/admin/auth/passkeys/{id}", s.withAdmin(s.handleDeletePasskey))

	mux.HandleFunc("GET /api/admin/dashboard/summary", s.withAdmin(s.handleDashboardSummary))
	mux.HandleFunc("GET /api/admin/tickets", s.withAdmin(s.handleTicketsList))
	mux.HandleFunc("GET /api/admin/tickets/{id}", s.withAdmin(s.handleTicketDetails))
	mux.HandleFunc("GET /api/admin/tickets/{id}/messages", s.withAdmin(s.handleTicketMessages))
	mux.HandleFunc("GET /api/admin/tickets/{id}/messages/{messageID}/media", s.withAdmin(s.handleTicketMessageMedia))
	mux.HandleFunc("POST /api/admin/tickets/{id}/reply", s.withAdmin(s.handleTicketReply))
	mux.HandleFunc("POST /api/admin/tickets/{id}/priority", s.withAdmin(s.handleTicketPriority))
	mux.HandleFunc("POST /api/admin/tickets/{id}/close", s.withAdmin(s.handleTicketClose))
	mux.HandleFunc("POST /api/admin/tickets/{id}/reopen", s.withAdmin(s.handleTicketReopen))
	mux.HandleFunc("GET /api/admin/tickets/{id}/notes", s.withAdmin(s.handleTicketNotes))
	mux.HandleFunc("POST /api/admin/tickets/{id}/notes", s.withAdmin(s.handleTicketAddNote))
	mux.HandleFunc("GET /api/admin/tickets/{id}/export", s.withAdmin(s.handleTicketExport))
	mux.HandleFunc("GET /api/admin/canned", s.withAdmin(s.handleCannedList))
	mux.HandleFunc("POST /api/admin/canned", s.withAdmin(s.handleCannedCreate))
	mux.HandleFunc("DELETE /api/admin/canned/{shortcut}", s.withAdmin(s.handleCannedDelete))
	mux.HandleFunc("GET /api/admin/banned", s.withAdmin(s.handleBannedList))
	mux.HandleFunc("POST /api/admin/banned", s.withAdmin(s.handleBannedCreate))
	mux.HandleFunc("DELETE /api/admin/banned/{userID}", s.withAdmin(s.handleBannedDelete))

	if s.cfg.WebAdmin.BasePath == "/" {
		mux.Handle("/", s.staticHandler())
		return
	}

	mux.Handle(s.cfg.WebAdmin.BasePath+"/", s.staticHandler())
	if s.cfg.WebAdmin.BasePath != "/" {
		mux.HandleFunc("GET "+s.cfg.WebAdmin.BasePath, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, adminIndexPath(s.cfg.WebAdmin.BasePath), http.StatusTemporaryRedirect)
		})
	}
}

func (s *Server) staticHandler() http.Handler {
	distDir := filepath.Join("web", "admin-ui", "dist")
	indexFile := filepath.Join(distDir, "index.html")
	routeBasePath := s.cfg.WebAdmin.BasePath
	if routeBasePath == "/" {
		routeBasePath = ""
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if routeBasePath != "" {
			path = strings.TrimPrefix(path, routeBasePath)
		}

		clean := strings.TrimPrefix(filepath.Clean("/"+path), "/")
		if clean == "" || clean == "." {
			http.ServeFile(w, r, indexFile)
			return
		}

		target := filepath.Join(distDir, clean)
		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			http.ServeFile(w, r, target)
			return
		}

		http.ServeFile(w, r, indexFile)
	})
}

func (s *Server) withAdmin(next func(http.ResponseWriter, *http.Request, *database.WebAdminUser)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminUser, err := s.getSessionUser(r.Context(), r)
		if err != nil {
			s.writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		next(w, r, adminUser)
	}
}

func (s *Server) getSessionUser(ctx context.Context, r *http.Request) (*database.WebAdminUser, error) {
	session, err := s.getSession(ctx, r)
	if err != nil {
		return nil, err
	}

	user, err := s.deps.DB.GetWebAdminUserByID(ctx, session.AdminUserID)
	if err != nil || user == nil {
		return nil, errStateNotFound
	}
	return user, nil
}

func (s *Server) sessionKey(sessionID string) string {
	return "admin:session:" + sessionID
}

func (s *Server) stateKey(prefix, value string) string {
	return "admin:" + prefix + ":" + value
}

func randomToken(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func isSecureRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}

	if proto := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]); strings.EqualFold(proto, "https") {
		return true
	}

	for _, item := range strings.Split(r.Header.Get("Forwarded"), ",") {
		for _, part := range strings.Split(item, ";") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(strings.ToLower(part), "proto=") && strings.EqualFold(strings.Trim(strings.TrimPrefix(part, "proto="), "\""), "https") {
				return true
			}
		}
	}

	return false
}

func (s *Server) createSession(ctx context.Context, w http.ResponseWriter, r *http.Request, adminUserID int, method string) error {
	sessionID, err := randomToken(32)
	if err != nil {
		return err
	}

	session := adminSession{
		ID:          sessionID,
		AdminUserID: adminUserID,
		AuthMethod:  method,
		ExpiresAt:   time.Now().Add(time.Duration(s.cfg.WebAdmin.SessionTTLHours) * time.Hour),
	}

	if err := s.store.SetJSON(ctx, s.sessionKey(sessionID), session, time.Until(session.ExpiresAt)); err != nil {
		return err
	}
	if err := s.deps.DB.UpdateWebAdminLastLogin(ctx, adminUserID); err != nil {
		utils.LogError("update web admin last login", err)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.WebAdmin.SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(r),
		Expires:  session.ExpiresAt,
	})

	return nil
}

func (s *Server) clearSession(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(s.cfg.WebAdmin.SessionCookieName); err == nil && cookie.Value != "" {
		_ = s.store.Delete(ctx, s.sessionKey(cookie.Value))
	}
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.WebAdmin.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(r),
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func (s *Server) getSession(ctx context.Context, r *http.Request) (*adminSession, error) {
	cookie, err := r.Cookie(s.cfg.WebAdmin.SessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil, errStateNotFound
	}

	var session adminSession
	if err := s.store.GetJSON(ctx, s.sessionKey(cookie.Value), &session); err != nil {
		return nil, err
	}
	if time.Now().After(session.ExpiresAt) {
		return nil, errStateNotFound
	}
	return &session, nil
}

func (s *Server) readJSON(r *http.Request, dest interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dest)
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiResponse{Data: data})
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiResponse{Error: message})
}

func (s *Server) parseTicketID(r *http.Request) (int, error) {
	value := r.PathValue("id")
	return strconv.Atoi(value)
}

func (s *Server) ticketByRequest(ctx context.Context, r *http.Request) (*database.Ticket, error) {
	ticketID, err := s.parseTicketID(r)
	if err != nil {
		return nil, err
	}
	return s.deps.TicketService.GetTicketByID(ctx, ticketID)
}

func (s *Server) passwordHash(username string) (string, bool) {
	hash, ok := s.cfg.WebAdmin.Auth.PasswordUsers[username]
	return hash, ok
}

func comparePasswordHash(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
