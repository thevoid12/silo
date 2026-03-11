# LLM Provider Specification (V0)

Providers are how Silo's Brain talks to LLMs. In V0, Silo does not implement its own provider abstraction — Google's Agent Development Kit (ADK) handles all model communication natively. Silo's job is to configure ADK with the right model and API key, then let ADK do the rest.

---

## 1. Philosophy & Role

- **ADK is the provider layer.** Silo does not wrap, abstract, or re-implement model communication. ADK already speaks to Gemini natively and to OpenAI/Anthropic/others via LiteLLM. There is no `Provider` trait, no custom streaming parser, no token-level plumbing in Silo's codebase.
- **Silo's only responsibilities:** store API keys securely (vault), read provider config from `silo.toml`, and pass both into ADK at startup.
- **One provider in V0.** A single configured provider powers the Brain. Multi-provider, hot-swap, and fallback chains are V1.
- **Swappable via config, not code.** Changing from Gemini to OpenAI is a `silo.toml` edit and a restart — no code changes, no recompilation.

---

## 2. How ADK Handles Providers

ADK supports model communication through two paths:

### Native (Gemini)

ADK talks directly to the Gemini API. No proxy, no translation layer. This is the fastest and most tightly integrated path.

```go
import "google.golang.org/genai"

// Native Gemini — direct API call, no middleware
model := genai.GoogleAI("gemini-2.0-flash", apiKey)
```

### LiteLLM (OpenAI, Anthropic, and Others)

For non-Gemini providers, ADK uses LiteLLM as a unified translation layer. LiteLLM maps the request to the target provider's API format and handles auth, retries, and response parsing.

```go
import "github.com/google/adk-golang/pkg/models/litellm"

// OpenAI via LiteLLM
model := litellm.NewLiteLlmModel("openai/gpt-4o", apiKey)

// Anthropic via LiteLLM
model := litellm.NewLiteLlmModel("anthropic/claude-sonnet-4-20250514", apiKey)
```

### Local Models (Ollama)

For local/self-hosted models, ADK integrates via LiteLLM's Ollama support or direct Ollama configuration. No API key needed — the model runs on your machine.

```go
// Local model via Ollama + LiteLLM
model := litellm.NewLiteLlmModel("ollama/llama3", "")
```

---

## 3. What Silo Does NOT Implement

These were in the old Rust-based spec. They are gone in V0:

| Removed | Why |
|---------|-----|
| `Provider` trait | ADK is the abstraction. Silo doesn't need its own. |
| `TokenStream` / custom streaming | ADK handles streaming natively. |
| rig crate usage | Replaced entirely by ADK. |
| Custom proxy provider | LiteLLM + Ollama cover this use case. |
| Per-request model override | V1. In V0, the model is set in config. |
| `FallbackProvider` | V1. |

---

## 4. API Key Management

**"No env business"** — API keys are stored in the encrypted vault only. No `.env` files, no shell exports, no plaintext on disk. This is unchanged from the original philosophy.

### Flow: Key → Vault → ADK

```
silo init (or silo vault set)
  → user enters API key
  → key encrypted with XChaCha20-Poly1305 + Argon2id
  → stored in ~/.silo/vault.enc

silo start
  → vault decrypted at startup
  → key read from vault
  → injected into ADK model config
  → ADK uses key for all LLM calls
```

The key never touches disk in plaintext. It lives in memory only for the duration of the Silo process.

### Key Injection (Go)

```go
func buildModel(cfg ProviderConfig, vault *Vault) (genai.Model, error) {
    apiKey, err := vault.Get(cfg.Provider + "_api_key")
    if err != nil {
        return nil, fmt.Errorf("no API key for %s in vault: %w", cfg.Provider, err)
    }

    switch cfg.Provider {
    case "gemini":
        return genai.GoogleAI(cfg.Model, apiKey), nil
    case "openai", "anthropic":
        prefix := cfg.Provider + "/" + cfg.Model
        return litellm.NewLiteLlmModel(prefix, apiKey), nil
    case "ollama":
        return litellm.NewLiteLlmModel("ollama/"+cfg.Model, ""), nil
    default:
        return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
    }
}
```

---

## 5. Configuration in `silo.toml`

### V0: Single Provider

```toml
[providers]
default = "gemini"

[providers.gemini]
model = "gemini-2.0-flash"
```

Or with OpenAI:

```toml
[providers]
default = "openai"

[providers.openai]
model = "gpt-4o"
```

Or with a local model:

```toml
[providers]
default = "ollama"

[providers.ollama]
model = "llama3"
# No API key needed — Ollama runs locally
```

### V1: Multiple Providers

