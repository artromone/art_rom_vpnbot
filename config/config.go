package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	TelegramBotToken string
	ChannelUsername  string
	XrayAPIAddress   string
	XrayTag          string
	ServerDomain     string
	ServerPort       int
	ConfigPath       string
	DatabasePath     string
	DataDir          string
}

func Load() *Config {
	dataDir := "./data"

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		panic("Failed to create data directory: " + err.Error())
	}

	dbPath := filepath.Join(dataDir, "users.db") + "?_timeout=5000&_journal_mode=WAL&_busy_timeout=5000"

	return &Config{
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		ChannelUsername:  "@art_rom",
		XrayAPIAddress:   "127.0.0.1:10085",
		XrayTag:          "vless_tls",
		ServerDomain:     "artr.ignorelist.com",
		ServerPort:       443,
		ConfigPath:       "/usr/local/etc/xray/config.json",
		DatabasePath:     dbPath,
		DataDir:          dataDir,
	}
}
