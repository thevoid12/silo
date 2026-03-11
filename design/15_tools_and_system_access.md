# Tools & System Access Specification (V0)

This spec defines how Silo's tools interact with the host operating system — file access, application invocation, email, and system integration. The core principle: **Silo runs as the user, with the user's permissions, gated by human approval. No containers required.**

---

## 1. The Problem

A useful local agent needs to:
- Read a `.docx` from `~/Documents/` and summarize it
- Edit a spreadsheet and save it back
- Send an email with an attachment
- Open a file in the user's default app (LibreOffice, Preview, etc.)
- Run `git` in the user's project directory, not in `~/.silo/workspace/`

Restricting everything to `~/.silo/workspace/` makes Silo useless for real work. But unrestricted access with no guardrails is dangerous. And requiring Docker/Podman to use Silo defeats the "download, run, done" philosophy.

### Why Not Containers

| Concern | Container Approach | Silo's Approach |
|---------|-------------------|-----------------|
| File access | Volume mounts, permission mapping, UID remapping | Direct OS access — same permissions as the user |
| System apps | Can't use host LibreOffice/Pandoc from inside a container | Invokes whatever the user has installed |
| Installation | User must install Docker/Podman + pull images | Single binary, zero dependencies |
| Desktop integration | Can't open files in host apps, can't send emails | Full host integration |
| Raspberry Pi | Docker on ARM is heavy (300MB+ runtime overhead) | Go binary is 15MB |
| Offline use | Need pre-pulled images | Works immediately |

**Containers solve the wrong problem.** Silo isn't running untrusted third-party code in V0 — it's running shell commands that the user sees and approves. The human-in-the-loop approval is the primary security boundary, not process isolation.

V1's Wazero WASM sandbox is for **untrusted third-party tools** (community plugins). Host system access (files, apps, email) always runs as the user, always with approval.

---

## 2. Execution Model

### How Tools Run

All tool execution goes through `os/exec.CommandContext`. The Go process runs as the current OS user. Tools inherit the user's filesystem permissions, installed applications, and network access.

```
User Message → Agent (ADK) → Tool Call → BeforeToolCallback
                                              │
                                    ┌─────────┴──────────┐
                                    │                      │
                              Allowlist check         Approval gate
                              (is command safe?)       (does user approve?)
                                    │                      │
                                    └─────────┬──────────┘
                                              │
                                     Sandbox decision (§12)
                                     (read-only? → direct)
                                     (writes? → sandbox)
                                              │
                                    ┌─────────┴──────────┐
                                    │                      │
                              Direct exec             Sandboxed exec
                              (read-only cmds)        (bwrap / sandbox-exec)
                                    │                      │
                                    └─────────┬──────────┘
                                              │
                                    ┌─────────┴──────────┐
                                    │                      │
                              Host filesystem         Host applications
                              (user's permissions)    (whatever is installed)
```

### Three Execution Contexts

| Context | Working Directory | Filesystem Scope | Approval | Use Case |
|---------|------------------|------------------|----------|----------|
| **Project** | User-specified `--cwd` or current dir | Full user filesystem | Per tool rules | `silo chat` in a project directory |
| **Desktop** | `$HOME` | Full user filesystem | Per tool rules | Electron desktop app |
| **Headless** | Configurable in `silo.toml` | Configurable scope | Configurable | Server/daemon mode |

**Key change from previous spec:** The default working directory is NOT `~/.silo/workspace/`. It is the user's current directory (CLI) or `$HOME` (desktop). `~/.silo/workspace/` is only used as a scratch area for temporary files the agent creates on its own.

---

## 3. Filesystem Access Model

### Tiered Access

```toml
[tools.filesystem]
# Where tools execute by default
working_directory = "~"              # $HOME for desktop, $CWD for CLI

# Directories the agent can freely access (read + write)
# Glob patterns supported
allowed_paths = [
    "~/Documents",
    "~/Desktop",
    "~/Downloads",
    "~/Projects",
    "~/.silo/workspace",            # scratch area, always allowed
    "/tmp",
]

# Directories always blocked — even if a parent is in allowed_paths
# These are never accessible, not even with user approval
blocked_paths = [
    "~/.ssh",                        # SSH keys
    "~/.gnupg",                      # GPG keys
    "~/.silo/vault.enc",            # encrypted vault
    "~/.aws",                        # AWS credentials
    "~/.kube",                       # Kubernetes configs
    "~/.config/gcloud",             # GCP credentials
]

# Maximum file size the agent can read into context (prevents OOM)
max_read_size = "10mb"

# Maximum file size the agent can write
max_write_size = "50mb"
```

### Access Rules

| Action | In `allowed_paths` | Not in `allowed_paths` | In `blocked_paths` |
|--------|-------------------|----------------------|-------------------|
| Read file | Allowed (approval per tool config) | Allowed with approval | **Always blocked** |
| Write file | Allowed (approval per tool config) | Allowed with approval | **Always blocked** |
| Delete file | Requires explicit approval | Requires explicit approval | **Always blocked** |
| List directory | Allowed (approval per tool config) | Allowed with approval | **Always blocked** |

### Path Resolution

