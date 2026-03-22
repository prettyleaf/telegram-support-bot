// Package app provides the main application structure with dependency injection.
package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"remnafamily/internal/config"
	"remnafamily/internal/database"
	"remnafamily/internal/handlers"
	"remnafamily/internal/keyboards"
	"remnafamily/internal/messages"
	"remnafamily/internal/services"
	"remnafamily/internal/state"
	"remnafamily/internal/types"
	"remnafamily/internal/utils"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// App represents the main application with all its dependencies.
type App struct {
	Config        *config.Config
	Bot           *bot.Bot
	DB            *database.Repository
	State         *state.AppState
	deps          *types.Dependencies
	ctx           context.Context
	cancelFunc    context.CancelFunc
	userHandlers  *handlers.UserHandlers
	adminHandlers *handlers.AdminHandlers
}

// New creates a new application instance with all dependencies initialized.
func New(ctx context.Context) (*App, error) {
	// Load configuration from .env
	if err := config.Load(); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Load YAML configuration
	yamlConfig, err := config.LoadYAMLConfig("config.yaml")
	if err != nil {
		return nil, fmt.Errorf("load YAML config: %w", err)
	}

	// Store YAML config globally
	config.YAMLCfg = yamlConfig
	utils.LogSystem("Configuration loaded (log_level=%s)", yamlConfig.Bot.LogLevel)

	// Update logger level from YAML config
	if err := utils.UpdateLogLevel(); err != nil {
		utils.Logger.Warnf("Failed to update log level: %v", err)
	}

	// Create app context
	appCtx, cancel := context.WithCancel(ctx)

	// Connect to database
	if err := database.Connect(appCtx, yamlConfig); err != nil {
		cancel()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	utils.LogSystem("Database connected")

	// Run database migrations
	if err := database.MigrateUp(appCtx); err != nil {
		cancel()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	// Create repository
	db := database.NewRepository()

	// Create app state
	appState := state.NewAppState()

	// Add admins to state
	for _, id := range config.Cfg.AdminIDs {
		appState.AddAdmin(id)
	}
	utils.LogSystem("Loaded %d admins from config", len(config.Cfg.AdminIDs))

	// Create bot instance
	opts := []bot.Option{
		bot.WithDefaultHandler(func(ctx context.Context, b *bot.Bot, update *models.Update) {
			// Default handler - will be overridden
		}),
	}

	b, err := bot.New(config.Cfg.BotToken, opts...)
	if err != nil {
		database.Close()
		cancel()
		return nil, fmt.Errorf("create bot: %w", err)
	}

	// Create app
	app := &App{
		Config:     config.Cfg,
		Bot:        b,
		DB:         db,
		State:      appState,
		ctx:        appCtx,
		cancelFunc: cancel,
	}

	// Create dependencies
	app.deps = app.createDependencies()

	// Create handlers
	app.userHandlers = handlers.NewUserHandlers(app.deps)
	app.adminHandlers = handlers.NewAdminHandlers(app.deps)

	// Register handlers
	app.registerHandlers()

	return app, nil
}

// createDependencies creates and initializes all service dependencies.
func (a *App) createDependencies() *types.Dependencies {
	remnawaveService := services.NewRemnawaveService(a.Config)
	if remnawaveService != nil {
		utils.LogSystem("Remnawave panel integration enabled")
	} else if a.Config != nil && (a.Config.RemnawaveURL != "" || a.Config.RemnawaveToken != "") {
		utils.Logger.Warn("Remnawave panel integration is partially configured and will stay disabled")
	}

	return &types.Dependencies{
		Bot:              a.Bot,
		DB:               a.DB,
		State:            a.State,
		Config:           a.Config,
		CaptchaService:   services.NewCaptchaService(),
		TicketService:    services.NewTicketService(a.DB),
		RemnawaveService: remnawaveService,
	}
}

// registerHandlers registers all bot handlers.
func (a *App) registerHandlers() {
	// User commands (in private chats)
	a.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypePrefix, a.handlePrivateCommand(a.userHandlers.HandleStart))
	a.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/newticket", bot.MatchTypePrefix, a.handlePrivateCommand(a.userHandlers.HandleNewTicket))
	a.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/mytickets", bot.MatchTypePrefix, a.handlePrivateCommand(a.userHandlers.HandleMyTickets))
	a.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/devices", bot.MatchTypePrefix, a.handlePrivateCommand(a.userHandlers.HandleDevices))
	a.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/cancel", bot.MatchTypePrefix, a.handlePrivateCommand(a.userHandlers.HandleCancel))
	a.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/language", bot.MatchTypePrefix, a.handlePrivateCommand(a.userHandlers.HandleLanguage))

	// Admin commands
	a.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/help", bot.MatchTypePrefix, a.adminHandlers.HandleAdminHelp)
	a.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/tickets", bot.MatchTypePrefix, a.adminHandlers.HandleTickets)
	a.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/close", bot.MatchTypePrefix, a.adminHandlers.HandleClose)
	a.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/reopen", bot.MatchTypePrefix, a.adminHandlers.HandleReopen)
	a.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/priority", bot.MatchTypePrefix, a.adminHandlers.HandlePriority)
	a.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/info", bot.MatchTypePrefix, a.adminHandlers.HandleInfo)
	a.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/ban", bot.MatchTypePrefix, a.adminHandlers.HandleBan)
	a.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/unban", bot.MatchTypePrefix, a.adminHandlers.HandleUnban)
	a.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/banned", bot.MatchTypePrefix, a.adminHandlers.HandleBanned)
	a.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/note", bot.MatchTypePrefix, a.adminHandlers.HandleNote)
	a.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/notes", bot.MatchTypePrefix, a.adminHandlers.HandleNotes)
	a.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/stats", bot.MatchTypePrefix, a.adminHandlers.HandleStats)
	a.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/canned", bot.MatchTypePrefix, a.adminHandlers.HandleCanned)
	a.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/addcanned", bot.MatchTypePrefix, a.adminHandlers.HandleAddCanned)
	a.Bot.RegisterHandler(bot.HandlerTypeMessageText, "/delcanned", bot.MatchTypePrefix, a.adminHandlers.HandleDelCanned)

	// Callback queries
	a.Bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "", bot.MatchTypePrefix, a.handleCallback)

	// Default message handler
	a.Bot.RegisterHandler(bot.HandlerTypeMessageText, "", bot.MatchTypePrefix, a.handleMessage)
}

