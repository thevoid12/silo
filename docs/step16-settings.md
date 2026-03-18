# Step 16 — Settings View

## What's new

A Settings panel accessible from the bottom of the left sidebar (⚙️ Settings button).

## Settings exposed

| Setting | Description |
|---|---|
| Provider | Default LLM provider (gemini / openai) |
| Gemini model | Model name for Gemini requests |
| OpenAI model | Model name for OpenAI requests |
| Max iterations | How many agent loop steps before stopping |
| Tool approval mode | always / per-tool / never |
| Shell timeout | Max seconds a shell command may run |

## How it works

- `GET /silo/settings` returns current values from viper (merged defaults + user config)
- `PATCH /silo/settings` accepts any subset of fields, applies them in-memory via `viper.Set()`, and persists to `~/.silo/silo.toml`
- Changes take effect immediately for subsequent requests (no restart needed for agent/shell/approval settings)
- Secrets (API key, provider selection for auth) remain in the vault and are managed via the Vault panel

## Storage

Settings are persisted to `~/.silo/silo.toml`. The project defaults in `config/silo.toml` are never modified.
