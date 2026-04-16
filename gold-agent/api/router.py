"""
FastAPI application factory for the Gold Trading Agent.

Creates and configures the FastAPI app with CORS middleware and all API routes.
Call create_app() once at startup, passing the live dependencies.
"""

import logging

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from .handlers import init_handlers, router
from .websocket_hub import WebSocketHub
from ..exchange.binance_rest import BinanceRestClient
from ..exchange.polymarket_rest import PolymarketRestClient

logger = logging.getLogger(__name__)


def create_app(
    hub: WebSocketHub,
    binance_client: BinanceRestClient,
    polymarket_client: PolymarketRestClient,
) -> FastAPI:
    """Create the FastAPI application and wire all dependencies.

    Args:
        hub:              WebSocketHub that broadcasts real-time events.
        binance_client:   Binance REST client (may be in not-configured state).
        polymarket_client: Polymarket REST client (may be in not-configured state).

    Returns:
        Fully configured FastAPI application ready to serve requests.
    """
    init_handlers(hub, binance_client, polymarket_client)

    app = FastAPI(
        title="Gold Trading Agent",
        version="2.0.0",
        # Disable docs in production via env override if needed; fine to expose in dev.
        docs_url="/docs",
        redoc_url="/redoc",
    )

    # CORS — allow the React dashboard on any origin in dev.
    # Restrict allow_origins to the dashboard host in production.
    app.add_middleware(
        CORSMiddleware,
        allow_origins=["*"],
        allow_credentials=True,
        allow_methods=["GET", "OPTIONS"],
        allow_headers=["Content-Type"],
    )

    app.include_router(router)

    logger.info("FastAPI app created and routes registered")
    return app
