# ADK Integration Specification (V0)

This spec defines how Silo integrates Google's Agent Development Kit (ADK) as its agent loop runtime. ADK provides the ReAct loop, tool calling protocol, session management primitives, and model abstraction. Silo wraps ADK with its own security, approval, audit, and desktop layers — the parts ADK does not provide.

---

## 1. Why ADK

### What ADK Gives Us

ADK (`google.golang.org/adk`) provides a production-grade agent loop with:

- **ReAct loop** with configurable iteration limits
- **Tool calling protocol** — function calling with structured schemas
- **Callback hooks** — before/after model and tool calls
- **Session management** — pluggable session service interfaces
- **Memory service** — pluggable memory service interfaces
- **Multi-model support** — Gemini native, others via adapters
- **Streaming** — token-by-token streaming with event callbacks

### What ADK Does NOT Provide (Silo Must Build)

| Capability | Why ADK Lacks It | Silo Implementation |
|-----------|------------------|---------------------|
| Encrypted vault | ADK is cloud-native, assumes secret managers | XChaCha20-Poly1305 + Argon2id local vault |
| Process/WASM sandboxing | ADK trusts tool execution environment | wazero sandbox with fuel metering |
| Tool approval protocol | ADK assumes automated tool execution | Human-in-the-loop approval via SSE + modal |
| Shell allowlists | ADK does not restrict shell commands | Allowlist/blocklist in config, validated pre-execution |
| Audit trail | ADK has basic logging, not forensic audit | Signed event trail, tamper detection |
| Desktop app | ADK is a library, not an application | Electron shell with Go sidecar |
| CLI | ADK has no CLI surface | cobra-based CLI with TUI |
| Configuration system | ADK uses code-based config | TOML hot-reload, layered resolution |
| Gateway / HTTP server | ADK agents are in-process | Axum-equivalent HTTP + SSE server |
| Bearer token auth | ADK has no auth layer | Gateway perimeter auth |
| Cost tracking | ADK surfaces token counts, does not track costs | SQLite usage tracker with per-model pricing |

### Dependency

```go
// go.mod
require (
    google.golang.org/adk v0.2.0  // pin specific version
)
```

Pin a specific version. Do not use `latest`. ADK is pre-1.0 and may have breaking changes. Update the pin deliberately after testing.

---

## 2. Model Configuration

### Gemini (Native)

ADK supports Gemini models natively through its built-in `google.genai` integration:

```go
import (
    "google.golang.org/adk"
    "google.golang.org/adk/models/gemini"
)

func newGeminiAgent(cfg *config.ProviderConfig) (*adk.Agent, error) {
    model := gemini.NewModel(cfg.Model) // e.g., "gemini-2.0-flash"

    agent := adk.NewAgent(
        adk.WithName("silo"),
        adk.WithModel(model),
        adk.WithDescription("Silo local-first AI assistant"),
        adk.WithInstruction(loadSystemPrompt(cfg)),
    )

    return agent, nil
}
```

### OpenAI / Anthropic via LiteLLM

For non-Gemini models, ADK supports OpenAI-compatible endpoints. Silo routes through LiteLLM (a local proxy that translates between model APIs):

```go
import (
    "google.golang.org/adk"
    "google.golang.org/adk/models/litellm"
)

func newOpenAIAgent(cfg *config.ProviderConfig) (*adk.Agent, error) {
    // LiteLLM proxy running locally or configure OpenAI-compatible endpoint
    model := litellm.NewModel(
        cfg.Model,                          // e.g., "openai/gpt-4o"
        litellm.WithBaseURL(cfg.BaseURL),   // e.g., "http://localhost:4000"
        litellm.WithAPIKey(cfg.APIKey),
    )

    agent := adk.NewAgent(
        adk.WithName("silo"),
        adk.WithModel(model),
        adk.WithDescription("Silo local-first AI assistant"),
        adk.WithInstruction(loadSystemPrompt(cfg)),
    )

    return agent, nil
}
```

### Provider Factory