```go
func (p *PathPolicy) Check(path string) (AccessLevel, error) {
    resolved := resolvePath(path) // expand ~, resolve symlinks, canonicalize

    // 1. Blocked paths are absolute — no override
    for _, blocked := range p.BlockedPaths {
        if isUnderPath(resolved, blocked) {
            return Blocked, fmt.Errorf("path %q is in blocked_paths", path)
        }
    }

    // 2. Check allowed paths
    for _, allowed := range p.AllowedPaths {
        if isUnderPath(resolved, allowed) {
            return Allowed, nil
        }
    }

    // 3. Not explicitly allowed — requires approval
    return NeedsApproval, nil
}
```

### Symlink Safety

Symlinks are resolved before access checks. A symlink in `~/Documents/` that points to `~/.ssh/` will be blocked. This prevents symlink traversal attacks where the LLM crafts a path that appears safe but resolves to a sensitive location.

```go
resolved, err := filepath.EvalSymlinks(path)
if err != nil {
    return Blocked, fmt.Errorf("cannot resolve path: %w", err)
}
// Now check resolved path against blocked/allowed lists
```

---

## 4. Built-in Tools (V0)

V0 ships four built-in tools. Each is registered as an ADK tool and goes through the approval pipeline.

### 4.1 `bash` — Shell Execution

The general-purpose tool. Can invoke any command the user has installed.

```go
type BashTool struct {
    policy     *ShellPolicy      // allowlist/blocklist
    pathPolicy *PathPolicy       // filesystem access rules
    timeout    time.Duration
}
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `cmd` | string | Yes | The shell command to execute |
| `cwd` | string | No | Working directory (default: session working dir) |

**Execution:**
```go
cmd := exec.CommandContext(ctx, "bash", "-c", command)
cmd.Dir = resolveWorkingDir(cwd, sessionCwd)
cmd.Env = buildSafeEnv()          // filtered environment (see §5)
cmd.Stdout = &limitedWriter{max: maxOutputBytes}
cmd.Stderr = &limitedWriter{max: maxOutputBytes}
```

### 4.2 `read_file` — Read File Contents

Reads a file and returns its contents to the agent. Handles text files directly. For binary formats (.docx, .pdf, .xlsx), uses converters (see §6).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | File path to read |
| `encoding` | string | No | Force encoding (default: auto-detect) |
| `start_line` | int | No | Start reading from this line (1-indexed) |
| `end_line` | int | No | Stop reading at this line |

**Binary format handling:**
```go
func (t *ReadFileTool) Execute(ctx context.Context, args map[string]any) (string, error) {
    path := args["path"].(string)

    // Check filesystem access
    if err := t.pathPolicy.CheckRead(path); err != nil {
        return "", err
    }

    ext := filepath.Ext(path)
    switch ext {
    case ".pdf":
        return extractPDFText(ctx, path)    // pdftotext or Go PDF lib
    case ".docx":
        return extractDocxText(ctx, path)   // unzip + parse XML, or pandoc
    case ".xlsx", ".csv":
        return extractSpreadsheet(ctx, path)
    default:
        return readTextFile(path, args)
    }
}
```

### 4.3 `write_file` — Write File Contents

Writes or overwrites a file.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | File path to write |
| `content` | string | Yes | File contents |
| `mode` | string | No | `"overwrite"` (default) or `"append"` |

### 4.4 `list_dir` — List Directory Contents

Lists files in a directory with metadata.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Directory path |
| `recursive` | bool | No | Recurse into subdirectories (default: false, max depth: 3) |
| `pattern` | string | No | Glob pattern filter (e.g., `"*.go"`) |

Returns structured output: name, size, modification time, type (file/dir/symlink).

---

## 5. Environment Handling

### The Problem with Empty Environments

The previous spec mandated `cmd.Env = []string{}` (empty environment). This breaks most real-world commands:
- `git` needs `HOME` to find `.gitconfig`
- `python3` needs `PATH` and possibly `PYTHONPATH`
- `curl` needs `HOME` for `.curlrc` and CA certificates
- Many tools need `LANG`/`LC_ALL` for encoding

### Filtered Environment (V0)

Instead of empty, Silo provides a **filtered environment** — a curated subset of the user's environment with sensitive variables removed.

```go
func buildSafeEnv() []string {
    // Start with essential variables from the user's environment
    passthrough := []string{
        "HOME", "USER", "LOGNAME",
        "PATH",                          // user's full PATH — needed to find tools
        "LANG", "LC_ALL", "LC_CTYPE",  // encoding
        "TERM",                          // terminal type (for CLI tools)
        "SHELL",                         // user's shell
        "TMPDIR", "TMP", "TEMP",        // temp directories
        "XDG_DATA_HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME",
        "DISPLAY", "WAYLAND_DISPLAY",   // for GUI app launching (Linux)
    }

    env := make([]string, 0, len(passthrough))
    for _, key := range passthrough {
        if val := os.Getenv(key); val != "" {
            env = append(env, key+"="+val)
        }
    }

    return env
}
```

### Always Stripped

These variables are **never** passed to tool execution, regardless of configuration:

| Variable | Why |
|----------|-----|
| `AWS_*` | AWS credentials |
| `GOOGLE_APPLICATION_CREDENTIALS` | GCP service account |
| `AZURE_*` | Azure credentials |
| `OPENAI_API_KEY`, `ANTHROPIC_API_KEY` | LLM API keys |
| `GITHUB_TOKEN`, `GH_TOKEN` | GitHub credentials |
| `SILO_VAULT_PASSWORD` | Vault password |
| `DATABASE_URL` | Database connection strings |
| `*_SECRET`, `*_KEY`, `*_PASSWORD`, `*_TOKEN` | Pattern-based stripping |

```go
var sensitivePatterns = []string{
    "AWS_", "AZURE_", "GOOGLE_APPLICATION",
    "_SECRET", "_KEY", "_PASSWORD", "_TOKEN",
    "OPENAI_", "ANTHROPIC_", "SILO_VAULT",
    "DATABASE_URL", "GITHUB_TOKEN", "GH_TOKEN",
}

