import asyncio
import logging
from dotenv import load_dotenv

from app.bot import SupportBot
from config import load_config

logging.basicConfig(
    level=logging.INFO, format="%(asctime)s - %(name)s - %(levelname)s - %(message)s"
)
logger = logging.getLogger(__name__)


async def main() -> None:
    load_dotenv()
    config = load_config()
    bot = SupportBot(config)
    try:
        logger.info("Starting support bot")
        await bot.start()
    except (KeyboardInterrupt, SystemExit):
        logger.info("Bot stopped")
    except Exception as e:
        logger.error(f"Error running bot: {e}")


if __name__ == "__main__":
    asyncio.run(main())