The provider factory creates the appropriate ADK agent based on configuration:

```go
func NewAgent(cfg *config.AppConfig, vault SecretVault) (*adk.Agent, error) {
    providerCfg := cfg.Providers.Active()

    switch providerCfg.Provider {
    case "google":
        apiKey, err := vault.GetSecret("google_api_key")
        if err != nil {
            return nil, fmt.Errorf("google API key not found in vault: %w", err)
        }
        providerCfg.APIKey = apiKey
        return newGeminiAgent(providerCfg)

    case "openai", "anthropic":
        apiKey, err := vault.GetSecret(providerCfg.Provider + "_api_key")
        if err != nil {
            return nil, fmt.Errorf("%s API key not found in vault: %w", providerCfg.Provider, err)
        }
        providerCfg.APIKey = apiKey
        return newOpenAIAgent(providerCfg)

    case "custom_proxy":
        return newOpenAIAgent(providerCfg) // custom proxy is OpenAI-compatible

    default:
        return nil, fmt.Errorf("unknown provider: %s", providerCfg.Provider)
    }
}
```

---

## 3. Agent Setup

### Full Wiring Example

```go
package brain

import (
    "context"
    "fmt"

    "google.golang.org/adk"
    "google.golang.org/adk/session"
    "google.golang.org/adk/memory"
    "google.golang.org/adk/tool"

    "github.com/siloframework/silo/internal/config"
    "github.com/siloframework/silo/internal/muscle"
    "github.com/siloframework/silo/internal/vault"
    "github.com/siloframework/silo/internal/usage"
)

type SiloBrain struct {
    agent          *adk.Agent
    sessionService session.Service
    memoryService  memory.Service
    approver       *ToolApprover
    usageTracker   usage.Tracker
    config         *config.ConfigHolder
}

func NewSiloBrain(
    cfg *config.ConfigHolder,
    vaultSvc vault.SecretVault,
    sessionSvc session.Service,
    memorySvc memory.Service,
    tools []tool.Tool,
    usageTracker usage.Tracker,
) (*SiloBrain, error) {
    appCfg := cfg.Load()

    agent, err := NewAgent(appCfg, vaultSvc)
    if err != nil {
        return nil, fmt.Errorf("create agent: %w", err)
    }

    approver := NewToolApprover(appCfg)

    // Register tools with the agent
    for _, t := range tools {
        agent.AddTool(t)
    }

    // Wire callback hooks
    agent.SetBeforeToolCallback(approver.BeforeToolCallback)
    agent.SetAfterToolCallback(newAuditLogger().AfterToolCallback)
    agent.SetBeforeModelCallback(newSessionLoader(sessionSvc, memorySvc).BeforeModelCallback)
    agent.SetAfterModelCallback(newUsageExtractor(usageTracker).AfterModelCallback)

    return &SiloBrain{
        agent:          agent,
        sessionService: sessionSvc,
        memoryService:  memorySvc,
        approver:       approver,
        usageTracker:   usageTracker,
        config:         cfg,
    }, nil
}
```

### Tool Registration

Silo's tools (bash, file read, file write, web fetch) are registered as ADK tools:

```go
import "google.golang.org/adk/tool"

func newBashTool(cfg *config.ToolsConfig) tool.Tool {
    return tool.NewTool(
        tool.WithName("bash"),
        tool.WithDescription("Execute a shell command"),
        tool.WithParameters(map[string]tool.Parameter{
            "cmd": {
                Type:        "string",
                Description: "The shell command to execute",
                Required:    true,
            },
        }),
        tool.WithHandler(func(ctx context.Context, args map[string]any) (string, error) {
            cmd, ok := args["cmd"].(string)
            if !ok {
                return "", fmt.Errorf("cmd must be a string")
            }
            return executeBash(ctx, cmd, cfg)
        }),
    )
}
```

---

## 4. Callback Hooks

ADK provides four callback injection points in its agent loop. Silo uses all four to layer its security, approval, audit, and tracking concerns.

