# Desktop App Specification (V0)

UI/UX Requirements (Slick as a Core Goal)
Design Targets

    premium, calm, high-contrast
    subtle motion, springy transitions
    zero "developer vibes" in default mode

Performance Targets

    60fps animations
    <100ms input-to-feedback
    no blocking spinners (always show progress state)

- understand 16_design_language.md for ui design
Silo's desktop app provides a native GUI for interacting with the agent. It lives in `desktop/` inside the same repo as the Go code, but is a completely separate build artifact — `go build` ignores it, and `npm run build` inside `desktop/` produces the Electron installer independently.

The Electron app bundles the pre-compiled `silo` binary as a sidecar (placed in `desktop/resources/sidecar/` by the Makefile at package time). On launch, Electron spawns it; on quit, Electron sends SIGTERM. All communication is standard HTTP + SSE over localhost. The Go codebase has no knowledge of Electron.

- end goal of destop app is to look and feel like a claude coworker app. use lightblue+white ui theme,use the same font we use for claude code
---

## 1. Architecture Overview

### Component Diagram

```
┌────────────────────────────────────────────────────┐
│                  Electron Shell                     │
│  ┌──────────────┐  ┌───────────────────────────┐   │
│  │ Main Process  │  │    Renderer Process       │   │
│  │ (Node.js)     │  │    (React + TypeScript)   │   │
│  │               │  │                           │   │
│  │ • Spawn Go    │  │ • Chat view               │   │
│  │ • Port mgmt   │  │ • Tool approval modal     │   │
│  │ • Window mgmt │  │ • Settings view           │   │
│  │ • Tray (V1)   │  │ • Session sidebar         │   │
│  └──────┬───────┘  └────────────┬──────────────┘   │
│         │                       │                   │
│         │    preload bridge     │                   │
│         └───────────┬───────────┘                   │
│                     │ IPC (contextBridge)            │
└─────────────────────┼──────────────────────────────┘
                      │
           HTTP + SSE │ 127.0.0.1:{random_port}
                      │
┌─────────────────────┼──────────────────────────────┐
│              Go Binary (sidecar)                    │
│                     │                               │
│  ┌─────────────────────────────────────────────┐   │
│  │              Gateway (HTTP + SSE)            │   │
│  │  Same protocol as headless mode              │   │
│  ├─────────┬──────────┬──────────┬─────────────┤   │
│  │  Brain  │  Muscle  │  Vault   │  Sessions   │   │
│  └─────────┴──────────┴──────────┴─────────────┘   │
└────────────────────────────────────────────────────┘
```

### Key Principles

1. **Sidecar model.** The Go binary runs as a child process of the Electron main process. When Electron exits, it sends a graceful shutdown signal to the Go process.
2. **Same protocol.** The Go binary does not know it is being driven by Electron. It serves the exact same HTTP + SSE API as when running standalone. The desktop app is architecturally identical to a Telegram adapter or CLI client.
3. **Swappable shell.** Because the coupling is only through HTTP + SSE, migrating from Electron to Wails or Tauri requires only rewriting the frontend layer. The Go binary, all APIs, and all business logic remain untouched.
4. **Offline-first.** The desktop app works without internet for local tools and cached sessions. LLM calls require network connectivity to providers.
5. **Separate build, same repo.** `desktop/` lives alongside the Go code but is a completely independent build. `go build` produces the `silo` binary; `make desktop` compiles the binary, copies it into `desktop/resources/sidecar/`, and runs `electron-builder` to produce the installer.

---

## 2. IPC Protocol

### Startup Sequence

```
1. Electron main process starts
2. Find Go binary path (bundled inside app resources)
3. Pick a random available port on 127.0.0.1
4. Spawn Go binary: `silo start --port {random_port} --desktop-mode`
5. Go binary starts HTTP+SSE server on 127.0.0.1:{random_port}
6. Go binary writes "ready" to stdout once the server is listening
7. Electron main process reads "ready" signal
8. Electron reads bearer token from Go process stdout (or ~/.silo/token)
9. Electron creates BrowserWindow, renderer connects to Go backend
```

### Port Selection

