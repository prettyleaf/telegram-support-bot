package types

import (
	"remnafamily/internal/config"
	"remnafamily/internal/database"
	"remnafamily/internal/services"
	"remnafamily/internal/state"

	"github.com/go-telegram/bot"
)

// Dependencies holds all the dependencies needed by handlers and services
type Dependencies struct {
	Bot              *bot.Bot
	DB               *database.Repository
	State            *state.AppState
	Config           *config.Config
	CaptchaService   *services.CaptchaService
	TicketService    *services.TicketService
	RemnawaveService *services.RemnawaveService
}
