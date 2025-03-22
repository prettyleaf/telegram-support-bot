import logging
from functools import partial

from aiogram import Router, F
from aiogram.filters import Command
from aiogram.types import Message

from telegram_bot_support.services.telegram_service import TelegramService

logger = logging.getLogger(__name__)


async def handle_start(message: Message, telegram_service: TelegramService) -> None:
    if not message.from_user:
        return
    user_id = message.from_user.id
    await telegram_service.get_topic_by_tg_id(user_id)
    if telegram_service.welcome_message:
        await message.answer(telegram_service.welcome_message)


async def handle_support_message(
    message: Message, telegram_service: TelegramService
) -> None:
    await telegram_service.forward_to_user(message)


async def handle_user_message(
    message: Message, telegram_service: TelegramService
) -> None:
    await telegram_service.forward_to_support(message)


def register_handlers(
    router: Router, telegram_service: TelegramService, enable_start_command: bool
) -> None:

    if enable_start_command:
        router.message.register(
            partial(handle_start, telegram_service=telegram_service), Command("start")
        )

    router.message.register(
        partial(handle_user_message, telegram_service=telegram_service),
        F.chat.type == "private",
        ~F.from_user.is_bot,
        ~F.text.startswith("/"),
    )

    router.message.register(
        partial(handle_support_message, telegram_service=telegram_service),
        F.chat.id == telegram_service.support_chat_id,
        F.message_thread_id.is_not(None),
        ~F.from_user.is_bot,
    )

    logger.info("All message handlers registered")