The Electron main process binds to port `0` (OS-assigned), reads the actual port, then passes it to the Go process via `--port` flag. This avoids port conflicts when multiple instances run.

```typescript
// main/sidecar.ts
import { spawn } from 'child_process';
import getPort from 'get-port';

async function startSidecar(): Promise<{ port: number; process: ChildProcess }> {
    const port = await getPort(); // random available port
    const siloPath = getSiloBinaryPath(); // platform-specific

    const child = spawn(siloPath, [
        'start',
        '--port', String(port),
        '--desktop-mode',
        '--no-daemon',
    ], {
        stdio: ['ignore', 'pipe', 'pipe'],
    });

    // Wait for "ready" signal
    await waitForReady(child.stdout);

    return { port, process: child };
}
```

### Communication Protocol

All communication uses standard HTTP and SSE — the same protocol defined in [03_gateway.md](03_gateway.md):

| Operation | Protocol | Endpoint |
|-----------|----------|----------|
| Send chat message | POST (SSE response) | `/silo/brain/chat` |
| Approve/deny tool | POST | `/silo/brain/tool-approval` |
| List sessions | GET | `/silo/vault/sessions` |
| Delete session | DELETE | `/silo/vault/sessions/{id}` |
| Search memory | POST | `/silo/vault/query` |
| Health check | GET | `/silo/health` |
| Get status | GET | `/silo/status` |
| List tools | GET | `/silo/muscle/tools` |
| Get/set config | GET/POST | `/silo/config` |

### SSE Event Types

The Electron app consumes the exact same SSE event types as any other client:

| Event | Desktop Behavior |
|-------|-----------------|
| `event: session` | Update session sidebar, show session info |
| `event: token` | Append text to chat view (streaming) |
| `event: tool_call` | Show tool call block in chat (tool name + args) |
| `event: tool_pending` | Show approval modal (approve/deny buttons) |
| `event: tool_result` | Show tool output in chat, collapse tool block |
| `event: tool_denied` | Show denial notice, close approval modal |
| `event: error` | Show error toast or inline error |
| `event: done` | Mark response as complete, re-enable input |

### `--desktop-mode` Flag

When the Go binary starts with `--desktop-mode`, it:

1. Binds only to `127.0.0.1` (never `0.0.0.0`) — defense-in-depth.
2. Prints the bearer token to stdout on startup (Electron reads it).
3. Logs to `~/.silo/logs/` instead of stdout (Electron owns stdout for IPC signals).
4. Disables the daemon/fork behavior (Electron manages the process lifecycle).
5. Exits cleanly on SIGTERM (Electron sends this on quit).

---

## 3. Security

### Loopback Only

The Go binary in desktop mode binds exclusively to `127.0.0.1`. No external network access to the API is possible.

### Bearer Token

The Go binary generates a session-specific bearer token on each launch. The Electron main process reads this token from stdout and passes it to the renderer via the preload bridge. The token is never written to the renderer's DOM or localStorage.

```typescript
// preload/index.ts
import { contextBridge, ipcRenderer } from 'electron';

contextBridge.exposeInMainWorld('silo', {
    getPort: () => ipcRenderer.invoke('get-silo-port'),
    getToken: () => ipcRenderer.invoke('get-silo-token'),
    // Renderer never sees the raw token — it calls these IPC methods
});
```

### Content Security Policy

The Electron renderer has a strict CSP:

```
default-src 'self';
connect-src http://127.0.0.1:*;
script-src 'self';
style-src 'self' 'unsafe-inline';
```

No external network requests from the renderer. All LLM communication flows through the Go backend.

---

## 4. V0 UI Components

### 4.1 Chat View (Primary)

The main interaction surface. Full-width when session sidebar is collapsed.

