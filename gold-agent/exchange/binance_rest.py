"""
Binance REST client for account data and order placement.

Uses the synchronous ``binance-sdk-spot`` SpotRestAPI client, bridged to
async via ``asyncio.to_thread`` so the event loop is never blocked.
"""

from __future__ import annotations

import asyncio
import logging
from decimal import Decimal, ROUND_DOWN

from binance_sdk_spot.rest_api import SpotRestAPI
from binance_common.configuration import ConfigurationRestAPI
from binance_sdk_spot.rest_api.models.enums import (
    NewOrderSideEnum,
    NewOrderTypeEnum,
    NewOrderTimeInForceEnum,
)

from domain.types import ExchangeBalance, ExchangeBalanceStatus, Order, OrderSide, OrderStatus

logger = logging.getLogger(__name__)


class BinanceRestClient:
    """REST client for Binance account queries and order placement.

    Args:
        api_key:    Binance API key. Empty string disables authenticated calls.
        api_secret: Binance API secret. Empty string disables authenticated calls.
        base_url:   Binance REST base URL, e.g. ``"https://testnet.binance.vision"``.
    """

    def __init__(self, api_key: str, api_secret: str, base_url: str) -> None:
        self._configured = bool(api_key and api_secret)
        # Cache of symbol -> LOT_SIZE stepSize string fetched from exchange info.
        self._lot_step_cache: dict[str, str] = {}
        if self._configured:
            self._client = SpotRestAPI(
                ConfigurationRestAPI(
                    api_key=api_key,
                    api_secret=api_secret,
                    base_path=base_url,
                )
            )

    async def fetch_usdt_balance(self) -> ExchangeBalance:
        """Return the free USDT balance from the Binance account.

        Returns:
            ExchangeBalance with:
            - ``status="not_configured"`` when api_key or api_secret is empty.
            - ``status="ok"`` and the free USDT balance string on success.
            - ``status="error"`` if the API call fails.
        """
        if not self._configured:
            return ExchangeBalance(balance="0", status=ExchangeBalanceStatus.NOT_CONFIGURED)

        try:
            response = await asyncio.to_thread(self._client.get_account)
        except Exception as exc:
            logger.error(
                "BinanceRestClient.fetch_usdt_balance: API call failed: %s", exc
            )
            return ExchangeBalance(balance="0", status=ExchangeBalanceStatus.ERROR)

        for balance in response.data().balances:
            if balance.asset == "USDT":
                return ExchangeBalance(
                    balance=balance.free,
                    status=ExchangeBalanceStatus.OK,
                )

        # USDT asset not present in account — return zero rather than an error.
        logger.info(
            "BinanceRestClient.fetch_usdt_balance: USDT not found in balances, returning 0"
        )
        return ExchangeBalance(balance="0", status=ExchangeBalanceStatus.OK)

    async def _get_lot_step(self, symbol: str) -> str:
        """Return the LOT_SIZE stepSize for a symbol, fetching and caching exchange info."""
        if symbol in self._lot_step_cache:
            return self._lot_step_cache[symbol]

        try:
            info = await asyncio.to_thread(self._client.exchange_info, symbol=symbol)
            for sym_info in info.data().symbols:
                if sym_info.symbol == symbol:
                    for f in sym_info.filters:
                        filter_dict = f if isinstance(f, dict) else vars(f)
                        if filter_dict.get("filterType") == "LOT_SIZE":
                            step = filter_dict.get("stepSize", "1")
                            self._lot_step_cache[symbol] = step
                            return step
        except Exception as exc:
            logger.warning(
                "BinanceRestClient._get_lot_step: failed to fetch exchange info for %s: %s",
                symbol,
                exc,
            )

        # Fallback: no rounding applied.
        self._lot_step_cache[symbol] = "1"
        return "1"

    @staticmethod
    def _apply_lot_step(quantity: str, step: str) -> str:
        """Truncate quantity down to the nearest LOT_SIZE step."""
        step_dec = Decimal(step)
        if step_dec <= 0:
            return quantity
        qty_dec = Decimal(quantity)
        truncated = (qty_dec // step_dec) * step_dec
        # Match decimal places of step for formatting.
        decimal_places = abs(step_dec.normalize().as_tuple().exponent)
        return f"{truncated:.{decimal_places}f}"

    async def place_limit_order(
        self,
        symbol: str,
        side: str,
        quantity: str,
        price: str,
    ) -> Order:
        """Place a GTC limit order on Binance Spot.

        Args:
            symbol:   Trading pair, e.g. ``"BTCUSDT"``.
            side:     ``"BUY"`` or ``"SELL"``.
            quantity: Order quantity as a string to preserve precision.
            price:    Limit price as a string to preserve precision.

        Returns:
            Order with ``status=OrderStatus.PENDING`` and the Binance
            ``orderId`` stored in ``external_order_id``.

        Raises:
            RuntimeError: If the client is not configured.
            Exception:    On any Binance API error.
        """
        if not self._configured:
            raise RuntimeError(
                "BinanceRestClient.place_limit_order: client is not configured — "
                "api_key and api_secret must be set"
            )

        step = await self._get_lot_step(symbol)
        adjusted_quantity = self._apply_lot_step(quantity, step)

        logger.info(
            "BinanceRestClient.place_limit_order: symbol=%s side=%s quantity=%s->%s price=%s",
            symbol,
            side,
            quantity,
            adjusted_quantity,
            price,
        )

        side_enum = NewOrderSideEnum(side)

        response = await asyncio.to_thread(
            self._client.new_order,
            symbol,
            side_enum,
            NewOrderTypeEnum.LIMIT,
            time_in_force=NewOrderTimeInForceEnum.GTC,
            quantity=float(adjusted_quantity),
            price=float(price),
        )

        # Binance returns order_id as an int; convert to str for the domain model.
        external_order_id = str(response.data().order_id)

        logger.info(
            "BinanceRestClient.place_limit_order: order placed — "
            "symbol=%s side=%s external_order_id=%s",
            symbol,
            side,
            external_order_id,
        )

        return Order(
            exchange="binance",
            external_order_id=external_order_id,
            side=OrderSide(side),
            symbol=symbol,
            quantity=adjusted_quantity,
            price=price,
            status=OrderStatus.PENDING,
        )