func isSensitive(key string) bool {
    upper := strings.ToUpper(key)
    for _, pattern := range sensitivePatterns {
        if strings.Contains(upper, pattern) {
            return true
        }
    }
    return false
}
```

### Configuration

```toml
[tools.environment]
# Additional variables to pass through (on top of the defaults)
extra_passthrough = ["GOPATH", "GOROOT", "JAVA_HOME", "NODE_PATH"]

# Additional variables to always strip (on top of the defaults)
extra_strip = ["MY_COMPANY_SECRET"]

# Set to true to pass the FULL user environment (not recommended)
passthrough_all = false
```

---

## 6. System Application Integration

### Philosophy

Silo does **not** embed application-specific code (no LibreOffice SDK, no Outlook COM automation). Instead, it invokes applications through their **CLI interfaces**. Most productivity apps have CLI modes:

- LibreOffice: `libreoffice --headless --convert-to pdf doc.docx`
- Pandoc: `pandoc doc.docx -o doc.pdf`
- ImageMagick: `convert image.png -resize 50% thumb.png`
- FFmpeg: `ffmpeg -i video.mp4 -vn audio.mp3`
- macOS `open`: `open -a "Preview" file.pdf`

The agent discovers what's available and uses it. If the user doesn't have LibreOffice, the agent can suggest installing it or use an alternative.

### Platform-Aware App Detection

On startup (or first use), Silo probes for common system tools and caches the results:

```go
type SystemCapabilities struct {
    // Document processing
    HasLibreOffice bool   // libreoffice --headless
    HasPandoc      bool   // pandoc
    HasPDFToText   bool   // pdftotext (poppler-utils)

    // Media
    HasFFmpeg      bool   // ffmpeg
    HasImageMagick bool   // convert

    // Dev tools
    HasGit         bool
    HasDocker      bool   // detected but NOT required
    HasPython      bool
    HasNode        bool

    // Platform
    OS             string // "darwin", "linux", "windows"
    OpenCommand    string // "open" (mac), "xdg-open" (linux), "start" (windows)
    ClipboardRead  string // "pbpaste" (mac), "xclip -o" (linux)
    ClipboardWrite string // "pbcopy" (mac), "xclip -i" (linux)

    // Email
    HasMailCommand bool   // sendmail, msmtp, or mutt
    HasSMTPConfig  bool   // silo.toml [tools.email] configured
}

func DetectCapabilities() *SystemCapabilities {
    caps := &SystemCapabilities{OS: runtime.GOOS}

    // Detect by running `command -v <tool>`
    caps.HasLibreOffice = commandExists("libreoffice")
    caps.HasPandoc = commandExists("pandoc")
    caps.HasGit = commandExists("git")
    // ... etc

    switch runtime.GOOS {
    case "darwin":
        caps.OpenCommand = "open"
        caps.ClipboardRead = "pbpaste"
        caps.ClipboardWrite = "pbcopy"
    case "linux":
        caps.OpenCommand = "xdg-open"
        caps.ClipboardRead = "xclip -selection clipboard -o"
        caps.ClipboardWrite = "xclip -selection clipboard -i"
    case "windows":
        caps.OpenCommand = "start"
        caps.ClipboardRead = "powershell Get-Clipboard"
        caps.ClipboardWrite = "clip"
    }

    return caps
}
```

### System Prompt Injection

Detected capabilities are injected into the system prompt so the agent knows what's available:

```
[System tools available on this machine]
- OS: macOS (darwin/arm64)
- Document processing: pandoc, pdftotext
- Media: ffmpeg, imagemagick
- Dev: git, python3, node
- Open files: use `open <path>` to open in default app
- Clipboard: pbcopy/pbpaste
- Email: msmtp configured (see tools.email config)
[End system tools]
```

If LibreOffice isn't installed, the agent won't try to call `libreoffice`. It will use `pandoc` or another available tool, or tell the user what to install.

---

## 7. Common Workflows

### Reading a .docx File

```
User: "Summarize the report in ~/Documents/Q1-report.docx"

Agent thinks: read_file tool with path ~/Documents/Q1-report.docx
  → PathPolicy: ~/Documents is in allowed_paths ✓
  → read_file detects .docx extension
  → Uses pandoc (detected in capabilities): pandoc -t plain Q1-report.docx
  → Returns extracted text to agent
  → Agent summarizes
```

No approval needed for `read_file` if configured as `"never"` in approval rules. The user pre-approved reads in `allowed_paths`.

### Editing a Spreadsheet

```
User: "Add a totals row to ~/Documents/budget.xlsx"

