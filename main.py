import telebot
from telebot import types
import threading
import time
from datetime import datetime

token = "8058291568:AAGwGoOB50Fmfr8T0EthT_zlkKwYE0gjeak"
channel = "@art_rom"

check = types.ReplyKeyboardMarkup(row_width=1, resize_keyboard=True)
but1 = types.KeyboardButton("Получить VPN")
check.add(but1)

bot = telebot.TeleBot(token)

# Dictionary to store user subscription status and last check time
user_subscriptions = {}

def check_subscription(user_id):
    """Check if user is subscribed to the channel"""
    try:
        status = ['creator', 'administrator', 'member']
        current_status = bot.get_chat_member(chat_id=channel, user_id=user_id).status
        return current_status in status
    except Exception as e:
        print(f"Error checking subscription for user {user_id}: {e}")
        return False

def check_all_subscriptions():
    """Periodically check all users' subscription status"""
    while True:
        current_time = datetime.now()
        
        # Copy dictionary to avoid runtime modification issues
        users_to_check = user_subscriptions.copy()
        
        for user_id, data in users_to_check.items():
            try:
                was_subscribed = data['subscribed']
                currently_subscribed = check_subscription(user_id)
                
                # Update subscription status
                user_subscriptions[user_id]['subscribed'] = currently_subscribed
                
                # Send messages if status changed
                if was_subscribed and not currently_subscribed:
                    bot.send_message(user_id, 
                                   "Вы отписались от канала! Для доступа к VPN необходимо быть подписанным на канал:\n\n"
                                   "[ПОДПИСАТЬСЯ](https://t.me/art_rom)",
                                   parse_mode='Markdown')
                elif not was_subscribed and currently_subscribed:
                    bot.send_message(user_id,
                                   "Спасибо за подписку! Теперь вы можете получить VPN, нажав на кнопку ниже.",
                                   reply_markup=check)
                
            except Exception as e:
                print(f"Error processing user {user_id}: {e}")
                
        # Sleep for specified interval (e.g., 1 hour = 3600 seconds)
        time.sleep(3600)

@bot.message_handler(commands=['start'])
def welcome(message):
    user_id = message.from_user.id
    # Initialize or update user data
    user_subscriptions[user_id] = {
        'subscribed': check_subscription(user_id),
        'last_check': datetime.now()
    }
    
    bot.send_message(message.chat.id, 
                    text="Привет!\n\n"
                    "Я настроил для себя VPN с локацией в США, который не режет скорость. Для получения бесплатного ключа подпишись на мой канал:\n\n"
                    "[ПОДПИСАТЬСЯ](https://t.me/art_rom)\n\nИ смело жми кнопку ниже ↓\n\n",
                    reply_markup=check,
                    parse_mode='Markdown')

@bot.message_handler(func=lambda message: True, content_types=['text'])
def handle_text(message):
    user_id = message.from_user.id
    
    if message.text == "Получить VPN":
        if check_subscription(user_id):
            chat_id = message.chat.id
            bot.send_message(chat_id, 
                           "Отлично, высылаю тебе ключ и инструкцию по установке:\n\n",
                           disable_web_page_preview=True)
        else:
            chat_id = message.chat.id
            bot.send_message(chat_id,
                           text="Чтобы пользоваться VPN, сначала нужно подписаться на канал:\n"
                           "\n\n[ПОДПИСАТЬСЯ](https://t.me/art_rom)",
                           parse_mode='Markdown')

# Start subscription checker in a separate thread
checker_thread = threading.Thread(target=check_all_subscriptions, daemon=True)
checker_thread.start()

# Start the bot
bot.polling(none_stop=True)
