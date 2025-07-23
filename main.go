package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

const (
	channelUsername = "@art_rom" // Замените на @username вашего канала
)

var (
	db   *sql.DB
	dbMu sync.Mutex // Мьютекс для синхронизации доступа к базе данных
)

func main() {
	// Инициализация базы данных
	var err error
	db, err = sql.Open("sqlite3", "./users.db?_timeout=5000&_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	// Ограничиваем количество соединений
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Создание таблицы пользователей
	createTable()

	// Инициализация бота
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		log.Fatal("Failed to create bot:", err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	// Запуск фоновой проверки подписок
	go checkSubscriptionsRoutine(bot)

	// Настройка обновлений
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	updates := bot.GetUpdatesChan(updateConfig)

	// Обработка сообщений
	for update := range updates {
		if update.Message == nil {
			continue
		}

		go handleMessage(bot, update) // Обрабатываем сообщения в отдельной горутине
	}
}

func handleMessage(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	userID := update.Message.From.ID
	username := update.Message.From.UserName

	// Обработка команды /start
	if update.Message.Text == "/start" {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Привет! Я бот для проверки подписки. Используйте /check для проверки подписки и получения UUID.")
		bot.Send(msg)
		return
	}

	// Обработка команды /check
	if update.Message.Text == "/check" {
		// Проверяем подписку пользователя
		isSubscribed, err := checkSubscription(bot, userID, channelUsername)
		if err != nil {
			log.Printf("Error checking subscription: %v", err)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Произошла ошибка при проверке подписки. Пожалуйста, попробуйте позже.")
			bot.Send(msg)
			return
		}

		if isSubscribed {
			// Генерируем или получаем существующий UUID
			userUUID, err := getOrCreateUUID(userID, username)
			if err != nil {
				log.Printf("Error generating UUID: %v", err)
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Произошла ошибка при генерации UUID. Пожалуйста, попробуйте позже.")
				bot.Send(msg)
				return
			}

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Вы подписаны на канал! Ваш UUID: %s", userUUID))
			bot.Send(msg)
		} else {
			// Если пользователь не подписан, удаляем его из базы, если он там есть
			dbMu.Lock()
			_, err := db.Exec("DELETE FROM users WHERE user_id = ?", userID)
			dbMu.Unlock()
			if err != nil {
				log.Printf("Error deleting user %d: %v", userID, err)
			}

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Вы не подписаны на канал %s. Пожалуйста, подпишитесь и попробуйте снова.", channelUsername))
			bot.Send(msg)
		}
		return
	}

	// Обработка других сообщений
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Используйте /check для проверки подписки и получения UUID.")
	bot.Send(msg)
}

func createTable() {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		user_id INTEGER PRIMARY KEY,
		username TEXT,
		uuid TEXT,
		created_at TIMESTAMP
	);`

	dbMu.Lock()
	defer dbMu.Unlock()

	_, err := db.Exec(query)
	if err != nil {
		log.Fatal("Failed to create table:", err)
	}
}

func checkSubscription(bot *tgbotapi.BotAPI, userID int64, channelUsername string) (bool, error) {
	// Проверяем статус пользователя в канале
	config := tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			SuperGroupUsername: channelUsername,
			UserID:             userID,
		},
	}

	member, err := bot.GetChatMember(config)
	if err != nil {
		return false, err
	}

	// Проверяем, подписан ли пользователь
	return member.Status == "member" || member.Status == "administrator" || member.Status == "creator", nil
}

func getOrCreateUUID(userID int64, username string) (string, error) {
	var userUUID string

	dbMu.Lock()
	defer dbMu.Unlock()

	// Проверяем, есть ли уже UUID для пользователя
	err := db.QueryRow("SELECT uuid FROM users WHERE user_id = ?", userID).Scan(&userUUID)
	if err == nil {
		// UUID уже существует
		return userUUID, nil
	}

	if err != sql.ErrNoRows {
		// Произошла ошибка при запросе
		return "", err
	}

	// Генерируем новый UUID
	userUUID = uuid.New().String()

	// Сохраняем пользователя в БД
	_, err = db.Exec(
		"INSERT INTO users (user_id, username, uuid, created_at) VALUES (?, ?, ?, ?)",
		userID, username, userUUID, time.Now(),
	)
	if err != nil {
		return "", err
	}

	return userUUID, nil
}

func checkSubscriptionsRoutine(bot *tgbotapi.BotAPI) {
	// Запускаем проверку каждые 1-24 часа
	for {
		// Случайная задержка от 1 до 24 часов (для тестирования используем секунды)
		sleepHours := rand.Intn(23) + 1
		// time.Sleep(time.Duration(sleepHours) * time.Minute)
		time.Sleep(time.Duration(sleepHours) * time.Second)

		log.Println("Starting subscription check for all users...")

		dbMu.Lock()
		// Проверяем всех пользователей в базе
		rows, err := db.Query("SELECT user_id, username FROM users")
		if err != nil {
			log.Printf("Error querying users: %v", err)
			dbMu.Unlock()
			continue
		}

		// Собираем информацию о пользователях для проверки
		type UserInfo struct {
			ID       int64
			Username string
		}
		var users []UserInfo
		for rows.Next() {
			var user UserInfo
			if err := rows.Scan(&user.ID, &user.Username); err != nil {
				log.Printf("Error scanning user info: %v", err)
				continue
			}
			users = append(users, user)
		}
		rows.Close()
		dbMu.Unlock()

		// Проверяем подписки и удаляем пользователей, если необходимо
		for _, user := range users {
			// Проверяем подписку
			isSubscribed, err := checkSubscription(bot, user.ID, channelUsername)
			if err != nil {
				log.Printf("Error checking subscription for user %d: %v", user.ID, err)
				continue
			}

			// Если пользователь отписался, удаляем его из базы и отправляем уведомление
			if !isSubscribed {
				dbMu.Lock()
				_, err := db.Exec("DELETE FROM users WHERE user_id = ?", user.ID)
				dbMu.Unlock()
				if err != nil {
					log.Printf("Error deleting user %d: %v", user.ID, err)
				} else {
					log.Printf("User %d removed due to unsubscription", user.ID)

					// Отправляем сообщение пользователю об отписке
					msg := tgbotapi.NewMessage(user.ID, fmt.Sprintf(
						"Вы отписались от канала %s. Ваш доступ к сервису был аннулирован. Чтобы восстановить доступ, подпишитесь на канал и используйте команду /check.",
						channelUsername))

					if _, err := bot.Send(msg); err != nil {
						log.Printf("Error sending unsubscription notification to user %d: %v", user.ID, err)
					} else {
						log.Printf("Unsubscription notification sent to user %d", user.ID)
					}
				}
			}
		}

		log.Println("Subscription check completed")
	}
}
