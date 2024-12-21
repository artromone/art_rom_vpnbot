import uuid
import requests
import logging
from time import sleep
from requests.exceptions import ConnectionError
from config import API_HOST, API_PORT, INBOUND_TAG

logger = logging.getLogger(__name__)

class XRayAPI:
    def __init__(self):
        self.base_url = f"{API_HOST}:{API_PORT}/handler"
        
    def add_client(self, user_id: int) -> dict:
        retries = 2
        while retries > 0:
            try:
                client = {
                    "id": str(uuid.uuid4()),
                    "email": f"tg_{user_id}@vpn.local"
                }

                payload = {
                    "tag": INBOUND_TAG,
                    "operation": "add",
                    "client": client
                }

                response = requests.post(
                    f"{self.base_url}/add",
                    json=payload,
                    headers={"Content-Type": "application/json"}
                )

                if response.status_code == 200:
                    logger.info(f"Successfully added client for user {user_id}")
                    return client
                else:
                    logger.error(f"Failed to add client: {response.text}")
                    return None

            except ConnectionError as e:
                retries -= 1
                logger.error(f"Connection error: {e}, retries left: {retries}")
                sleep(1)  # Wait before retrying
                if retries == 0:
                    logger.error("Max retries reached. Could not connect to the XRay API.")
                    return None
            except Exception as e:
                logger.error(f"Error adding client: {e}")
                return None
