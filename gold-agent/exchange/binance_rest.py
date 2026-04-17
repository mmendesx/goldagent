"""
Binance REST client for account data and order placement.

Uses the synchronous ``binance-sdk-spot`` SpotRestAPI client, bridged to
async via ``asyncio.to_thread`` so the event loop is never blocked.
"""

from __future__ import annotations

import asyncio
import logging

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

        for balance in response.data.balances:
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

        logger.info(
            "BinanceRestClient.place_limit_order: symbol=%s side=%s quantity=%s price=%s",
            symbol,
            side,
            quantity,
            price,
        )

        side_enum = NewOrderSideEnum(side)

        response = await asyncio.to_thread(
            self._client.new_order,
            symbol,
            side_enum,
            NewOrderTypeEnum.LIMIT,
            time_in_force=NewOrderTimeInForceEnum.GTC,
            quantity=float(quantity),
            price=float(price),
        )

        # Binance returns order_id as an int; convert to str for the domain model.
        external_order_id = str(response.data.order_id)

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
            quantity=quantity,
            price=price,
            status=OrderStatus.PENDING,
        )