// handlePrivateCommand wraps a handler to only work in private chats
func (a *App) handlePrivateCommand(handler func(ctx context.Context, b *bot.Bot, update *models.Update)) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}
		if update.Message.Chat.Type != "private" {
			return
		}
		handler(ctx, b, update)
	}
}

// handleCallback routes callback queries to the appropriate handler
func (a *App) handleCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	data := update.CallbackQuery.Data
	userID := update.CallbackQuery.From.ID

	// Admin callbacks
	if config.IsAdmin(userID) {
		if (strings.HasPrefix(data, "ticket:") && data != "ticket:create") ||
			strings.HasPrefix(data, "priority:") ||
			strings.HasPrefix(data, "tickets:") ||
			strings.HasPrefix(data, "banned:") ||
			strings.HasPrefix(data, "canned:") {
			a.adminHandlers.HandleAdminCallback(ctx, b, update)
			return
		}
	}

	// User callbacks
	a.userHandlers.HandleUserCallback(ctx, b, update)
}

// handleMessage routes messages to the appropriate handler
func (a *App) handleMessage(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	message := update.Message

	if message.Text != "" && strings.HasPrefix(message.Text, "/") {
		if message.Chat.ID == config.Cfg.SupportChatID && config.IsAdmin(message.From.ID) {
			a.adminHandlers.HandleAdminDynamicCommand(ctx, b, update)
		}
		return
	}

	// Check if this is in the support chat (admin message)
	if message.Chat.ID == config.Cfg.SupportChatID {
		if config.IsAdmin(message.From.ID) {
			a.adminHandlers.HandleAdminMessage(ctx, b, update)
		}
		return
	}

	// Private chat messages from users
	if message.Chat.Type == "private" {
		a.userHandlers.HandleUserMessage(ctx, b, update)
	}
}

// Start starts the application.
func (a *App) Start() error {
	utils.LogSystem("Bot is starting...")

	// Get bot info
	me, err := a.Bot.GetMe(a.ctx)
	if err != nil {
		return fmt.Errorf("get bot info: %w", err)
	}
	utils.LogSystem("Bot started as @%s", me.Username)
	utils.LogSystem("Support chat ID: %d", config.Cfg.SupportChatID)
	utils.LogSystem("Admins: %v", config.Cfg.AdminIDs)

	// Start bot in background
	go a.Bot.Start(a.ctx)

	if config.YAMLCfg != nil && config.YAMLCfg.Tickets.AutoCloseHours > 0 {
		go a.startAutoCloseLoop()
	}

	return nil
}

// Stop stops the application gracefully.
func (a *App) Stop() error {
	utils.LogSystem("Shutting down...")

	// Cancel context
	a.cancelFunc()

	// Close database
	database.Close()

	utils.LogSystem("Shutdown complete")
	return nil
}

func (a *App) startAutoCloseLoop() {
	a.runAutoClosePass()

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.runAutoClosePass()
		}
	}
}

func (a *App) runAutoClosePass() {
	if config.YAMLCfg == nil || config.YAMLCfg.Tickets.AutoCloseHours <= 0 {
		return
	}

	cutoff := time.Now().Add(-time.Duration(config.YAMLCfg.Tickets.AutoCloseHours) * time.Hour)
	tickets, err := a.deps.TicketService.GetInactiveOpenTickets(a.ctx, cutoff)
	if err != nil {
		utils.LogError("load inactive tickets for auto-close", err)
		return
	}
	if len(tickets) == 0 {
		return
	}

	for _, ticket := range tickets {
		if ticket == nil {
			continue
		}

		if err := a.deps.TicketService.CloseTicket(a.ctx, a.Bot, ticket.ID, 0, "Auto Close"); err != nil {
			utils.LogError("auto-close ticket", err)
			continue
		}

		utils.LogTicket("Ticket #%s auto-closed after inactivity", ticket.TicketNumber)
		a.State.ClearUserState(ticket.UserID)

		lang, err := a.DB.GetUserLanguage(a.ctx, ticket.UserID)
		if err != nil {
			lang = "en"
		}

		_, _ = a.Bot.SendMessage(a.ctx, &bot.SendMessageParams{
			ChatID:    ticket.UserID,
			Text:      messages.GetTicketClosedMessageLang(ticket.TicketNumber, lang),
			ParseMode: models.ParseModeHTML,
		})

		if ticket.ThreadID > 1 {
			_, _ = a.Bot.SendMessage(a.ctx, &bot.SendMessageParams{
				ChatID:          ticket.SupportChatID,
				MessageThreadID: int(ticket.ThreadID),
				Text:            "🔒 <b>Ticket auto-closed due to inactivity.</b>",
				ParseMode:       models.ParseModeHTML,
				ReplyMarkup:     keyboards.TicketReopenKeyboard(ticket.ID),
			})
		}
	}
}
