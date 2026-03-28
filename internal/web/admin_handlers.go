package web

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"remnafamily/internal/config"
	"remnafamily/internal/database"
	"remnafamily/internal/services"
	"remnafamily/internal/utils"

	"github.com/go-telegram/bot/models"
)

type dashboardSummaryDTO struct {
	PeriodDays    int `json:"periodDays"`
	Opened        int `json:"opened"`
	Closed        int `json:"closed"`
	CurrentlyOpen int `json:"currentlyOpen"`
	TotalClosed   int `json:"totalClosed"`
}

type ticketsListDTO struct {
	Items      []ticketDTO `json:"items"`
	Page       int         `json:"page"`
	PageSize   int         `json:"pageSize"`
	Total      int         `json:"total"`
	TotalPages int         `json:"totalPages"`
	Filter     string      `json:"filter"`
}

type ticketDTO struct {
	ID           int        `json:"id"`
	TicketNumber string     `json:"ticketNumber"`
	UserID       int64      `json:"userId"`
	DisplayName  string     `json:"displayName"`
	Status       string     `json:"status"`
	Priority     string     `json:"priority"`
	Subject      *string    `json:"subject,omitempty"`
	ThreadID     int64      `json:"threadId"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	ClosedAt     *time.Time `json:"closedAt,omitempty"`
}

type ticketMessageDTO struct {
	ID             int       `json:"id"`
	IsAdmin        bool      `json:"isAdmin"`
	SenderID       int64     `json:"senderId"`
	SenderName     *string   `json:"senderName,omitempty"`
	SenderUsername *string   `json:"senderUsername,omitempty"`
	MessageText    *string   `json:"messageText,omitempty"`
	HasMedia       bool      `json:"hasMedia"`
	MediaType      *string   `json:"mediaType,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

type ticketNoteDTO struct {
	ID        int       `json:"id"`
	TicketID  int       `json:"ticketId"`
	AdminID   int64     `json:"adminId"`
	AdminName *string   `json:"adminName,omitempty"`
	NoteText  string    `json:"noteText"`
	CreatedAt time.Time `json:"createdAt"`
}

type cannedResponseDTO struct {
	ID        int       `json:"id"`
	Shortcut  string    `json:"shortcut"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedBy int64     `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type bannedUserDTO struct {
	ID           int       `json:"id"`
	UserID       int64     `json:"userId"`
	DisplayName  string    `json:"displayName"`
	UserName     *string   `json:"userName,omitempty"`
	UserUsername *string   `json:"userUsername,omitempty"`
	BannedBy     int64     `json:"bannedBy"`
	BannedByName *string   `json:"bannedByName,omitempty"`
	Reason       *string   `json:"reason,omitempty"`
	BannedAt     time.Time `json:"bannedAt"`
}

type replyRequest struct {
	Text string `json:"text"`
}

type priorityRequest struct {
	Priority string `json:"priority"`
}

type noteRequest struct {
	Text string `json:"text"`
}

type banRequest struct {
	UserID int64  `json:"userId"`
	Reason string `json:"reason"`
}

type cannedRequest struct {
	Shortcut string `json:"shortcut"`
	Title    string `json:"title"`
	Content  string `json:"content"`
}

func (s *Server) handleDashboardSummary(w http.ResponseWriter, r *http.Request, adminUser *database.WebAdminUser) {
	days := 7
	if value := strings.TrimSpace(r.URL.Query().Get("days")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			days = parsed
		}
	}

	endDate := time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	startDate := endDate.AddDate(0, 0, -days)

	stats, err := s.deps.TicketService.GetTicketStats(r.Context(), startDate, endDate)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to load ticket stats")
		return
	}

	openCount, _ := s.deps.TicketService.CountTicketsByFilter(r.Context(), "open")
	closedCount, _ := s.deps.TicketService.CountTicketsByFilter(r.Context(), "closed")

	s.writeJSON(w, http.StatusOK, dashboardSummaryDTO{
		PeriodDays:    days,
		Opened:        stats.TicketsOpened,
		Closed:        stats.TicketsClosed,
		CurrentlyOpen: openCount,
		TotalClosed:   closedCount,
	})
}