### 4.1 BeforeToolCallback: Approval and Allowlist

Called before every tool execution. This is where Silo implements its human-in-the-loop approval protocol and shell allowlists.

```go
type ToolApprover struct {
    config    *config.ConfigHolder
    pending   sync.Map // map[string]chan bool — pending approval channels
    eventSink chan<- SseEvent
}

func (a *ToolApprover) BeforeToolCallback(
    ctx context.Context,
    toolName string,
    args map[string]any,
) (bool, error) {
    cfg := a.config.Load()

    // 1. Shell allowlist check (for bash tool)
    if toolName == "bash" {
        cmd, _ := args["cmd"].(string)
        if !isCommandAllowed(cmd, cfg.Tools.Shell.Allowlist, cfg.Tools.Shell.Blocklist) {
            a.eventSink <- SseEvent{
                Type: "tool_denied",
                Data: ToolDeniedData{
                    Tool:   toolName,
                    Reason: fmt.Sprintf("command %q blocked by shell allowlist", cmd),
                },
            }
            return false, nil // block execution, do not error
        }
    }

    // 2. Check approval mode
    mode := getApprovalMode(toolName, cfg.Tools.Approval)

    switch mode {
    case "never":
        // Auto-approve
        a.eventSink <- SseEvent{Type: "tool_call", Data: ToolCallData{Tool: toolName, Args: args}}
        return true, nil

    case "always":
        // Request human approval
        callID := uuid.NewString()
        approvalCh := make(chan bool, 1)
        a.pending.Store(callID, approvalCh)
        defer a.pending.Delete(callID)

        // Emit pending event — Electron/CLI shows approval modal
        a.eventSink <- SseEvent{
            Type: "tool_pending",
            Data: ToolPendingData{CallID: callID, Tool: toolName, Args: args},
        }

        // Wait for approval with timeout
        timeout := time.Duration(cfg.Tools.Approval.Timeout) * time.Second
        select {
        case approved := <-approvalCh:
            if approved {
                return true, nil
            }
            a.eventSink <- SseEvent{
                Type: "tool_denied",
                Data: ToolDeniedData{CallID: callID, Tool: toolName, Reason: "denied by user"},
            }
            return false, nil
        case <-time.After(timeout):
            a.eventSink <- SseEvent{
                Type: "tool_denied",
                Data: ToolDeniedData{CallID: callID, Tool: toolName, Reason: "approval timeout"},
            }
            return false, nil
        case <-ctx.Done():
            return false, ctx.Err()
        }

    default:
        return true, nil
    }
}

// Called by the gateway when POST /silo/brain/tool-approval is received
func (a *ToolApprover) ResolveApproval(callID string, approved bool) error {
    val, ok := a.pending.Load(callID)
    if !ok {
        return fmt.Errorf("unknown call_id: %s", callID)
    }
    ch := val.(chan bool)
    ch <- approved
    return nil
}
```

### 4.2 AfterToolCallback: Audit Logging

Called after every tool execution. Records the tool invocation, arguments, result, and duration for the audit trail.

```go
type AuditLogger struct {
    logger *zap.Logger
}

func (a *AuditLogger) AfterToolCallback(
    ctx context.Context,
    toolName string,
    args map[string]any,
    result string,
    err error,
    duration time.Duration,
) {
    success := err == nil
    output := result
    if err != nil {
        output = err.Error()
    }

    // Structured audit log entry
    a.logger.Info("tool_executed",
        zap.String("tool", toolName),
        zap.Any("args", redactSensitive(args)),
        zap.Bool("success", success),
        zap.Int("output_bytes", len(output)),
        zap.Duration("duration", duration),
        zap.String("request_id", requestIDFromCtx(ctx)),
        zap.String("session_id", sessionIDFromCtx(ctx)),
    )

    // Emit SSE event
    // (eventSink is accessed via context or struct field)
}
```

### 4.3 BeforeModelCallback: Session History Loading

Called before every LLM invocation. Loads session history and memory recall into the context.

