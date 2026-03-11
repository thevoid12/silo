# CLI, Installation & Desktop App Specification

The CLI is Silo's primary entry point for configuration, management, and interactive chat. The desktop app (Electron) provides the graphical experience. This spec defines the command tree, installation, output conventions, and the relationship between CLI and desktop.

---

## 1. CLI Architecture

### Binary & Build

Silo ships as a **single statically-linked Go binary** named `silo`. Built with `CGO_ENABLED=0` for zero runtime dependencies. Target size: **15-25 MB** stripped (`go build -ldflags="-s -w"`).

The binary is built from a single `main.go` entry point in `cmd/silo/`. All subcommands register through the root cobra command.

### Argument Parsing

Argument parsing uses **cobra** for command structure and **viper** for configuration binding. Each subcommand is a `*cobra.Command` with its `RunE` handler in a dedicated file under `internal/cmd/`.

```go
// cmd/silo/main.go
func main() {
    root := cmd.NewRootCmd()
    if err := root.Execute(); err != nil {
        os.Exit(1)
    }
}
```

Viper binds configuration from three sources in precedence order:

```
CLI flags  >  Environment variables  >  silo.toml  >  Built-in defaults
```

Environment variables follow the convention `SILO_<SECTION>_<KEY>`, e.g., `SILO_VAULT_PASSWORD`.

### Global Flags

Every subcommand inherits these global flags via the root command's `PersistentFlags()`:

| Flag | Short | Description |
|------|-------|-------------|
| `--config <path>` | | Path to `silo.toml` (default: `~/.silo/silo.toml`) |
| `--data-dir <path>` | | Override data directory (default: `~/.silo/`) |
| `--verbose` | `-v` | Increase log verbosity (repeat for more: `-vv`) |
| `--quiet` | `-q` | Suppress all output except errors |
| `--json` | | Machine-readable JSON output |
| `--no-color` | | Disable colored output |

`--json` and `--quiet` are mutually exclusive. If both are supplied, the CLI exits with code **2** (usage error).

