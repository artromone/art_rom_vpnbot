package services

import (
	"fmt"
	"log"
	"math/rand"
	"time"
	"xray-telegram-bot/config"
	"xray-telegram-bot/messages"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramService struct {
	bot         *tgbotapi.BotAPI
	config      *config.Config
	userService *UserService
}

func NewTelegramService(bot *tgbotapi.BotAPI, cfg *config.Config, userService *UserService) *TelegramService {
	return &TelegramService{
		bot:         bot,
		config:      cfg,
		userService: userService,
	}
}

func (s *TelegramService) HandleMessage(update tgbotapi.Update) {
	userID := update.Message.From.ID
	username := update.Message.From.UserName

	switch update.Message.Text {
	case "/start":
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, messages.StartMessage)
		s.bot.Send(msg)
		return

	case "/check":
		s.handleCheckCommand(update.Message.Chat.ID, userID, username)
		return

	default:
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, messages.HelpMessage)
		s.bot.Send(msg)
	}
}

func (s *TelegramService) handleCheckCommand(chatID, userID int64, username string) {
	isSubscribed, err := s.checkSubscription(userID)
	if err != nil {
		log.Printf("Error checking subscription: %v", err)
		msg := tgbotapi.NewMessage(chatID, messages.SubscriptionCheckError)
		s.bot.Send(msg)
		return
	}

	if isSubscribed {
		userUUID, vlessURL, err := s.userService.GetOrCreateVlessConfig(userID, username)
		if err != nil {
			log.Printf("Error generating VLESS config: %v", err)
			msg := tgbotapi.NewMessage(chatID, messages.ConfigGenerationError)
			s.bot.Send(msg)
			return
		}

		responseText := fmt.Sprintf(messages.SubscribedMessage, userUUID, vlessURL)
		msg := tgbotapi.NewMessage(chatID, responseText)
		msg.ParseMode = "Markdown"
		s.bot.Send(msg)
	} else {
		if err := s.userService.RemoveUser(userID); err != nil {
			log.Printf("Error removing user %d: %v", userID, err)
		}

		responseText := fmt.Sprintf(messages.NotSubscribedMessage, s.config.ChannelUsername)
		msg := tgbotapi.NewMessage(chatID, responseText)
		s.bot.Send(msg)
	}
}

func (s *TelegramService) checkSubscription(userID int64) (bool, error) {
	config := tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			SuperGroupUsername: s.config.ChannelUsername,
			UserID:             userID,
		},
	}

	member, err := s.bot.GetChatMember(config)
	if err != nil {
		return false, err
	}

	return member.Status == "member" || member.Status == "administrator" || member.Status == "creator", nil
}

func (s *TelegramService) StartSubscriptionChecker() {
	go func() {
		for {
			sleepHours := rand.Intn(23) + 1
			time.Sleep(time.Duration(sleepHours) * time.Hour)

			log.Println("Starting subscription check for all users...")
			s.checkAllSubscriptions()
			log.Println("Subscription check completed")
		}
	}()
}

func (s *TelegramService) checkAllSubscriptions() {
	users, err := s.userService.GetAllUsers()
	if err != nil {
		log.Printf("Error querying users: %v", err)
		return
	}

	for _, user := range users {
		isSubscribed, err := s.checkSubscription(user.ID)
		if err != nil {
			log.Printf("Error checking subscription for user %d: %v", user.ID, err)
			continue
		}

		if !isSubscribed {
			if err := s.userService.RemoveUser(user.ID); err != nil {
				log.Printf("Error removing user %d: %v", user.ID, err)
			} else {
				log.Printf("User %d removed due to unsubscription", user.ID)

				notificationText := fmt.Sprintf(messages.UnsubscriptionNotification, s.config.ChannelUsername)
				msg := tgbotapi.NewMessage(user.ID, notificationText)

				if _, err := s.bot.Send(msg); err != nil {
					log.Printf("Error sending unsubscription notification to user %d: %v", user.ID, err)
				} else {
					log.Printf("Unsubscription notification sent to user %d", user.ID)
				}
			}
		}
	}
}