```go
type SessionLoader struct {
    sessionService session.Service
    memoryService  memory.Service
}

func (s *SessionLoader) BeforeModelCallback(
    ctx context.Context,
    messages []adk.Message,
) ([]adk.Message, error) {
    sessionID := sessionIDFromCtx(ctx)
    if sessionID == "" {
        return messages, nil
    }

    // 1. Load session history (sliding window based on token budget)
    history, err := s.sessionService.GetMessages(ctx, sessionID)
    if err != nil {
        // Graceful degradation — continue without history
        zap.L().Warn("failed to load session history",
            zap.String("session_id", sessionID),
            zap.Error(err),
        )
        return messages, nil
    }

    // 2. Memory recall — search for relevant knowledge
    query := extractQueryFromMessages(messages)
    memories, err := s.memoryService.Search(ctx, query)
    if err != nil {
        // Graceful degradation — continue without memory
        zap.L().Warn("memory recall failed", zap.Error(err))
        memories = nil
    }

    // 3. Assemble context: system prompt + memory + history + current message
    assembled := assembleContext(history, memories, messages)
    return assembled, nil
}
```

### 4.4 AfterModelCallback: Token Usage Extraction

Called after every LLM response. Extracts token counts and records usage for cost tracking.

```go
type UsageExtractor struct {
    tracker usage.Tracker
}

func (u *UsageExtractor) AfterModelCallback(
    ctx context.Context,
    response *adk.ModelResponse,
) error {
    if response == nil || response.Usage == nil {
        return nil
    }

    record := usage.Record{
        SessionID:    sessionIDFromCtx(ctx),
        RequestID:    requestIDFromCtx(ctx),
        Provider:     response.Provider,
        Model:        response.Model,
        InputTokens:  response.Usage.InputTokens,
        OutputTokens: response.Usage.OutputTokens,
        TotalTokens:  response.Usage.TotalTokens,
        Timestamp:    time.Now(),
    }

    // Calculate cost based on per-model pricing table
    record.Cost = calculateCost(record.Model, record.InputTokens, record.OutputTokens)

    if err := u.tracker.Record(ctx, record); err != nil {
        zap.L().Warn("failed to record usage",
            zap.Error(err),
        )
        // Non-fatal — do not fail the agent loop over usage tracking
    }

    return nil
}
```

---

## 5. Session Adapter

### ADK's DatabaseSessionService

ADK provides a `DatabaseSessionService` interface. Silo implements it backed by SQLite at `~/.silo/sessions.db`:

```go
import (
    "google.golang.org/adk/session"
    "github.com/jmoiron/sqlx"
    _ "modernc.org/sqlite"
)

// SiloSessionService implements adk's session.Service interface
type SiloSessionService struct {
    db *sqlx.DB
}

func NewSiloSessionService(dbPath string) (*SiloSessionService, error) {
    db, err := sqlx.Open("sqlite", dbPath)
    if err != nil {
        return nil, fmt.Errorf("open sessions db: %w", err)
    }

    // WAL mode, foreign keys, busy timeout
    pragmas := []string{
        "PRAGMA journal_mode=WAL",
        "PRAGMA foreign_keys=ON",
        "PRAGMA busy_timeout=5000",
    }
    for _, p := range pragmas {
        if _, err := db.Exec(p); err != nil {
            return nil, fmt.Errorf("pragma %q: %w", p, err)
        }
    }

    if err := runMigrations(db); err != nil {
        return nil, fmt.Errorf("migrations: %w", err)
    }

    return &SiloSessionService{db: db}, nil
}

// Implements session.Service
func (s *SiloSessionService) GetSession(ctx context.Context, id string) (*session.Session, error) {
    // SELECT from sessions table, load messages
    // ...
}

func (s *SiloSessionService) CreateSession(ctx context.Context) (*session.Session, error) {
    // INSERT new session with UUID
    // ...
}

func (s *SiloSessionService) SaveSession(ctx context.Context, sess *session.Session) error {
    // Upsert session + messages in transaction
    // ...
}

func (s *SiloSessionService) DeleteSession(ctx context.Context, id string) error {
    // DELETE cascade
    // ...
}

func (s *SiloSessionService) ListSessions(ctx context.Context) ([]*session.Session, error) {
    // SELECT with ordering and pagination
    // ...
}
```