Agent thinks:
  1. read_file ~/Documents/budget.xlsx
     → Extracts to CSV via: libreoffice --headless --convert-to csv budget.xlsx
     → Reads the CSV data

  2. write_file /tmp/silo-budget-modified.csv
     → Writes modified CSV

  3. bash: libreoffice --headless --convert-to xlsx /tmp/silo-budget-modified.csv
     → Converts back to .xlsx

  4. bash: cp /tmp/silo-budget-modified.xlsx ~/Documents/budget.xlsx
     → Approval prompt: "Overwrite ~/Documents/budget.xlsx? [y/n]"
```

### Sending an Email

```
User: "Email the report summary to alice@example.com"

Agent thinks: bash tool with email command
  → Capabilities show: msmtp is configured
  → Composes email via bash:
    echo "Subject: Q1 Report Summary\n\nHere is the summary..." | msmtp alice@example.com
  → Approval prompt shows the full command including recipient and body
  → User approves → email sent
```

For email, Silo provides an optional SMTP configuration:

```toml
[tools.email]
enabled = true
method = "smtp"                    # "smtp", "sendmail", or "system"
smtp_host = "smtp.gmail.com"
smtp_port = 587
smtp_user = "user@gmail.com"
# Password stored in vault: silo vault set email_smtp_password
```

When configured, the agent uses a dedicated `send_email` tool (V1) instead of raw shell commands. In V0, it uses whatever mail CLI the user has (`msmtp`, `mutt`, `sendmail`, or `open mailto:...`).

### Opening a File in Default App

```
User: "Open the PDF I just created"

Agent: bash tool → open ~/Documents/output.pdf (macOS)
       bash tool → xdg-open ~/Documents/output.pdf (Linux)
       bash tool → start ~/Documents/output.pdf (Windows)
```

The agent uses `SystemCapabilities.OpenCommand` to pick the right command for the platform.

---

## 8. Binary File Handling

### The Problem

LLMs work with text. Binary files (.docx, .pdf, .xlsx, .png) need conversion before the agent can reason about them.

### Conversion Pipeline

```
Binary file → Converter → Text/structured output → Agent context
```

| Format | Converter (preference order) | Output |
|--------|------------------------------|--------|
| `.pdf` | `pdftotext` (poppler), Go `pdfcpu` lib | Plain text |
| `.docx` | `pandoc -t plain`, Go `docx` lib, `unzip` + XML parse | Plain text |
| `.xlsx` | `libreoffice --convert-to csv`, Go `excelize` lib | CSV text |
| `.csv` | Direct read | CSV text |
| `.json` | Direct read | JSON text |
| `.png/.jpg/.gif` | Pass to LLM as multimodal input (if provider supports) | Image token |
| `.md/.txt/.html` | Direct read (html: strip tags) | Plain text |

### Go-Native Converters (No External Dependencies)

For cases where system tools aren't available, Silo includes lightweight Go libraries as fallbacks:

```go
// go.mod — optional, zero-CGO document processing
require (
    github.com/pdfcpu/pdfcpu v0.6.0       // PDF text extraction
    github.com/nguyenthenguyen/docx v0.0.0 // DOCX text extraction
    github.com/xuri/excelize/v2 v2.8.0     // XLSX read/write
)
```

These are **fallbacks**. If `pandoc` or `pdftotext` is installed, Silo prefers them (better output quality). The Go libs ensure basic functionality even on a bare system.

### Size Limits

Large files are truncated or chunked:

```go
const (
    MaxReadSize    = 10 * 1024 * 1024  // 10 MB — max file to read into context
    MaxContextSize = 100_000           // ~100K chars — max text sent to LLM
)
```

If a file exceeds `MaxContextSize` after conversion, the agent receives a truncated version with a note: `"[Truncated: showing first 100,000 characters of 450,000. Use start_line/end_line for specific sections.]"`

---

## 9. Headless Mode Specifics

In headless mode (`silo start --headless`), the agent serves external adapters. File access rules differ slightly:

### Restricted by Default

Headless mode defaults to tighter restrictions because there's no human watching the terminal:

```toml
[tools.filesystem]
# Headless default: only workspace and /tmp
allowed_paths = ["~/.silo/workspace", "/tmp"]
blocked_paths = ["~/.ssh", "~/.gnupg", "~/.silo/vault.enc", "~/.aws"]
working_directory = "~/.silo/workspace"
```

To enable broader access in headless mode, the user must explicitly configure it:

```toml
[tools.filesystem]
allowed_paths = ["~/", "/tmp"]     # explicit opt-in to home directory
```

### API-Driven File Operations

External adapters can also use the tool execution API directly:

```
POST /silo/muscle/execute
{
    "tool": "read_file",
    "args": { "path": "~/Documents/report.docx" }
}
```

Same access rules apply. If the path isn't in `allowed_paths`, the request is rejected with a clear error.

---

## 10. Security Boundaries

### What Protects the User

```
Layer 1: blocked_paths          — ~/.ssh, ~/.gnupg, vault, credentials NEVER accessible
Layer 2: allowed_paths          — agent can only access configured directories
Layer 3: Shell allowlist        — only permitted commands can execute
Layer 4: Dangerous pattern block — rm -rf /, sudo, etc. always blocked
Layer 5: Human approval         — user sees and approves every action
Layer 6: Temp dir isolation      — write-commands run in isolated temp dir, outputs copied back (§12)
Layer 7: Env stripping          — secrets never leak via environment variables
Layer 8: Symlink resolution     — can't bypass blocks via symlinks
Layer 9: Size limits            — can't OOM the system with huge reads
```

### What This Does NOT Protect Against

- **A malicious user who approves everything** — Silo is a tool, not a guardian. If the user approves `rm -rf ~`, that's on them.
- **LLM prompt injection that the user doesn't catch** — This is why approval exists. The user must read the command before approving. V1's prompt safety pipeline adds automated detection.
- **Time-of-check-time-of-use (TOCTOU)** — A file could change between the access check and the read. This is inherent to filesystem operations and not practically preventable without containers.

### Desktop vs. Headless vs. CLI Security Posture

| Aspect | CLI | Desktop | Headless |
|--------|-----|---------|----------|
| Default `working_directory` | `$CWD` (user's current dir) | `$HOME` | `~/.silo/workspace` |
| Default `allowed_paths` | `["~/", "/tmp"]` | `["~/", "/tmp"]` | `["~/.silo/workspace", "/tmp"]` |
| Approval present | Yes (stdin prompt) | Yes (modal dialog) | Depends on adapter |
| Environment | Filtered user env | Filtered user env | Minimal env |
| Who's watching | User at terminal | User at desktop | Nobody (or remote adapter) |

---

## 11. Configuration Reference

### Full `[tools]` Section

```toml
[tools.approval]
mode = "per-tool"                    # "all", "none", or "per-tool"
timeout = 30                         # seconds before auto-deny

