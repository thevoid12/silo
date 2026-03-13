# Project Coding Guidelines

This document defines coding standards, project layout, naming conventions, and engineering practices for the Silo project. Silo is implemented in Go with an Electron desktop frontend. These guidelines are mandatory for all contributors and enforced by CI.

---

## 1. Language Choice: Go

Silo V2 is written in Go. The rationale:

- **Single static binary**: `CGO_ENABLED=0` produces a zero-dependency binary that runs on Raspberry Pi to cloud servers.
- **Concurrency primitives**: Goroutines and channels are a natural fit for SSE streaming, tool execution, and the ReAct loop.
- **Fast compilation**: Sub-second incremental builds keep the feedback loop tight.
- **Stdlib richness**: `net/http`, `crypto/*`, `encoding/json`, `database/sql` cover 80% of needs without external dependencies.
- **Cross-compilation**: `GOOS=linux GOARCH=arm64 go build` — no toolchain setup.

The Electron desktop app is TypeScript/React but communicates with Go exclusively through a defined IPC contract (local HTTP + SSE). The two codebases are loosely coupled.

---

## 2. Project Layout

inside package we have each package 
and inside each package we will have a model folder for all the interfaces and structs
all request model from the ui needs to be end with Request, reponse should we end with Response validated and sanitized (use google's validator)
```
silo/
├── cmd/
│   └── silo/                   # main.go — single entry point
│       └── main.go
├── pkg/                   # Private packages (not importable externally)
│   ├── brain/                  # Agent loop, ReAct, context assembly
│   │   ├── brain.go            # Brain interface + DefaultBrain
│   │   ├── react.go            # ReAct loop implementation
│   │   ├── context.go          # Context assembly, token budgets
│   │   └── approval.go         # Tool approval protocol
│   ├── gateway/                # HTTP server, routes, middleware
│   │   ├── server.go           # Gin router setup
│   │   ├── routes.go           # Route registration
│   │   ├── middleware/         # Auth, CORS, request ID, size limit
│   │   └── sse.go              # SSE streaming helpers
│   ├── provider/               # LLM provider abstraction
│   │   ├── provider.go         # Provider interface
│   │   ├── openai.go           # OpenAI implementation
│   │   ├── anthropic.go        # Anthropic implementation
│   │   ├── google.go           # Google Gemini (via ADK)
│   │   └── proxy.go            # Custom OpenAI-compatible proxy
│   ├── muscle/                 # Tool execution and registry
│   │   ├── muscle.go           # Muscle interface + DefaultMuscle
│   │   ├── registry.go         # Tool registry
│   │   ├── bash.go             # BashTool implementation
│   │   └── sandbox/            # WASM sandbox (wazero)
│   ├── vault/                  # Encrypted secret storage
│   │   ├── vault.go            # SecretVault interface + implementation
│   │   └── crypto.go           # XChaCha20-Poly1305 + Argon2id
│   ├── session/                # Session persistence
│   │   ├── store.go            # SessionStore interface + SQLite impl
│   │   └── schema.go           # DDL, migrations
│   ├── memory/                 # Knowledge store
│   │   ├── store.go            # MemoryStore interface + SQLite impl
│   │   ├── search.go           # Hybrid search (FTS5 + vector + RRF)
│   │   ├── embeddings.go       # Embedding API client
│   │   └── ingestion.go        # Document chunking
│   ├── config/                 # Configuration loading + hot-reload
│   │   ├── config.go           # AppConfig struct, defaults, validation
│   │   └── watcher.go          # fsnotify-based hot-reload
│   ├── usage/                  # Token/cost tracking
│   │   └── tracker.go          # UsageTracker interface + SQLite impl
│   └── cli/                    # CLI command handlers
│       ├── start.go
│       ├── chat.go
│       ├── vault_cmd.go
│       ├── provider_cmd.go
│       └── ...
├── pkg/                        # Public interfaces (if any — keep minimal)
│   └── types/                  # Shared types for external consumers
│       ├── events.go           # SseEvent, ChatRequest, etc.
│       └── errors.go           # ProblemDetail (RFC 7807)
├── desktop/                    # Electron app (separate build)
│   ├── package.json
│   ├── src/
│   │   ├── main/               # Electron main process
│   │   ├── renderer/           # React app
│   │   └── preload/            # Preload scripts
│   └── electron-builder.yml
├── go.mod
├── go.sum
├── Makefile
├── .goreleaser.yml
└── spec/                       # This spec directory
```

### Layout Rules

1. **`cmd/silo/main.go`** is the only `package main`. It wires dependencies and calls into `pkg/`.
2. **`pkg/`** holds all business logic. Nothing in `pkg/` is importable by external modules — this is Go's built-in encapsulation.
3. **`pkg/`** is reserved for types that external adapters (Telegram bot, web UI) might import. Keep it minimal. If in doubt, put it in `pkg/`.
4. **No `util/` or `common/` packages.** Every package has a clear domain name.
5. **One package per domain concern.** Do not merge session + memory + vault into one package.

---

## 3. Module Path and Dependencies

### Module Path

```
module github.com/siloframework/silo
```

### Dependency Philosophy

**Minimize. Prefer stdlib. Zero-CGO.**

| Need | Choice | Rationale |
|------|--------|-----------|
| HTTP server | `github.com/gin-gonic/gin` | High-performance, built-in routing/middleware/binding |
| SQLite | `mattn/sqlite` + `github.com/jmoiron/sqlx` | Pure Go driver, sqlx for ergonomic query building |
| CLI | `cobra` + `pflag` | Industry standard, subcommand tree support |
| Config | `koanf` or `viper` | TOML loading, env override, hot-reload hooks |
| TOML | `github.com/BurntSushi/toml` | Fast, well-maintained |
| WASM sandbox | `github.com/tetratelabs/wazero` | Pure Go WebAssembly runtime, zero CGO |
| Crypto (AEAD) | `golang.org/x/crypto/chacha20poly1305` + `golang.org/x/crypto/argon2` | Stdlib-adjacent, audited |
| Filesystem watch | `github.com/fsnotify/fsnotify` | Config hot-reload |
| Testing | `github.com/stretchr/testify` | Assertions + mocking |
| UUID | `github.com/google/uuid` | Standard UUID generation |
| Structured logging | `go.uber.org/zap` | High-performance structured logging |
| ADK | `google.golang.org/adk` | Google Agent Development Kit |

### Banned Dependencies

- **Any CGO dependency** — breaks cross-compilation, complicates Docker builds, increases binary size.
- **ORM libraries** (gorm, ent) — use `sqlx` for query building with raw SQL. No magic, no code generation.
- **`reflect`-heavy libraries** unless absolutely necessary.

---

## 4. Interface Conventions

### Define Where Consumed

Interfaces are defined in the package that **uses** them, not the package that implements them. This is canonical Go.

```go
// pkg/brain/brain.go — Brain USES Provider, so Provider interface lives here
// (or in a shared types package if multiple consumers exist)
package brain

type Provider interface {
    Chat(ctx context.Context, messages []Message, tools []ToolDef) (TokenStream, error)
    Name() string
    Model() string
    ContextWindow() int
}
```

The `pkg/provider/` package implements this interface but does not define it.

**Exception:** When multiple packages consume the same interface (e.g., `SessionStore` used by both `brain` and `gateway`), define it in `pkg/types/` or a shared `pkg/types/` package.

### Small Interfaces

Interfaces should have **1-3 methods**. If an interface has more than 5 methods, it is doing too much — split it.

```go
// Good: focused interface
type SessionStore interface {
    GetOrCreate(ctx context.Context, sessionID string) (*Session, error)
    Append(ctx context.Context, sessionID string, msg Message) error
    GetMessages(ctx context.Context, sessionID string, limit int) ([]Message, error)
}

// Bad: kitchen sink
type Store interface {
    GetSession(...)
    CreateSession(...)
    DeleteSession(...)
    ListSessions(...)
    GetMessages(...)
    AppendMessage(...)
    Compact(...)
    Archive(...)
    // ... 15 more methods
}
```

### Accepting Interfaces, Returning Structs

Functions accept interfaces and return concrete types:

```go
// Good
func NewDefaultBrain(p Provider, m Muscle, s SessionStore) *DefaultBrain { ... }

// Bad
func NewDefaultBrain(p Provider, m Muscle, s SessionStore) Brain { ... }
```

---

## 5. Error Handling

### Wrapping with Context

Always wrap errors with `fmt.Errorf` and `%w` to preserve the error chain:

```go
row := db.QueryRowContext(ctx, query, sessionID)
if err := row.Scan(&session.ID, &session.CreatedAt); err != nil {
    return nil, fmt.Errorf("session store: get session %q: %w", sessionID, err)
}
```

### Custom Error Types

Define sentinel errors for expected conditions. Use custom error types for structured error data:
define the error as a enum and use it.
```go
var (
    ErrSessionNotFound = errors.New("session not found")
    ErrVaultLocked     = errors.New("vault is locked")
    ErrToolDenied      = errors.New("tool execution denied")
    ErrMaxIterations   = errors.New("maximum iterations reached")
)

type ProviderError struct {
    Provider string
    Status   int
    Message  string
}

func (e *ProviderError) Error() string {
    return fmt.Sprintf("provider %s: HTTP %d: %s", e.Provider, e.Status, e.Message)
}
```

### Rules

1. **Never panic in library code.** Panics are reserved for truly unrecoverable programmer errors (e.g., invalid regex literal). All `pkg/` packages return errors.
2. **Never ignore errors.** If you intentionally discard an error, document why with a comment.
3. **Check errors immediately.** No `err` variable should live more than one line before being checked.
4. **`errors.Is()` and `errors.As()` for matching** — never compare error strings.

---

## 6. Concurrency Patterns

### Context Propagation

Every function that does I/O or could block takes `context.Context` as the first parameter:

```go
func (b *DefaultBrain) Chat(ctx context.Context, req ChatRequest) (<-chan SseEvent, error) {
    // ...
    messages, err := b.sessionStore.GetMessages(ctx, session.ID, limit)
    // ...
    stream, err := b.provider.Chat(ctx, messages, tools)
    // ...
}
```

### Goroutines and Channels

- **Channels for data flow.** SSE events flow through `chan SseEvent`. Tool results collected via channels.
- **`sync.WaitGroup` for fan-out/fan-in.** Parallel tool execution.
- **`context.WithCancel` / `context.WithTimeout` for lifecycle.** Tool approval timeout, LLM call timeout.
- **Never launch a goroutine without a cancellation path.** Every goroutine must respect context cancellation.

```go
// Parallel tool execution
func (b *DefaultBrain) executeTools(ctx context.Context, calls []ToolCall) []ToolResult {
    results := make([]ToolResult, len(calls))
    var wg sync.WaitGroup
    for i, call := range calls {
        wg.Add(1)
        go func(i int, call ToolCall) {
            defer wg.Done()
            results[i] = b.muscle.Execute(ctx, call.Name, call.Args)
        }(i, call)
    }
    wg.Wait()
    return results
}
```

### Shared State

- **`sync.RWMutex`** for infrequently-written, frequently-read shared state (e.g., tool registry).
- **`sync.Map`** only when the key set is stable and access is highly concurrent (rare).
- **`atomic.Value`** for hot-reloaded config (store `*AppConfig`, load on every request).

```go
// Config hot-reload pattern
type ConfigHolder struct {
    config atomic.Value // stores *AppConfig
}

func (h *ConfigHolder) Load() *AppConfig {
    return h.config.Load().(*AppConfig)
}

func (h *ConfigHolder) Store(cfg *AppConfig) {
    h.config.Store(cfg)
}
```

---

## 7. Testing

### Table-Driven Tests

All unit tests use table-driven style:

```go
func TestBudgetArithmetic(t *testing.T) {
    tests := []struct {
        name          string
        contextWindow int
        systemTokens  int
        toolTokens    int
        wantBudget    int
    }{
        {
            name:          "gpt-4o standard",
            contextWindow: 128000,
            systemTokens:  500,
            toolTokens:    1200,
            wantBudget:    122204, // 128000 - 500 - 1200 - 4096
        },
        {
            name:          "small model tight budget",
            contextWindow: 4096,
            systemTokens:  500,
            toolTokens:    200,
            wantBudget:    0, // clamped to 0
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := calculateHistoryBudget(tt.contextWindow, tt.systemTokens, tt.toolTokens)
            assert.Equal(t, tt.wantBudget, got)
        })
    }
}
```

### Test Packages

- **Unit tests** in `_test.go` files alongside the code, same package (access to unexported identifiers).
- **Integration tests** in `_test.go` files with `package foo_test` (black-box, tests public API only).
- **Test fixtures** in `testdata/` directories.

### HTTP Testing

Use `net/http/httptest` for gateway tests:

```go
func TestHealthEndpoint(t *testing.T) {
    srv := setupTestServer(t) // creates router with all dependencies
    req := httptest.NewRequest("GET", "/silo/health", nil)
    req.Header.Set("Authorization", "Bearer test-token")
    w := httptest.NewRecorder()

    srv.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)
    assert.Contains(t, w.Body.String(), `"status":"ok"`)
}
```

### Mocks

Use interfaces for dependency injection. Test doubles are hand-written structs, not generated mocks (unless the interface is large):

```go
type mockProvider struct {
    chatFunc func(ctx context.Context, msgs []Message, tools []ToolDef) (TokenStream, error)
}

func (m *mockProvider) Chat(ctx context.Context, msgs []Message, tools []ToolDef) (TokenStream, error) {
    return m.chatFunc(ctx, msgs, tools)
}
```

### Test Tags

Integration tests that require external services (LLM APIs, network) use build tags:

```go
//go:build integration

package provider_test
```

Run with: `go test -tags=integration ./...`

### Coverage

Target: **80%+ line coverage** for `pkg/` packages. Not a hard gate, but PRs that significantly decrease coverage require justification.

---

## 8. Naming Conventions

### Packages

- **Short, lowercase, single-word** when possible: `brain`, `vault`, `memory`, `config`.
- **No underscores or mixedCaps** in package names.
- **No stutter:** `session.SessionStore` is bad. `session.Store` is good. Exception: when the package name alone is ambiguous.

### Variables and Functions

- **`camelCase`** for unexported, **`PascalCase`** for exported. Standard Go.
- **Receivers:** short, 1-2 letter, consistent within a type. `b` for `*DefaultBrain`, `s` for `*SqliteSessionStore`.
- **Acronyms:** `ID` not `Id`, `HTTP` not `Http`, `SSE` not `Sse`, `URL` not `Url`.

### Files

- **`snake_case.go`** for all Go files.
- **One primary type per file** when the type is substantial (e.g., `brain.go` defines `DefaultBrain`).
- **`_test.go`** suffix for test files (Go convention).

### Constants

```go
const (
    DefaultPort          = 5110
    DefaultMaxIterations = 25
    DefaultApprovalTimeout = 30 * time.Second
    MaxRequestSize       = 10 << 20 // 10 MB
)
```

---

## 9. Security Coding Practices

### Secrets

- **Never log secrets.** API keys, bearer tokens, vault passwords must never appear in log output. Use `zap` with a custom encoder or field hook that redacts fields tagged as sensitive.
- **Zero after use.** Sensitive byte slices should be zeroed after use. Use a `defer` to clear buffers holding decrypted vault contents.
- **No secrets in config.** `silo.toml` never contains API keys or passwords. All secrets live in the encrypted vault (`~/.silo/vault.enc`).

### Input Validation

- **Validate all external input** at the gateway boundary. Size limits, JSON schema validation, path traversal checks.
- **Sanitize tool arguments** before execution. Shell metacharacter detection for `BashTool`.
- **Bound all loops and allocations.** Max iterations on the ReAct loop. Max output size from tool execution. Max memory entries returned from search.

### Cryptographic Practices

- **Use `crypto/rand` only.** Never `math/rand` for security-sensitive operations.
- **Constant-time comparison** for token validation: `subtle.ConstantTimeCompare()`.
- **Pin cipher parameters.** Argon2id: memory=64MB, iterations=3, parallelism=4, salt=16 bytes, key=32 bytes. XChaCha20-Poly1305 for AEAD.

### SQL

- **Always use parameterized queries.** Never string-concatenate SQL.
- **Use `sqlx` named queries** for clarity when queries have many parameters.
- **Validate identifiers** if dynamic table/column names are unavoidable (they should not be).

```go
// Good — sqlx with named parameters
db.NamedGetContext(ctx, &session, "SELECT * FROM sessions WHERE id = :id", map[string]any{"id": sessionID})

// Good — sqlx with positional parameters
db.GetContext(ctx, &session, "SELECT * FROM sessions WHERE id = ?", sessionID)

// Forbidden
db.QueryContext(ctx, "SELECT * FROM sessions WHERE id = '" + sessionID + "'")
```

---

## 10. Build and Release

### Local Development

```bash
# Build
go build -o bin/silo ./cmd/silo/

# Run tests
go test ./...

# Run integration tests
go test -tags=integration ./...

# Lint
golangci-lint run ./...

# Format
gofmt -w .
goimports -w .
```

### CI Pipeline

Every PR runs:

1. `go vet ./...`
2. `golangci-lint run` (with `.golangci.yml` config)
3. `go test -race ./...` (race detector on)
4. `go test -tags=integration ./...` (if API keys available)
5. `go build ./cmd/silo/` (verify it compiles)

### Cross-Platform Release

Use `goreleaser` for producing release binaries:

```yaml
# .goreleaser.yml
builds:
  - main: ./cmd/silo/
    binary: silo
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.ShortCommit}}
      - -X main.date={{.Date}}

archives:
  - format: tar.gz
    name_template: "silo_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        format: zip
```

Binary size target: **< 20 MB** stripped (`-s -w` ldflags).

### Version Embedding

```go
var (
    version = "dev"
    commit  = "none"
    date    = "unknown"
)

func main() {
    // Available via `silo --version`
}
```

---

## 11. Electron Frontend Guidelines

### Separation of Concerns

The Electron desktop app and the Go binary are **separate codebases** with a **defined IPC boundary**. The Go binary knows nothing about Electron. The Electron app knows nothing about Go internals. They communicate via local HTTP + SSE, the same protocol any external adapter uses.

### TypeScript/React Standards

- **TypeScript strict mode** — `"strict": true` in `tsconfig.json`.
- **React functional components** with hooks. No class components.
- **State management:** Zustand or React Context for V0. No Redux.
- **Styling:** Tailwind CSS for rapid iteration.
- **IPC types:** Auto-generated from Go types or maintained as a shared `.d.ts` contract file.

### Build Tooling

- `electron-builder` for packaging.
- `vite` for frontend bundling (fast HMR during development).
- `electron-vite` to unify the build pipeline.

---

## 12. Code Review Checklist

Every PR must pass this checklist:

- [ ] No new dependencies without justification in PR description
- [ ] No CGO dependencies
- [ ] All errors handled (no `_` for error returns without comment)
- [ ] `context.Context` propagated through all I/O paths
- [ ] No secrets in logs or config files
- [ ] SQL uses parameterized queries only
- [ ] New interfaces have 1-5 methods
- [ ] Table-driven tests for new logic
- [ ] `go vet` and `golangci-lint` pass
- [ ] Race detector clean (`-race` flag)

---

## 13. Cross-References

| Spec | Relevance |
|------|-----------|
| [01_wants_and_intro.md](01_wants_and_intro.md) | Core philosophies, security-first approach |
| [03_gateway.md](03_gateway.md) | HTTP server, routes, SSE streaming |
| [07_security.md](07_security.md) | Five-layer security model, sandbox, vault |
| [09_configuration.md](09_configuration.md) | Config struct hierarchy, hot-reload |
| [13_desktop_app.md](13_desktop_app.md) | Electron architecture, IPC contract |
| [14_adk_integration.md](14_adk_integration.md) | ADK usage patterns, callback hooks |
