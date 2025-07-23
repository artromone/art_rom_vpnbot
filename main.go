package main

import (
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
	channelUsername = "@art_rom"
	xrayAPIAddress  = "127.0.0.1:10085"
	xrayTag         = "vless_tls"
	serverDomain    = "artr.ignorelist.com"
	serverPort      = 443
	configPath      = "/usr/local/etc/xray/config.json"
)

var (
	db   *sql.DB
	dbMu sync.Mutex
)

// Xray API structures
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
	// Database initialization
	var err error
	db, err = sql.Open("sqlite3", "./users.db?_timeout=5000&_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	createTable()

	// Initialize Xray API
	if err := initXrayAPI(); err != nil {
		log.Printf("Warning: Failed to initialize Xray API: %v", err)
		log.Println("Please ensure Xray API is properly configured")
	}

	// Test API connectivity
	if err := testXrayAPI(); err != nil {
		log.Printf("Warning: Xray API test failed: %v", err)
	} else {
		log.Println("Xray API is accessible")
	}

	// Bot initialization
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		log.Fatal("Failed to create bot:", err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	go checkSubscriptionsRoutine(bot)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	updates := bot.GetUpdatesChan(updateConfig)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		go handleMessage(bot, update)
	}
}

// Test Xray API connectivity using command line
func testXrayAPI() error {
	cmd := exec.Command("xray", "api", "inbounduser",
		"--server="+xrayAPIAddress,
		"-tag="+xrayTag)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("API test failed: %v, output: %s", err, string(output))
	}

	log.Printf("API test successful: %s", string(output))
	return nil
}

// Command line implementation for adding users
func addUserToXray(userUUID, email string) error {
	// Create user JSON
	user := XrayUser{
		Email: email,
		ID:    userUUID,
		Flow:  "xtls-rprx-vision",
	}

	userJSON, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user: %v", err)
	}

	// Create temporary file for user data
	tmpFile, err := os.CreateTemp("", "xray_user_*.json")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.Write(userJSON); err != nil {
		return fmt.Errorf("failed to write user data: %v", err)
	}
	tmpFile.Close()

	// Execute xray command
	cmd := exec.Command("xray", "api", "inbounduser", "add",
		"--server="+xrayAPIAddress,
		"-tag="+xrayTag,
		"-user="+string(userJSON))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add user via CLI: %v, output: %s", err, string(output))
	}

	log.Printf("User %s added to Xray successfully: %s", email, string(output))
	return nil
}

// Также добавляем функцию для добавления в конфигурационный файл как резервный метод
func addUserToConfig(userUUID, email string) error {
	config, err := readXrayConfig()
	if err != nil {
		return fmt.Errorf("failed to read config: %v", err)
	}

	// Найти нужный inbound
	for i, inbound := range config.Inbounds {
		inboundMap, ok := inbound.(map[string]interface{})
		if !ok {
			continue
		}

		tag, exists := inboundMap["tag"]
		if !exists || tag != xrayTag {
			continue
		}

		settings, ok := inboundMap["settings"].(map[string]interface{})
		if !ok {
			continue
		}

		clients, ok := settings["clients"].([]interface{})
		if !ok {
			clients = []interface{}{}
		}

		// Создать нового клиента
		newClient := map[string]interface{}{
			"email": email,
			"id":    userUUID,
			"flow":  "xtls-rprx-vision",
		}

		clients = append(clients, newClient)
		settings["clients"] = clients
		config.Inbounds[i] = inboundMap

		// Сохранить конфигурацию
		if err := writeXrayConfig(config); err != nil {
			return fmt.Errorf("failed to write config: %v", err)
		}

		log.Printf("User %s added to config file", email)
		return nil
	}

	return fmt.Errorf("inbound with tag %s not found", xrayTag)
}

// Функция для удаления пользователя из конфигурации
func removeUserFromConfig(email string) error {
	config, err := readXrayConfig()
	if err != nil {
		return fmt.Errorf("failed to read config: %v", err)
	}

	// Найти нужный inbound
	for i, inbound := range config.Inbounds {
		inboundMap, ok := inbound.(map[string]interface{})
		if !ok {
			continue
		}

		tag, exists := inboundMap["tag"]
		if !exists || tag != xrayTag {
			continue
		}

		settings, ok := inboundMap["settings"].(map[string]interface{})
		if !ok {
			continue
		}

		clients, ok := settings["clients"].([]interface{})
		if !ok {
			continue
		}

		// Найти и удалить клиента
		var newClients []interface{}
		for _, client := range clients {
			clientMap, ok := client.(map[string]interface{})
			if !ok {
				continue
			}

			if clientEmail, exists := clientMap["email"]; !exists || clientEmail != email {
				newClients = append(newClients, client)
			}
		}

		settings["clients"] = newClients
		config.Inbounds[i] = inboundMap

		// Сохранить конфигурацию
		if err := writeXrayConfig(config); err != nil {
			return fmt.Errorf("failed to write config: %v", err)
		}

		log.Printf("User %s removed from config file", email)
		return nil
	}

	return fmt.Errorf("inbound with tag %s not found", xrayTag)
}

// Улучшенная функция добавления пользователя с резервным методом
func addUserToXrayWithFallback(userUUID, email string) error {
	// Сначала пробуем через API
	if err := addUserToXray(userUUID, email); err != nil {
		log.Printf("API method failed: %v, trying config file method", err)

		// Если API не работает, добавляем в конфиг файл
		if err := addUserToConfig(userUUID, email); err != nil {
			return fmt.Errorf("both API and config methods failed: %v", err)
		}

		// Перезапускаем Xray для применения изменений
		if err := restartXray(); err != nil {
			log.Printf("Warning: failed to restart Xray: %v", err)
		}
	}

	return nil
}