[tools.approval.rules]
bash = "always"                      # always ask before shell commands
read_file = "never"                  # auto-approve reads in allowed_paths
write_file = "always"                # always ask before writes
list_dir = "never"                   # auto-approve directory listing
send_email = "always"                # always ask before sending email

[tools.shell]
allowed_commands = [
    "ls", "cat", "head", "tail", "wc", "grep", "find", "echo",
    "date", "pwd", "whoami", "uname", "curl", "wget",
    "python3", "node", "ruby", "git", "mkdir", "touch", "cp", "mv",
    "pandoc", "pdftotext", "libreoffice", "ffmpeg", "convert",
    "open", "xdg-open",             # platform file opener
    "pbcopy", "pbpaste",            # macOS clipboard
    "msmtp", "sendmail",            # email
]
blocked_patterns = [
    "rm -rf /", "dd if=/dev/zero", "sudo", "su", "eval", "exec",
    "nc", "ncat", "netcat", "ssh", "scp", "chmod 777",
    "> /etc/", "> /dev/", ">> /proc/",
    "| sh", "| bash", "| zsh",
]
max_output_bytes = 1_048_576         # 1 MiB
timeout_secs = 30

[tools.filesystem]
working_directory = "~"              # default CWD for tool execution
allowed_paths = ["~/", "/tmp"]
blocked_paths = [
    "~/.ssh", "~/.gnupg", "~/.silo/vault.enc",
    "~/.aws", "~/.kube", "~/.config/gcloud",
]
max_read_size = "10mb"
max_write_size = "50mb"

[tools.environment]
extra_passthrough = []               # additional env vars to allow
extra_strip = []                     # additional env vars to block
passthrough_all = false              # DANGEROUS: pass full env

[tools.email]
enabled = false                      # enable email sending
method = "system"                    # "smtp", "sendmail", or "system"
# smtp_host, smtp_port, smtp_user — only if method = "smtp"
# Password: silo vault set email_smtp_password
```

### Go Struct

```go
type FilesystemConfig struct {
    WorkingDirectory string   `mapstructure:"working_directory"` // "~"
    AllowedPaths     []string `mapstructure:"allowed_paths"`
    BlockedPaths     []string `mapstructure:"blocked_paths"`
    MaxReadSize      string   `mapstructure:"max_read_size"`     // "10mb"
    MaxWriteSize     string   `mapstructure:"max_write_size"`    // "50mb"
}

type EnvironmentConfig struct {
    ExtraPassthrough []string `mapstructure:"extra_passthrough"`
    ExtraStrip       []string `mapstructure:"extra_strip"`
    PassthroughAll   bool     `mapstructure:"passthrough_all"`
}

