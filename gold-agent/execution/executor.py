"""
Order executor: routes a TradeIntent to Binance or Polymarket, persists the
resulting Order, and returns it.  In dry-run mode the intent is logged and
None is returned without making any external API call.
"""

from __future__ import annotations

import logging

from domain.types import Order, TradeIntent
from exchange.binance_rest import BinanceRestClient
from exchange.polymarket_rest import PolymarketRestClient
from storage import postgres
from config import settings

logger = logging.getLogger(__name__)


class Executor:
    """Routes TradeIntents to the appropriate exchange and persists orders.

    Args:
        binance_client:    Configured BinanceRestClient instance.
        polymarket_client: Configured PolymarketRestClient instance.
    """

    def __init__(
        self,
        binance_client: BinanceRestClient,
        polymarket_client: PolymarketRestClient,
    ) -> None:
        self._binance = binance_client
        self._polymarket = polymarket_client

    async def execute(self, intent: TradeIntent) -> Order | None:
        """Execute a TradeIntent as a limit order.

        Routing:
        - Binance:     symbols ending in "USDT"
        - Polymarket:  everything else (symbol treated as a token/condition ID)

        In dry-run mode (``GOLD_DRY_RUN=true``): logs the intent, returns None
        without placing any order or writing to the database.

        Returns:
            The persisted Order on success, or None if dry-run or on any error.
        """
        if settings.gold_dry_run:
            logger.info(
                "DRY RUN: would execute %s %s qty=%.6f @ %.2f",
                intent.side.value,
                intent.symbol,
                intent.position_size_qty,
                intent.estimated_entry_price,
            )
            return None

        try:
            if self._is_binance_symbol(intent.symbol):
                order = await self._execute_binance(intent)
            else:
                order = await self._execute_polymarket(intent)

            order_id = await postgres.save_order(order)
            order.id = order_id

            logger.info(
                "order placed: %s %s qty=%s @ %s external_id=%s",
                order.side.value,
                order.symbol,
                order.quantity,
                order.price,
                order.external_order_id,
            )
            return order

        except Exception as exc:
            logger.error(
                "order placement failed for %s: %s",
                intent.symbol,
                exc,
                exc_info=True,
            )
            return None

    # ------------------------------------------------------------------
    # Routing
    # ------------------------------------------------------------------

    def _is_binance_symbol(self, symbol: str) -> bool:
        """Return True when the symbol belongs on Binance (ends with USDT)."""
        return symbol.endswith("USDT")

    # ------------------------------------------------------------------
    # Exchange dispatch
    # ------------------------------------------------------------------

    async def _execute_binance(self, intent: TradeIntent) -> Order:
        """Place a Binance GTC limit order for the given intent."""
        return await self._binance.place_limit_order(
            symbol=intent.symbol,
            side=intent.side.value,
            quantity=f"{intent.position_size_qty:.6f}",
            price=f"{intent.estimated_entry_price:.2f}",
        )

    async def _execute_polymarket(self, intent: TradeIntent) -> Order:
        """Place a Polymarket GTC limit order for the given intent."""
        return await self._polymarket.place_order(
            token_id=intent.symbol,
            side=intent.side.value,
            price=intent.estimated_entry_price,
            size=intent.position_size_qty,
        )