### Schema

Same schema as defined in [06_sessions.md](06_sessions.md), adapted for ADK's session structure:

```sql
CREATE TABLE IF NOT EXISTS sessions (
    id             TEXT PRIMARY KEY,
    created_at     TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at     TEXT NOT NULL DEFAULT (datetime('now')),
    status         TEXT NOT NULL DEFAULT 'active',  -- active, expired, deleted
    message_count  INTEGER NOT NULL DEFAULT 0,
    metadata       TEXT  -- JSON blob for ADK session metadata
);

CREATE TABLE IF NOT EXISTS messages (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id  TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    role        TEXT NOT NULL,  -- user, assistant, tool, system
    content     TEXT NOT NULL,
    tool_calls  TEXT,           -- JSON array of tool calls (nullable)
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    token_count INTEGER
);

CREATE INDEX idx_messages_session ON messages(session_id, id);
```

---

## 6. Memory Adapter

### V0: InMemoryMemoryService

For V0, Silo uses ADK's built-in `InMemoryMemoryService` as a starting point, supplemented by Silo's own SQLite-backed search:

```go
import "google.golang.org/adk/memory"

// V0: Use ADK's in-memory service for the ADK integration layer,
// but Silo's own MemoryStore (SQLite + FTS5) handles persistence.
// The BeforeModelCallback injects memory recall results into context.
func newMemoryService() memory.Service {
    return memory.NewInMemoryService()
}
```

In V0, memory recall is handled in the `BeforeModelCallback` (section 4.3) using Silo's own `internal/memory` package (SQLite + FTS5 + cosine similarity). ADK's memory service is used only to satisfy the interface contract.

### V1: Custom MemoryService with FTS5 + Vector

In V1, Silo will implement a full ADK `memory.Service` that wraps the SQLite memory store:

```go
// V1: Full ADK memory service backed by Silo's memory store
type SiloMemoryService struct {
    store *memory.SqliteMemoryStore // Silo's internal/memory package
}

func (m *SiloMemoryService) Search(ctx context.Context, query string) ([]memory.Entry, error) {
    // Delegates to Silo's hybrid search: FTS5/BM25 + cosine similarity + RRF
    results, err := m.store.Search(ctx, query, SearchOptions{
        TopK:       10,
        MinScore:   0.3,
        EntryTypes: []string{"fact", "preference", "chunk"},
    })
    if err != nil {
        return nil, err
    }

    // Convert Silo results to ADK memory entries
    entries := make([]memory.Entry, len(results))
    for i, r := range results {
        entries[i] = memory.Entry{
            Content:  r.Content,
            Score:    r.Score,
            Metadata: r.Metadata,
        }
    }
    return entries, nil
}

func (m *SiloMemoryService) Store(ctx context.Context, entry memory.Entry) error {
    // Delegates to Silo's store: chunk, embed, insert
    return m.store.Store(ctx, entry.Content, entry.Metadata)
}
```

---

## 7. Running the Agent

### Chat Flow