type EmailConfig struct {
    Enabled  bool   `mapstructure:"enabled"`
    Method   string `mapstructure:"method"`    // "smtp", "sendmail", "system"
    SMTPHost string `mapstructure:"smtp_host"`
    SMTPPort int    `mapstructure:"smtp_port"`
    SMTPUser string `mapstructure:"smtp_user"`
}
```

---

## 12. Temp Directory Isolation (V0)

Commands that perform writes, calculations, or modifications run inside an **isolated temp directory**. This is a single, cross-platform approach — the same Go code runs on macOS, Linux, and Windows. No kernel sandboxing, no platform-specific tools, no extra dependencies to install.

### Why Not Platform-Specific Sandboxes

There is no single kernel-level sandbox that works on all operating systems:
- Linux has bubblewrap, Landlock, namespaces — none work on macOS/Windows
- macOS has `sandbox-exec` (Seatbelt) — doesn't exist on Linux/Windows
- Windows has App Containers/Job Objects — completely different API

Claude Code uses platform-specific primitives (bwrap on Linux, sandbox-exec on macOS, no Windows support yet). This means maintaining 2-3 separate sandbox backends with different behaviors and edge cases.

**Silo takes a different approach**: one isolation strategy, all platforms, zero dependencies. The tradeoff is that isolation is enforced by Silo (application-level), not by the kernel. Combined with the existing allowlist, blocklist, blocked_paths, filtered environment, and human approval layers — this provides meaningful protection without platform fragmentation.

### How It Works

```
1. Agent decides to run: libreoffice --headless --convert-to pdf report.docx
2. Silo creates temp dir:  os.MkdirTemp("", "silo-sandbox-")
3. Silo copies/symlinks input files into temp dir
4. Command runs with cwd=tempdir, filtered env, timeout
5. Silo scans temp dir for new/modified files (the outputs)
6. Agent decides where outputs go → user approves final destination
7. Silo copies approved outputs to target path
8. Temp dir deleted (os.RemoveAll)
```

```
┌─────────────────────────────────────────────────────────┐
│                      Host Filesystem                      │
│                                                           │
│  ┌─────────────────────────────────────────────────────┐ │
│  │         Temp Directory: /tmp/silo-sandbox-<uuid>/    │ │
│  │                                                      │ │
│  │  input/                                              │ │
│  │  ├── report.docx  ← symlinked from ~/Documents/     │ │
│  │                                                      │ │
│  │  output/                                             │ │
│  │  ├── report.pdf   ← created by libreoffice          │ │
│  │                                                      │ │
│  │  Command runs here (cwd = this dir)                  │ │
│  └─────────────────────────────────────────────────────┘ │
│                                                           │
│  After completion:                                        │
│  output/report.pdf → ~/Documents/report.pdf (approved)    │
│  Temp dir deleted                                         │
└─────────────────────────────────────────────────────────┘
```

### 12.1 Sandbox Manager

```go
type Sandbox struct {
    baseDir    string        // os.TempDir() by default
    maxSize    int64         // max total output size (default 50MB)
    timeout    time.Duration // inherited from shell tool timeout
    pathPolicy *PathPolicy  // blocked_paths still enforced on outputs
}

type SandboxSession struct {
    ID        string    // UUID
    Dir       string    // /tmp/silo-sandbox-<uuid>/
    InputDir  string    // /tmp/silo-sandbox-<uuid>/input/
    OutputDir string    // /tmp/silo-sandbox-<uuid>/output/
    CreatedAt time.Time
}

func (s *Sandbox) NewSession(ctx context.Context) (*SandboxSession, error) {
    id := uuid.New().String()
    dir, err := os.MkdirTemp(s.baseDir, "silo-sandbox-")
    if err != nil {
        return nil, fmt.Errorf("create sandbox dir: %w", err)
    }

    inputDir := filepath.Join(dir, "input")
    outputDir := filepath.Join(dir, "output")
    os.MkdirAll(inputDir, 0700)
    os.MkdirAll(outputDir, 0700)

    return &SandboxSession{
        ID: id, Dir: dir,
        InputDir: inputDir, OutputDir: outputDir,
        CreatedAt: time.Now(),
    }, nil
}
```

### 12.2 Input Staging

Input files are symlinked (not copied) into the sandbox for efficiency. On Windows where symlinks need elevation, files are copied instead.

```go
func (ss *SandboxSession) StageInput(srcPath string) (string, error) {
    resolved, err := filepath.EvalSymlinks(srcPath)
    if err != nil {
        return "", fmt.Errorf("resolve input path: %w", err)
    }

    destPath := filepath.Join(ss.InputDir, filepath.Base(resolved))

    // Prefer symlink (zero-copy), fall back to copy
    if err := os.Symlink(resolved, destPath); err != nil {
        // Windows without developer mode, or cross-device
        if err := copyFile(resolved, destPath); err != nil {
            return "", fmt.Errorf("stage input: %w", err)
        }
    }

    return destPath, nil
}
```

### 12.3 Sandboxed Execution

The command runs with the sandbox directory as its working directory. The command can still read the host filesystem (it's the same process, same user), but all relative-path outputs land in the sandbox.

```go
func (ss *SandboxSession) Execute(ctx context.Context, command string, env []string) (*ExecResult, error) {
    // Rewrite the command to use sandbox paths
    // Input files are in ./input/, outputs go to ./output/
    sandboxedCmd := rewriteCommandPaths(command, ss)

    cmd := exec.CommandContext(ctx, "bash", "-c", sandboxedCmd)
    cmd.Dir = ss.Dir
    cmd.Env = env  // already filtered by buildSafeEnv()
    cmd.Stdout = &limitedWriter{max: maxOutputBytes}
    cmd.Stderr = &limitedWriter{max: maxOutputBytes}

    err := cmd.Run()
    return &ExecResult{
        ExitCode: cmd.ProcessState.ExitCode(),
        Stdout:   cmd.Stdout.(*limitedWriter).String(),
        Stderr:   cmd.Stderr.(*limitedWriter).String(),
    }, err
}

