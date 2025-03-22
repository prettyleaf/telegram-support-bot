import asyncio
import json
import logging
from pathlib import Path

logger = logging.getLogger(__name__)


class StorageService:
    def __init__(self, storage_file: str = "storage.json") -> None:
        self.storage_file = Path(storage_file)
        self._topic_by_user: dict[int, int] = {}
        self._user_by_topic: dict[int, int] = {}
        self._load_data()

    def _load_data(self) -> None:
        if not self.storage_file.exists():
            return
        try:
            with open(self.storage_file, "r", encoding="utf-8") as f:
                data = json.load(f)
                self._topic_by_user = {
                    int(k): v for k, v in data.get("topic_by_user", {}).items()
                }
                self._user_by_topic = {
                    int(k): v for k, v in data.get("user_by_topic", {}).items()
                }
            logger.info(
                f"Loaded {len(self._topic_by_user)} user-topic relations from storage"
            )
        except (json.JSONDecodeError, IOError) as e:
            logger.error(f"Error loading data from {self.storage_file}: {e}")
            self._topic_by_user = {}
            self._user_by_topic = {}

    async def _save_data(self) -> None:
        try:
            data = {
                "topic_by_user": {str(k): v for k, v in self._topic_by_user.items()},
                "user_by_topic": {str(k): v for k, v in self._user_by_topic.items()},
            }
            await asyncio.to_thread(self._write_to_file, data)
        except IOError:
            logger.critical("Critical storage failure: data not saved!")

    def _write_to_file(self, data: dict[str, dict[str, int]]) -> None:
        try:
            self.storage_file.parent.mkdir(parents=True, exist_ok=True)
            with open(self.storage_file, "w", encoding="utf-8") as f:
                json.dump(data, f)
        except IOError as e:
            logger.error(f"Error writing to {self.storage_file}: {e}")

    def get_topic_id(self, user_id: int) -> int | None:
        return self._topic_by_user.get(user_id)

    def get_user_id(self, topic_id: int) -> int | None:
        return self._user_by_topic.get(topic_id)

    async def save_relation(self, user_id: int, topic_id: int) -> None:
        self._topic_by_user[user_id] = topic_id
        self._user_by_topic[topic_id] = user_id
        await self._save_data()
        logger.info(f"Relation saved: user_id={user_id}, topic_id={topic_id}")

    async def delete_relation(self, user_id: int) -> None:
        if user_id in self._topic_by_user:
            topic_id = self._topic_by_user.pop(user_id)
            self._user_by_topic.pop(topic_id, None)
            await self._save_data()
            logger.info(f"Relation deleted for user_id={user_id}")