```go
func (b *SiloBrain) Chat(ctx context.Context, req ChatRequest) (<-chan SseEvent, error) {
    eventCh := make(chan SseEvent, 64)

    go func() {
        defer close(eventCh)

        // Create or resume ADK session
        var sess *session.Session
        var err error
        if req.SessionID != "" {
            sess, err = b.sessionService.GetSession(ctx, req.SessionID)
        }
        if sess == nil {
            sess, err = b.sessionService.CreateSession(ctx)
        }
        if err != nil {
            eventCh <- SseEvent{Type: "error", Data: ErrorData{Message: err.Error()}}
            return
        }

        eventCh <- SseEvent{
            Type: "session",
            Data: SessionData{SessionID: sess.ID, Resumed: req.SessionID != ""},
        }

        // Inject session ID into context for callbacks
        ctx = withSessionID(ctx, sess.ID)

        // Set the event sink for the tool approver
        b.approver.eventSink = eventCh

        // Run the ADK agent — this drives the ReAct loop
        runner := adk.NewRunner(b.agent, b.sessionService)

        // Stream events from ADK
        stream := runner.RunStream(ctx, sess.ID, req.Message)
        for event := range stream {
            switch e := event.(type) {
            case *adk.TextEvent:
                eventCh <- SseEvent{Type: "token", Data: TokenData{Content: e.Text}}
            case *adk.ToolCallEvent:
                eventCh <- SseEvent{Type: "tool_call", Data: ToolCallData{
                    CallID: e.ID, Tool: e.Name, Args: e.Args,
                }}
            case *adk.ToolResultEvent:
                eventCh <- SseEvent{Type: "tool_result", Data: ToolResultData{
                    CallID: e.ID, Tool: e.Name, Success: e.Error == nil, Output: e.Result,
                }}
            case *adk.ErrorEvent:
                eventCh <- SseEvent{Type: "error", Data: ErrorData{Message: e.Error.Error()}}
            case *adk.DoneEvent:
                eventCh <- SseEvent{Type: "done"}
            }
        }

        // Save session state
        _ = b.sessionService.SaveSession(ctx, sess)
    }()

    return eventCh, nil
}
```

### Configuration Mapping

ADK agent configuration maps to `silo.toml` keys:

| silo.toml Key | ADK Equivalent | Description |
|---------------|----------------|-------------|
| `agent.max_iterations` | `adk.WithMaxIterations(n)` | ReAct loop iteration limit |
| `agent.system_prompt_path` | `adk.WithInstruction(text)` | System prompt content |
| `agent.tool_parallel` | `adk.WithParallelTools(bool)` | Parallel tool execution |
| `providers.default` | `adk.WithModel(model)` | Which model to use |
| `tools.approval.mode` | `BeforeToolCallback` | Approval logic (Silo-side) |
| `tools.approval.timeout` | `BeforeToolCallback` | Approval timeout (Silo-side) |

---

## 8. ADK Boundary: What Silo Wraps vs. Delegates

### Delegates to ADK (Use Directly)

| Capability | ADK Component | Notes |
|-----------|---------------|-------|
| ReAct loop | `adk.Runner.Run()` / `RunStream()` | Core agent loop |
| Tool schema definitions | `adk/tool.Tool` | JSON schema for function calling |
| Model abstraction | `adk/models/*` | Gemini native, LiteLLM for others |
| Streaming | `adk.Runner.RunStream()` | Token-by-token event stream |
| Session interface | `adk/session.Service` | Silo implements with SQLite |
| Memory interface | `adk/memory.Service` | V0: in-memory, V1: custom |

### Wraps ADK (Silo Adds Its Layer)

| Capability | Silo Layer | How |
|-----------|------------|-----|
| Tool approval | `BeforeToolCallback` | Intercepts tool calls, requires human approval |
| Shell allowlists | `BeforeToolCallback` | Validates bash commands against allowlist |
| Audit trail | `AfterToolCallback` | Logs every tool execution with structured data |
| Session history | `BeforeModelCallback` | Loads SQLite history into context |
| Memory recall | `BeforeModelCallback` | Injects FTS5+vector search results |
| Usage tracking | `AfterModelCallback` | Extracts token counts, calculates cost |
| Encrypted secrets | Vault (outside ADK) | API keys decrypted from vault, passed to ADK |
| WASM sandbox | Muscle (outside ADK) | Tool handlers execute inside wazero sandbox |

### Completely Independent of ADK (Silo-Only)

