"""
pytest configuration for gold-agent.

Sets up sys.path so that absolute imports resolve correctly, and pre-populates
all required Settings env vars so config.py can initialise without a .env file.
This keeps tests hermetic — they do not depend on the developer's local .env.
"""
import os
import sys

# Ensure gold-agent/ is the top-level package root for absolute imports.
sys.path.insert(0, os.path.dirname(__file__))

# Pre-populate every required field in Settings before any module imports
# config.py (which executes Settings() at module level).
# GOLD_SYMBOLS is a JSON array because pydantic_settings decodes list fields
# from env vars as JSON before the custom validator fires.
_TEST_ENV = {
    "BINANCE_API_KEY": "test-key",
    "BINANCE_API_SECRET": "test-secret",
    "BINANCE_WEBSOCKET_STREAM_URL": "wss://test.example.com",
    "BINANCE_WEBSOCKET_API_URL": "wss://test.example.com/api",
    "BINANCE_REST_API_URL": "https://test.example.com",
    "GOLD_SYMBOLS": '["BTCUSDT"]',
    "GOLD_DATABASE_URL": "postgres://test:test@localhost:5432/test",
    "GOLD_REDIS_URL": "redis://localhost:6379/0",
    "GOLD_CONFIDENCE_THRESHOLD": "70",
    "GOLD_MAX_POSITIONS": "3",
    "GOLD_MAX_DRAWDOWN_PERCENT": "15.0",
}

for key, value in _TEST_ENV.items():
    os.environ.setdefault(key, value)
