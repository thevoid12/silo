# Session Management Specification (V0)

Sessions give the agent multi-turn memory. A session is a conversation between one sender and the agent, identified by the opaque `session_id` that adapters pass on chat requests (see 05_channels.md §5). In V0, sessions are managed by ADK's `SessionService`.

---

## 1. Overview

Without sessions, every message is isolated — the agent has no memory of prior turns. Sessions solve this by persisting messages to SQLite and reassembling relevant history into the LLM context window on each turn.

**Key design decisions:**

- **ADK's `SessionService`** — replaces custom session store. ADK provides `DatabaseSessionService` with SQLite backend.
- **SQLite, not Postgres** — local-first philosophy. Single file, zero config, WAL mode for concurrent reads.
- **Adapter owns `session_id`** — the agent treats it as an opaque string. Adapters generate IDs that encode channel + sender (e.g., `tg-12345-67890`). See 05_channels.md §5 for the ID generation contract.

---

## 2. ADK Session Service

V0 uses ADK's built-in `DatabaseSessionService` backed by SQLite.

### Storage

- **File**: `~/.silo/sessions.db`
- **Engine**: `modernc.org/sqlite` (pure Go, no CGO) via `sqlx` for query building
- **Schema**: Managed by ADK — Silo does not define custom session tables in V0

### Go Setup

```go
import "google.golang.org/adk/sessions"

sessionService, err := sessions.NewDatabaseSessionService(
    sessions.WithSQLitePath("~/.silo/sessions.db"),
)
```

### What ADK Handles

- Session creation and retrieval
- Message append and retrieval
- Context window management (basic)
- SQLite WAL mode, concurrent access

### What Silo Adds

- `session_id` generation for CLI (`cli-<uuid4>`)
- Session list/get/delete API endpoints (thin wrapper over ADK)
- Session metadata (title from first message, provider snapshot)

---

## 3. Session Lifecycle

```
new session_id → CREATE → active
                            │ (messages flow)
                            │ (retention_days exceeded)
                          EXPIRE → expired (V1)
                            │ (explicit delete)
                          DELETE → removed from DB
```

### CREATE

Agent receives a `session_id` it hasn't seen before → ADK creates a new session with the first message.

- `title` is set from the first user message (truncated to 80 chars) — stored in Silo's session metadata
- Session is `active`

### RESUME

Agent receives a known `session_id` → ADK loads session history, appends new message.

### DELETE

Hard delete via API (`DELETE /silo/vault/sessions/{session_id}`). Removes all associated data.

### Ephemeral Sessions

If `session_id` is omitted from the request, Silo generates `cli-<uuid4>`. These sessions are persisted in the DB but never automatically resumed.

---

## 4. Context Assembly (V0 — ADK Managed)

ADK handles basic context assembly in V0. The agent's instruction (system prompt) and tool definitions are provided at agent creation time. ADK manages the conversation history window.

### Layout

```
┌─────────────────────────────────────────────────┐
│ 1. System prompt (agent instruction)   (always)  │
│ 2. Tool definitions                    (always)  │
│ 3. Session message history    (ADK managed)      │
│ 4. New user message            (current turn)    │
│ ─── reserved space for response ───              │
└─────────────────────────────────────────────────┘
```

### V1 Additions

- Memory recall injection (from custom MemoryService)
- Compaction summaries
- Budget arithmetic with fine-grained token counting

---

## 5. Configuration (`silo.toml`)

```toml
[session]
db_path = "~/.silo/sessions.db"      # SQLite database path
retention_days = 30                    # days before sessions expire (0 = never expire) — V1
```

**Why a separate DB:** Sessions contain conversation text, not secrets. Keeping them separate from the encrypted vault means:
- Users can back up, inspect, and query conversations with standard SQLite tools.
- The vault's encryption boundary isn't expanded to cover non-secret data.

---

## 6. API Surface

### V0 Endpoints

