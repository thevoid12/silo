# LLM Provider Specification

Providers are how Silo's Brain talks to LLMs. Silo supports any OpenAI-compatible endpoint and Anthropic natively, with a single flat config and a single vault key.

---

## 1. Philosophy

- **One active provider at a time.** A single `[providers]` block in config selects the provider, model, and optional base URL. No per-provider subsections.
- **Generic by default.** The config keys are `provider`, `model`, and `base_url` — not tied to any specific vendor. Changing providers is a config edit, not a code change.
- **Two underlying SDKs.** `openai-go` covers everything that speaks the OpenAI spec (OpenAI, Gemini via compat endpoint, OpenRouter, Ollama, vLLM, LM Studio). `anthropic-sdk-go` covers Anthropic's native spec. No LiteLLM dependency.
- **ADK stays as the agent framework.** The Google ADK handles the agent loop, tool calling, and streaming. New providers implement ADK's `model.LLM` interface via format adapters — no changes to the agent loop, session service, or approval system.
- **API keys in the vault only.** No `.env` files, no shell exports, no plaintext on disk.

---

## 2. Provider Routing

```
provider == "anthropic"  →  anthropic-sdk-go  (native Anthropic spec)
anything else            →  openai-go         (OpenAI-compat spec)
```

Built-in base URL defaults (used when `base_url` is empty):

| Provider name | Default base URL |
|---|---|
| `openai` | `https://api.openai.com/v1` |
| `gemini` | `https://generativelanguage.googleapis.com/v1beta/openai/` |
| `openrouter` | `https://openrouter.ai/api/v1` |
| any other name | requires `base_url` to be set |

Setting `base_url` overrides the default for any provider name, enabling self-hosted or custom endpoints.

---

## 3. Configuration

### `silo.toml` — flat, single provider

```toml
[providers]
default  = "gemini"
model    = "gemini-2.0-flash"
base_url = ""   # empty = use built-in default for that provider name
```

**Examples:**

```toml
# OpenAI
default = "openai"
model   = "gpt-4o"

# Anthropic
default = "anthropic"
model   = "claude-opus-4-5"

# OpenRouter
default  = "openrouter"
model    = "meta-llama/llama-3.3-70b-instruct"
base_url = "https://openrouter.ai/api/v1"

# Ollama (self-hosted)
default  = "ollama"
model    = "llama3"
base_url = "http://localhost:11434/v1"
```

There are no per-provider subsections (`[providers.gemini]` etc.) — a single block covers everything.

---

## 4. API Key Management

Single vault key for whichever provider is currently active:

```
llm_api_key
```

### Flow: Key → Vault → SDK

```
silo init (or silo vault set llm_api_key)
  → user enters API key
  → key encrypted with XChaCha20-Poly1305 + Argon2id
  → stored in ~/.silo/vault.enc

silo start
  → vault decrypted at startup
  → llm_api_key read from vault
  → injected into the active provider adapter
  → adapter uses key for all LLM calls
```

The key never touches disk in plaintext. It lives in memory only for the duration of the Silo process.

For self-hosted providers that need no key (e.g. Ollama), `llm_api_key` can be set to any non-empty placeholder or omitted if the vault has no such entry.

---

## 5. Adapter Architecture

ADK's `model.LLM` interface is the extension point:

```go
type LLM interface {
    Name() string
    GenerateContent(ctx context.Context, req *LLMRequest, stream bool) iter.Seq2[*LLMResponse, error]
}
```

