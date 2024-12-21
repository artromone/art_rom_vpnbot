import json
import os
import logging
import uuid

class XRayAPI:
    def __init__(self, config_path="config.json"):
        self.config_path = config_path
        self.load_config()

    def load_config(self):
        with open(self.config_path, 'r') as file:
            self.config = json.load(file)

    def save_config(self):
        with open(self.config_path, 'w') as file:
            json.dump(self.config, file, indent=4)

    def add_client(self, user_id: int) -> dict:
        # Создаем новый UUID для клиента
        client_id = str(uuid.uuid4())

        # Добавляем клиента в список
        new_client = {
            "id": client_id,
            "email": f"user{user_id}@myserver",  # Можно добавить пользовательский email
            "flow": "xtls-rprx-vision"
        }

        # Добавляем нового клиента в конфигурацию
        for inbound in self.config['inbounds']:
            if inbound['port'] == 443 and inbound['protocol'] == 'vless':
                inbound['settings']['clients'].append(new_client)
                break

        # Сохраняем обновленную конфигурацию
        self.save_config()

        # Перезапуск XRay
        self.restart_xray()

        return {"id": client_id}

    def restart_xray(self):
        # Пример перезапуска XRay через системный вызов
        try:
            os.system("systemctl restart xray")  # Замените на нужную команду для вашего окружения
        except Exception as e:
            logging.error(f"Ошибка при перезапуске XRay: {e}")

    def handle_message(self, message):
        if message.text != "Получить VPN":
            return

        user_id = message.from_user.id

        if not self.check_subscription(user_id):
            self.bot.send_message(
                message.chat.id,
                "Сначала подпишитесь на канал:\n\n"
                f"[ПОДПИСАТЬСЯ](https://t.me/{CHANNEL_ID.lstrip('@')})",
                parse_mode='Markdown'
            )
            return

        # Add client to XRay
        client = self.add_client(user_id)
        if not client:
            self.bot.send_message(
                message.chat.id,
                "Произошла ошибка при создании VPN. Попробуйте позже."
            )
            return

        # Send configuration instructions
        self.bot.send_message(
            message.chat.id,
            NEKORAY_INSTRUCTIONS.format(
                server_address="your-domain.com",  # Configure in .env
                server_port=443,                   # Configure in .env
                uuid=client['id'],
                ws_path="/websocket"              # Configure in .env
            ),
            disable_web_page_preview=True
        )