// Rewrites paths so the command operates on sandbox copies
// e.g., "libreoffice --convert-to pdf ~/Documents/report.docx"
// becomes "libreoffice --convert-to pdf ./input/report.docx"
// with --outdir ./output/
func rewriteCommandPaths(command string, ss *SandboxSession) string {
    // Tool-specific output directory flags
    // libreoffice: --outdir ./output/
    // pandoc: -o ./output/<name>
    // ffmpeg: output path rewritten
    // python3/node: cwd is already the sandbox
    return command // actual implementation rewrites based on tool detection
}
```

### 12.4 Output Collection & Approval

After execution, Silo scans the sandbox for new files and presents them to the user for approval before copying to the final destination.

```go
func (ss *SandboxSession) CollectOutputs() ([]SandboxOutput, error) {
    var outputs []SandboxOutput

    err := filepath.WalkDir(ss.Dir, func(path string, d fs.DirEntry, err error) error {
        if err != nil || d.IsDir() {
            return err
        }

        // Skip input files (they were staged, not created)
        if strings.HasPrefix(path, ss.InputDir) {
            return nil
        }

        info, _ := d.Info()
        // Only include files created/modified after sandbox start
        if info.ModTime().After(ss.CreatedAt) {
            rel, _ := filepath.Rel(ss.Dir, path)
            outputs = append(outputs, SandboxOutput{
                SandboxPath: path,
                RelPath:     rel,
                Size:        info.Size(),
                ModTime:     info.ModTime(),
            })
        }
        return nil
    })

    return outputs, err
}

type SandboxOutput struct {
    SandboxPath string    // full path inside sandbox
    RelPath     string    // relative to sandbox dir
    Size        int64
    ModTime     time.Time
}

// Copy an approved output to its final destination
func (ss *SandboxSession) CopyOutput(output SandboxOutput, destPath string, policy *PathPolicy) error {
    // Enforce blocked_paths on the destination
    access, err := policy.Check(destPath)
    if err != nil || access == Blocked {
        return fmt.Errorf("destination %q is blocked", destPath)
    }

    return copyFile(output.SandboxPath, destPath)
}
```

### 12.5 Cleanup

```go
func (ss *SandboxSession) Cleanup() error {
    return os.RemoveAll(ss.Dir)
}

// Called on Silo startup to clean orphaned sandboxes (from crashes)
func CleanOrphanedSandboxes(baseDir string, maxAge time.Duration) {
    entries, _ := os.ReadDir(baseDir)
    for _, e := range entries {
        if strings.HasPrefix(e.Name(), "silo-sandbox-") {
            info, _ := e.Info()
            if time.Since(info.ModTime()) > maxAge {
                os.RemoveAll(filepath.Join(baseDir, e.Name()))
            }
        }
    }
}
```

### 12.6 Which Commands Get Sandboxed

Not every command needs sandboxing. Read-only commands run directly. Commands that produce output or modify files run in the sandbox.

```go
type SandboxPolicy struct {
    // Commands that always bypass the sandbox (read-only, no side effects)
    DirectCommands []string  // ["ls", "cat", "head", "tail", "grep", "find", "wc",
                             //  "date", "pwd", "whoami", "uname", "git status",
                             //  "git log", "git diff"]

    // Commands that always use the sandbox (produce output files)
    SandboxedCommands []string // ["libreoffice", "pandoc", "ffmpeg", "convert",
                               //  "python3", "node", "ruby"]
}

func shouldSandbox(command string, policy *SandboxPolicy) bool {
    baseCmd := extractBaseCommand(command)

    for _, direct := range policy.DirectCommands {
        if baseCmd == direct { return false }
    }
    for _, sb := range policy.SandboxedCommands {
        if baseCmd == sb { return true }
    }

    // Heuristic: if command contains output redirection or write flags, sandbox it
    if containsWriteIndicators(command) { return true }

    // Default: sandbox unknown commands
    return true
}
```

### 12.7 What This Protects Against (and What It Doesn't)

**Protects:**
- Accidental writes to wrong locations (command outputs land in sandbox, not scattered across filesystem)
- Orphaned temp files (sandbox cleanup removes everything)
- Multi-step workflows leaving intermediate artifacts
- Agent writing to unexpected paths via relative path tricks (cwd is the sandbox)

**Does NOT protect (application-level, not kernel-enforced):**
- A command using absolute paths to write outside the sandbox (e.g., `echo "pwned" > /etc/hosts`)
- This is caught by **other layers**: blocked_paths, shell allowlist, dangerous pattern blocklist, and human approval

The temp directory sandbox is **Layer 6** in the defense stack. It doesn't need to be a security boundary on its own — it works alongside the other 8 layers (see §10).

### 12.8 Configuration

```toml
[tools.sandbox]
enabled = true                          # false to disable sandboxing
direct_commands = [                     # bypass sandbox (read-only)
    "ls", "cat", "head", "tail", "grep", "find",
    "wc", "date", "pwd", "whoami", "uname",
    "git status", "git log", "git diff",
]
max_output_size = "50mb"                # max total sandbox output
cleanup_orphans_after = "1h"            # clean old sandbox dirs on startup
```

### 12.9 Verification

```bash
# 1. Sandboxed file conversion
silo chat
> Convert ~/Documents/report.docx to PDF

# Agent flow:
# 1. Creates /tmp/silo-sandbox-abc123/
# 2. Symlinks report.docx → /tmp/silo-sandbox-abc123/input/report.docx
# 3. Runs: libreoffice --headless --convert-to pdf ./input/report.docx --outdir ./output/
# 4. Output: /tmp/silo-sandbox-abc123/output/report.pdf
# 5. Agent: "Created report.pdf (245KB). Save to ~/Documents/report.pdf?"
# 6. User approves → file copied → sandbox deleted

