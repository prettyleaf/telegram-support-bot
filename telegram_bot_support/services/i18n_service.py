import json
import logging
from functools import lru_cache
from importlib.resources import files

logger = logging.getLogger(__name__)


class I18nService:
    def __init__(self, default_lang: str):
        self.default_lang = default_lang
        self.translations = {}
        self._load_translations()

    def _load_translations(self) -> None:
        try:
            content = files("telegram_bot_support.data").joinpath("i18n.json").read_text(encoding="utf-8")
            logger.info(f"Translations loaded.")
            self.translations = json.loads(content)
        except FileNotFoundError as e:
            logger.error(f"i18n.json file not found: {e}")
            raise
        except json.JSONDecodeError as e:
            logger.error(f"Invalid JSON in i18n.json: {e}")
            raise
        except Exception as e:
            logger.error(f"Unexpected error: {e}")
            raise RuntimeError(f"Failed to load translations")

    @lru_cache(maxsize=128)
    def get(self, key: str, lang: str = None) -> str:
        lang = lang or self.default_lang
        lang_dict = self.translations.get(lang, {})
        return lang_dict.get(key, key)
