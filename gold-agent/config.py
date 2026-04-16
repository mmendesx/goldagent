from typing import Annotated

from pydantic import field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
    )

    # Binance
    binance_api_key: str
    binance_api_secret: str
    binance_websocket_stream_url: str
    binance_websocket_api_url: str
    binance_rest_api_url: str

    # Polymarket (all optional)
    polymarket_api_key: str = ""
    polymarket_api_secret: str = ""
    polymarket_api_passphrase: str = ""
    polymarket_wallet_address: str = ""
    # Private key for on-chain order signing via py-clob-client (hex, 0x-prefixed).
    # Required for balance queries and order placement; leave empty for read-only mode.
    polymarket_private_key: str = ""

    # Trading
    gold_symbols: list[str]
    gold_default_interval: str = "5m"
    gold_confidence_threshold: int = 70
    gold_max_positions: int = 3
    gold_max_position_size_percent: float = 10.0
    gold_max_drawdown_percent: float = 15.0
    gold_dry_run: bool = False

    # Infrastructure
    gold_http_port: int = 8080
    gold_database_url: str
    gold_redis_url: str

    # LLM
    anthropic_api_key: str = ""
    gold_llm_model: str = "claude-sonnet-4-6"
    gold_llm_context_candles: int = 50

    @field_validator("gold_symbols", mode="before")
    @classmethod
    def parse_gold_symbols(cls, value: object) -> list[str]:
        if isinstance(value, list):
            return value
        if isinstance(value, str):
            return [symbol.strip() for symbol in value.split(",") if symbol.strip()]
        raise ValueError(f"GOLD_SYMBOLS must be a comma-separated string, got: {type(value)}")


settings = Settings()
