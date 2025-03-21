import json
import logging
from functools import lru_cache
from pathlib import Path

logger = logging.getLogger(__name__)


class I18nService:
    def __init__(self, default_lang: str, i18n_file: str = "i18n.json"):
        self.i18n_file = Path(i18n_file)
        self.default_lang = default_lang
        self.translations = {}
        self._load_translations()

    def _load_translations(self) -> None:
        if not self.i18n_file.exists():
            raise FileNotFoundError(f"i18n file {self.i18n_file} does not exist.")
        try:
            with self.i18n_file.open("r", encoding="utf-8") as f:
                self.translations = json.load(f)
            logger.info(f"Translations loaded from {self.i18n_file}")
        except Exception as e:
            logger.error(f"Error loading i18n file: {e}")
            self.translations = {}

    @lru_cache(maxsize=128)
    def get(self, key: str, lang: str = None) -> str:
        lang = lang or self.default_lang
        lang_dict = self.translations.get(lang, {})
        return lang_dict.get(key, key)
