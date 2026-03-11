# Configuration System Specification (V0)

This spec defines Silo's configuration system — the single source of truth for all runtime behavior. V0 uses **static configuration** — changes require restart.

---

## 1. Configuration Architecture

### Single Source of Truth

`silo.toml` (default: `~/.silo/silo.toml`) is the authoritative configuration file for all Silo runtime behavior.

### Layered Resolution

Configuration values are resolved in this order (highest priority first):

```
CLI flags  >  Environment variables  >  silo.toml  >  Built-in defaults
```

At each layer, only values explicitly set participate. Unset values fall through to the next layer.

### Go Struct Hierarchy

All configuration is parsed into typed Go structs via `viper`:

```go
type AppConfig struct {
    Gateway   GatewayConfig   `mapstructure:"gateway"`
    Providers ProvidersConfig `mapstructure:"providers"`
    Tools     ToolsConfig     `mapstructure:"tools"`
    Session   SessionConfig   `mapstructure:"session"`
    Vault     VaultConfig     `mapstructure:"vault"`
    Logging   LoggingConfig   `mapstructure:"logging"`
    Agent     AgentConfig     `mapstructure:"agent"`
}
```

### Parsing

- **Library**: `github.com/spf13/viper` for TOML parsing + environment variable binding
- **V0**: Static config — loaded once at startup, restart to apply changes
- **V1**: Hot-reload via `github.com/fsnotify/fsnotify` file watcher

---

## 2. Canonical Schema Reference

### Gateway

```go
type GatewayConfig struct {
    Host           string   `mapstructure:"host"`            // "127.0.0.1"
    Port           int      `mapstructure:"port"`            // 5110
    BrainEnabled   bool     `mapstructure:"brain_enabled"`   // true
    MaxRequestSize string   `mapstructure:"max_request_size"` // "10mb"
    CorsOrigins    []string `mapstructure:"cors_origins"`    // ["*"]
    NginxConfig    bool     `mapstructure:"nginx_config"`    // true
    Auth           AuthConfig `mapstructure:"auth"`
    Timeouts       TimeoutConfig `mapstructure:"timeouts"`
}

type AuthConfig struct {
    Enabled bool `mapstructure:"enabled"` // true
}

type TimeoutConfig struct {
    Read  string `mapstructure:"read"`  // "30s"
    Write string `mapstructure:"write"` // "60s"
    Idle  string `mapstructure:"idle"`  // "120s"
}
```

### Providers

```go
type ProvidersConfig struct {
    Default  string                    `mapstructure:"default"`  // "gemini"
    Gemini   *ProviderConfig           `mapstructure:"gemini"`
    OpenAI   *ProviderConfig           `mapstructure:"openai"`
}

type ProviderConfig struct {
    Model string `mapstructure:"model"` // e.g., "gemini-2.0-flash"
}
```

### Tools

```go
type ToolsConfig struct {
    Approval    ApprovalConfig    `mapstructure:"approval"`
    Shell       ShellConfig       `mapstructure:"shell"`
    Filesystem  FilesystemConfig  `mapstructure:"filesystem"`
    Environment EnvironmentConfig `mapstructure:"environment"`
    Sandbox     SandboxConfig     `mapstructure:"sandbox"`
}

type ApprovalConfig struct {
    Mode    string            `mapstructure:"mode"`    // "per-tool"
    Timeout int               `mapstructure:"timeout"` // 30
    Rules   map[string]string `mapstructure:"rules"`   // {"bash": "always", ...}
}

type ShellConfig struct {
    AllowedCommands  []string `mapstructure:"allowed_commands"`
    BlockedPatterns  []string `mapstructure:"blocked_patterns"`
    MaxOutputBytes   int      `mapstructure:"max_output_bytes"`   // 1048576
    TimeoutSecs      int      `mapstructure:"timeout_secs"`       // 30
}

type FilesystemConfig struct {
    WorkingDirectory string   `mapstructure:"working_directory"`  // "~"
    AllowedPaths     []string `mapstructure:"allowed_paths"`      // ["~/", "/tmp"]
    BlockedPaths     []string `mapstructure:"blocked_paths"`      // ["~/.ssh", ...]
    MaxReadSize      string   `mapstructure:"max_read_size"`      // "10mb"
    MaxWriteSize     string   `mapstructure:"max_write_size"`     // "50mb"
}

type EnvironmentConfig struct {
    ExtraPassthrough []string `mapstructure:"extra_passthrough"`
    ExtraStrip       []string `mapstructure:"extra_strip"`
    PassthroughAll   bool     `mapstructure:"passthrough_all"`    // false
}

type SandboxConfig struct {
    Enabled              bool     `mapstructure:"enabled"`                // true
    DirectCommands       []string `mapstructure:"direct_commands"`        // ["ls", "cat", ...]
    MaxOutputSize        string   `mapstructure:"max_output_size"`        // "50mb"
    CleanupOrphansAfter  string   `mapstructure:"cleanup_orphans_after"`  // "1h"
}
```