`--no-color` is also implicitly enabled when stdout is not a TTY (detected via `os.Stdout.Fd()` + `term.IsTerminal()`), or when the `NO_COLOR` environment variable is set (per <https://no-color.org>).

### V0 Command Tree

```
silo
├── init                    # First-run wizard
├── start                   # Launch desktop app (or headless with --headless)
├── chat                    # Readline interactive CLI chat
├── vault
│   ├── init                # Create vault, set password
│   ├── set <key>           # Store secret (no-echo prompt for value)
│   ├── get <key>           # Retrieve secret
│   └── list                # List secret keys (not values)
├── version                 # Version + build info
└── doctor                  # Basic health check
```

This is the entire V0 surface. Every command listed above ships in the first release. Nothing else.

### V1 Command Additions

The following commands are deferred to V1. They are listed here for planning purposes only and must not be implemented in V0.

```
silo
├── stop                    # Graceful shutdown (daemon mode)
├── restart                 # Stop + start
├── status                  # Show running state
├── provider
│   ├── add <name>          # Add LLM provider
│   ├── list                # Show configured providers
│   ├── remove <name>       # Remove provider
│   └── test <name>         # Verify connectivity
├── memory
│   ├── search <query>      # Search knowledge store
│   ├── add <content>       # Store a fact
│   ├── list                # List entries
│   ├── delete <id>         # Delete entry
│   └── ingest <file>       # Ingest document
├── config
│   ├── show                # Dump effective config
│   ├── set <key> <value>   # Modify silo.toml
│   └── edit                # Open in $EDITOR
├── logs
│   ├── tail                # Stream live logs
│   └── search <query>      # Search logs
├── usage
│   ├── show                # Usage/cost summary
│   └── detail              # Per-call history
├── tool
│   ├── list                # List tools
│   ├── install <source>    # Install tool
│   └── remove <name>       # Remove tool
└── completions <shell>     # Shell completions (bash, zsh, fish)
```

---

## 2. Installation

### CLI Distribution

| Channel | Command / Method |
|---------|------------------|
| Install script | `curl -fsSL https://install.silo.dev \| sh` |
| Homebrew | `brew install silo` |
| Go install | `go install github.com/silo-org/silo/cmd/silo@latest` |
| GitHub Releases | Download binary from release page |

The install script auto-detects architecture (`arm64` / `amd64`) and OS (`linux` / `darwin`), downloads the correct binary, places it in `~/.local/bin/` (or `/usr/local/bin/` with sudo), and verifies the SHA-256 checksum.

GitHub Releases publishes binaries for all supported platforms:

| OS | Architecture | Binary Name |
|----|-------------|-------------|
| macOS | arm64 | `silo-darwin-arm64` |
| macOS | amd64 | `silo-darwin-amd64` |
| Linux | arm64 | `silo-linux-arm64` |
| Linux | amd64 | `silo-linux-amd64` |
| Windows | amd64 | `silo-windows-amd64.exe` |

### Desktop App Distribution

The desktop app bundles the Electron shell with the Go binary as a sidecar. See [13_desktop_app.md](13_desktop_app.md) for the full desktop specification.

| Platform | Format | Notes |
|----------|--------|-------|
| macOS | `.dmg` | Universal binary (arm64 + amd64), code-signed + notarized |
| Windows | `.msi` / `.exe` | NSIS or WiX installer |
| Linux | `.AppImage` / `.deb` | AppImage for portability, `.deb` for Debian/Ubuntu |

Desktop installers include the `silo` CLI binary and add it to `PATH`, so users who install the desktop app also get the CLI.

---

## 3. `silo init` -- First-Run Wizard

Interactive setup, target completion: **under 60 seconds**.

### Steps

1. **Create directory structure**:
   ```
   ~/.silo/
   ├── silo.toml           # Main configuration
   ├── vault.enc           # Encrypted vault
   ├── logs/               # Log files
   └── workspace/          # Working directory for tool execution
   ```

2. **Set vault password** -- prompts twice for confirmation using no-echo input (`term.ReadPassword()`). Derives encryption key using Argon2id (see [07_security.md](07_security.md)).

3. **Configure LLM provider** -- prompts for provider name (OpenAI, Anthropic, Google) and API key. Stores key in vault.

4. **Write `silo.toml`** with defaults.

### Interactive Flow

```
$ silo init

  Welcome to Silo!

  Step 1/3: Vault Setup
  Create a password to encrypt your secrets.
  Password: ********
  Confirm:  ********
  ✓ Vault created

  Step 2/3: LLM Provider
  Which provider? [openai/anthropic/google]: anthropic
  API key: sk-ant-••••••••
  ✓ Provider configured

  Step 3/3: Configuration
  ✓ Config written to ~/.silo/silo.toml

  Setup complete! Run `silo chat` to start chatting.
```

### Non-Interactive Init

For headless or automated setup:

```bash
SILO_VAULT_PASSWORD="strongpass" \
SILO_PROVIDER_NAME="openai" \
SILO_PROVIDER_API_KEY="sk-..." \
silo init --non-interactive
```

All required values are read from environment variables. Missing variables cause a clear error listing what is needed.

### First-Run Guard

Running any command that requires initialization (e.g., `silo start`, `silo chat`) without a prior `silo init` produces:

```
Error: Silo is not initialized.

Run `silo init` to set up your configuration and vault.
For automated setup, use `silo init --non-interactive` with environment variables.
```

Exit code: **3** (config error).

### Re-Running Init

Running `silo init` when `~/.silo/` already exists prompts:

```
Silo is already initialized at ~/.silo/

Overwrite existing configuration? This will NOT delete your vault. [y/N]:
```

Answering `y` regenerates `silo.toml` with defaults but preserves `vault.enc`. Answering `N` (default) aborts.

---

## 4. `silo start` -- Launch Desktop or Headless

`silo start` is the primary way to run Silo.

| Mode | Command | Behavior |
|------|---------|----------|
| Desktop (default) | `silo start` | Launches the Electron desktop app |
| Headless | `silo start --headless` | Starts the HTTP daemon in the foreground |

### Desktop Mode (Default)

`silo start` without flags launches the Electron desktop app. The Go binary starts as a sidecar process managed by Electron. See [13_desktop_app.md](13_desktop_app.md) for details on the desktop architecture.

If the Electron app is not installed (CLI-only installation), `silo start` prints:

```
Error: Desktop app not found.

Install the desktop app from https://silo.dev/download
Or run headless mode: silo start --headless
Or use the CLI chat: silo chat
```

Exit code: **1** (general error).

### Headless Mode

`silo start --headless` starts the HTTP gateway in the current terminal. Logs go to stdout. Ctrl-C triggers graceful shutdown.

```
$ silo start --headless

  Silo v0.1.0 starting in headless mode...
  Gateway listening on 127.0.0.1:8080
  Press Ctrl-C to stop.
```

Headless mode is the right choice for: servers, Raspberry Pi, Docker containers, CI/CD environments, and any system without a display.

### V0 Scope

In V0, there is no background daemon mode. Both desktop and headless run in the foreground:
- Desktop: Electron app window is open, Go sidecar runs as a child process.
- Headless: Go process runs in the terminal, Ctrl-C to stop.

Daemon mode (`silo start --daemon`), `silo stop`, `silo restart`, and `silo status` are deferred to V1.

---

## 5. `silo chat` -- Interactive CLI Chat

`silo chat` launches an interactive readline-based chat session in the terminal. It connects directly to the local agent (in-process, no HTTP round-trip needed).

### Implementation

The readline loop uses `github.com/chzyer/readline` for line editing, history, and input handling. Responses stream token-by-token to stdout.

```go
rl, _ := readline.NewEx(&readline.Config{
    Prompt:      "you> ",
    HistoryFile: filepath.Join(dataDir, ".chat_history"),
})
defer rl.Close()

for {
    line, err := rl.Readline()
    if err != nil { // EOF or Ctrl-D
        break
    }
    // Send to agent, stream response to stdout
}
```

### Session Flow

```
$ silo chat

  Silo v0.1.0 — Type /help for commands, Ctrl-D to exit.

you> What files are in the current directory?

silo> I'll check that for you.

  ┌─ Tool Call: bash ──────────────────────────
  │ ls -la
  │
  │ Approve? [y/n]: y
  └────────────────────────────────────────────

silo> Here are the files in the current directory:
  - README.md
  - main.go
  - go.mod

you>
```

### Tool Approval

When the agent wants to execute a tool, the CLI displays the tool name and arguments, then prompts with `[y/n]`. This is a simple blocking prompt -- no async UI needed.

| Input | Effect |
|-------|--------|
| `y` or `Y` or Enter | Approve and execute |
| `n` or `N` | Deny, agent receives denial and continues reasoning |

### Streaming Output

Agent responses stream token-by-token to stdout. The prompt prefix `silo>` is printed once at the start of the response. Tokens are written with `fmt.Print()` (no newline) as they arrive, producing a typewriter effect.

### In-Session Commands

| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/clear` | Clear the screen |
| `/exit` | Exit the chat (same as Ctrl-D) |
| `/new` | Start a new conversation (clear context) |

### Chat Flags

| Flag | Description |
|------|-------------|
| `--model <name>` | Override the default model for this session |
| `--provider <name>` | Override the default provider for this session |
| `--no-tools` | Disable tool use for this session |

---

## 6. `silo vault` -- Secret Management

The vault commands manage encrypted secrets stored in `~/.silo/vault.enc`. The vault must be initialized before other commands can use it.

### `silo vault init`

Creates the vault and sets the encryption password. This is normally handled by `silo init`, but can be run independently.

```
$ silo vault init
  Password: ********
  Confirm:  ********
  ✓ Vault created at ~/.silo/vault.enc
```

If a vault already exists, prompts for confirmation before overwriting.

### `silo vault set <key>`

Stores a secret. Prompts for the value with no-echo input.

```
$ silo vault set OPENAI_API_KEY
  Vault password: ********
  Value: ••••••••••••••
  ✓ Stored: OPENAI_API_KEY
```

If the key already exists, the value is overwritten silently.

### `silo vault get <key>`

Retrieves and displays a secret.

```
$ silo vault get OPENAI_API_KEY
  Vault password: ********
  sk-proj-abc123...xyz
```

With `--json`:

```json
{"key": "OPENAI_API_KEY", "value": "sk-proj-abc123...xyz"}
```

### `silo vault list`

Lists secret key names. Never displays values.

```
$ silo vault list
  Vault password: ********

  Keys:
    OPENAI_API_KEY
    ANTHROPIC_API_KEY
    SEARCH_API_KEY

  3 secrets stored
```

### Vault Password Handling

Every vault command prompts for the vault password. In V0, there is no password caching -- each invocation requires the password. The `SILO_VAULT_PASSWORD` environment variable is accepted as an alternative to interactive prompts (useful for scripts and CI).

Password caching (OS keychain integration) is deferred to V1.

---

## 7. `silo doctor` -- Health Check

`silo doctor` runs a series of environment checks and reports pass/warn/fail for each:

```
$ silo doctor

  Silo Doctor
  ───────────────────────────────────

  Data Directory   ✓  ~/.silo/ exists and is writable
  Config File      ✓  silo.toml is valid
  Vault            ✓  vault.enc exists and is readable
  Provider         ✓  anthropic configured and reachable
  Workspace        ✓  ~/.silo/workspace/ exists and is writable
  Go Runtime       ✓  go1.22.0 linux/arm64

  6 passed, 0 warnings, 0 failures
```

### Checks Performed

| Check | Pass | Warn | Fail |
|-------|------|------|------|
| Data directory | `~/.silo/` exists and writable | -- | Missing or not writable |
| Config file | `silo.toml` parses and validates | Non-default values detected | Missing or invalid |
| Vault | `vault.enc` exists and readable | -- | Missing or corrupted |
| Provider | At least one provider configured and reachable (sends lightweight API call) | Provider configured but unreachable | No provider configured |
| Workspace | `~/.silo/workspace/` exists and writable | -- | Missing or not writable |
| Go runtime | Reports Go version and platform | -- | (always passes) |

Exit code: **0** if all checks pass or warn, **1** if any check fails.

With `--json`, outputs structured results:

```json
{
  "checks": [
    {"name": "data_directory", "status": "pass", "message": "~/.silo/ exists and is writable"},
    {"name": "provider", "status": "warn", "message": "anthropic configured but unreachable"}
  ],
  "summary": {"pass": 5, "warn": 1, "fail": 0}
}
```

---

## 8. `silo version` -- Version Info

```
$ silo version

  silo 0.1.0 (abc1234 2026-03-15)
  Go:        go1.22.0
  Platform:  darwin/arm64
```

With `--json`:

```json
{
  "version": "0.1.0",
  "commit": "abc1234",
  "build_date": "2026-03-15",
  "go_version": "go1.22.0",
  "platform": "darwin/arm64"
}
```

Version information is embedded at build time via `-ldflags`:

```bash
go build -ldflags="-s -w \
  -X main.version=0.1.0 \
  -X main.commit=$(git rev-parse --short HEAD) \
  -X main.buildDate=$(date -u +%Y-%m-%d)" \
  ./cmd/silo/
```

---

## 9. Output & UX

### Output Modes

| Mode | Trigger | Behavior |
|------|---------|----------|
| Human (default) | TTY detected | Colored, formatted output |
| JSON | `--json` flag | Machine-readable JSON on stdout, errors on stderr |
| Quiet | `--quiet` flag | Errors only (stderr) |
| Plain | `--no-color` flag or non-TTY or `NO_COLOR` env | Human-readable, no ANSI escapes |

### Colors

Colored output uses ANSI escape codes directly (no heavy dependency needed). Color mapping:

| Color | Usage |
|-------|-------|
| Green | Success, pass, checkmarks |
| Yellow | Warnings |
| Red | Errors, failures |
| Cyan | Prompts, interactive input |
| Bold white | Labels, headings |
| Gray (dim) | Hints, supplementary text |

### Progress Indicators

Long-running operations (vault key derivation, provider connectivity test) display a simple spinner on stderr so it does not interfere with `--json` output on stdout.

### Error Messages

All errors include **actionable next steps**:

```
Error: Provider "openai" is not reachable.

  Could not connect to api.openai.com:443 (connection timed out).

  Try:
    1. Check your internet connection.
    2. Verify the API key: silo vault get OPENAI_API_KEY
    3. Check provider status: https://status.openai.com
```

Errors are written to stderr. In `--json` mode, errors are also JSON:

```json
{"error": "provider_unreachable", "message": "Could not connect to api.openai.com:443", "suggestions": ["Check your internet connection", "Verify the API key"]}
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Usage error (bad arguments, invalid flags) |
| 3 | Configuration error (missing or invalid config, not initialized) |
| 4 | Vault error (cannot unlock, corrupted, missing) |
| 5 | Connection error (provider or network unreachable) |

---

## 10. Project Layout (CLI-Related)

```
cmd/
└── silo/
    └── main.go                 # Entry point, calls cmd.NewRootCmd().Execute()

internal/
└── cmd/
    ├── root.go                 # Root cobra command, global flags, viper binding
    ├── init.go                 # silo init
    ├── start.go                # silo start / silo start --headless
    ├── chat.go                 # silo chat
    ├── vault.go                # silo vault {init,set,get,list}
    ├── version.go              # silo version
    └── doctor.go               # silo doctor
```

Each file in `internal/cmd/` contains one top-level command (or command group). Subcommands (e.g., `vault init`, `vault set`) are registered as children of the parent command within the same file.

---

## 11. Verification Walkthroughs

### Fresh Install to First Chat (CLI Only)

```bash
# 1. Install the CLI
curl -fsSL https://install.silo.dev | sh
# -> Downloads silo binary to ~/.local/bin/silo

# 2. Verify installation
silo version
# -> silo 0.1.0 (abc1234 2026-03-15)
# -> Go: go1.22.0
# -> Platform: darwin/arm64

# 3. Initialize
silo init
# -> Prompts for vault password, provider, API key
# -> Creates ~/.silo/ with silo.toml and vault.enc

# 4. Health check
silo doctor
# -> All checks pass

# 5. Start chatting
silo chat
# -> Readline prompt opens, type a message, get a response
```

### Desktop App Install to First Chat

```bash
# 1. Install desktop app (macOS)
# -> Download .dmg from https://silo.dev/download, drag to Applications

# 2. Launch from CLI
silo start
# -> Electron window opens with chat interface

# OR launch from Applications folder / Dock
# -> Same Electron window, Go sidecar starts automatically

# 3. CLI is also available (bundled with desktop install)
silo chat
# -> Works independently of the desktop app
```

### Headless Server Setup (Raspberry Pi)

```bash
# 1. Install
curl -fsSL https://install.silo.dev | sh

# 2. Non-interactive init
SILO_VAULT_PASSWORD="strongpass" \
SILO_PROVIDER_NAME="anthropic" \
SILO_PROVIDER_API_KEY="sk-ant-..." \
silo init --non-interactive

# 3. Start headless
silo start --headless
# -> Gateway listening on 127.0.0.1:8080
# -> Ctrl-C to stop

# 4. In another terminal, use CLI chat
silo chat
# -> Connects to local agent, readline prompt
```

### Vault Management

```bash
# 1. Store a new secret
silo vault set GITHUB_TOKEN
# -> Prompts for vault password, then value (no-echo)

# 2. List all secrets
silo vault list
# -> Shows key names only

# 3. Retrieve a secret
silo vault get GITHUB_TOKEN
# -> Prompts for vault password, prints value

# 4. Scripted access (CI/CD)
SILO_VAULT_PASSWORD="strongpass" silo vault get GITHUB_TOKEN --json
# -> {"key": "GITHUB_TOKEN", "value": "ghp_abc123"}
```

### JSON Output for Scripting

```bash
# Version info as JSON
silo version --json | jq .version
# -> "0.1.0"

# Doctor results as JSON
silo doctor --json | jq '.checks[] | select(.status == "fail")'
# -> (empty if healthy)

# Vault list as JSON
SILO_VAULT_PASSWORD="pass" silo vault list --json
# -> {"keys": ["OPENAI_API_KEY", "ANTHROPIC_API_KEY"], "count": 2}
```

---

## 12. Cross-References

| Spec | Interaction |
|------|-------------|
| [07_security.md](07_security.md) | Vault encryption parameters (Argon2id, XChaCha20-Poly1305) define how `silo vault` commands encrypt/decrypt |
| [09_configuration.md](09_configuration.md) | `silo.toml` schema and hot-reload behavior; V1 `silo config` commands operate on this |
| [13_desktop_app.md](13_desktop_app.md) | `silo start` launches the Electron app; desktop architecture and Go sidecar communication |
| [04_providers.md](04_providers.md) | Provider configuration that `silo init` writes; V1 `silo provider` commands manage |
| [10_memory.md](10_memory.md) | Knowledge store that V1 `silo memory` commands manage |
| [11_agent.md](11_agent.md) | Agent loop that `silo chat` and desktop app invoke |
| [12_logging.md](12_logging.md) | Log output that V1 `silo logs` and `silo usage` commands query |

---

## 13. V0 / V1 Scoping Summary

### V0 Ships

| Feature | Section |
|---------|---------|
| Single Go binary, `CGO_ENABLED=0`, 15-25 MB stripped | 1 |
| Cobra + viper CLI framework | 1 |
| Global flags (`--config`, `--data-dir`, `-v`, `--quiet`, `--json`, `--no-color`) | 1 |
| `silo init` wizard (interactive + `--non-interactive`) | 3 |
| `silo start` (desktop launch + `--headless` mode) | 4 |
| `silo chat` readline interactive chat with streaming and tool approval | 5 |
| `silo vault init/set/get/list` | 6 |
| `silo doctor` health checks | 7 |
| `silo version` / `silo version --json` | 8 |
| Colored output, JSON mode, quiet mode, exit codes | 9 |
| Actionable error messages | 9 |
| Desktop app: `.dmg` (macOS), `.msi` (Windows), `.AppImage`/`.deb` (Linux) | 2 |
| Install script, Homebrew, `go install`, GitHub Releases | 2 |

### V1 Deferred

| Feature | Section |
|---------|---------|
| Daemon mode (`silo start --daemon`, `silo stop`, `silo restart`, `silo status`) | 4 |
| `silo provider add/list/remove/test` | 1 |
| `silo memory search/add/list/delete/ingest` | 1 |
| `silo config show/set/edit` | 1 |
| `silo logs tail/search` | 1 |
| `silo usage show/detail` | 1 |
| `silo tool list/install/remove` | 1 |
| `silo completions <shell>` | 1 |
| `silo vault delete/rotate-key/export/import` | 6 |
| Vault password caching (OS keychain) | 6 |
| `silo chat --new/--session/--list` session management flags | 5 |