func (s *Server) handleTicketsList(w http.ResponseWriter, r *http.Request, adminUser *database.WebAdminUser) {
	filter := strings.TrimSpace(r.URL.Query().Get("filter"))
	if filter == "" {
		filter = "open"
	}

	page := 1
	if value := strings.TrimSpace(r.URL.Query().Get("page")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			page = parsed
		}
	}

	pageSize := 20
	if value := strings.TrimSpace(r.URL.Query().Get("pageSize")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}

	total, err := s.deps.TicketService.CountTicketsByFilter(r.Context(), filter)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to count tickets")
		return
	}

	offset := (page - 1) * pageSize
	tickets, err := s.deps.TicketService.GetTicketsPage(r.Context(), filter, pageSize, offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to load tickets")
		return
	}

	totalPages := total / pageSize
	if total%pageSize != 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}

	items := make([]ticketDTO, 0, len(tickets))
	for _, ticket := range tickets {
		items = append(items, toTicketDTO(ticket))
	}

	s.writeJSON(w, http.StatusOK, ticketsListDTO{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
		Filter:     filter,
	})
}

func (s *Server) handleTicketDetails(w http.ResponseWriter, r *http.Request, adminUser *database.WebAdminUser) {
	ticket, err := s.ticketByRequest(r.Context(), r)
	if err != nil || ticket == nil {
		s.writeError(w, http.StatusNotFound, "ticket not found")
		return
	}
	s.writeJSON(w, http.StatusOK, toTicketDTO(ticket))
}

