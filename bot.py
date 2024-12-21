import telebot
from telebot import types
import threading
import time
from datetime import datetime
import logging
from config import BOT_TOKEN, CHANNEL_ID
from xray_api import XRayAPI
from messages import WELCOME_MESSAGE, NEKORAY_INSTRUCTIONS

# Setup logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

class VPNBot:
    def __init__(self):
        self.bot = telebot.TeleBot(BOT_TOKEN)
        self.xray_api = XRayAPI()
        self.user_subscriptions = {}
        
        # Setup keyboard
        self.keyboard = types.ReplyKeyboardMarkup(row_width=1, resize_keyboard=True)
        self.keyboard.add(types.KeyboardButton("Получить VPN"))
        
        # Setup message handlers
        self.setup_handlers()
        
        # Start subscription checker
        self.checker_thread = threading.Thread(
            target=self.check_all_subscriptions,
            daemon=True
        )
        self.checker_thread.start()
    
    def setup_handlers(self):
        @self.bot.message_handler(commands=['start'])
        def welcome(message):
            self.handle_start(message)
            
        @self.bot.message_handler(func=lambda message: True, content_types=['text'])
        def handle_text(message):
            self.handle_message(message)
    
    def check_subscription(self, user_id):
        try:
            status = ['creator', 'administrator', 'member']
            current_status = self.bot.get_chat_member(
                chat_id=CHANNEL_ID,
                user_id=user_id
            ).status
            return current_status in status
        except Exception as e:
            logger.error(f"Error checking subscription: {e}")
            return False
    
    def check_all_subscriptions(self):
        while True:
            users_to_check = self.user_subscriptions.copy()
            
            for user_id, data in users_to_check.items():
                try:
                    was_subscribed = data['subscribed']
                    currently_subscribed = self.check_subscription(user_id)
                    
                    self.user_subscriptions[user_id]['subscribed'] = currently_subscribed
                    
                    if was_subscribed and not currently_subscribed:
                        self.bot.send_message(
                            user_id,
                            "Вы отписались от канала! Для доступа к VPN необходимо быть подписанным:\n\n"
                            f"[ПОДПИСАТЬСЯ](https://t.me/{CHANNEL_ID.lstrip('@')})",
                            parse_mode='Markdown'
                        )
                    elif not was_subscribed and currently_subscribed:
                        self.bot.send_message(
                            user_id,
                            "Спасибо за подписку! Теперь вы можете получить VPN.",
                            reply_markup=self.keyboard
                        )
                except Exception as e:
                    logger.error(f"Error processing user {user_id}: {e}")
            
            time.sleep(3600)
    
    def handle_start(self, message):
        user_id = message.from_user.id
        self.user_subscriptions[user_id] = {
            'subscribed': self.check_subscription(user_id),
            'last_check': datetime.now()
        }
        
        self.bot.send_message(
            message.chat.id,
            WELCOME_MESSAGE.format(
                channel_link=f"https://t.me/{CHANNEL_ID.lstrip('@')}"
            ),
            reply_markup=self.keyboard,
            parse_mode='Markdown'
        )
    
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
        client = self.xray_api.add_client(user_id)
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
    
    def run(self):
        logger.info("Starting VPN bot...")
        self.bot.polling(none_stop=True)

if __name__ == "__main__":
    bot = VPNBot()
    bot.run()
