# Gateway Specification (V0)

The gateway is Silo's HTTP entry point for headless mode — external adapters (Telegram, Slack, web UI, Python scripts) communicate with Silo through the gateway. In V0, the gateway is **headless-mode only**. Desktop and CLI modes use the Go core in-process (no HTTP hop).

---

## 1. Framework & Runtime

- **`gin` (`github.com/gin-gonic/gin`)** — high-performance HTTP framework with built-in routing, middleware, and parameter binding.
- The gateway runs as an HTTP server only in **headless mode** (`silo start --headless`). Desktop and CLI modes embed the Go core directly and do not start the HTTP server.
- Gin provides composable middleware, route grouping, path parameter extraction, and JSON binding out of the box.

---

## 2. Binding & Port

- **Default:** `127.0.0.1:5110` (S-I-L-O → 5-1-1-0)
- Fully configurable in `silo.toml`: host, port.
- **Unix socket mode** (`~/.silo/silo.sock`) for paranoid/edge/Pi deployments — can disable TCP entirely. Uses Go `net.Listen("unix", path)`.

---

## 3. Two API Surfaces

### OpenAI-Compatible (Ecosystem Interop)

Drop-in compatibility so any tool that speaks the OpenAI API can talk to Silo.

| Method | Path                      | Description                          |
|--------|---------------------------|--------------------------------------|
| POST   | `/v1/chat/completions`    | SSE streaming, OpenAI request/response format |

### Custom Silo API (Full Primitive Access)

Grouped by body part (Brain, Muscle, Vault) for clarity.

| Method | Path                      | Description                          |
|--------|---------------------------|--------------------------------------|
| POST   | `/silo/brain/chat`        | Send message to agent (SSE streaming with rich events). Accepts optional `session_id` to resume a conversation (see 05_channels.md §4). |
| POST   | `/silo/brain/tool-approval` | Approve or deny a pending tool call (see 05_channels.md §3) |
| POST   | `/silo/muscle/execute`    | Execute a tool                       |
| GET    | `/silo/muscle/tools`      | List available tools                 |
| GET    | `/silo/vault/sessions`              | List sessions (with filters)         |
| GET    | `/silo/vault/sessions/{session_id}` | Get session metadata                 |
| DELETE | `/silo/vault/sessions/{session_id}` | Delete session and all data          |
| GET    | `/silo/health`            | Health check                         |
| GET    | `/silo/status`            | Running info, uptime, mode           |

**V0 note:** Memory endpoints (`/silo/vault/memory/*`, `/silo/vault/store`, `/silo/vault/query`) are deferred to V1 since V0 uses ADK's `InMemoryMemoryService`.

---

## 4. SSE Streaming

### `/v1/chat/completions`

Standard OpenAI streaming format (`data: {...}\n\n` with `[DONE]` sentinel).

### `/silo/brain/chat`

Rich custom event types that let frontends build UIs showing the agent loop in real-time:

| Event Type     | Payload Description                |
|----------------|------------------------------------|
| `event: token`       | LLM output chunk                   |
| `event: tool_call`   | Brain wants to run a tool          |
| `event: tool_pending` | Tool awaiting human approval (includes `call_id`, tool, args) |
| `event: tool_result` | Tool finished executing            |
| `event: tool_denied` | Tool was denied by user            |
| `event: session`     | Session metadata (session_id, resumed, message_count) — emitted first |
| `event: error`       | Something failed                   |
| `event: done`        | Stream complete                    |

### Go SSE Implementation

SSE streaming uses Go's `http.Flusher` interface:

```go
func handleChatSSE(w http.ResponseWriter, r *http.Request) {
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "SSE not supported", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    for event := range eventChan {
        fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, event.Data)
        flusher.Flush()
    }
}
```

---

## 5. Internal Routing

### Interface Dispatch

The gateway holds references to shared services via Go interfaces on an `AppState` struct:

```go
type AppState struct {
    Agent    *core.SiloAgent    // ADK agent wrapper
    Vault    vault.SecretVault  // Encrypted secret storage
    Config   *config.AppConfig  // Static config (restart to apply)
    Sessions adk.SessionService // ADK session service
}
```

Route handlers call interface methods directly — no serialization overhead between internal components.

### Event Bus

A Go channel-based pub/sub publishes events for telemetry and logging:

```go
type EventBus struct {
    subscribers []chan Event
    mu          sync.RWMutex
}
```

Subscribers receive events without blocking the hot path.

---

## 6. Mode Handling

The gateway exposes the **same API surface** regardless of who's calling. It doesn't know or care about the client.

| Config                         | Mode     | Behavior                                                                 |
|--------------------------------|----------|--------------------------------------------------------------------------|
| `brain_enabled = true`         | Active   | `/silo/brain/chat` uses the ADK agent loop                              |
| `brain_enabled = false`        | Headless | `/silo/brain/*` returns `400 Bad Request: Internal Brain Disabled`      |

In Headless mode, external code (Python, Go, whatever) calls `/silo/muscle/*` directly to use Silo as a local execution daemon.

**V0 scope:** The gateway is only active in headless mode (`silo start --headless`). Desktop and CLI use the Go core directly in-process.

---

## 7. Security Middleware Pipeline (V0)

All layers are toggleable in `silo.toml`. Applied in order via gin middleware chain:

