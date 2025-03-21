import os


def load_config() -> dict[str, any]:
    return {
        "token": os.environ.get("BOT_TOKEN"),
        "support_chat_id": os.environ.get("SUPPORT_CHAT_ID"),
        "enable_start_command": os.environ.get("ENABLE_START_COMMAND", "False").lower()
        == "true",
        "welcome_message": os.environ.get("WELCOME_MESSAGE"),
        "lang": os.environ.get("APP_LANG"),
    }
