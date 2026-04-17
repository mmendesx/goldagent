"""
Polymarket CLOB REST client for account data and order placement.

Uses the synchronous ``py-clob-client`` ClobClient, bridged to async via
``asyncio.to_thread`` so the event loop is never blocked.

Level 2 authentication (required for balance queries and order placement)
needs both API credentials *and* a private key for on-chain signing.
Pass the wallet's private key as ``private_key``; leave it empty to run in
read-only / unconfigured mode.
"""

from __future__ import annotations

import asyncio
import logging

from py_clob_client.client import ClobClient
from py_clob_client.clob_types import (
    ApiCreds,
    AssetType,
    BalanceAllowanceParams,
    OrderArgs,
)
from py_clob_client.order_builder.constants import BUY, SELL

from domain.types import ExchangeBalance, ExchangeBalanceStatus, Order, OrderSide, OrderStatus

logger = logging.getLogger(__name__)

_POLYMARKET_CLOB_HOST = "https://clob.polymarket.com"
_POLYGON_CHAIN_ID = 137


class PolymarketRestClient:
    """REST client for Polymarket CLOB balance queries and order placement.

    Args:
        api_key:        Polymarket L2 API key.
        api_secret:     Polymarket L2 API secret.
        passphrase:     Polymarket L2 API passphrase.
        private_key:    Wallet private key (hex, ``0x``-prefixed) used by the
                        py-clob-client for on-chain order signing.  Required for
                        L2 operations; leave empty for unconfigured mode.
        wallet_address: Wallet address (informational; stored for logging).
    """

    def __init__(
        self,
        api_key: str,
        api_secret: str,
        passphrase: str,
        private_key: str,
        wallet_address: str,
    ) -> None:
        self._configured = bool(api_key and api_secret and passphrase and private_key)
        self._wallet_address = wallet_address
        # Circuit breaker: once auth fails, stop hammering the API until restart.
        self._auth_failed = False

        if self._configured:
            creds = ApiCreds(
                api_key=api_key,
                api_secret=api_secret,
                api_passphrase=passphrase,
            )
            self._client = ClobClient(
                host=_POLYMARKET_CLOB_HOST,
                chain_id=_POLYGON_CHAIN_ID,
                key=private_key,
                creds=creds,
            )

    async def fetch_usdc_balance(self) -> ExchangeBalance:
        """Return the USDC collateral balance from the Polymarket CLOB API.

        Returns:
            ExchangeBalance with:
            - ``status="not_configured"`` when credentials or private key are absent.
            - ``status="ok"`` and the USDC balance string on success.
            - ``status="error"`` if the API call fails.

        On persistent auth failure (401) the client trips an in-memory circuit
        breaker so subsequent polls short-circuit without hitting the network.
        Restart the process to re-test credentials.
        """
        if not self._configured:
            return ExchangeBalance(balance="0", status=ExchangeBalanceStatus.NOT_CONFIGURED)

        if self._auth_failed:
            return ExchangeBalance(balance="0", status=ExchangeBalanceStatus.ERROR)

        try:
            result = await asyncio.to_thread(
                self._client.get_balance_allowance,
                params=BalanceAllowanceParams(asset_type=AssetType.COLLATERAL),
            )
        except Exception as exc:
            # Detect 401/unauthorized once and latch — don't spam logs every poll.
            msg = str(exc)
            if "401" in msg or "Unauthorized" in msg or "Invalid api key" in msg:
                if not self._auth_failed:
                    logger.error(
                        "PolymarketRestClient: auth failed (invalid api key). "
                        "Disabling balance polling until restart. Error: %s",
                        exc,
                    )
                self._auth_failed = True
            else:
                logger.error(
                    "PolymarketRestClient.fetch_usdc_balance: API call failed: %s", exc
                )
            return ExchangeBalance(balance="0", status=ExchangeBalanceStatus.ERROR)

        balance = result.get("balance", "0") if result else "0"
        if not balance:
            balance = "0"

        return ExchangeBalance(balance=balance, status=ExchangeBalanceStatus.OK)

    async def place_order(
        self,
        token_id: str,
        side: str,
        price: float,
        size: float,
    ) -> Order:
        """Place a GTC limit order on Polymarket via the CLOB API.

        Args:
            token_id: Polymarket conditional token ID for the market outcome.
            side:     ``"BUY"`` or ``"SELL"``.
            price:    Limit price between 0.01 and 1.00 (probability).
            size:     Number of contracts (shares) to trade.

        Returns:
            Order with ``status=OrderStatus.PENDING`` and the Polymarket
            ``orderID`` stored in ``external_order_id``.

        Raises:
            RuntimeError: If the client is not configured.
            Exception:    On any Polymarket API error.
        """
        if not self._configured:
            raise RuntimeError(
                "PolymarketRestClient.place_order: client is not configured — "
                "api_key, api_secret, passphrase, and private_key must all be set"
            )

        clob_side = BUY if side == "BUY" else SELL

        logger.info(
            "PolymarketRestClient.place_order: token_id=%s side=%s price=%s size=%s",
            token_id,
            side,
            price,
            size,
        )

        response = await asyncio.to_thread(
            self._client.create_and_post_order,
            OrderArgs(
                token_id=token_id,
                price=price,
                size=size,
                side=clob_side,
            ),
        )

        # py-clob-client returns the raw JSON dict from the CLOB API.
        # Polymarket's POST /order response includes {"orderID": "..."}.
        external_order_id = str(response.get("orderID", "")) if response else ""

        logger.info(
            "PolymarketRestClient.place_order: order placed — "
            "token_id=%s side=%s external_order_id=%s",
            token_id,
            side,
            external_order_id,
        )

        return Order(
            exchange="polymarket",
            external_order_id=external_order_id,
            side=OrderSide(side),
            symbol=token_id,
            quantity=str(size),
            price=str(price),
            status=OrderStatus.PENDING,
        )
