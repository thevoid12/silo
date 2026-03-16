# Step 12 — Session Persistence via SQLite

## What changed

Sessions now survive server restarts. Previously, the gateway used an in-memory session store that was wiped on every `silo stop / silo start`. Conversations are now persisted to a local SQLite database.

## How it works

- The server opens `~/.silo/sessions.db` on startup using the ADK `DatabaseSessionService` (GORM + mattn/go-sqlite3).
- Schema is auto-migrated on first run — no manual setup needed.
- Clients reuse a session by passing `"session_id"` in the chat request body. The ID is returned in every `event: done` payload.
- If no `session_id` is supplied, a new session is created automatically.

## Configuration

```toml
[session]
db_path        = "~/.silo/sessions.db"
retention_days = 30
```

`db_path` can be overridden in `~/.silo/silo.toml`.

## Database layout

| Table              | Managed by           | Purpose                                      |
|--------------------|----------------------|----------------------------------------------|
| `sessions`         | ADK / GORM AutoMigrate | Active sessions with state                 |
| `events`           | ADK / GORM AutoMigrate | Per-turn conversation events               |
| `app_states`       | ADK / GORM AutoMigrate | App-level shared state                     |
| `user_states`      | ADK / GORM AutoMigrate | Per-user shared state                      |
| `session_metadata` | sqlc / dbal          | Lightweight title + timestamp for listing sessions |

## Generated DBAL code

SQL schema and queries live in `pkg/db/sqlc/silo/`. Run `sqlc generate` from `pkg/db/sqlc/` to regenerate `pkg/db/dbal/` after schema changes.