func (s *Server) handleTicketMessages(w http.ResponseWriter, r *http.Request, adminUser *database.WebAdminUser) {
	ticket, err := s.ticketByRequest(r.Context(), r)
	if err != nil || ticket == nil {
		s.writeError(w, http.StatusNotFound, "ticket not found")
		return
	}

	messages, err := s.deps.TicketService.GetTicketMessages(r.Context(), ticket.ID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to load ticket messages")
		return
	}

	items := make([]ticketMessageDTO, 0, len(messages))
	for _, message := range messages {
		items = append(items, ticketMessageDTO{
			ID:             message.ID,
			IsAdmin:        message.IsAdmin,
			SenderID:       message.SenderID,
			SenderName:     message.SenderName,
			SenderUsername: message.SenderUsername,
			MessageText:    message.MessageText,
			HasMedia:       message.HasMedia,
			MediaType:      message.MediaType,
			CreatedAt:      message.CreatedAt,
		})
	}

	s.writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleTicketReply(w http.ResponseWriter, r *http.Request, adminUser *database.WebAdminUser) {
	ticket, err := s.ticketByRequest(r.Context(), r)
	if err != nil || ticket == nil {
		s.writeError(w, http.StatusNotFound, "ticket not found")
		return
	}
	if ticket.IsClosed() {
		s.writeError(w, http.StatusConflict, "ticket is closed")
		return
	}

	text, uploads, err := readTicketReplyPayload(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	body := utils.SanitizeTelegramHTML(text)
	senderName, senderUsername := adminSenderIdentity(adminUser)
	if len(uploads) == 0 {
		userMessage, threadMessage, err := s.deps.TicketService.SendSupportHTMLReply(
			r.Context(),
			s.deps.Bot,
			ticket,
			body,
			adminUser.DisplayName,
		)
		if err != nil {
			utils.LogError("send support reply from web", err)
			s.writeError(w, http.StatusInternalServerError, "failed to deliver reply")
			return
		}

		ticketMessage := &database.TicketMessage{
			TicketID:          ticket.ID,
			SenderID:          webAdminActorID(adminUser.ID),
			SenderName:        senderName,
			SenderUsername:    senderUsername,
			IsAdmin:           true,
			MessageText:       stringPtr(body),
			MessageID:         messageIDPtr(userMessage),
			OriginalMessageID: messageIDPtr(threadMessage),
		}

		if err := s.deps.TicketService.SaveMessage(r.Context(), ticketMessage); err != nil {
			utils.LogError("save web admin message", err)
		}

		s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	for index, upload := range uploads {
		caption := ""
		if index == 0 {
			caption = body
		}

		userMessage, threadMessage, mediaType, err := s.deps.TicketService.SendSupportUploadReply(
			r.Context(),
			s.deps.Bot,
			ticket,
			upload,
			caption,
			adminUser.DisplayName,
		)
		if err != nil {
			utils.LogError("send support upload from web", err)
			s.writeError(w, http.StatusInternalServerError, "failed to deliver attachment")
			return
		}

		ticketMessage := &database.TicketMessage{
			TicketID:          ticket.ID,
			SenderID:          webAdminActorID(adminUser.ID),
			SenderName:        senderName,
			SenderUsername:    senderUsername,
			IsAdmin:           true,
			MessageText:       stringPtr(caption),
			MessageID:         messageIDPtr(userMessage),
			OriginalMessageID: messageIDPtr(threadMessage),
			HasMedia:          true,
			MediaType:         stringPtr(mediaType),
		}

		if err := s.deps.TicketService.SaveMessage(r.Context(), ticketMessage); err != nil {
			utils.LogError("save web admin upload message", err)
		}
	}

	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleTicketPriority(w http.ResponseWriter, r *http.Request, adminUser *database.WebAdminUser) {
	ticket, err := s.ticketByRequest(r.Context(), r)
	if err != nil || ticket == nil {
		s.writeError(w, http.StatusNotFound, "ticket not found")
		return
	}

	var req priorityRequest
	if err := s.readJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	priority := strings.ToLower(strings.TrimSpace(req.Priority))
	switch priority {
	case config.PriorityLow, config.PriorityNormal, config.PriorityHigh, config.PriorityUrgent:
	default:
		s.writeError(w, http.StatusBadRequest, "invalid priority")
		return
	}

	if err := s.deps.TicketService.UpdateTicketPriority(r.Context(), ticket.ID, priority); err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to update ticket priority")
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleTicketClose(w http.ResponseWriter, r *http.Request, adminUser *database.WebAdminUser) {
	ticket, err := s.ticketByRequest(r.Context(), r)
	if err != nil || ticket == nil {
		s.writeError(w, http.StatusNotFound, "ticket not found")
		return
	}

	if err := s.deps.TicketService.CloseTicket(r.Context(), s.deps.Bot, ticket.ID, webAdminActorID(adminUser.ID), adminUser.DisplayName); err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to close ticket")
		return
	}
	s.deps.State.ClearUserState(ticket.UserID)
	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleTicketReopen(w http.ResponseWriter, r *http.Request, adminUser *database.WebAdminUser) {
	ticket, err := s.ticketByRequest(r.Context(), r)
	if err != nil || ticket == nil {
		s.writeError(w, http.StatusNotFound, "ticket not found")
		return
	}

	if err := s.deps.TicketService.ReopenTicket(r.Context(), s.deps.Bot, ticket.ID); err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to reopen ticket")
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleTicketNotes(w http.ResponseWriter, r *http.Request, adminUser *database.WebAdminUser) {
	ticket, err := s.ticketByRequest(r.Context(), r)
	if err != nil || ticket == nil {
		s.writeError(w, http.StatusNotFound, "ticket not found")
		return
	}

	notes, err := s.deps.TicketService.GetTicketNotes(r.Context(), ticket.ID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to load ticket notes")
		return
	}

	items := make([]ticketNoteDTO, 0, len(notes))
	for _, note := range notes {
		items = append(items, ticketNoteDTO{
			ID:        note.ID,
			TicketID:  note.TicketID,
			AdminID:   note.AdminID,
			AdminName: note.AdminName,
			NoteText:  note.NoteText,
			CreatedAt: note.CreatedAt,
		})
	}

	s.writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleTicketAddNote(w http.ResponseWriter, r *http.Request, adminUser *database.WebAdminUser) {
	ticket, err := s.ticketByRequest(r.Context(), r)
	if err != nil || ticket == nil {
		s.writeError(w, http.StatusNotFound, "ticket not found")
		return
	}

	var req noteRequest
	if err := s.readJSON(r, &req); err != nil || strings.TrimSpace(req.Text) == "" {
		s.writeError(w, http.StatusBadRequest, "note text is required")
		return
	}

	if err := s.deps.TicketService.AddNote(r.Context(), ticket.ID, webAdminActorID(adminUser.ID), adminUser.DisplayName, strings.TrimSpace(req.Text)); err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to add note")
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleTicketExport(w http.ResponseWriter, r *http.Request, adminUser *database.WebAdminUser) {
	ticket, err := s.ticketByRequest(r.Context(), r)
	if err != nil || ticket == nil {
		s.writeError(w, http.StatusNotFound, "ticket not found")
		return
	}

	messages, err := s.deps.TicketService.GetTicketMessages(r.Context(), ticket.ID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to load ticket messages")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", formatTicketExportFileName(ticket.TicketNumber)))
	_, _ = w.Write([]byte(buildTicketHistoryHTML(ticket, messages)))
}

func (s *Server) handleCannedList(w http.ResponseWriter, r *http.Request, adminUser *database.WebAdminUser) {
	items, err := s.deps.TicketService.GetAllCannedResponses(r.Context())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to load canned responses")
		return
	}

	result := make([]cannedResponseDTO, 0, len(items))
	for _, item := range items {
		result = append(result, cannedResponseDTO{
			ID:        item.ID,
			Shortcut:  item.Shortcut,
			Title:     item.Title,
			Content:   item.Content,
			CreatedBy: item.CreatedBy,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		})
	}

	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleCannedCreate(w http.ResponseWriter, r *http.Request, adminUser *database.WebAdminUser) {
	var req cannedRequest
	if err := s.readJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Shortcut = sanitizeShortcut(strings.TrimSpace(req.Shortcut))
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	if req.Shortcut == "" || req.Title == "" || req.Content == "" {
		s.writeError(w, http.StatusBadRequest, "shortcut, title, and content are required")
		return
	}

	if !isValidShortcut(req.Shortcut) {
		s.writeError(w, http.StatusBadRequest, "shortcut must use only letters, numbers, and underscores")
		return
	}

	if err := s.deps.TicketService.CreateCannedResponse(r.Context(), req.Shortcut, req.Title, req.Content, webAdminActorID(adminUser.ID)); err != nil {
		s.writeError(w, http.StatusConflict, "failed to save canned response")
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleCannedDelete(w http.ResponseWriter, r *http.Request, adminUser *database.WebAdminUser) {
	shortcut := sanitizeShortcut(strings.TrimSpace(r.PathValue("shortcut")))
	if shortcut == "" {
		s.writeError(w, http.StatusBadRequest, "invalid canned shortcut")
		return
	}
	if err := s.deps.TicketService.DeleteCannedResponse(r.Context(), shortcut); err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to delete canned response")
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleBannedList(w http.ResponseWriter, r *http.Request, adminUser *database.WebAdminUser) {
	users, err := s.deps.TicketService.GetAllBannedUsers(r.Context())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to load banned users")
		return
	}

	items := make([]bannedUserDTO, 0, len(users))
	for _, user := range users {
		items = append(items, bannedUserDTO{
			ID:           user.ID,
			UserID:       user.UserID,
			DisplayName:  bannedUserDisplayName(user),
			UserName:     user.UserName,
			UserUsername: user.UserUsername,
			BannedBy:     user.BannedBy,
			BannedByName: user.BannedByName,
			Reason:       user.Reason,
			BannedAt:     user.BannedAt,
		})
	}

	s.writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleBannedCreate(w http.ResponseWriter, r *http.Request, adminUser *database.WebAdminUser) {
	var req banRequest
	if err := s.readJSON(r, &req); err != nil || req.UserID == 0 {
		s.writeError(w, http.StatusBadRequest, "userId is required")
		return
	}

	if err := s.deps.TicketService.BanUser(r.Context(), req.UserID, nil, nil, webAdminActorID(adminUser.ID), adminUser.DisplayName, strings.TrimSpace(req.Reason)); err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to ban user")
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleBannedDelete(w http.ResponseWriter, r *http.Request, adminUser *database.WebAdminUser) {
	userID, err := strconv.ParseInt(r.PathValue("userID"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	if err := s.deps.TicketService.UnbanUser(r.Context(), userID); err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to unban user")
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func toTicketDTO(ticket *database.Ticket) ticketDTO {
	return ticketDTO{
		ID:           ticket.ID,
		TicketNumber: ticket.TicketNumber,
		UserID:       ticket.UserID,
		DisplayName:  ticket.GetDisplayName(),
		Status:       ticket.Status,
		Priority:     ticket.Priority,
		Subject:      ticket.Subject,
		ThreadID:     ticket.ThreadID,
		CreatedAt:    ticket.CreatedAt,
		UpdatedAt:    ticket.UpdatedAt,
		ClosedAt:     ticket.ClosedAt,
	}
}

func webAdminActorID(adminUserID int) int64 {
	return -int64(adminUserID)
}

func adminSenderIdentity(adminUser *database.WebAdminUser) (*string, *string) {
	if adminUser == nil {
		return nil, nil
	}

	name := adminUser.DisplayName
	var username *string
	switch {
	case adminUser.Username != nil && *adminUser.Username != "":
		value := *adminUser.Username
		username = &value
	case adminUser.GitHubLogin != nil && *adminUser.GitHubLogin != "":
		value := *adminUser.GitHubLogin
		username = &value
	}
	return &name, username
}

func sanitizeShortcut(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(value, "/"))
	return strings.ToLower(value)
}

func isValidShortcut(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' {
			continue
		}
		return false
	}
	return true
}

func formatTicketExportFileName(ticketNumber string) string {
	ticketNumber = strings.ReplaceAll(strings.TrimSpace(ticketNumber), "#", "")
	if ticketNumber == "" {
		ticketNumber = "ticket"
	}
	return fmt.Sprintf("ticket-%s-history.html", ticketNumber)
}

func buildTicketHistoryHTML(ticket *database.Ticket, messages []*database.TicketMessage) string {
	var sb strings.Builder
	sb.WriteString("<!DOCTYPE html><html><head><meta charset=\"utf-8\">")
	sb.WriteString("<title>Ticket History</title>")
	sb.WriteString("<style>")
	sb.WriteString("body{font-family:Arial,sans-serif;background:#f5f5f5;color:#222;margin:24px;}")
	sb.WriteString(".meta,.message{background:#fff;border:1px solid #ddd;border-radius:8px;padding:16px;margin-bottom:12px;}")
	sb.WriteString(".role{font-weight:bold;margin-bottom:6px;}")
	sb.WriteString(".time{color:#666;font-size:12px;margin-bottom:8px;}")
	sb.WriteString(".text{white-space:pre-wrap;}")
	sb.WriteString(".tag{display:inline-block;background:#eef3ff;border-radius:999px;padding:2px 8px;font-size:12px;margin-left:8px;}")
	sb.WriteString("</style></head><body>")

	sb.WriteString("<div class=\"meta\">")
	sb.WriteString(fmt.Sprintf("<h1>Ticket #%s</h1>", html.EscapeString(ticket.TicketNumber)))
	sb.WriteString(fmt.Sprintf("<div><b>Status:</b> %s</div>", html.EscapeString(ticket.Status)))
	sb.WriteString(fmt.Sprintf("<div><b>Priority:</b> %s</div>", html.EscapeString(ticket.Priority)))
	sb.WriteString(fmt.Sprintf("<div><b>User:</b> %s</div>", html.EscapeString(ticket.GetDisplayName())))
	sb.WriteString(fmt.Sprintf("<div><b>User ID:</b> %d</div>", ticket.UserID))
	sb.WriteString(fmt.Sprintf("<div><b>Created:</b> %s</div>", ticket.CreatedAt.Format("2006-01-02 15:04:05 MST")))
	if ticket.ClosedAt != nil {
		sb.WriteString(fmt.Sprintf("<div><b>Closed:</b> %s</div>", ticket.ClosedAt.Format("2006-01-02 15:04:05 MST")))
	}
	if ticket.Subject != nil && *ticket.Subject != "" {
		sb.WriteString(fmt.Sprintf("<div><b>Subject:</b> %s</div>", html.EscapeString(*ticket.Subject)))
	}
	sb.WriteString(fmt.Sprintf("<div><b>Messages:</b> %d</div>", len(messages)))
	sb.WriteString("</div>")

	for idx, msg := range messages {
		role := "User"
		if msg.IsAdmin {
			role = "Support"
		}
		text := ""
		if msg.MessageText != nil {
			text = utils.RichTextHTMLForDisplay(*msg.MessageText)
		}
		if text == "" && msg.HasMedia {
			text = "<i>[attachment]</i>"
		}
		if text == "" {
			text = "<i>[no text]</i>"
		}
		sb.WriteString("<div class=\"message\">")
		sb.WriteString(fmt.Sprintf("<div class=\"role\">%d. %s%s</div>", idx+1, html.EscapeString(exportSenderName(msg)), exportMediaTag(msg)))
		sb.WriteString(fmt.Sprintf("<div class=\"time\">%s | %s</div>", msg.CreatedAt.Format("2006-01-02 15:04:05 MST"), role))
		sb.WriteString(fmt.Sprintf("<div class=\"text\">%s</div>", text))
		sb.WriteString("</div>")
	}

	sb.WriteString("</body></html>")
	return sb.String()
}

func exportSenderName(msg *database.TicketMessage) string {
	if msg == nil {
		return "Unknown"
	}
	if msg.SenderUsername != nil && *msg.SenderUsername != "" {
		return "@" + *msg.SenderUsername
	}
	if msg.SenderName != nil && *msg.SenderName != "" {
		return *msg.SenderName
	}
	if msg.IsAdmin {
		return "Support"
	}
	return "User"
}

func exportMediaTag(msg *database.TicketMessage) string {
	if msg == nil || !msg.HasMedia || msg.MediaType == nil || *msg.MediaType == "" {
		return ""
	}
	return fmt.Sprintf(" <span class=\"tag\">%s</span>", html.EscapeString(*msg.MediaType))
}

const maxTicketReplyUploadBytes = 20 << 20

func readTicketReplyPayload(r *http.Request) (string, []services.SupportUpload, error) {
	if r == nil {
		return "", nil, fmt.Errorf("invalid request")
	}

	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxTicketReplyUploadBytes * 2); err != nil {
			return "", nil, fmt.Errorf("invalid attachment payload")
		}

		text := strings.TrimSpace(r.FormValue("text"))
		files := r.MultipartForm.File["files"]
		uploads := make([]services.SupportUpload, 0, len(files))
		for _, header := range files {
			if header == nil || header.Size == 0 {
				continue
			}

			file, err := header.Open()
			if err != nil {
				return "", nil, fmt.Errorf("failed to read attachment")
			}

			data, readErr := io.ReadAll(io.LimitReader(file, maxTicketReplyUploadBytes+1))
			_ = file.Close()
			if readErr != nil {
				return "", nil, fmt.Errorf("failed to read attachment")
			}
			if len(data) > maxTicketReplyUploadBytes {
				return "", nil, fmt.Errorf("attachments must be 20 MB or smaller")
			}
			if len(data) == 0 {
				continue
			}

			detectedType := strings.TrimSpace(header.Header.Get("Content-Type"))
			if detectedType == "" {
				detectedType = http.DetectContentType(data)
			}

			uploads = append(uploads, services.SupportUpload{
				Filename:    header.Filename,
				ContentType: detectedType,
				Data:        data,
			})
		}

		if text == "" && len(uploads) == 0 {
			return "", nil, fmt.Errorf("reply text or attachment is required")
		}
		return text, uploads, nil
	}

	var req replyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Text) == "" {
		return "", nil, fmt.Errorf("reply text or attachment is required")
	}

	return strings.TrimSpace(req.Text), nil, nil
}

func stringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func messageIDPtr(message *models.Message) *int64 {
	if message == nil {
		return nil
	}
	id := int64(message.ID)
	return &id
}

func bannedUserDisplayName(user *database.BannedUser) string {
	if user == nil {
		return "Unknown"
	}
	if user.UserUsername != nil && *user.UserUsername != "" {
		return "@" + *user.UserUsername
	}
	if user.UserName != nil && *user.UserName != "" {
		return *user.UserName
	}
	return fmt.Sprintf("User %d", user.UserID)
}