Under `/silo/vault/sessions` — exposed in headless mode via the gateway.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/silo/vault/sessions` | List sessions (with filters) |
| GET | `/silo/vault/sessions/{session_id}` | Get session metadata |
| DELETE | `/silo/vault/sessions/{session_id}` | Delete session and all data |

#### `GET /silo/vault/sessions`

List sessions with optional filters and pagination.

**Query parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `status` | string | `active` | Filter by status: `active` or `all` |
| `limit` | integer | 20 | Max results (1–100) |
| `offset` | integer | 0 | Pagination offset |
| `sort` | string | `updated_at` | Sort field: `updated_at` or `created_at` |
| `order` | string | `desc` | Sort order: `asc` or `desc` |

**Response (200):**

```json
{
  "sessions": [
    {
      "session_id": "tg-12345-67890",
      "title": "List files in /tmp",
      "status": "active",
      "message_count": 47,
      "created_at": 1709654400000,
      "updated_at": 1709740800000
    }
  ],
  "total": 1,
  "limit": 20,
  "offset": 0
}
```

#### `GET /silo/vault/sessions/{session_id}`

Get metadata for a single session.

**Response (200):**

```json
{
  "session_id": "tg-12345-67890",
  "title": "List files in /tmp",
  "status": "active",
  "message_count": 47,
  "created_at": 1709654400000,
  "updated_at": 1709740800000
}
```

**Error (404):**

```json
{
  "type": "silo/session-not-found",
  "title": "Session Not Found",
  "status": 404,
  "detail": "No session with id 'tg-12345-99999'"
}
```

#### `DELETE /silo/vault/sessions/{session_id}`

Delete a session and all associated data.

**Response (204):** No content.

**Error (404):** Same format as GET.

---

## 7. SSE Event: `session`

Emitted **first** on the `/silo/brain/chat` SSE stream — tells the frontend whether this is a fresh or resumed conversation.

```
event: session
data: {"session_id": "tg-12345-67890", "resumed": true, "message_count": 47}
```

**Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `session_id` | string | The session ID (adapter-provided or auto-generated) |
| `resumed` | boolean | `true` if session existed before, `false` if just created |
| `message_count` | integer | Number of messages in the session |

---

## 8. CLI Integration

### V0: Ephemeral Sessions

- `silo chat` generates `cli-<uuid4>` per launch → fresh session each time.
- Sessions **are** persisted in the DB, but the CLI never auto-resumes them.
- CLI conversations are recoverable via the API even though the CLI doesn't expose history.

### V1: Persistent Sessions

- `silo chat` → resumes the most recent active CLI session
- `silo chat --new` → force a fresh session
- `silo chat --session <id>` → resume a specific session
- `silo chat --list` → interactive session picker

---

## 9. V0 / V1 Scoping Summary

### V0 Ships

| Feature | Section |
|---------|---------|
| ADK DatabaseSessionService with SQLite | §2 |
| Session create/resume via `session_id` | §3 |
| Message persistence (via ADK) | §2 |
| Session list/get/delete API (3 endpoints) | §6 |
| `[session]` configuration in `silo.toml` | §5 |
| CLI ephemeral sessions (`cli-<uuid4>`) | §8 |
| `event: session` SSE event | §7 |

### V1 Deferred

| Feature | Notes |
|---------|-------|
| Auto-compaction (LLM summarization) | ADK handles basic context for V0 |
| Compaction archive tables | Not needed without compaction |
| Budget arithmetic for sliding window | ADK manages context in V0 |
| Session GC background loop | Manual delete sufficient for V0 |
| Session expiration | retention_days enforcement |
| CLI session persistence + picker | `--session`, `--list` flags |
| JSONL export/import | |
| Manual compaction endpoint | |
| Paginated message history API | |
| Exact tokenizer bindings | |
| Per-session model override | |
| Session branching/forking | |

---

## 10. Cross-References

| Spec | Interaction |
|------|-------------|
| [03_gateway.md §3](03_gateway.md) | Session API endpoints registered in the gateway |
| [05_channels.md §5](05_channels.md) | `session_id` on chat requests, adapter ID generation |
| [11_agent.md](11_agent.md) | ADK agent uses SessionService for conversation history |
| [14_adk_integration.md](14_adk_integration.md) | ADK DatabaseSessionService setup and configuration |

---

## 11. Verification Walkthroughs

### Fresh Session

1. Adapter sends chat request with `session_id: "tg-123-456"` (first time).
2. ADK creates new session in SQLite.
3. SSE emits `event: session` with `resumed: false, message_count: 1`.
4. Agent processes message, stores assistant response.

### Resume

1. Same `session_id` arrives again.
2. ADK loads existing session history.
3. SSE emits `event: session` with `resumed: true, message_count: N`.
4. Agent has full conversation context.

### CLI Ephemeral

1. `silo chat` launches, generates `cli-<uuid4>`.
2. Session created in DB, messages flow normally.
3. User quits CLI. Next `silo chat` launch generates a new `cli-<uuid4>`.
4. Old CLI session visible via `GET /silo/vault/sessions`.
