import json
import os
import logging
import uuid

class XRayAPI:
    def __init__(self, config_path="/usr/local/etc/xray/config.json"):
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
