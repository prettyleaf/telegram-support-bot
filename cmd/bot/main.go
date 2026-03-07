package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"remnafamily/internal/app"
	"remnafamily/internal/utils"
)

func main() {
	// Initialize logger
	if err := utils.InitLogger(); err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer utils.Sync()

	utils.Logger.Info("")
	utils.LogSystem("🎫 Starting Support Ticket Bot")
	utils.Logger.Info("")

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create application with dependency injection
	application, err := app.New(ctx)
	if err != nil {
		utils.Logger.Fatalf("Failed to create application: %v", err)
	}

	// Start application
	if err := application.Start(); err != nil {
		utils.Logger.Fatalf("Failed to start application: %v", err)
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	<-sigChan

	utils.Logger.Info("")
	utils.LogSystem("Received shutdown signal")

	// Stop application gracefully
	if err := application.Stop(); err != nil {
		utils.Logger.Errorf("Error during shutdown: %v", err)
	}
}
