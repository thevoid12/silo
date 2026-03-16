# Silo — Implementation Plan (Go + ADK + Electron)

> Silo is a local-first, security-first AI agent framework.
> This plan covers the pivot from Rust multi-crate workspace to a single Go module
> with Google ADK for agent orchestration and Electron for the desktop UI.

---

## Workspace Layout
inside package we have each package 
and inside each package we will have a model folder for all the interfaces and structs
all request model from the ui needs to be end with Request, reponse should we end with Response validated and sanitized (use google's validator)
```
silo/
├── cmd/silo/                   # Go binary entry point (cobra root)
│   └── main.go
├── pkg/
│   ├── core/                   # Core agent setup (ADK wiring, shared state)
│   ├── vault/                  # XChaCha20-Poly1305 + Argon2id encrypted vault
│   ├── shell/                  # Shell tool (os/exec, sandbox, allowlist, blocklist)
│   ├── approval/               # Tool approval (Go channels, sync.Map)
│   ├── config/                 # TOML config parsing (viper)
│   ├── gateway/                # HTTP server (gin, SSE, auth middleware)
│   ├── logging/                # zap setup, formatters
│   └── ipc/                    # IPC bridge for desktop mode
├── desktop/                    # Electron app
│   ├── main.js                 # Electron main process
│   ├── src/                    # React/Svelte frontend
│   ├── package.json
│   └── electron-builder.yml
├── go.mod
├── go.sum
├── silo.toml.example
├── Makefile
└── goreleaser.yml
```

---

## Key Go Dependencies

| Package | Replaces | Purpose |
|---------|----------|---------|
| `google.golang.org/adk` | rig, custom ReAct | Agent loop, model gateway, sessions, tools |
| `github.com/spf13/cobra` | clap | CLI framework |
| `github.com/spf13/viper` | serde + toml | Config parsing |
| `github.com/gin-gonic/gin` | axum | HTTP framework |
| `golang.org/x/crypto` | chacha20poly1305 + argon2 crates | Vault encryption |
| `mattn/sqlite` | rusqlite | SQLite driver (pure Go, no CGO) |
| `github.com/jmoiron/sqlx` | — | Ergonomic SQL query building |
| `go.uber.org/zap` | tracing | Structured logging |
| `github.com/google/uuid` | uuid crate | UUID generation |
| `github.com/chzyer/readline` | ratatui | CLI line editing |

---

## Phase A: Skeleton + Vault (Foundation)

### Step 1 — Go module, cobra CLI scaffold, viper config

- `go mod init github.com/user/silo`
- `cmd/silo/main.go` with cobra root command
- Subcommand stubs: `init`, `chat`, `start`, `stop`, `status`, `version`, `doctor`, `vault`
- `pkg/config/` reads `~/.silo/silo.toml` via viper
- Data directory: `~/.silo/` (keys/, workspace/, silo.toml, *.db files)

**Files:**
- `cmd/silo/main.go`
- `pkg/config/config.go`
- `silo.toml.example`
- `go.mod`

### Step 2 — Vault: XChaCha20-Poly1305 + Argon2id

- `pkg/vault/vault.go` — encrypt, decrypt, key derivation
- Argon2id derives a 256-bit key from the user's password
- XChaCha20-Poly1305 AEAD for secret storage
- Vault file: `~/.silo/vault.enc` 
- `silo vault set <key>` — prompts for value, encrypts, stores
- `silo vault get <key>` — decrypts, prints to stdout
- `silo vault list` — lists stored key names (not values)
- `silo vault delete <key>`

**Files:**
- `pkg/vault/vault.go`
- `pkg/vault/vault_test.go`
- `pkg/vault/crypto.go`

### Step 3 — silo init wizard

- Interactive prompts: vault password, provider selection, API key
- Creates `~/.silo/` directory tree
- Writes `~/.silo/silo.toml` with defaults
- Stores the provider API key in the vault
- Generates a random bearer token for gateway auth, stores in vault

**Files:**
- `cmd/silo/init.go`

### Step 4 — silo version, silo doctor

- `silo version` — prints version, commit hash, build date (ldflags) /version/version.go has the version
- `silo doctor` — checks:
  - `~/.silo/` exists and is writable
  - `silo.toml` parses correctly
  - Vault file exists and can be unlocked
  - Provider API key is set
  - SQLite works (open + close a temp db)

**Files:**
- `cmd/silo/version.go`
- `cmd/silo/doctor.go`

---

## Phase B: Agent + Shell Tool (First Chat)

### Step 5 — ADK agent setup
- refer agent.md
- `pkg/core/agent.go` — creates an ADK `Agent` with:
  - System prompt (from config or default) add sys_prompt.toml inside config and add the system prompts there and use
  - Model selection (any model can be selected)
  - Tool registry
- ADK handles the ReAct loop, tool dispatch, and streaming internally
- Provider API key loaded from vault at startup

**Files:**
- `pkg/core/agent.go`
- `pkg/core/runner.go`

### Step 6 — Shell tool as ADK FunctionTool

- `pkg/shell/tool.go` — implements ADK `FunctionTool` interface
- Executes commands via `os/exec.CommandContext` with configurable timeout
- Allowlist: commands that run without approval (e.g., `ls`, `cat`, `git status`)
- Blocklist: commands that are always rejected (e.g., `rm -rf /`, `mkfs`)
- Everything else requires approval (Step 7)
- Working directory defaults to user's CWD (CLI) or $HOME (desktop). See 15_tools_and_system_access.md
- **Temp directory isolation**: Write-commands run in an isolated temp dir (cross-platform, zero dependencies). Input files symlinked in, outputs collected and copied back with approval. Read-only commands run directly. See 15_tools_and_system_access.md §12
- Captures stdout, stderr, exit code; returns structured result to agent

**Files:**
- `pkg/shell/tool.go`
- `pkg/shell/tool_test.go`
- `pkg/shell/policy.go`
- `pkg/shell/sandbox.go`

### Step 7 — Tool approval via Go channels

- `pkg/approval/approval.go`
- When a command needs approval:
  1. Shell tool sends an `ApprovalRequest` on a channel
  2. The UI layer (CLI or gateway) receives it, prompts the user
  3. User responds approve/deny
  4. Response sent back via a per-request response channel
- `sync.Map` tracks pending approvals by request ID
- Timeout: if no response within configurable duration, deny by default

**Files:**
- `pkg/approval/approval.go`
- `pkg/approval/approval_test.go`

### Step 8 — silo chat (interactive CLI)

- `cmd/silo/chat.go` — readline-based interactive loop
- Unlocks vault, loads provider key, initializes ADK runner
- User types a message, agent streams response tokens to stdout
- Tool calls print `[tool: shell] command: ...` before execution
- Approval prompts inline: `Allow "git diff"? [y/n]:`
- `/exit` or Ctrl-D to quit
- Session state held in memory for the duration of the chat

**Files:**
- `cmd/silo/chat.go`

---

### Verification: After Phase B

Run these checks to confirm CLI chat works end-to-end:

```bash
# 1. Build
go build -o silo ./cmd/silo

# 2. Init
./silo init
# -> creates ~/.silo/, prompts for vault password and provider key

# 3. Doctor
./silo doctor
# -> all checks pass

# 4. Chat
./silo chat
> What files are in the current directory?
# -> agent calls shell tool with "ls", streams result back
# -> if command not in allowlist, approval prompt appears

> Create a file called hello.txt with "Hello, Silo" inside.
# -> agent calls shell tool, you approve, file is created

> /exit
```

**MILESTONE: CLI chat works end-to-end.**

---

## Phase C: Gateway + Headless (Adapter-Ready)

### Step 9 — Chi HTTP server with auth
- chat,cli,electron app,external channels like telegram everything has common entry points and largely common core flow
- `pkg/gateway/server.go` — gin router
- Bearer token auth middleware (token from vault)
- Endpoints:
  - `GET /health` — 200 OK (no auth)
  - `GET /silo/status` — server info, uptime, version
- `silo start` launches the server in background (writes PID to `~/.silo/silo.pid`)
- `silo stop` sends SIGTERM to PID
- `silo status` checks if server is running

**Files:**
- `pkg/gateway/server.go`
- `pkg/gateway/middleware.go`
- `cmd/silo/start.go`
- `cmd/silo/stop.go`
- `cmd/silo/status.go`

### Step 10 — /silo/brain/chat SSE endpoint

- `POST /silo/brain/chat` — accepts JSON body `{ "message": "...", "session_id": "..." }`
- Wires to ADK Runner, streams response as SSE events:
  - `event: token` — `data: {"text": "..."}`
  - `event: tool_call` — `data: {"tool": "shell", "args": {"command": "ls"}}`
  - `event: tool_result` — `data: {"tool": "shell", "output": "...", "exit_code": 0}`
  - `event: done` — `data: {}`
  - `event: error` — `data: {"message": "..."}`
- Content-Type: `text/event-stream`
- If a tool call requires approval, emits `event: approval_required` and pauses

**Files:**
- `pkg/gateway/chat.go`

### Step 11 — /silo/brain/tool-approval endpoint

- `POST /silo/brain/tool-approval` — JSON body `{ "request_id": "...", "approved": true }`
- Looks up pending approval in `sync.Map`, sends response on the channel
- Returns 404 if request ID not found, 200 on success
- Timeout: pending approvals expire after configured duration

**Files:**
- `pkg/gateway/approval.go`

### Step 12 — Session persistence via ADK DatabaseSessionService

- ADK provides `DatabaseSessionService` backed by SQLite
- Configure ADK runner to use `mattn/sqlite` as the driver via `sqlx`
- the database file structure is already present,sqlc,dbal etc follow that
- Sessions persist across server restarts
- Session ID returned on first chat, client sends it on subsequent requests

**Files:**
- `pkg/core/session.go`

---

### Verification: After Phase C

Run these checks to confirm the gateway works for external adapters:

```bash
# 1. Start server
./silo start
# -> server listening on :8420

# 2. Health check
curl http://localhost:8420/health
# -> 200 OK

# 3. Chat via SSE
curl -N -H "Authorization: Bearer <token>" \
     -H "Content-Type: application/json" \
     -d '{"message": "List files in the workspace"}' \
     http://localhost:8420/silo/brain/chat
# -> SSE stream: token events, tool_call, tool_result, done

# 4. Tool approval flow
# When approval_required event fires:
curl -H "Authorization: Bearer <token>" \
     -H "Content-Type: application/json" \
     -d '{"request_id": "<id>", "approved": true}' \
     http://localhost:8420/silo/brain/tool-approval
# -> 200 OK, SSE stream resumes

# 5. Stop server
./silo stop
```

**MILESTONE: External adapters (Telegram, etc.) can connect.**

---

## Phase D: Desktop App (Primary UI)

### Step 13 — Electron scaffold + IPC bridge

- `desktop/` — Electron app with React or Svelte frontend
- `desktop/main.js` — Electron main process:
  - Spawns `silo start` as a child process on app launch
  - Sends SIGTERM on app quit
  - IPC bridge: renderer communicates with Go backend via HTTP (localhost)
- `pkg/ipc/` — any Go-side helpers for desktop-specific needs
- Dev mode: `npm run dev` proxies to Go backend

**Files:**
- `desktop/main.js`
- `desktop/package.json`
- `desktop/electron-builder.yml`
- `pkg/ipc/ipc.go`

### Step 14 — Chat view

- Streaming chat UI: messages render as tokens arrive via SSE
- Markdown rendering for agent responses (code blocks, lists, etc.)
- User input at the bottom, auto-scroll, message history
- Session selector in sidebar
- Visual indicators for tool calls (spinner, command preview)

**Files:**
- `desktop/src/views/Chat.{jsx,svelte}`
- `desktop/src/components/Message.{jsx,svelte}`
- `desktop/src/lib/sse.{js,ts}`

### Step 15 — Tool approval modal

- When `event: approval_required` arrives, show a modal:
  - Tool name, command to be executed
  - Approve / Deny buttons
  - "Always allow this command" checkbox (adds to allowlist)
- Posts to `/silo/brain/tool-approval`

**Files:**
- `desktop/src/components/ApprovalModal.{jsx,svelte}`

### Step 16 — Settings view

- Provider configuration: select provider, enter/update API key
- Vault management: change password, list keys
- Shell policy: edit allowlist/blocklist
- Config file editor (silo.toml)
- All changes go through the Go backend API

**Files:**
- `desktop/src/views/Settings.{jsx,svelte}`

### Step 17 — Packaging

- `electron-builder.yml` config for:
  - macOS: `.dmg`
  - Windows: `.msi` (via NSIS or wix)
  - Linux: `.AppImage`
- Go binary bundled inside the Electron app (platform-specific)
- `Makefile` targets: `make desktop-mac`, `make desktop-win`, `make desktop-linux`

**Files:**
- `desktop/electron-builder.yml`
- `Makefile` (updated)

---

### Verification: After Phase D

Run these checks to confirm the desktop app works:

```bash
# 1. Dev mode
cd desktop && npm run dev
# -> Electron window opens, Go backend starts automatically

# 2. Chat
# Type a message in the chat input, verify streaming response appears
# Trigger a tool call, verify the approval modal appears
# Approve, verify the tool runs and result appears in chat

# 3. Settings
# Open settings, change a config value, verify silo.toml is updated
# Add a new vault secret, verify it persists

# 4. Package
make desktop-mac
# -> produces .dmg in desktop/dist/
# Open the .dmg, drag to Applications, launch, verify it works standalone
```

**MILESTONE: Desktop app ships.**

---

## Phase E: Polish

### Step 18 — Structured logging

- `pkg/logging/logging.go` — configures `zap` with:
  - JSON encoder for file output (`~/.silo/silo.log`)
  - Console encoder for stderr (when running in foreground)
  - Log level from config (default: info)
- All packages use `logger.With(zap.String("component", "vault"))` etc.

**Files:**
- `pkg/logging/logging.go`

### Step 19 — Error messages with actionable next steps

- Wrap all user-facing errors with context and a suggested fix
- Examples:
  - `vault locked: run "silo init" to set up your vault`
  - `provider key not found: run "silo vault set provider-key"`
  - `server already running on :8420: run "silo stop" first`

### Step 20 — silo doctor health checks

- Expand doctor from Step 4 with:
  - Network connectivity to provider API
  - Disk space check
  - Port availability (8420)
  - Desktop app binary presence (if packaging is done)
  - Version check (compare with latest release)

### Step 21 — Distribution

- `goreleaser.yml` — builds for darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64
- Homebrew formula: `brew install silo`
- Install script: `curl -sSL https://get.silo.dev | sh`
- `Makefile` ties it all together: `make build`, `make test`, `make release`

**Files:**
- `goreleaser.yml`
- `Makefile`

---

## Summary

| Phase | Steps | Milestone |
|-------|-------|-----------|
| A — Skeleton + Vault | 1-4 | Foundation in place |
| B — Agent + Shell Tool | 5-8 | CLI chat works end-to-end |
| C — Gateway + Headless | 9-12 | External adapters can connect |
| D — Desktop App | 13-17 | Desktop app ships |
| E — Polish | 18-21 | Production-ready distribution |
