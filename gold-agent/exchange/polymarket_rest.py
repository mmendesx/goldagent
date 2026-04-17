"""
Polymarket CLOB REST client for account data and order placement.

Uses the synchronous ``py-clob-client`` ClobClient, bridged to async via
``asyncio.to_thread`` so the event loop is never blocked.

L2 credentials are derived from the private key via ``create_or_derive_api_creds``
rather than passed manually, which is the correct flow for all signature types.
"""

from __future__ import annotations

import asyncio
import logging

from py_clob_client.client import ClobClient
from py_clob_client.clob_types import (
    AssetType,
    BalanceAllowanceParams,
    OrderArgs,
    OrderType,
)
from py_clob_client.constants import POLYGON
from py_clob_client.order_builder.constants import BUY, SELL

from domain.types import ExchangeBalance, ExchangeBalanceStatus, Order, OrderSide, OrderStatus

logger = logging.getLogger(__name__)

_POLYMARKET_CLOB_HOST = "https://clob.polymarket.com"


class PolymarketRestClient:
    """REST client for Polymarket CLOB balance queries and order placement.

    L2 API credentials are derived from ``private_key`` via
    ``ClobClient.create_or_derive_api_creds()`` at construction time.
    Manually configured API key/secret/passphrase are not required.

    Args:
        private_key:    Wallet private key (hex, ``0x``-prefixed).
                        Required for L2 operations; leave empty for unconfigured mode.
        wallet_address: Wallet address (informational; stored for logging).
        signature_type: 0=EOA, 1=Email/Magic login, 2=Browser wallet/Safe.
        funder:         Proxy/safe address holding USDC collateral.
                        Required for signature_type 1 and 2.
    """

    def __init__(
        self,
        private_key: str,
        wallet_address: str,
        signature_type: int = 0,
        funder: str = "",
    ) -> None:
        self._configured = False
        self._wallet_address = wallet_address
        self._auth_failed = False

        if not private_key:
            logger.info("PolymarketRestClient: private_key not set — running unconfigured")
            return

        if signature_type != 0 and not funder:
            logger.warning(
                "PolymarketRestClient: signature_type=%d requires POLYMARKET_FUNDER — "
                "auth will fail without it",
                signature_type,
            )

        client_kwargs: dict = {
            "host": _POLYMARKET_CLOB_HOST,
            "chain_id": POLYGON,
            "key": private_key,
        }
        if signature_type != 0:
            client_kwargs["signature_type"] = signature_type
            client_kwargs["funder"] = funder

        try:
            self._client = ClobClient(**client_kwargs)
            creds = self._client.create_or_derive_api_creds()
            self._client.set_api_creds(creds)
            self._configured = True
            logger.info(
                "PolymarketRestClient: L2 credentials derived — wallet=%s signature_type=%d",
                wallet_address,
                signature_type,
            )
        except Exception as exc:
            logger.error(
                "PolymarketRestClient: failed to derive L2 credentials: %s — "
                "running unconfigured",
                exc,
            )

    async def fetch_usdc_balance(self) -> ExchangeBalance:
        """Return the USDC collateral balance from the Polymarket CLOB API.

        Returns:
            ExchangeBalance with:
            - ``status="not_configured"`` when private_key is absent or cred derivation failed.
            - ``status="ok"`` and the USDC balance string on success.
            - ``status="error"`` if the API call fails.

        On persistent auth failure (401) the client trips an in-memory circuit
        breaker so subsequent polls short-circuit without hitting the network.
        Restart the process to re-derive credentials.
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
            msg = str(exc)
            if "401" in msg or "Unauthorized" in msg or "Invalid api key" in msg:
                if not self._auth_failed:
                    logger.error(
                        "PolymarketRestClient: auth failed after credential derivation. "
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
                "private_key must be set and credential derivation must succeed"
            )

        clob_side = BUY if side == "BUY" else SELL

        logger.info(
            "PolymarketRestClient.place_order: token_id=%s side=%s price=%s size=%s",
            token_id,
            side,
            price,
            size,
        )

        order = await asyncio.to_thread(
            self._client.create_order,
            OrderArgs(
                token_id=token_id,
                price=price,
                size=size,
                side=clob_side,
            ),
        )
        response = await asyncio.to_thread(
            self._client.post_order,
            order,
            OrderType.GTC,
        )

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