| Capability | Location | Notes |
|-----------|----------|-------|
| Gateway HTTP server | `internal/gateway/` | Axum-equivalent, SSE streaming |
| Bearer token auth | `internal/gateway/middleware/` | Gateway perimeter security |
| Configuration system | `internal/config/` | TOML hot-reload, layered resolution |
| CLI | `internal/cli/` | cobra subcommands, TUI |
| Desktop app | `desktop/` | Electron shell, Go sidecar |
| Encrypted vault | `internal/vault/` | XChaCha20-Poly1305 + Argon2id |
| WASM sandbox | `internal/muscle/sandbox/` | wazero with fuel metering |
| Cost tracking DB | `internal/usage/` | SQLite usage tables |

---

## 9. Fallback: Running Without ADK

If ADK proves unsuitable (breaking API changes, licensing issues, performance problems), Silo can fall back to a custom ReAct loop. The callback hooks map directly to function calls in a hand-written loop:

```go
// Custom loop equivalent (no ADK dependency)
func (b *SiloBrain) customReactLoop(ctx context.Context, req ChatRequest, eventCh chan<- SseEvent) {
    for iteration := 0; iteration < b.config.Load().Agent.MaxIterations; iteration++ {
        // BeforeModelCallback equivalent
        messages := b.loadContext(ctx, req.SessionID)

        // Call LLM directly
        stream, err := b.provider.Chat(ctx, messages, b.tools)
        if err != nil { /* retry logic */ }

        // Stream tokens
        var toolCalls []ToolCall
        for chunk := range stream {
            if chunk.IsText() {
                eventCh <- SseEvent{Type: "token", Data: TokenData{Content: chunk.Text}}
            } else {
                toolCalls = append(toolCalls, chunk.ToolCall)
            }
        }

        // AfterModelCallback equivalent
        b.recordUsage(ctx, stream.Usage())

        if len(toolCalls) == 0 {
            eventCh <- SseEvent{Type: "done"}
            return
        }

        // BeforeToolCallback equivalent
        for _, tc := range toolCalls {
            if !b.approver.Check(ctx, tc) { continue }
            result := b.muscle.Execute(ctx, tc.Name, tc.Args)
            // AfterToolCallback equivalent
            b.auditLog(ctx, tc, result)
        }
    }
}
```

This fallback is kept as a design reference, not shipped code. The ADK integration is the primary path.

---

## 10. Testing Strategy

### Unit Tests

- **Callback hooks:** Test each callback in isolation with mock ADK events.
- **Tool approver:** Test allowlist matching, approval flow (approve, deny, timeout).
- **Usage extractor:** Test cost calculation with known token counts.
- **Session adapter:** Test SQLite CRUD operations.

### Integration Tests

- **Full agent loop:** Start ADK agent with mock LLM, verify SSE event sequence.
- **Tool approval flow:** Spawn agent, trigger tool call, approve via API, verify execution.
- **Session persistence:** Chat, close, resume — verify history is preserved.

### ADK Version Pinning Tests

CI runs tests against the pinned ADK version. A separate job runs against `latest` to detect upcoming breakage early (allowed to fail).

---

## 11. Cross-References

| Spec | Interaction |
|------|-------------|
| [02_project_coding_guidelines.md](02_project_coding_guidelines.md) | Go conventions, dependency policy |
| [03_gateway.md](03_gateway.md) | SSE event types consumed by the gateway from ADK events |
| [05_channels.md](05_channels.md) | Tool approval protocol — ADK's BeforeToolCallback implements it |
| [06_sessions.md](06_sessions.md) | Session schema used by the ADK session adapter |
| [07_security.md](07_security.md) | Shell allowlists, WASM sandbox — enforced in BeforeToolCallback and tool handlers |
| [09_configuration.md](09_configuration.md) | Config keys mapped to ADK agent options |
| [10_memory.md](10_memory.md) | Memory store used in BeforeModelCallback for context injection |
| [11_agent.md](11_agent.md) | ReAct loop architecture — ADK replaces custom Rust implementation |
| [12_logging.md](12_logging.md) | Audit logging in AfterToolCallback |
| [13_desktop_app.md](13_desktop_app.md) | Desktop app consumes the same SSE events ADK produces |