# 2. Sandboxed Python script
> Run this Python script that processes data.csv

# 1. Creates sandbox, stages data.csv
# 2. Runs: python3 ./input/process.py (cwd = sandbox dir)
# 3. Script outputs go to sandbox dir
# 4. Outputs collected, user approves destinations
# 5. Sandbox cleaned up

# 3. Read-only command — no sandbox
> What files are in my project?

# ls ~/Projects/myapp/
# Direct execution, no sandbox needed (read-only command)
```

---

## 13. V0 / V1 Scoping

### V0 Ships

| Feature | Section |
|---------|---------|
| Tiered filesystem access (allowed/blocked/needs-approval) | §3 |
| Four built-in tools (bash, read_file, write_file, list_dir) | §4 |
| Filtered environment (safe subset of user env) | §5 |
| Platform-aware app detection | §6 |
| System capabilities in system prompt | §6 |
| Go-native PDF/DOCX/XLSX fallback readers | §8 |
| Configurable `[tools.filesystem]` and `[tools.environment]` | §11 |
| Headless mode with tighter defaults | §9 |
| Symlink resolution for access checks | §3 |
| Temp directory isolation (cross-platform, single codebase) | §12 |
| Sandbox input staging (symlink/copy) and output collection | §12 |
| Orphaned sandbox cleanup on startup | §12 |

### V1 Deferred

| Feature | Notes |
|---------|-------|
| `send_email` dedicated tool | V0: use bash + msmtp/sendmail |
| `clipboard` dedicated tool | V0: use bash + pbcopy/xclip |
| `open_app` dedicated tool | V0: use bash + open/xdg-open |
| WASM sandbox for third-party tools | Wazero with fuel metering |
| Per-tool filesystem scoping | Each tool gets its own allowed_paths |
| File change monitoring | Watch for external changes to files agent wrote |
| Undo/revert for file writes | Automatic backup before overwrite |
| MCP server tool integration | Third-party tools via Model Context Protocol |
| Remote file access (S3, GCS) | Cloud storage integration |
| OS-native kernel sandbox (bwrap/sandbox-exec) | Optional hardening layer on top of temp dir isolation |
| Overlay filesystem (overlayfs/FUSE) | Copy-on-write isolation for complex workflows |

---

## 14. Cross-References

| Spec | Interaction |
|------|-------------|
| [07_security.md §3, §4](07_security.md) | Process isolation, shell allowlist, blocked patterns |
| [09_configuration.md](09_configuration.md) | `[tools.*]` config sections |
| [11_agent.md §3, §4](11_agent.md) | Tool registration, BeforeToolCallback for approval |
| [14_adk_integration.md §3, §4](14_adk_integration.md) | ADK tool wiring, callback hooks |
| [12_logging.md §5](12_logging.md) | Tool execution events logged |
| [13_desktop_app.md §4](13_desktop_app.md) | Tool approval modal in desktop UI |

---

## 15. Verification Walkthroughs

### 1. Read a .docx from User's Documents

```
1. User: "Summarize ~/Documents/report.docx"
2. Agent calls: read_file(path="~/Documents/report.docx")
3. PathPolicy: ~/Documents/ is in allowed_paths ✓
4. Approval: read_file rule is "never" → auto-approved
5. read_file detects .docx → checks capabilities → pandoc available
6. Runs: pandoc -t plain ~/Documents/report.docx
7. Returns extracted text (truncated to MaxContextSize if needed)
8. Agent summarizes the text
```

### 2. Git Operations in a Project Directory

```
1. User runs: silo chat (from ~/Projects/myapp/)
2. User: "What changed since last commit?"
3. Agent calls: bash(cmd="git diff HEAD~1", cwd="~/Projects/myapp/")
4. PathPolicy: ~/Projects/ is in allowed_paths ✓
5. Shell allowlist: "git" is allowed ✓
6. Approval: bash rule is "always" → user sees "git diff HEAD~1" → approves
7. Environment: HOME, PATH, GIT_* passed through (git needs them)
8. Agent shows the diff
```

### 3. Blocked Path Attempt

```
1. User: "Show me my SSH keys"
2. Agent calls: read_file(path="~/.ssh/id_ed25519")
3. PathPolicy: ~/.ssh is in blocked_paths ✗
4. Tool returns: "Access denied: ~/.ssh is in blocked_paths"
5. Agent tells user: "I can't access ~/.ssh — it's a protected directory."
```

### 4. Email Sending

```
1. User: "Email the summary to team@example.com"
2. Agent calls: bash(cmd='echo "Subject: Summary\n\n..." | msmtp team@example.com')
3. Shell allowlist: "echo" allowed, "msmtp" allowed ✓
4. Dangerous pattern check: no matches ✓
5. Approval: bash rule is "always" → user sees full command including email body → approves
6. Email sent via msmtp
```

### 5. Headless Mode — Restricted Scope

```
1. External adapter: POST /silo/muscle/execute
   { "tool": "read_file", "args": { "path": "~/Documents/secret.txt" } }
2. Headless default: allowed_paths = ["~/.silo/workspace", "/tmp"]
3. ~/Documents/ is NOT in allowed_paths
4. Response: 403 { "error": "path not in allowed_paths", "detail": "..." }
```
