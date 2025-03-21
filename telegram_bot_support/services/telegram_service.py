import logging
from typing import Final

from aiogram import Bot
from aiogram.types import Message
from aiogram.exceptions import TelegramAPIError

from telegram_bot_support.services.i18n_service import I18nService
from telegram_bot_support.services.storage_service import StorageService

logger = logging.getLogger(__name__)


class TelegramService:
    def __init__(
        self,
        bot: Bot,
        config: dict[str, any],
        storage: StorageService,
        i18n: I18nService,
    ) -> None:
        self.bot = bot
        self.storage = storage
        self.i18n = i18n
        self.support_chat_id: Final[int] = int(config.get("support_chat_id"))
        self.welcome_message: str | None = config.get("welcome_message")

    async def get_topic_by_tg_id(self, user_id: int) -> int:
        if topic_id := self.storage.get_topic_id(user_id):
            return topic_id
        return await self.create_topic_by_tg_id(user_id)

    async def create_topic_by_tg_id(self, user_id: int) -> int:
        try:
            topic = await self.bot.create_forum_topic(
                chat_id=self.support_chat_id, name=f"{user_id}"
            )
            topic_id = topic.message_thread_id
            await self.storage.save_relation(user_id, topic_id)
            logger.info(f"Created new topic {topic_id} for user {user_id}")
            return topic_id
        except TelegramAPIError as e:
            logger.error(f"Error creating topic for user {user_id}: {e}")
            raise

    async def forward_to_support(self, message: Message) -> None:
        if not message.from_user:
            return
        user_id = message.from_user.id
        try:
            topic_id = await self.get_topic_by_tg_id(user_id)
            await self.bot.forward_message(
                chat_id=self.support_chat_id,
                from_chat_id=user_id,
                message_id=message.message_id,
                message_thread_id=topic_id,
            )
            logger.debug(
                f"Message forwarded to support: user={user_id}, topic={topic_id}"
            )
        except Exception as e:
            logger.error(f"Failed to forward message to support: {e}. Try to create new topic")
            await self.storage.delete_relation(user_id)
            await self.create_topic_by_tg_id(user_id)

    async def forward_to_user(self, message: Message) -> None:
        if not message.message_thread_id:
            return
        topic_id = message.message_thread_id
        if user_id := self.storage.get_user_id(topic_id):
            try:
                await self.bot.copy_message(
                    chat_id=user_id,
                    from_chat_id=self.support_chat_id,
                    message_id=message.message_id,
                )
                logger.debug(
                    f"Message forwarded to user: topic={topic_id}, user={user_id}"
                )
            except Exception as e:
                logger.error(f"Failed to forward message to user {user_id}: {e}")
                await self.bot.send_message(
                    chat_id=self.support_chat_id,
                    message_thread_id=topic_id,
                    text=self.i18n.get("error_forward_user"),
                )
