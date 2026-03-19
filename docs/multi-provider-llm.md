# Multi-Provider LLM Support

Silo now supports any LLM provider via a flat config. One provider is active at a time.

## Config (`~/.silo/silo.toml` or `config/silo.toml`)

```toml
[providers]
default  = "gemini"          # provider name
model    = "gemini-2.0-flash"
base_url = ""                # leave blank for built-in default
```

Built-in base URLs (no `base_url` needed):
| Provider | Endpoint |
|----------|----------|
| `gemini` | `https://generativelanguage.googleapis.com/v1beta/openai/` |
| `openai` | `https://api.openai.com/v1` |
| `openrouter` | `https://openrouter.ai/api/v1` |
| `anthropic` | native Anthropic SDK (no base URL) |

For self-hosted (Ollama, vLLM, etc.) set `base_url` to your endpoint.

## Provider Routing

- `provider = "anthropic"` → Anthropic native SDK
- Everything else → OpenAI-compat SDK with the resolved base URL

## Single API Key

One vault key for whichever provider is active:

```
silo vault set llm_api_key
```

## Changing Provider

Via CLI:
```
silo config set provider openrouter
silo config set model mistralai/mistral-7b-instruct
silo config set base_url https://openrouter.ai/api/v1
silo vault set llm_api_key
```

Via Desktop: Settings → Provider / Model / Base URL fields → Save Changes.

Changes take effect on the next server start (or hot-reload if the server reads from viper at runtime).
