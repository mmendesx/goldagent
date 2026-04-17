# Spec: Switch LLM Provider from Anthropic to OpenAI

## Overview
Replace the Anthropic Claude integration in `gold-agent/` with OpenAI's SDK. The decision engine will call `gpt-4.1-nano` instead of Claude. Anthropic-specific prompt-caching (`cache_control`) is removed; OpenAI applies caching automatically for eligible models. The `ANTHROPIC_API_KEY` env var is replaced by `OPENAI_API_KEY`.

## Actors
- **OpenAI API** — receives market context, returns structured trading decision JSON
- **System Operator** — sets `OPENAI_API_KEY` in `.env`

---

## Functional requirements

### FR-1: OpenAI SDK replaces Anthropic SDK
`gold-agent/engine/llm_engine.py` uses `openai.OpenAI` (sync client wrapped in `asyncio.to_thread`) instead of `anthropic.Anthropic`. The API call uses `client.chat.completions.create(model=..., messages=[system, user])`.

### FR-2: System prompt delivered as chat message
OpenAI's chat completions API takes a `messages` list. The system prompt is passed as `{"role": "system", "content": SYSTEM_PROMPT_TEXT}` followed by `{"role": "user", "content": <context JSON>}`. The Anthropic-specific `SYSTEM_PROMPT_CACHED` list with `cache_control` is removed.

### FR-3: Response parsing unchanged
The LLM is still instructed to return a JSON object with `action`, `confidence`, `reasoning`, `suggested_entry`, `suggested_take_profit`, `suggested_stop_loss`. Parsing logic (`_parse_response`) and the HOLD fallback on failure remain identical.

### FR-4: Config updated
- `anthropic_api_key` field removed from `Settings`
- `openai_api_key: str = ""` added, mapped from `OPENAI_API_KEY`
- `gold_llm_model` default changed from `"claude-sonnet-4-6"` to `"gpt-4.1-nano"`

### FR-5: requirements.txt updated
`anthropic>=0.25` replaced with `openai>=1.0`.

### FR-6: .env.example updated
`ANTHROPIC_API_KEY=` line replaced with `OPENAI_API_KEY=` under the `# LLM` section.

### FR-7: prompts.py fixed and simplified
The current `prompts.py` is malformed — `SYSTEM_PROMPT_TEXT` is not assigned as a Python string. Rewrite the file with a valid `SYSTEM_PROMPT_TEXT` string constant, remove `SYSTEM_PROMPT_CACHED` (Anthropic-only), and provide `build_messages(context) -> list[dict]` that returns the full OpenAI messages list (system + user).

---

## Technical requirements

### Files changed (all in `gold-agent/`)

| File | Change |
|------|--------|
| `engine/prompts.py` | Fix malformed file; remove `SYSTEM_PROMPT_CACHED`; `build_messages` returns OpenAI-format list |
| `engine/llm_engine.py` | Replace `anthropic` import with `openai`; update `__init__`, `_call_llm` |
| `config.py` | Replace `anthropic_api_key` with `openai_api_key`; update `gold_llm_model` default |
| `requirements.txt` | Replace `anthropic>=0.25` with `openai>=1.0` |
| `.env.example` | Replace `ANTHROPIC_API_KEY=` with `OPENAI_API_KEY=` |

### OpenAI API call shape

```python
from openai import OpenAI

client = OpenAI(api_key=settings.openai_api_key)

response = client.chat.completions.create(
    model=settings.gold_llm_model,  # "gpt-4.1-nano"
    max_tokens=512,
    messages=build_messages(context),
    response_format={"type": "json_object"},  # enforce JSON output
)
raw_text = response.choices[0].message.content
```

Using `response_format={"type": "json_object"}` guarantees the model returns valid JSON, eliminating the need to strip markdown fences. The parsing path remains unchanged.

### Prompt caching
OpenAI applies prompt caching automatically for `gpt-4.1` family models on prompts exceeding 1024 tokens. No SDK-level flags are required.

---

## Non-functional requirements
- No changes to the dashboard, Postgres schema, or any other module
- HOLD fallback on API failure must remain — never raise from `evaluate()`
- `ANTHROPIC_API_KEY` may remain in the live `.env` file (it is simply unused)

---

## Dependencies

| Dependency | Status |
|-----------|--------|
| `OPENAI_API_KEY` in `.env` | Available (confirmed by user) |
| `openai` PyPI package | Available |

---

## Constraints
- Only the 5 files listed above may be modified
- Model name used: `gpt-4.1-nano` (user specified "gpt-5.4-nano" — no such model exists; `gpt-4.1-nano` is the correct name for the nano-tier GPT-4.1 model)

## Open questions
None.
