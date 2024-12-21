import os
from dotenv import load_dotenv

load_dotenv()

# Bot Configuration
BOT_TOKEN = os.getenv('BOT_TOKEN')
CHANNEL_ID = os.getenv('CHANNEL_ID')
API_HOST = os.getenv('XRAY_API_HOST', 'http://127.0.0.1')
API_PORT = os.getenv('XRAY_API_PORT', '10085')
INBOUND_TAG = os.getenv('XRAY_INBOUND_TAG', 'vmess-inbound')