// Функция перезапуска Xray
func restartXray() error {
	cmd := exec.Command("systemctl", "restart", "xray")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart xray: %v, output: %s", err, string(output))
	}

	log.Println("Xray restarted successfully")
	return nil
}

// Остальные функции остаются без изменений
func handleMessage(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	userID := update.Message.From.ID
	username := update.Message.From.UserName

	if update.Message.Text == "/start" {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Привет! Я бот для проверки подписки. Используйте /check для проверки подписки и получения конфигурации VPN.")
		bot.Send(msg)
		return
	}

	if update.Message.Text == "/check" {
		isSubscribed, err := checkSubscription(bot, userID, channelUsername)
		if err != nil {
			log.Printf("Error checking subscription: %v", err)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Произошла ошибка при проверке подписки. Пожалуйста, попробуйте позже.")
			bot.Send(msg)
			return
		}

		if isSubscribed {
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
			if err := removeUserFromXrayWithFallback(userID); err != nil {
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

	return member.Status == "member" || member.Status == "administrator" || member.Status == "creator", nil
}

func getOrCreateVlessConfig(userID int64, username string) (string, string, error) {
	var userUUID string

	dbMu.Lock()
	defer dbMu.Unlock()

	err := db.QueryRow("SELECT uuid FROM users WHERE user_id = ?", userID).Scan(&userUUID)
	if err == nil {
		vlessURL := generateVlessURL(userUUID, fmt.Sprintf("user_%d", userID))
		return userUUID, vlessURL, nil
	}

	if err != sql.ErrNoRows {
		return "", "", err
	}

	userUUID = uuid.New().String()
	email := fmt.Sprintf("user_%d@myserver", userID)

	// Используем улучшенную функцию с резервным методом
	if err := addUserToXrayWithFallback(userUUID, email); err != nil {
		return "", "", fmt.Errorf("failed to add user to Xray: %v", err)
	}

	_, err = db.Exec(
		"INSERT INTO users (user_id, username, uuid, created_at) VALUES (?, ?, ?, ?)",
		userID, username, userUUID, time.Now(),
	)
	if err != nil {
		if removeErr := removeUserFromXrayWithFallback(userID); removeErr != nil {
			log.Printf("Error cleaning up user after database insert failure: %v", removeErr)
		}
		return "", "", err
	}

	vlessURL := generateVlessURL(userUUID, email)
	return userUUID, vlessURL, nil
}

func checkSubscriptionsRoutine(bot *tgbotapi.BotAPI) {
	for {
		sleepHours := rand.Intn(23) + 1
		time.Sleep(time.Duration(sleepHours) * time.Hour)

		log.Println("Starting subscription check for all users...")

		dbMu.Lock()
		rows, err := db.Query("SELECT user_id, username FROM users")
		if err != nil {
			log.Printf("Error querying users: %v", err)
			dbMu.Unlock()
			continue
		}

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

		for _, user := range users {
			isSubscribed, err := checkSubscription(bot, user.ID, channelUsername)
			if err != nil {
				log.Printf("Error checking subscription for user %d: %v", user.ID, err)
				continue
			}

			if !isSubscribed {
				if err := removeUserFromXrayWithFallback(user.ID); err != nil {
					log.Printf("Error removing user %d from Xray: %v", user.ID, err)
				}

				dbMu.Lock()
				_, err := db.Exec("DELETE FROM users WHERE user_id = ?", user.ID)
				dbMu.Unlock()
				if err != nil {
					log.Printf("Error deleting user %d: %v", user.ID, err)
				} else {
					log.Printf("User %d removed due to unsubscription", user.ID)

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

func initXrayAPI() error {
	config, err := readXrayConfig()
	if err != nil {
		return err
	}

	if config.API != nil {
		return nil
	}

	log.Println("Adding API configuration to Xray config...")

	config.API = map[string]interface{}{
		"tag": "api",
		"services": []string{
			"HandlerService",
			"StatsService",
		},
	}

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

	apiOutbound := map[string]interface{}{
		"protocol": "freedom",
		"tag":      "api",
	}

	config.Outbounds = append(config.Outbounds, apiOutbound)

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

func generateVlessURL(userUUID, name string) string {
	return fmt.Sprintf("vless://%s@%s:%d?security=tls&type=tcp&flow=xtls-rprx-vision&encryption=none#%s",
		userUUID, serverDomain, serverPort, name)
}

// Улучшенная функция удаления пользователя с резервным методом
func removeUserFromXrayWithFallback(userID int64) error {
	email := fmt.Sprintf("user_%d@myserver", userID)

	// Сначала пробуем через API
	if err := removeUserFromXrayByEmail(email); err != nil {
		log.Printf("API method failed: %v, trying config file method", err)

		// Если API не работает, удаляем из конфиг файла
		if err := removeUserFromConfig(email); err != nil {
			return fmt.Errorf("both API and config methods failed: %v", err)
		}

		// Перезапускаем Xray для применения изменений
		if err := restartXray(); err != nil {
			log.Printf("Warning: failed to restart Xray: %v", err)
		}
	}

	return nil
}

// Command line implementation for removing users
func removeUserFromXrayByEmail(email string) error {
	cmd := exec.Command("xray", "api", "inbounduser", "remove",
		"--server="+xrayAPIAddress,
		"-tag="+xrayTag,
		"-email="+email)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove user via CLI: %v, output: %s", err, string(output))
	}

	log.Printf("User %s removed from Xray successfully: %s", email, string(output))
	return nil
}