```
┌──────────────────────────────────────────────────┐
│  Session: work-tasks-2026           [≡] [⚙] [+] │
├──────────────────────────────────────────────────┤
│                                                  │
│  You: List the files in my project               │
│                                                  │
│  Silo: I'll list the files for you.              │
│                                                  │
│  ┌─ Tool: bash ─────────────────────────────┐    │
│  │ cmd: "ls -la ~/project"                  │    │
│  │ ▶ Documents/  src/  README.md  go.mod    │    │
│  └──────────────────────────────────────────┘    │
│                                                  │
│  Silo: Here are the files in your project:       │
│  - Documents/ (directory)                        │
│  - src/ (directory)                              │
│  - README.md                                     │
│  - go.mod                                        │
│                                                  │
├──────────────────────────────────────────────────┤
│  [📎] Type a message...                    [Send] │
└──────────────────────────────────────────────────┘
```

**Features:**
- Streaming text display (token-by-token as SSE events arrive)
- Markdown rendering for assistant responses
- Code blocks with syntax highlighting and copy button
- Tool call blocks: collapsible, show tool name + arguments + output
- Auto-scroll during streaming, pause on manual scroll-up
- Input: multi-line with Shift+Enter, send with Enter

### 4.2 Tool Approval Modal

Triggered by `event: tool_pending`. Overlays the chat view.

```
┌──────────────────────────────────────┐
│        Tool Approval Required        │
│                                      │
│  Tool: bash                          │
│  Command: rm -rf ./build/            │
│                                      │
│  ┌──────────────────────────────┐    │
│  │ rm -rf ./build/              │    │
│  └──────────────────────────────┘    │
│                                      │
│  ⏱ Auto-deny in 28s                  │
│                                      │
│     [Deny]              [Approve]    │
└──────────────────────────────────────┘
```

**Features:**
- Shows tool name and full arguments
- Syntax-highlighted command preview for bash tools
- Countdown timer showing time until auto-deny (from `tools.approval.timeout`)
- Keyboard shortcuts: Enter = Approve, Escape = Deny
- Cannot be dismissed without choosing (no click-outside-to-close)
- POSTs decision to `POST /silo/brain/tool-approval { call_id, approved }`

### 4.3 Session Sidebar

Collapsible panel on the left side.

```
┌──────────────┐
│  Sessions    │
│  ──────────  │
│  ● work-tasks│ ← active
│    3 msgs    │
│    2m ago    │
│              │
│  ○ debug-log │
│    12 msgs   │
│    1h ago    │
│              │
│  ○ research  │
│    8 msgs    │
│    3d ago    │
│              │
│  [+ New]     │
│              │
│  ──────────  │
│  [Search...] │
└──────────────┘
```

**Features:**
- Lists sessions from `GET /silo/vault/sessions` (sorted by last activity)
- Click to switch session (reloads chat view with session history)
- New session button
- Right-click context menu: Rename, Delete, Export
- Session search/filter

### 4.4 Settings View

Accessible via gear icon. Separate view (not a modal).

```
┌──────────────────────────────────────────────────┐
│  Settings                               [← Back] │
├──────────────────────────────────────────────────┤
│                                                  │
│  Provider                                        │
│  ┌────────────────────────────────────────────┐  │
│  │ Active: openai / gpt-4o                   │  │
│  │ [Change Provider]  [Test Connection]       │  │
│  └────────────────────────────────────────────┘  │
│                                                  │
│  Agent                                           │
│  ┌────────────────────────────────────────────┐  │
│  │ Max iterations: [25    ]                   │  │
│  │ Approval mode:  [per-tool ▾]               │  │
│  │ System prompt:  [Default ▾]                │  │
│  └────────────────────────────────────────────┘  │
│                                                  │
│  Security                                        │
│  ┌────────────────────────────────────────────┐  │
│  │ Shell allowlist: [edit...]                 │  │
│  │ Vault status: Unlocked                     │  │
│  │ [Rotate Token]  [Lock Vault]               │  │
│  └────────────────────────────────────────────┘  │
│                                                  │
│  Usage                                           │
│  ┌────────────────────────────────────────────┐  │
│  │ Today: 12,450 tokens ($0.03)              │  │
│  │ This month: 342,100 tokens ($0.89)        │  │
│  │ [View Details]                             │  │
│  └────────────────────────────────────────────┘  │
│                                                  │
└──────────────────────────────────────────────────┘
```

**Features:**
- Provider selection and configuration
- Agent settings (reads/writes `silo.toml` via Go API)
- Security status and actions
- Usage summary (from `GET /silo/usage`)
- All changes are applied immediately (hot-reload via Go config watcher)

