# Phase C Complete — Gateway + Headless

## Milestone

External adapters (Telegram, desktop app, REST clients) can now connect to the Silo agent over HTTP.

## What was built

| Step | Feature | Endpoints / Commands |
|------|---------|---------------------|
| 9    | HTTP gateway with bearer auth | `GET /health`, `GET /silo/status`, `silo start/stop/status` |
| 10   | SSE chat streaming wired to ADK runner | `POST /silo/brain/chat` |
| 11   | Tool approval over HTTP | `POST /silo/brain/tool-approval` |
| 12   | Session persistence via SQLite | Sessions survive server restarts |

## Running the Phase C verification suite

The server must be running before executing the tests.

```bash
# Start the server
silo start

# Export your gateway token (printed by silo init)
export SILO_TOKEN=<your-bearer-token>

# Run all Phase C checks
./test/run_phase_c.sh
```

Individual test scripts in `test/`:

| Script | Checks |
|--------|--------|
| `01_health.sh` | `GET /health` returns 200, no auth required |
| `02_auth.sh` | 401 without token, 401 wrong token, 200 correct token |
| `03_chat_sse.sh` | SSE stream emits `token` and `done` events |
| `04_session_persist.sh` | Follow-up message reuses same session_id |
| `05_tool_approval.sh` | 400 missing id, 404 unknown id, 401 no auth |

## Environment variables for tests

| Variable | Default | Description |
|----------|---------|-------------|
| `SILO_TOKEN` | — (required) | Bearer token from vault |
| `SILO_BASE_URL` | `http://127.0.0.1:5110` | Gateway base URL |
| `CHAT_TIMEOUT` | `60` | Seconds to wait for SSE stream to complete |
| `SILO_SESSION_ID` | set by `03_chat_sse.sh` | Passed to `04_session_persist.sh` automatically by `run_phase_c.sh` |
