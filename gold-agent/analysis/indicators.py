import asyncio
import logging
from datetime import datetime

import pandas as pd
import pandas_ta as ta

from domain.types import Candle, Indicator
from storage import postgres

logger = logging.getLogger(__name__)

MIN_CANDLES = 20  # Minimum for Bollinger Bands warm-up


async def compute_indicators(
    symbol: str,
    interval: str,
    candle: Candle,
) -> Indicator | None:
    """
    Fetch the last 200 candles, compute all technical indicators for the given
    candle, persist the result, and return the Indicator.

    Returns None if there is insufficient candle history (< MIN_CANDLES).
    The candle must already be persisted before this is called because
    save_indicator resolves the FK via a subquery on (symbol, interval, open_time).
    """
    candles = await postgres.fetch_candles(symbol, interval, limit=200)

    if len(candles) < MIN_CANDLES:
        logger.debug(
            "insufficient candles for indicator computation",
            extra={
                "symbol": symbol,
                "interval": interval,
                "count": len(candles),
                "required": MIN_CANDLES,
            },
        )
        return None

    df = _build_dataframe(candles)

    indicator = await asyncio.to_thread(
        _compute_sync, df, symbol, interval, candle.open_time
    )

    if indicator is None:
        return None

    await postgres.save_indicator(indicator)

    return indicator


def _build_dataframe(candles: list[Candle]) -> pd.DataFrame:
    """Convert a list of Candle objects to a OHLCV DataFrame indexed by open_time."""
    data = {
        "timestamp": [c.open_time for c in candles],
        "open": [float(c.open_price) for c in candles],
        "high": [float(c.high_price) for c in candles],
        "low": [float(c.low_price) for c in candles],
        "close": [float(c.close_price) for c in candles],
        "volume": [float(c.volume) for c in candles],
    }
    df = pd.DataFrame(data)
    df.set_index("timestamp", inplace=True)
    df.sort_index(inplace=True)
    return df


def _first_col(result: pd.DataFrame | None, prefix: str) -> pd.Series | None:
    """
    Return the first column whose name starts with prefix, or None.

    pandas-ta column names vary by version. For example, Bollinger Bands can
    produce 'BBL_20_2.0' or 'BBL_20_2'. Using prefix matching avoids hard-coded
    version-specific names for BBL_/BBM_/BBU_, MACD_/MACDh_/MACDs_, and ATRr_/ATR_.
    """
    if result is None:
        return None
    cols = [c for c in result.columns if c.startswith(prefix)]
    if not cols:
        return None
    return result[cols[0]]


def _last(series: pd.Series | None) -> float | None:
    """Extract the last value of a Series as a float, returning None if NaN."""
    if series is None or series.empty:
        return None
    val = series.iloc[-1]
    return None if pd.isna(val) else float(val)


def _compute_sync(
    df: pd.DataFrame,
    symbol: str,
    interval: str,
    candle_open_time: datetime,
) -> Indicator | None:
    """
    Synchronous indicator computation. Intended to run inside asyncio.to_thread.

    Computes RSI(14), MACD(12,26,9), Bollinger Bands(20,2sigma), EMA(9,21,50,200),
    VWAP, and ATR(14) using pandas-ta. All values are extracted for the last row
    of the DataFrame (most recent closed candle).

    Returns None on any computation error to allow the caller to continue.
    """
    try:
        rsi = ta.rsi(df["close"], length=14)

        macd_result = ta.macd(df["close"], fast=12, slow=26, signal=9)
        macd_line = _last(_first_col(macd_result, "MACD_"))
        macd_hist = _last(_first_col(macd_result, "MACDh_"))
        macd_signal = _last(_first_col(macd_result, "MACDs_"))

        bbands = ta.bbands(df["close"], length=20, std=2)
        bb_lower = _last(_first_col(bbands, "BBL_"))
        bb_middle = _last(_first_col(bbands, "BBM_"))
        bb_upper = _last(_first_col(bbands, "BBU_"))

        ema9 = ta.ema(df["close"], length=9)
        ema21 = ta.ema(df["close"], length=21)
        ema50 = ta.ema(df["close"], length=50)
        ema200 = ta.ema(df["close"], length=200)

        vwap = ta.vwap(df["high"], df["low"], df["close"], df["volume"])

        # ATR column name differs across pandas-ta versions: ATRr_14 vs ATR_14.
        atr_result = ta.atr(df["high"], df["low"], df["close"], length=14)
        atr_series: pd.Series | None = None
        if atr_result is not None:
            if isinstance(atr_result, pd.DataFrame):
                # Some versions return a DataFrame — pick the first column.
                atr_series = _first_col(atr_result, "ATR")
            else:
                atr_series = atr_result

        return Indicator(
            symbol=symbol,
            interval=interval,
            candle_open_time=candle_open_time,
            rsi=_last(rsi),
            macd_line=macd_line,
            macd_signal=macd_signal,
            macd_histogram=macd_hist,
            bb_upper=bb_upper,
            bb_middle=bb_middle,
            bb_lower=bb_lower,
            ema_9=_last(ema9),
            ema_21=_last(ema21),
            ema_50=_last(ema50),
            ema_200=_last(ema200),
            vwap=_last(vwap),
            atr=_last(atr_series),
        )

    except Exception as exc:
        logger.error(
            "indicator computation failed",
            extra={"symbol": symbol, "interval": interval, "error": str(exc)},
        )
        return None