### Session

```go
type SessionConfig struct {
    DBPath        string `mapstructure:"db_path"`         // "~/.silo/sessions.db"
    RetentionDays int    `mapstructure:"retention_days"`  // 30
}
```

### Vault

```go
type VaultConfig struct {
    Path   string       `mapstructure:"path"`   // "~/.silo/vault.enc"
    Argon2 Argon2Config `mapstructure:"argon2"`
}

type Argon2Config struct {
    MemoryMiB   int `mapstructure:"memory_mib"`   // 8
    Iterations  int `mapstructure:"iterations"`    // 8
    Parallelism int `mapstructure:"parallelism"`   // 2
}
```

### Agent

```go
type AgentConfig struct {
    MaxIterations    int    `mapstructure:"max_iterations"`     // 25
    SystemPromptPath string `mapstructure:"system_prompt_path"` // ""
    RetryOnError     int    `mapstructure:"retry_on_error"`     // 1
    RetryBackoffMs   int    `mapstructure:"retry_backoff_ms"`   // 1000
}
```

### Logging

```go
type LoggingConfig struct {
    Level  string `mapstructure:"level"`   // "info"
    Format string `mapstructure:"format"`  // "auto"
    LogDir string `mapstructure:"log_dir"` // "~/.silo/logs"
}
```

---

## 3. V0 Minimal Configuration (`silo.toml`)

The complete V0 configuration file with defaults:

```toml
# ── Providers ──────────────────────────────────────────────────────
[providers]
default = "gemini"

[providers.gemini]
model = "gemini-2.0-flash"

[providers.openai]
model = "gpt-4o"                          # via LiteLLM

# ── Tools ──────────────────────────────────────────────────────────
[tools.approval]
mode = "per-tool"                         # "all", "none", or "per-tool"
timeout = 30                              # seconds before auto-deny

[tools.approval.rules]
bash = "always"                           # always ask before shell commands

[tools.shell]
allowed_commands = [
    "ls", "cat", "head", "tail", "wc", "grep", "find", "echo",
    "date", "pwd", "whoami", "uname", "curl", "wget",
    "python3", "node", "ruby", "git", "mkdir", "touch", "cp", "mv",
    "pandoc", "pdftotext", "libreoffice", "ffmpeg", "convert",
    "open", "xdg-open", "pbcopy", "pbpaste",
    "msmtp", "sendmail"
]
blocked_patterns = [
    "rm -rf /", "dd if=/dev/zero", "sudo", "su", "eval", "exec",
    "nc", "ncat", "netcat", "ssh", "scp", "chmod 777"
]
max_output_bytes = 1_048_576              # 1 MiB
timeout_secs = 30

[tools.filesystem]
working_directory = "~"                   # user's home (desktop), $CWD (CLI)
allowed_paths = ["~/", "/tmp"]
blocked_paths = [
    "~/.ssh", "~/.gnupg", "~/.silo/vault.enc",
    "~/.aws", "~/.kube", "~/.config/gcloud"
]
max_read_size = "10mb"
max_write_size = "50mb"

[tools.environment]
extra_passthrough = []
extra_strip = []
passthrough_all = false

[tools.sandbox]
enabled = true                               # false to disable all sandboxing
direct_commands = [                          # bypass sandbox (read-only commands)
    "ls", "cat", "head", "tail", "grep", "find",
    "wc", "date", "pwd", "whoami", "uname",
    "git status", "git log", "git diff",
]
max_output_size = "50mb"                     # max total sandbox output
cleanup_orphans_after = "1h"                 # clean old sandbox dirs on startup

# ── Gateway (headless mode) ───────────────────────────────────────
[gateway]
host = "127.0.0.1"
port = 5110
brain_enabled = true
max_request_size = "10mb"
cors_origins = ["*"]

[gateway.auth]
enabled = true

[gateway.timeouts]
read = "30s"
write = "60s"
idle = "120s"

# ── Sessions ──────────────────────────────────────────────────────
[session]
db_path = "~/.silo/sessions.db"
retention_days = 30

# ── Vault ─────────────────────────────────────────────────────────
[vault]
path = "~/.silo/vault.enc"

[vault.argon2]
memory_mib = 8
iterations = 8
parallelism = 2

# ── Agent ─────────────────────────────────────────────────────────
[agent]
max_iterations = 25
system_prompt_path = ""                   # empty = built-in default
retry_on_error = 1
retry_backoff_ms = 1000

# ── Logging ───────────────────────────────────────────────────────
[logging]
level = "info"                            # "debug", "info", "warn", "error"
format = "auto"                           # "auto", "json", "text"
log_dir = "~/.silo/logs"

# ── Security ──────────────────────────────────────────────────────
[security.network]
allow_public = false
```

