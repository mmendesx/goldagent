"""
LLM-backed decision engine for the gold-agent trading pipeline.

Submits a market context dict to Claude with a cached system prompt,
parses the JSON response into a Decision, persists it, and returns it.
On any failure — network error, malformed JSON, validation error — returns
a HOLD decision with confidence=0. Never raises from evaluate().
"""

import asyncio
import json
import logging
from datetime import datetime, timezone

import anthropic
from pydantic import ValidationError

from gold_agent.config import settings
from gold_agent.domain.types import Decision, DecisionAction, LLMDecisionResponse
from gold_agent.engine.prompts import SYSTEM_PROMPT_CACHED, build_messages
from gold_agent.storage import postgres

logger = logging.getLogger(__name__)


class LLMDecisionEngine:
    def __init__(self) -> None:
        self._client = anthropic.Anthropic(api_key=settings.anthropic_api_key)
        self._model = settings.gold_llm_model

    async def evaluate(
        self,
        context: dict,
        symbol: str,
        is_dry_run: bool = False,
    ) -> Decision:
        """
        Submit context to Claude, parse response, persist decision.

        On any failure (network, parse, validation), returns a HOLD decision
        with confidence=0. Never raises.
        """
        decision = await self._call_llm(context, symbol, is_dry_run)

        # Persist the decision regardless of action or dry-run flag.
        try:
            decision_id = await postgres.save_decision(decision)
            decision.id = decision_id
        except Exception as exc:
            logger.error(
                "failed to persist decision",
                extra={"symbol": symbol, "error": str(exc)},
            )

        return decision

    async def _call_llm(
        self,
        context: dict,
        symbol: str,
        is_dry_run: bool,
    ) -> Decision:
        """Call the Anthropic API and return a parsed Decision. Returns HOLD on any error."""
        try:
            response = await asyncio.to_thread(
                self._client.messages.create,
                model=self._model,
                max_tokens=512,
                system=SYSTEM_PROMPT_CACHED,
                messages=build_messages(context),
            )

            raw_text = response.content[0].text
            logger.debug(
                "llm response received",
                extra={"symbol": symbol, "preview": raw_text[:200]},
            )

            return self._parse_response(raw_text, symbol, is_dry_run)

        except Exception as exc:
            logger.error(
                "llm api call failed",
                extra={"symbol": symbol, "error": str(exc)},
            )
            return self._hold_decision(symbol, is_dry_run, reason="LLM_ERROR")

    def _parse_response(
        self,
        raw_text: str,
        symbol: str,
        is_dry_run: bool,
    ) -> Decision:
        """
        Parse the LLM JSON response into a Decision.

        Strips markdown code fences if present. Returns a HOLD decision on
        any parse or validation failure.
        """
        text = raw_text.strip()
        if text.startswith("```"):
            lines = text.split("\n")
            # Drop the opening fence line and the closing fence line.
            text = "\n".join(lines[1:-1])

        try:
            data = json.loads(text)
            llm_response = LLMDecisionResponse(**data)

            return Decision(
                symbol=symbol,
                action=llm_response.action,
                confidence=llm_response.confidence,
                reasoning=llm_response.reasoning,
                execution_status="pending",
                composite_score=str(llm_response.confidence),
                is_dry_run=is_dry_run,
                created_at=datetime.now(timezone.utc),
            )

        except (json.JSONDecodeError, ValidationError, KeyError, TypeError) as exc:
            logger.warning(
                "failed to parse llm response",
                extra={
                    "symbol": symbol,
                    "error": str(exc),
                    "raw_preview": raw_text[:500],
                },
            )
            return self._hold_decision(symbol, is_dry_run, reason="PARSE_ERROR")

    def _hold_decision(
        self,
        symbol: str,
        is_dry_run: bool,
        reason: str = "HOLD",
    ) -> Decision:
        """Construct a safe HOLD decision with zero confidence."""
        return Decision(
            symbol=symbol,
            action=DecisionAction.HOLD,
            confidence=0,
            reasoning=f"Defaulted to HOLD: {reason}",
            execution_status="rejected",
            rejection_reason=reason,
            is_dry_run=is_dry_run,
            created_at=datetime.now(timezone.utc),
        )
