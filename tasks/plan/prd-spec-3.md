# PRD: Switch LLM Provider from Anthropic to OpenAI

**Spec**: tasks/specs/spec-3-openai-llm-provider.md

## Summary

Replace the Anthropic Claude integration in `gold-agent/` with the OpenAI SDK, targeting `gpt-4.1-nano`. Five files change: `prompts.py` (fix + simplify), `llm_engine.py` (swap SDK), `config.py` (swap API key field), `requirements.txt` (swap package), `.env.example` (swap key name). All other modules are untouched.

---

## Behavior scenarios

### Feature: Configuration

#### Scenario: Settings loads OPENAI_API_KEY
  Given an `.env` file with `OPENAI_API_KEY=sk-xxx`
  When `Settings` is instantiated
  Then `settings.openai_api_key` equals `"sk-xxx"`

#### Scenario: Default model is gpt-4.1-nano
  Given an `.env` file that does not set `GOLD_LLM_MODEL`
  When `Settings` is instantiated
  Then `settings.gold_llm_model` equals `"gpt-4.1-nano"`

#### Scenario: ANTHROPIC_API_KEY is no longer a known field
  Given `Settings` instantiated without `ANTHROPIC_API_KEY`
  When `settings.anthropic_api_key` is accessed
  Then `AttributeError` is raised (field no longer exists)

---

### Feature: Prompts module

#### Scenario: SYSTEM_PROMPT_TEXT is a non-empty string
  Given the `prompts` module is imported
  When `SYSTEM_PROMPT_TEXT` is accessed
  Then it is a `str` with length > 100

#### Scenario: build_messages returns OpenAI-format list
  Given a non-empty context dict
  When `build_messages(context)` is called
  Then the result is a list of exactly 2 dicts
  And the first has `"role": "system"` and `"content"` equal to `SYSTEM_PROMPT_TEXT`
  And the second has `"role": "user"` and `"content"` as a compact JSON string

#### Scenario: SYSTEM_PROMPT_CACHED no longer exported
  Given the `prompts` module is imported
  When `from engine.prompts import SYSTEM_PROMPT_CACHED` is attempted
  Then `ImportError` is raised

---

### Feature: LLM Decision Engine

#### Scenario: OpenAI client is used for API calls
  Given a configured `LLMDecisionEngine` with a valid `OPENAI_API_KEY`
  When `evaluate(context, symbol)` is called
  Then `openai.OpenAI.chat.completions.create` is invoked (not `anthropic`)

#### Scenario: Valid JSON response produces correct Decision
  Given the OpenAI API returns `{"action":"BUY","confidence":78,"reasoning":"RSI oversold + MACD crossover","suggested_entry":65000,"suggested_take_profit":67000,"suggested_stop_loss":64000}`
  When `evaluate(context, "BTCUSDT")` is called
  Then a `Decision` with `action="BUY"` and `confidence=78` is returned

#### Scenario: API error defaults to HOLD
  Given the OpenAI API raises a network error
  When `evaluate(context, "BTCUSDT")` is called
  Then a `Decision` with `action="HOLD"` and `confidence=0` is returned
  And no exception propagates

#### Scenario: Malformed response defaults to HOLD
  Given the OpenAI API returns non-JSON text despite json_object mode
  When `evaluate(context, "BTCUSDT")` is called
  Then a `Decision` with `action="HOLD"` is returned

---

### Feature: Dependencies

#### Scenario: openai package is in requirements
  Given `requirements.txt` is read
  Then a line matching `openai>=1.0` is present
  And no line matching `anthropic` is present

---

## Tasks

### ICT-1: Fix and simplify prompts.py
- **What**: Rewrite `gold-agent/engine/prompts.py`. The current file is malformed — `SYSTEM_PROMPT_TEXT` is not assigned as a Python string (code is broken at import). Fix it: define `SYSTEM_PROMPT_TEXT` as a proper triple-quoted string with the full trading agent prompt. Remove `SYSTEM_PROMPT_CACHED` entirely. Rewrite `build_messages(context: dict) -> list[dict]` to return OpenAI chat format: `[{"role":"system","content":SYSTEM_PROMPT_TEXT}, {"role":"user","content": format_user_message(context)}]`. Keep `format_user_message` unchanged.
- **Where**: `gold-agent/engine/prompts.py`
- **Validated by**: SYSTEM_PROMPT_TEXT is a non-empty string, build_messages returns OpenAI-format list, SYSTEM_PROMPT_CACHED no longer exported
- **Estimate**: S

### ICT-2: Swap Anthropic SDK for OpenAI in llm_engine.py
- **What**: Replace `import anthropic` with `from openai import OpenAI`. In `__init__`, replace `anthropic.Anthropic(api_key=settings.anthropic_api_key)` with `OpenAI(api_key=settings.openai_api_key)`. In `_call_llm`, replace `self._client.messages.create(model=..., max_tokens=512, system=SYSTEM_PROMPT_CACHED, messages=...)` with `self._client.chat.completions.create(model=..., max_tokens=512, messages=build_messages(context), response_format={"type":"json_object"})`. Extract response text from `response.choices[0].message.content` instead of `response.content[0].text`. Update the import of `build_messages` (it no longer needs `SYSTEM_PROMPT_CACHED`). Remove the markdown fence-stripping logic (redundant when `response_format=json_object` is set, but keep it as a safety net).
- **Where**: `gold-agent/engine/llm_engine.py`
- **Validated by**: OpenAI client is used for API calls, Valid JSON response produces correct Decision, API error defaults to HOLD, Malformed response defaults to HOLD
- **Estimate**: S

### ICT-3: Update config.py and requirements.txt
- **What**: In `config.py`, rename `anthropic_api_key: str = ""` to `openai_api_key: str = ""` (mapped from `OPENAI_API_KEY`). Change `gold_llm_model` default from `"claude-sonnet-4-6"` to `"gpt-4.1-nano"`. In `requirements.txt`, replace `anthropic>=0.25` with `openai>=1.0`.
- **Where**: `gold-agent/config.py`, `gold-agent/requirements.txt`
- **Validated by**: Settings loads OPENAI_API_KEY, Default model is gpt-4.1-nano, ANTHROPIC_API_KEY is no longer a known field, openai package is in requirements
- **Estimate**: S

### ICT-4: Update .env.example
- **What**: In `.env.example`, under the `# LLM` section, replace `ANTHROPIC_API_KEY=` with `OPENAI_API_KEY=`. Update `GOLD_LLM_MODEL=claude-sonnet-4-6` to `GOLD_LLM_MODEL=gpt-4.1-nano`.
- **Where**: `.env.example`
- **Validated by**: (deployment hygiene — no automated BDD scenario)
- **Estimate**: S

---

## Open questions
None.

## Dependencies

| Dependency | Status |
|-----------|--------|
| `OPENAI_API_KEY` in `.env` | Confirmed available |
| `openai>=1.0` PyPI | Available |