---

## 4. Environment Variable Binding

Environment variables follow the naming convention `SILO_<SECTION>_<KEY>`:

| Env Var | Config Key | Example |
|---------|-----------|---------|
| `SILO_GATEWAY_PORT` | `gateway.port` | `9090` |
| `SILO_GATEWAY_HOST` | `gateway.host` | `0.0.0.0` |
| `SILO_PROVIDERS_DEFAULT` | `providers.default` | `openai` |
| `SILO_LOGGING_LEVEL` | `logging.level` | `debug` |
| `SILO_VAULT_PATH` | `vault.path` | `/custom/vault.enc` |

Viper handles the mapping automatically via `viper.SetEnvPrefix("SILO")` and `viper.AutomaticEnv()`.

---

## 5. Validation

Configuration is validated at startup before any component initializes:

```go
func (c *AppConfig) Validate() error {
    if c.Gateway.Port < 1 || c.Gateway.Port > 65535 {
        return fmt.Errorf("gateway.port must be 1-65535, got %d", c.Gateway.Port)
    }
    if c.Vault.Argon2.MemoryMiB < 1 {
        return fmt.Errorf("vault.argon2.memory_mib must be >= 1")
    }
    if c.Tools.Approval.Timeout < 0 {
        return fmt.Errorf("tools.approval.timeout must be >= 0")
    }
    // ... more validations
    return nil
}
```

Invalid configuration causes Silo to exit with code **3** (config error) and a clear error message with the invalid key and expected range.

---

## 6. V0/V1 Scoping Summary

### V0 Ships

| Feature | Section |
|---------|---------|
| TOML config file (`silo.toml`) | §1 |
| Viper parsing with Go struct hierarchy | §2 |
| Layered resolution (CLI > env > TOML > defaults) | §1 |
| Environment variable binding (`SILO_*`) | §4 |
| Startup validation with clear error messages | §5 |
| All V0 config keys documented | §3 |
| Static config (restart to apply) | §1 |

### V1 Deferred

| Feature | Notes |
|---------|-------|
| Hot-reload via `fsnotify` file watcher | Static config sufficient for V0 |
| `silo config show/set/edit/diff/validate` CLI commands | Edit `silo.toml` directly in V0 |
| Web-based configuration UI | |
| Per-key hot/cold classification | All keys are "cold" in V0 |
| SIGHUP-triggered reload | |
| Config diff tracking | |

---

## 7. Cross-References

| Spec | Interaction |
|------|-------------|
| [03_gateway.md §13](03_gateway.md) | Gateway config keys |
| [04_providers.md §9](04_providers.md) | Provider config keys |
| [05_channels.md §4](05_channels.md) | Tool approval config |
| [06_sessions.md §5](06_sessions.md) | Session config keys |
| [07_security.md §2, §4](07_security.md) | Vault and shell security config |
| [15_tools_and_system_access.md](15_tools_and_system_access.md) | Filesystem, environment, and email tool config |
| [08_cli.md](08_cli.md) | Global CLI flags override config |
| [11_agent.md](11_agent.md) | Agent config keys |
| [12_logging.md](12_logging.md) | Logging config keys |
