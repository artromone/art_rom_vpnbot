package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

const (
	channelUsername = "@art_rom" // Замените на @username вашего канала
	xrayAPIAddress  = "127.0.0.1:10085"
	xrayTag         = "vless_tls"
	serverDomain    = "artr.ignorelist.com"
	serverPort      = 443
	configPath      = "/usr/local/etc/xray/config.json"
)

var (
	db   *sql.DB
	dbMu sync.Mutex // Мьютекс для синхронизации доступа к базе данных
)

// Структуры для работы с Xray
type XrayUser struct {
	Email string `json:"email"`
	ID    string `json:"id"`
	Flow  string `json:"flow"`
}

type XrayConfig struct {
	Log       interface{}   `json:"log"`
	Routing   interface{}   `json:"routing"`
	Inbounds  []interface{} `json:"inbounds"`
	Outbounds []interface{} `json:"outbounds"`
	API       interface{}   `json:"api,omitempty"`
}

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

	// Инициализация Xray API (если еще не настроен)
	if err := initXrayAPI(); err != nil {
		log.Printf("Warning: Failed to initialize Xray API: %v", err)
		log.Println("Please ensure Xray API is properly configured")
	}

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
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Привет! Я бот для проверки подписки. Используйте /check для проверки подписки и получения конфигурации VPN.")
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
			// Генерируем или получаем существующий UUID и добавляем в Xray
			userUUID, vlessURL, err := getOrCreateVlessConfig(userID, username)
			if err != nil {
				log.Printf("Error generating VLESS config: %v", err)
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Произошла ошибка при генерации конфигурации. Пожалуйста, попробуйте позже.")
				bot.Send(msg)
				return
			}

			responseText := fmt.Sprintf("Вы подписаны на канал! \n\nВаш UUID: `%s`\n\nВаша VLESS конфигурация:\n`%s`", userUUID, vlessURL)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, responseText)
			msg.ParseMode = "Markdown"
			bot.Send(msg)
		} else {
			// Если пользователь не подписан, удаляем его из Xray и базы
			if err := removeUserFromXray(userID); err != nil {
				log.Printf("Error removing user %d from Xray: %v", userID, err)
			}

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
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Используйте /check для проверки подписки и получения конфигурации VPN.")
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

func getOrCreateVlessConfig(userID int64, username string) (string, string, error) {
	var userUUID string

	dbMu.Lock()
	defer dbMu.Unlock()

	// Проверяем, есть ли уже UUID для пользователя
	err := db.QueryRow("SELECT uuid FROM users WHERE user_id = ?", userID).Scan(&userUUID)
	if err == nil {
		// UUID уже существует, генерируем VLESS URL
		vlessURL := generateVlessURL(userUUID, fmt.Sprintf("user_%d", userID))
		return userUUID, vlessURL, nil
	}

	if err != sql.ErrNoRows {
		// Произошла ошибка при запросе
		return "", "", err
	}

	// Генерируем новый UUID
	userUUID = uuid.New().String()
	email := fmt.Sprintf("user_%d@myserver", userID)

	// Добавляем пользователя в Xray
	if err := addUserToXray(userUUID, email); err != nil {
		return "", "", fmt.Errorf("failed to add user to Xray: %v", err)
	}

	// Сохраняем пользователя в БД
	_, err = db.Exec(
		"INSERT INTO users (user_id, username, uuid, created_at) VALUES (?, ?, ?, ?)",
		userID, username, userUUID, time.Now(),
	)
	if err != nil {
		// Если не удалось сохранить в БД, удаляем из Xray
		removeUserFromXrayByEmail(email)
		return "", "", err
	}

	vlessURL := generateVlessURL(userUUID, email)
	return userUUID, vlessURL, nil
}

func checkSubscriptionsRoutine(bot *tgbotapi.BotAPI) {
	// Запускаем проверку каждые 1-24 часа
	for {
		// Случайная задержка от 1 до 24 часов (для тестирования используем секунды)
		sleepHours := rand.Intn(23) + 1
		// time.Sleep(time.Duration(sleepHours) * time.Hour)
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

			// Если пользователь отписался, удаляем его из базы и Xray
			if !isSubscribed {
				// Удаляем из Xray
				if err := removeUserFromXray(user.ID); err != nil {
					log.Printf("Error removing user %d from Xray: %v", user.ID, err)
				}

				// Удаляем из базы данных
				dbMu.Lock()
				_, err := db.Exec("DELETE FROM users WHERE user_id = ?", user.ID)
				dbMu.Unlock()
				if err != nil {
					log.Printf("Error deleting user %d: %v", user.ID, err)
				} else {
					log.Printf("User %d removed due to unsubscription", user.ID)

					// Отправляем сообщение пользователю об отписке
					msg := tgbotapi.NewMessage(user.ID, fmt.Sprintf(
						"Вы отписались от канала %s. Ваш доступ к VPN был аннулирован. Чтобы восстановить доступ, подпишитесь на канал и используйте команду /check.",
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

// Xray API functions

func initXrayAPI() error {
	// Проверяем, есть ли уже API в конфигурации
	config, err := readXrayConfig()
	if err != nil {
		return err
	}

	// Если API уже настроен, ничего не делаем
	if config.API != nil {
		return nil
	}

	log.Println("Adding API configuration to Xray config...")

	// Добавляем API конфигурацию
	config.API = map[string]interface{}{
		"tag": "api",
		"services": []string{
			"HandlerService",
			"StatsService",
		},
	}

	// Добавляем API inbound
	apiInbound := map[string]interface{}{
		"listen":   "127.0.0.1",
		"port":     10085,
		"protocol": "dokodemo-door",
		"settings": map[string]interface{}{
			"address": "127.0.0.1",
		},
		"tag": "api",
	}

	config.Inbounds = append(config.Inbounds, apiInbound)

	// Добавляем правило маршрутизации для API
	routing, ok := config.Routing.(map[string]interface{})
	if !ok {
		routing = make(map[string]interface{})
		config.Routing = routing
	}
	rules, ok := routing["rules"].([]interface{})
	if !ok {
		rules = []interface{}{}
	}

	apiRule := map[string]interface{}{
		"inboundTag":  []string{"api"},
		"outboundTag": "api",
		"type":        "field",
	}

	rules = append(rules, apiRule)
	routing["rules"] = rules

	// Добавляем API outbound
	apiOutbound := map[string]interface{}{
		"protocol": "freedom",
		"tag":      "api",
	}

	config.Outbounds = append(config.Outbounds, apiOutbound)

	// Сохраняем конфигурацию
	if err := writeXrayConfig(config); err != nil {
		return err
	}

	log.Println("API configuration added. Please restart Xray manually to apply changes.")
	return nil
}

func readXrayConfig() (*XrayConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config XrayConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func writeXrayConfig(config *XrayConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

func addUserToXray(userUUID, email string) error {
	user := XrayUser{
		Email: email,
		ID:    userUUID,
		Flow:  "xtls-rprx-vision",
	}

	userJSON, err := json.Marshal(user)
	if err != nil {
		return err
	}

	cmd := exec.Command("xray", "api", "inbounduser",
		"-s", xrayAPIAddress,
		"-tag", xrayTag,
		"-operation", "add",
		"-user", string(userJSON))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add user to Xray: %v, output: %s", err, string(output))
	}

	log.Printf("User %s added to Xray successfully", email)
	return nil
}

func removeUserFromXray(userID int64) error {
	email := fmt.Sprintf("user_%d@myserver", userID)
	return removeUserFromXrayByEmail(email)
}

func removeUserFromXrayByEmail(email string) error {
	cmd := exec.Command("xray", "api", "inbounduser",
		"-s", xrayAPIAddress,
		"-tag", xrayTag,
		"-operation", "remove",
		"-email", email)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove user from Xray: %v, output: %s", err, string(output))
	}

	log.Printf("User %s removed from Xray successfully", email)
	return nil
}

func generateVlessURL(userUUID, name string) string {
	return fmt.Sprintf("vless://%s@%s:%d?security=tls&type=tcp&flow=xtls-rprx-vision&encryption=none#%s",
		userUUID, serverDomain, serverPort, name)
}

func listXrayUsers() ([]XrayUser, error) {
	cmd := exec.Command("xray", "api", "inbounduser",
		"-s", xrayAPIAddress,
		"-tag", xrayTag)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to list Xray users: %v, stderr: %s", err, stderr.String())
	}

	// Парсим вывод (формат может отличаться в зависимости от версии Xray)
	var users []XrayUser
	// Здесь нужно будет добавить парсинг JSON ответа от Xray API
	// Формат ответа зависит от конкретной версии Xray

	return users, nil
}