```
Request
  │
  ├─ 1. CORS                        ← required for frontend dev
  ├─ 2. Request Size Limit           ← 10 MB default, prevents OOM on Pi part of config
  ├─ 3. Token Authentication         ← Bearer token, mandatory
  ├─ 4. Request Validation           ← Go struct validation
  └─ 5. Route Handler                ← Brain / Vault / Muscle dispatch
```

```go
r := gin.New()
r.Use(middleware.CORS())
r.Use(middleware.RequestSizeLimit(10 * 1024 * 1024))
r.Use(middleware.BearerAuth(vault))
r.Use(middleware.RequestID())
// ... route handlers
```

See [07_security.md](07_security.md) for the complete security model that this middleware pipeline is part of (Layer 1: Gateway Perimeter).

### Deferred to V1

- IP allowlist/blocklist
- Rate limiting (per-IP)
- Credential stripping from prompts
- Audit logging (to DB / OTEL)

See [12_logging.md](12_logging.md) for the unified logging specification. The gateway generates a `request_id` (format: `req_` + 16 hex chars) in middleware for every incoming request and returns it in the `X-Request-Id` response header.

---

## 8. Token Authentication

- **Auto-generated** on first run, stored in the encrypted vault (never plaintext on disk).
- Aligns with the "no env business" philosophy — no `.env` files, no shell exports.

### CLI Commands (V1)

| Command              | Description              |
|----------------------|--------------------------|
| `silo token show`    | Retrieve current token   |
| `silo token rotate`  | Regenerate token         |

In V0, the token is generated during `silo init` and stored in the vault.

---

## 9. Tool Execution

### V0 — Simple Direct Execution

```
POST /silo/muscle/execute
{
  "tool": "bash",
  "args": { "cmd": "ls" }
}
```

Gateway validates the request against the tool allowlist, then delegates to the shell executor.

### V1 — JIT Capability-Request Flow

Two-step pattern: request capability → receive scoped token → execute with token.

---

## 10. Error Format

All endpoints return errors in **RFC 7807 Problem Details** format: we use uber's zap logger to log

```json
{
  "type": "silo/auth-failed",
  "title": "Authentication Failed",
  "status": 401,
  "detail": "Invalid or missing Bearer token"
}
```

Standard error format compatible with OTEL, Grafana, Datadog for later tracing.

---

## 11. Reverse Proxy

- **Nginx config auto-generated by default** (`silo.toml: nginx_config = true`).
- Users can swap in Caddy, Traefik, or anything else — Silo just listens on its port/socket.
- Philosophy: best default out of the box, everything swappable.

---

## 12. Lifecycle Management

### PID File Lock (V1)

`~/.silo/silo.pid` — prevents duplicate instances. V1 feature (daemon mode).

### Graceful Shutdown

Uses Go's `context.Context` and `http.Server.Shutdown()`:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
server.Shutdown(ctx)
```

Drain in-flight requests (especially SSE streams), close DB connections, save state.

### CLI Commands (V1)

| Command          | Description                |
|------------------|----------------------------|
| `silo status`    | Show running state         |
| `silo stop`      | Graceful shutdown          |
| `silo restart`   | Stop + start               |

V0: Desktop and CLI run foreground. `Ctrl-C` to stop.

---

## 13. Gateway Configuration in `silo.toml`

```toml
[gateway]
host = "127.0.0.1"
port = 5110
brain_enabled = true
max_request_size = "10mb"
cors_origins = ["*"]
nginx_config = true           # auto-generate nginx conf

[gateway.auth]
enabled = true

[gateway.timeouts]
read = "30s"
write = "60s"
idle = "120s"
```

---

## 14. V0/V1 Scoping Summary

### V0 Ships

| Feature | Section |
|---------|---------|
| Gin HTTP server with middleware pipeline | §1, §7 |
| Bearer token auth | §8 |
| SSE streaming via http.Flusher | §4 |
| `/silo/brain/chat` and `/silo/brain/tool-approval` | §3 |
| `/silo/muscle/execute` and `/silo/muscle/tools` | §3 |
| Session endpoints (`/silo/vault/sessions/*`) | §3 |
| Health and status endpoints | §3 |
| RFC 7807 error format | §10 |
| Request ID generation and correlation | §7 |
| CORS, size limits, schema validation middleware | §7 |

### V1 Deferred

| Feature | Section |
|---------|---------|
| Memory API endpoints | §3 |
| Daemon mode (PID file, fork) | §12 |
| IP allowlist/blocklist | §7 |
| Rate limiting | §7 |
| TLS termination | §13 |
| Unix socket mode | §2 |
| OpenAI-compatible endpoint | §3 |
| Lifecycle CLI commands (stop, restart, status) | §12 |

---

## 15. Cross-References

| Spec | Interaction |
|------|-------------|
| [05_channels.md](05_channels.md) | Tool approval flow, adapter contract, SSE event types |
| [07_security.md](07_security.md) | Layer 1 Gateway Perimeter — middleware pipeline |
| [11_agent.md](11_agent.md) | ADK agent wiring, SSE event mapping |
| [12_logging.md](12_logging.md) | Request correlation IDs, structured logging |
| [14_adk_integration.md](14_adk_integration.md) | ADK session and agent setup |
