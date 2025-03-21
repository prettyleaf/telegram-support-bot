import logging

from aiogram import Bot, Dispatcher, Router
from aiogram.exceptions import TelegramAPIError

from handlers.message_handlers import register_handlers
from services.i18n_service import I18nService
from services.storage_service import StorageService
from services.telegram_service import TelegramService

logger = logging.getLogger(__name__)


class SupportBot:
    def __init__(self, config: dict[str, any], bot: Bot | None = None) -> None:
        self._validate_config(config)
        self.bot = bot if bot else Bot(token=config.get("token"))
        self.storage = StorageService()
        self.router = Router(name="support_bot_router")
        self.i18n = I18nService(default_lang=config.get("lang"))
        self.telegram = TelegramService(self.bot, config, self.storage, self.i18n)
        self.enable_start_command = config.get("enable_start_command", False)
        self.dispatcher = Dispatcher()

    @staticmethod
    def _validate_config(config: dict[str, any]) -> None:
        if not config.get("support_chat_id"):
            raise ValueError("SUPPORT_CHAT_ID must be in environment")
        if not config.get("token") and not config.get("_bot_instance"):
            raise ValueError(
                "The BOT_TOKEN is required if no bot instance is specified."
            )
        if not config.get("lang"):
            raise ValueError("APP_LANG is required.")
        if config.get("enable_start_command") and not config.get("welcome_message"):
            raise ValueError(
                "WELCOME_MESSAGE is required when ENABLE_START_COMMAND is true."
            )

    async def _check_support_chat(self) -> None:
        try:
            chat = await self.bot.get_chat(self.telegram.support_chat_id)
            if not getattr(chat, "is_forum", False):
                raise ValueError("Support chat should be a forum")
            logger.info(
                f"Successfully connected to support chat {self.telegram.support_chat_id}"
            )
        except TelegramAPIError as e:
            logger.error(f"Failed to access support chat: {e}")
            raise ValueError(f"Cannot access support chat: {e}")

    async def start(self) -> None:
        await self._check_support_chat()
        register_handlers(self.router, self.telegram, self.enable_start_command)
        self.dispatcher.include_router(self.router)
        await self.dispatcher.start_polling(self.bot)