`LLMRequest.Contents` is `[]*genai.Content` (Google's type). Each adapter:
1. Receives `[]*genai.Content` from ADK
2. Translates to provider-native format (OpenAI messages / Anthropic messages)
3. Calls the provider API
4. Translates response back to `*genai.Content` with `FunctionCall` parts

ADK's agent loop detects tool calls via `part.FunctionCall != nil` — as long as adapters correctly populate this field, the loop works transparently.

### New packages

```
pkg/core/model/
  openaicompat/
    model.go      # model.LLM impl — parameterised by base_url + api_key
    convert.go    # genai.Content ↔ openai message translation
  anthropic/
    model.go      # model.LLM impl — Anthropic native spec
    convert.go    # genai.Content ↔ anthropic message translation
```

### ModelFactory signature

```go
type ModelFactory func(ctx context.Context, modelName, apiKey, baseURL string) (model.LLM, error)
```

### ProviderConfig

```go
type ProviderConfig struct {
    Provider string
    LLMModel string
    BaseURL  string // optional, overrides built-in default
}
```

### Format translation — genai ↔ OpenAI

```
genai role "model"           → "assistant"
genai Part.Text              → string content / text content part
genai Part.InlineData        → base64 data URL image_url part  (multimodal)
genai Part.FunctionCall      → tool_calls[]{id, function{name, arguments}}
genai Part.FunctionResponse  → role "tool" message {tool_call_id, content}
```

### Format translation — genai ↔ Anthropic

```
genai Part.FunctionCall      → tool_use content block
genai Part.FunctionResponse  → tool_result content block
genai Part.InlineData        → image content block (base64)
```

---

## 6. Setup Flows

### `silo init`

```
$ silo init
  Provider (gemini / openai / anthropic / openrouter / custom): openai
  Model [gpt-4o]:
  Base URL (leave blank for built-in default):
  API key: sk-••••••••
  ✓ Key stored in vault as llm_api_key
  ✓ Provider "openai" / model "gpt-4o" saved to ~/.silo/silo.toml
```

Base URL is only prompted when the provider name is not a known built-in.

### if config updated
- if provider config is manually updated things has to reflect without restarting the app
### Desktop — Settings screen

- **Provider**: free-text input (any string — not a fixed dropdown)
- **Model**: text input (unchanged)
- **Base URL**: optional text input (placeholder: "leave blank for built-in default")
this will update the silo.toml file which is the source of truth
---

## 7. Streaming

ADK handles streaming. Adapters implement `iter.Seq2[*model.LLMResponse, error]` using each SDK's streaming API, setting `Partial: true` on delta chunks and `TurnComplete: true` on the final chunk. Silo's only streaming responsibility is the last mile: forwarding ADK's events as SSE to the gateway.

---

## 8. Supported Providers

| Provider | SDK path | Notes |
|---|---|---|
| **Gemini** | openai-compat → genai OpenAI endpoint | Default. Uses `https://generativelanguage.googleapis.com/v1beta/openai/` |
| **OpenAI** | openai-go | GPT-4o, o1, o3, etc. |
| **Anthropic** | anthropic-sdk-go | Claude Sonnet, Opus, Haiku, etc. |
| **OpenRouter** | openai-go + base_url | Access to 200+ models via one key |
| **Ollama / vLLM / LM Studio** | openai-go + base_url | Self-hosted, no key required |
| **Any OpenAI-compat endpoint** | openai-go + base_url | Custom deployments |

---

## 9. OpenRouter & OpenAI-compat Adapter Constraints

OpenRouter is a transport layer + model marketplace, not a guarantee of uniform behavior. The OpenAI-compat adapter being correct unlocks most models — but behavior varies.

### What works reliably

- **Basic text chat**: universally fine across all models
- **Streaming**: works when the adapter handles OpenAI delta format correctly; some models buffer internally or emit large chunks rather than token-by-token

### What is model-dependent

**Tool calling**
- Only works for models that advertise support
- Some models hallucinate tool schemas more aggressively
- Common failure modes: invalid JSON in arguments, missing `tool_call_id`, partial tool calls in stream
- **Streaming + tool calls is a subtle bug zone**: some models emit tool calls gradually with incomplete JSON mid-stream. The adapter must buffer the complete tool call before passing to ADK — never forward partial `tool_calls` chunks

**Multimodal (biggest trap)**
- Most OpenRouter models do not support images
- Failure modes: images silently ignored, degraded answers with no error, API accepts but model doesn't use the image
- Image content parts should only be sent when the model is known to support them

### The adapter must be defensive

OpenRouter is *compatible* with OpenAI, not *identical*. The adapter must tolerate:
- Extra or unknown fields in responses
- Missing fields in streaming chunks (especially `tool_call_id`, `finish_reason`)
- `arguments` field not always strict JSON (partial JSON mid-stream, non-standard escaping)
- Tool call IDs formatted differently across models

### Validation checklist (drives the adapter test suite)

| Check | What to cover |
|---|---|
| Streaming | partial chunks, final `TurnComplete`, no dropped tokens |
| Tool calling | valid JSON args, stable `tool_call_id`, multiple tool calls in one response |
| Multimodal | single image, large image, graceful failure for unsupported models |
| Error handling | invalid model name, quota exceeded, unexpected response shapes |

---

## 10. Error Handling

- **Missing API key:** Silo refuses to start if `llm_api_key` is absent from the vault for a provider that requires it. Error: `"llm_api_key not found in vault — run: silo vault set llm_api_key"`.
- **Unknown provider + no base_url:** Error at startup: `"provider 'foo' has no built-in base URL — set providers.base_url in config"`.
- **Auth failure:** Provider SDK surfaces the upstream error. Silo logs it and returns a structured error to the client.
- **Model not found:** Returned as-is from the provider. Silo does not maintain a model whitelist.
- **Self-hosted server unreachable:** Connection error surfaced with the configured `base_url` in the message for easy debugging.

---

## 11. Future (V1)

- Fallback chains: ordered provider list, try next on failure
- Hot-swap: change provider at runtime without restart
- Per-request provider override via API
- `silo provider add/list/remove/test/switch` commands
