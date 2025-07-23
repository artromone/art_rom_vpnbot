package xray

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"xray-telegram-bot/config"
	"xray-telegram-bot/models"
)

type Client struct {
	config *config.Config
}

func NewClient(cfg *config.Config) *Client {
	return &Client{config: cfg}
}

func (c *Client) TestAPI() error {
	cmd := exec.Command("xray", "api", "inbounduser",
		"--server="+c.config.XrayAPIAddress,
		"-tag="+c.config.XrayTag)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("API test failed: %v, output: %s", err, string(output))
	}

	log.Printf("API test successful: %s", string(output))
	return nil
}

func (c *Client) AddUser(userUUID, email string) error {
	if err := c.addUserToXrayAPI(userUUID, email); err != nil {
		log.Printf("API method failed: %v, trying config file method", err)

		if err := c.addUserToConfig(userUUID, email); err != nil {
			return fmt.Errorf("both API and config methods failed: %v", err)
		}

		if err := c.restartXray(); err != nil {
			log.Printf("Warning: failed to restart Xray: %v", err)
		}
	}

	return nil
}

func (c *Client) RemoveUser(email string) error {
	if err := c.removeUserFromXrayAPI(email); err != nil {
		log.Printf("API method failed: %v, trying config file method", err)

		if err := c.removeUserFromConfig(email); err != nil {
			return fmt.Errorf("both API and config methods failed: %v", err)
		}

		if err := c.restartXray(); err != nil {
			log.Printf("Warning: failed to restart Xray: %v", err)
		}
	}

	return nil
}

func (c *Client) addUserToXrayAPI(userUUID, email string) error {
	user := models.XrayUser{
		Email: email,
		ID:    userUUID,
		Flow:  "xtls-rprx-vision",
	}

	userJSON, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user: %v", err)
	}

	cmd := exec.Command("xray", "api", "inbounduser", "add",
		"--server="+c.config.XrayAPIAddress,
		"-tag="+c.config.XrayTag,
		"-user="+string(userJSON))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add user via CLI: %v, output: %s", err, string(output))
	}

	log.Printf("User %s added to Xray successfully: %s", email, string(output))
	return nil
}

func (c *Client) removeUserFromXrayAPI(email string) error {
	cmd := exec.Command("xray", "api", "inbounduser", "remove",
		"--server="+c.config.XrayAPIAddress,
		"-tag="+c.config.XrayTag,
		"-email="+email)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove user via CLI: %v, output: %s", err, string(output))
	}

	log.Printf("User %s removed from Xray successfully: %s", email, string(output))
	return nil
}

func (c *Client) addUserToConfig(userUUID, email string) error {
	config, err := c.readXrayConfig()
	if err != nil {
		return fmt.Errorf("failed to read config: %v", err)
	}

	for i, inbound := range config.Inbounds {
		inboundMap, ok := inbound.(map[string]interface{})
		if !ok {
			continue
		}

		tag, exists := inboundMap["tag"]
		if !exists || tag != c.config.XrayTag {
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

		newClient := map[string]interface{}{
			"email": email,
			"id":    userUUID,
			"flow":  "xtls-rprx-vision",
		}

		clients = append(clients, newClient)
		settings["clients"] = clients
		config.Inbounds[i] = inboundMap

		if err := c.writeXrayConfig(config); err != nil {
			return fmt.Errorf("failed to write config: %v", err)
		}

		log.Printf("User %s added to config file", email)
		return nil
	}

	return fmt.Errorf("inbound with tag %s not found", c.config.XrayTag)
}

func (c *Client) removeUserFromConfig(email string) error {
	config, err := c.readXrayConfig()
	if err != nil {
		return fmt.Errorf("failed to read config: %v", err)
	}

	for i, inbound := range config.Inbounds {
		inboundMap, ok := inbound.(map[string]interface{})
		if !ok {
			continue
		}

		tag, exists := inboundMap["tag"]
		if !exists || tag != c.config.XrayTag {
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

		if err := c.writeXrayConfig(config); err != nil {
			return fmt.Errorf("failed to write config: %v", err)
		}

		log.Printf("User %s removed from config file", email)
		return nil
	}

	return fmt.Errorf("inbound with tag %s not found", c.config.XrayTag)
}

func (c *Client) readXrayConfig() (*models.XrayConfig, error) {
	data, err := os.ReadFile(c.config.ConfigPath)
	if err != nil {
		return nil, err
	}

	var config models.XrayConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func (c *Client) writeXrayConfig(config *models.XrayConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.config.ConfigPath, data, 0644)
}

func (c *Client) restartXray() error {
	cmd := exec.Command("systemctl", "restart", "xray")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart xray: %v, output: %s", err, string(output))
	}

	log.Println("Xray restarted successfully")
	return nil
}

func (c *Client) InitAPI() error {
	config, err := c.readXrayConfig()
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

	if err := c.writeXrayConfig(config); err != nil {
		return err
	}

	log.Println("API configuration added. Please restart Xray manually to apply changes.")
	return nil
}

func (c *Client) GenerateVlessURL(userUUID, name string) string {
	return fmt.Sprintf("vless://%s@%s:%d?security=tls&type=tcp&flow=xtls-rprx-vision&encryption=none#%s",
		userUUID, c.config.ServerDomain, c.config.ServerPort, name)
}
