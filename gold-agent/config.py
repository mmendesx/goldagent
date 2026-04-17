from pydantic import Field
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
    binance_websocket_api_url: str = ""
    binance_rest_api_url: str

    # Polymarket (all optional)
    polymarket_api_key: str = ""
    polymarket_api_secret: str = ""
    polymarket_api_passphrase: str = ""
    polymarket_wallet_address: str = ""
    # Private key for on-chain order signing via py-clob-client (hex, 0x-prefixed).
    # Required for balance queries and order placement; leave empty for read-only mode.
    polymarket_private_key: str = ""
    # Signature type for L2 auth:
    #   0 = EOA (private_key directly owns funds)
    #   1 = Email / Magic login (funds in proxy contract; requires funder)
    #   2 = Browser wallet / Safe (funds in safe; requires funder)
    polymarket_signature_type: int = 0
    # Funder address: the address that actually holds USDC collateral.
    # Leave empty for EOA (signature_type=0); set to proxy/safe address for types 1/2.
    polymarket_funder: str = ""

    # Trading
    # Stored as str internally; parsed into a list via the property below.
    # pydantic-settings tries json.loads() on list[str] fields before validators
    # run, which fails on comma-separated values like "BTCUSDT,ETHUSDT".
    gold_symbols_raw: str = Field(default="", validation_alias="gold_symbols")
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
    openai_api_key: str = ""
    gold_llm_model: str = "gpt-5.4-nano"
    gold_llm_context_candles: int = 50

    @property
    def gold_symbols(self) -> list[str]:
        return [s.strip() for s in self.gold_symbols_raw.split(",") if s.strip()]


settings = Settings()