```toml
[providers]
default = "gemini"

[providers.gemini]
model = "gemini-2.0-flash"

[providers.openai]
model = "gpt-4o"

[providers.anthropic]
model = "claude-sonnet-4-20250514"

[providers.ollama]
model = "llama3"
```

In V1, the `default` key selects the active provider, and fallback chains become possible (see section 8).

### Provider Setup During `silo init`

In V0, the provider is configured during `silo init`. The init wizard asks:

```
$ silo init
  Select provider: [gemini] openai / anthropic / ollama
  Model: [gemini-2.0-flash]
  Enter API key: sk-••••••••
  ✓ Key encrypted and stored in vault
  ✓ Provider "gemini" configured in silo.toml
```

After init, the provider is set. To change it, edit `silo.toml` and update the vault key.

---

## 6. Streaming

ADK handles streaming natively. Silo does not parse SSE from upstream LLMs, does not buffer tokens, and does not implement any custom streaming logic.

### The Flow

```
LLM API → ADK (stream parse) → ADK Runner → Silo Brain → Gateway/CLI/Desktop
```

- **ADK Runner** receives streamed chunks from the LLM and emits events as part of its agent execution loop.
- **Silo Brain** listens to ADK Runner events and forwards them to the appropriate output channel.
- **Gateway** emits SSE events (`event: token`, `event: tool_call`, etc.) to HTTP clients.
- **CLI / Desktop** renders streamed tokens in real-time.

Silo's only streaming responsibility is the last mile: taking ADK's output events and formatting them as SSE for the gateway or as terminal output for the CLI. The heavy lifting (SSE parsing from LLM APIs, chunk reassembly, error recovery) is ADK's problem.

---

## 7. Supported Providers (V0)

| Provider | Path | API Key Required | Notes |
|----------|------|------------------|-------|
| **Gemini** | Native ADK | Yes | Default. Best integration, lowest latency. |
| **OpenAI** | ADK → LiteLLM | Yes | GPT-4o, GPT-4, o1, etc. |
| **Anthropic** | ADK → LiteLLM | Yes | Claude Sonnet, Opus, Haiku, etc. |
| **Ollama** | ADK → LiteLLM | No | Local models. Llama, Mistral, Phi, etc. |

All four are supported in V0 config, but only **one** is active at a time. The `default` key in `silo.toml` selects which one.

---

## 8. Provider Fallback & Hot-Swap (V1)

These are explicitly out of scope for V0 but designed for in V1:

- **Fallback chains:** ordered list of providers. If the primary fails, try the next.
  ```toml
  [providers.fallback]
  order = ["gemini", "openai", "anthropic"]
  ```
- **Hot-swap:** change providers at runtime without restarting Silo. Triggered via API or CLI.
- **Per-request provider override:** API callers can specify which provider to use per request.
- **Cost-based routing:** route to cheaper providers for simple tasks, expensive providers for complex ones.

In V0: if the configured provider fails, the Brain returns the error. No automatic retry, no fallback.

---

## 9. CLI Commands for Provider Management

### V0 Commands

Provider management in V0 is minimal — configuration happens during `silo init` and in `silo.toml`:

| Command | Description |
|---------|-------------|
| `silo init` | Configures provider, model, and API key as part of initial setup |
| `silo vault set <provider>_api_key` | Update or add an API key in the vault |
| `silo config set providers.default <name>` | Switch the active provider |

### V1 Commands

Full provider management CLI is a V1 feature:

| Command | Description |
|---------|-------------|
| `silo provider add <name>` | Add a provider — prompts for API key, stores in vault |
| `silo provider list` | Show configured providers and their status |
| `silo provider remove <name>` | Remove provider and delete its key from vault |
| `silo provider test <name>` | Send a test prompt to verify connectivity |
| `silo provider switch <name>` | Hot-swap the active provider at runtime |

---

## 10. Error Handling

- **Missing API key:** Silo refuses to start if the configured provider requires a key and none is found in the vault. Clear error message: `"No API key for gemini in vault. Run: silo vault set gemini_api_key"`.
- **Invalid key / auth failure:** ADK surfaces the upstream error. Silo logs it and returns a structured error to the client.
- **Model not found:** If the configured model doesn't exist on the provider, ADK returns an error. Silo does not maintain a model whitelist — the provider is the source of truth.
- **Rate limiting:** ADK / LiteLLM handle retry logic for transient errors. Silo surfaces persistent failures to the client.
- **Ollama not running:** If the configured provider is `ollama` and the local Ollama server is unreachable, Silo returns a clear error: `"Ollama server not reachable at localhost:11434. Is Ollama running?"`.
