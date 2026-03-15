# Phase B Complete — Core Agent + CLI Chat

## What was delivered

### Tool Approval (pkg/approval)
Users are now prompted inline before any shell command runs. The agent pauses, asks `Allow "cmd"? [y/n]`, and only proceeds if approved. Deny or timeout blocks the command.

### Shell Tool (pkg/shell)
The agent can run shell commands on your behalf. Commands are validated against an allowlist/blocklist from config. Commands outside the allowlist require explicit user approval per-invocation.

### Interactive Chat (silo chat)
Start a local AI chat session with `silo chat`. The session:
- Unlocks the vault to retrieve your API key
- Streams responses token-by-token
- Shows tool calls as they happen (`[tool: shell] cmd=ls`)
- Prompts for approval inline when the agent wants to run a command

Flags:
- `--model` — override the default model for this session
- `--provider` — override the default provider
- `--no-tools` — disable all tool use (pure chat)

### Bug fix: missing SetDefaults on startup
`config.SetDefaults()` was not being called, so all viper defaults (vault path, provider, model, etc.) were absent until a config file existed. Fixed by calling it in `initConfig()`. `silo init --non-interactive` and all other commands now work correctly on a fresh install.

## Verification
```
silo init --non-interactive   # SILO_VAULT_PASSWORD / SILO_PROVIDER / SILO_API_KEY
silo doctor                   # all 5 checks pass
silo chat                     # interactive session (requires real API key)
silo chat --no-tools          # pure text chat
```

## Error codes added
- `ErrMissingID` (code 0) — shared across all packages
- `ErrApprovalTimeout` (code 150), `ErrApprovalNotFound` (code 151)