---

## 5. Packaging and Distribution

### Platform Targets

| Platform | Format | Go Binary | Electron |
|----------|--------|-----------|----------|
| macOS (Intel) | `.dmg` | `darwin/amd64` | `darwin/x64` |
| macOS (Apple Silicon) | `.dmg` | `darwin/arm64` | `darwin/arm64` |
| Windows | `.msi` / `.exe` (NSIS) | `windows/amd64` | `win32/x64` |
| Linux | `.AppImage` + `.deb` | `linux/amd64` | `linux/x64` |
| Linux (ARM) | `.AppImage` + `.deb` | `linux/arm64` | `linux/arm64` |

### Go Binary Bundling

The Go binary is compiled per-platform and placed inside the Electron app's resources directory:

```
SiloApp.app/
└── Contents/
    └── Resources/
        └── sidecar/
            └── silo          # platform-specific Go binary
```

On Windows: `resources/sidecar/silo.exe`
On Linux: `resources/sidecar/silo`

### Build Pipeline

```bash
# 1. Build Go binary for target platform (repo root)
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o desktop/resources/sidecar/silo .

# 2. Build Electron app
cd desktop && npx electron-builder --mac --arm64
```

Both steps are wrapped in `make desktop` so CI just runs one command.

The CI pipeline builds all platform combinations in parallel.

### electron-builder Configuration

```yaml
# desktop/electron-builder.yml
appId: com.siloframework.silo
productName: Silo
directories:
  output: dist
  buildResources: resources

mac:
  target:
    - target: dmg
      arch: [x64, arm64]
  category: public.app-category.developer-tools
  hardenedRuntime: true
  entitlements: build/entitlements.mac.plist
  icon: resources/icon.icns

win:
  target:
    - target: nsis
      arch: [x64]
  icon: resources/icon.ico

linux:
  target:
    - target: AppImage
      arch: [x64, arm64]
    - target: deb
      arch: [x64, arm64]
  category: Development
  icon: resources/icons

extraResources:
  - from: resources/sidecar/
    to: sidecar/
    filter:
      - "**/*"
```

### Size Budget

| Component | Target Size |
|-----------|------------|
| Go binary (stripped, compressed) | ~12-15 MB |
| Electron runtime | ~65-70 MB |
| React app + assets | ~3-5 MB |
| **Total installed** | **~80-90 MB** |
| **Total download (compressed)** | **~45-55 MB** |

---

## 6. Process Lifecycle

### Startup

```
Electron launches
    │
    ├── Main process: spawn Go sidecar
    │       │
    │       ├── Go process starts HTTP server
    │       ├── Go process prints "ready\n" to stdout
    │       └── Go process prints "token:{bearer_token}\n" to stdout
    │
    ├── Main process: read port + token
    │
    ├── Main process: create BrowserWindow
    │
    └── Renderer: connect to http://127.0.0.1:{port}
            │
            ├── GET /silo/health → verify connection
            ├── GET /silo/vault/sessions → populate sidebar
            └── Ready for user interaction
```

### Shutdown

```
User clicks Quit (or Cmd+Q / Alt+F4)
    │
    ├── Electron main process: send SIGTERM to Go process
    │       │
    │       └── Go process: graceful shutdown
    │           ├── Drain in-flight SSE streams
    │           ├── Close SQLite connections
    │           ├── Write PID cleanup
    │           └── Exit 0
    │
    ├── Electron main process: wait up to 5s for Go exit
    │   └── If timeout: SIGKILL
    │
    └── Electron exits
```

### Crash Recovery

If the Go process crashes unexpectedly:

1. Electron detects the child process exit (non-zero code).
2. Show error dialog: "Silo backend crashed. [Restart] [Quit]"
3. On restart: re-spawn Go process, re-establish connection.
4. Session state survives (persisted in SQLite).

---

## 7. V0 / V1 Scoping

### V0 Ships

