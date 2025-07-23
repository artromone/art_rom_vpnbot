package main

import (
	"log"
	"xray-telegram-bot/config"
	"xray-telegram-bot/database"
	"xray-telegram-bot/services"
	"xray-telegram-bot/xray"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.New(cfg.DatabasePath)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	// Initialize Xray client
	xrayClient := xray.NewClient(cfg)

	// Initialize Xray API
	if err := xrayClient.InitAPI(); err != nil {
		log.Printf("Warning: Failed to initialize Xray API: %v", err)
		log.Println("Please ensure Xray API is properly configured")
	}

	// Test API connectivity
	if err := xrayClient.TestAPI(); err != nil {
		log.Printf("Warning: Xray API test failed: %v", err)
	} else {
		log.Println("Xray API is accessible")
	}

	// Initialize services
	userService := services.NewUserService(db, xrayClient)

	// Initialize Telegram bot
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramBotToken)
	if err != nil {
		log.Fatal("Failed to create bot:", err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	telegramService := services.NewTelegramService(bot, cfg, userService)

	// Start subscription checker
	telegramService.StartSubscriptionChecker()

	// Start bot
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	updates := bot.GetUpdatesChan(updateConfig)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		go telegramService.HandleMessage(update)
	}
}