| Feature | Details |
|---------|---------|
| Electron shell with Go sidecar | Core architecture |
| Chat view with streaming | Token-by-token display |
| Tool approval modal | Approve/deny with countdown |
| Session sidebar | List, switch, create, delete |
| Settings view | Provider, agent, security config |
| macOS + Windows + Linux packaging | Via electron-builder |
| Go binary bundled per-platform | Cross-compiled in CI |

### V1 Deferred

| Feature | Details |
|---------|---------|
| System tray | Minimize to tray, quick access menu, notification badges |
| Auto-update | `electron-updater` with GitHub Releases or custom update server |
| Themes | Light/dark/system, custom accent colors, syntax highlighting themes |
| Keyboard shortcuts | Customizable keybindings, command palette (Cmd+K) |
| Multi-window | Detach sessions into separate windows |
| File drag-and-drop | Drop files onto chat to ingest or reference |
| Memory browser | Visual interface for exploring stored knowledge |
| Tool output streaming | Stream tool stdout in real-time (not wait for completion) |
| Tray notifications | Desktop notifications for long-running tool completions |
| Local model support | UI for selecting and managing local LLM models |
| Wails/Tauri migration evaluation | Assess whether to reduce bundle size by dropping Electron |

---

## 8. Development Workflow

### Running Locally

```bash
# Terminal 1: Start Go backend
go run . start --serve --port 5110

# Terminal 2: Start Electron with hot-reload
cd desktop
SILO_DEV_PORT=5110 npm run dev    # vite dev server + electron
```

The `npm run dev` script checks `SILO_DEV_PORT` — if set, Electron skips spawning its own silo process and connects directly to the already-running one.

### Directory Structure

```
silo/
└── desktop/                # Electron app — separate build, ignored by go build
├── package.json
├── tsconfig.json
├── electron-builder.yml
├── vite.config.ts
├── src/
│   ├── main/
│   │   ├── index.ts          # Electron main entry
│   │   ├── sidecar.ts        # Spawn + manage silo binary
│   │   └── window.ts         # Window creation + management
│   ├── preload/
│   │   └── index.ts          # contextBridge API (port + token only)
│   ├── renderer/
│   │   ├── App.tsx            # Root React component
│   │   ├── index.html
│   │   ├── components/
│   │   │   ├── ChatView.tsx
│   │   │   ├── MessageBubble.tsx
│   │   │   ├── ToolCallBlock.tsx
│   │   │   ├── ApprovalModal.tsx
│   │   │   ├── SessionSidebar.tsx
│   │   │   ├── SettingsView.tsx
│   │   │   └── StatusBar.tsx
│   │   ├── hooks/
│   │   │   ├── useSSE.ts      # SSE connection + event parsing
│   │   │   ├── useSessions.ts
│   │   │   └── useConfig.ts
│   │   ├── stores/
│   │   │   ├── chatStore.ts   # Zustand store for chat state
│   │   │   └── appStore.ts    # Global app state
│   │   └── lib/
│   │       ├── api.ts         # HTTP client for Go backend
│   │       └── types.ts       # TypeScript types matching Go API
│   └── shared/
│       └── constants.ts       # Shared between main + renderer
└── resources/
    ├── sidecar/               # Pre-built silo binary placed here by `make desktop`
    │   └── silo               # (silo.exe on Windows)
    ├── icon.icns
    ├── icon.ico
    └── icons/                 # Linux icon sizes
```

> The `silo` repo has no `desktop/` directory and no Node.js dependencies.

---

## 9. Cross-References

| Spec | Interaction |
|------|-------------|
| [02_project_coding_guidelines.md](02_project_coding_guidelines.md) | TypeScript/React standards, Electron build tooling |
| [03_gateway.md](03_gateway.md) | HTTP + SSE protocol used for IPC. All endpoints identical. |
| [05_channels.md](05_channels.md) | Tool approval flow — desktop is one channel type |
| [07_security.md](07_security.md) | Bearer token auth, loopback binding, CSP |
| [08_cli.md](08_cli.md) | `--desktop-mode` flag, `--no-daemon` behavior |
| [09_configuration.md](09_configuration.md) | Settings view reads/writes config via Go API |
| [11_agent.md](11_agent.md) | SSE event types rendered in chat view |
